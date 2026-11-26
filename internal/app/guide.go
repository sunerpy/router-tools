package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
	"github.com/sunerpy/router-tools/internal/secret"
	"github.com/sunerpy/router-tools/internal/state"
)

// GuideRequest asks for a read-only guided plan.
type GuideRequest struct {
	Target    device.Target
	ProfileID string
	// Goals are a subset of ssh, backup, soft and reboot; ssh is always included.
	Goals []string
	// RootPassword is optional: the current root password, or with ResetRootPassword the one to set.
	RootPassword secret.Password
	// ResetRootPassword lets the enable flow change an unknown root password while SSH is open.
	ResetRootPassword bool
	HostKeyPolicy     hostkey.Policy
}

// GuideState is the read-only evidence a plan is based on.
type GuideState struct {
	SSH             device.PortState `json:"ssh,omitempty"`
	Telnet          device.PortState `json:"telnet,omitempty"`
	RootSSH         string           `json:"root_ssh"`
	Soft            SoftState        `json:"soft,omitempty"`
	Backup          *ManagedBackup   `json:"backup,omitempty"`
	Recovery        *state.Outcome   `json:"recovery,omitempty"`
	RecoveryBlocked bool             `json:"recovery_blocked"`
	Locked          bool             `json:"locked"`
}

// GuidePlanReport lists the steps a guided run would take. Confirmation and PlanDigest are present only
// when the plan is ready to apply.
type GuidePlanReport struct {
	SchemaVersion  int                   `json:"schema_version"`
	Operation      string                `json:"operation"`
	Status         string                `json:"status"`
	Host           string                `json:"host"`
	Device         device.Observation    `json:"device"`
	Profile        device.Profile        `json:"profile"`
	Candidates     []device.ProfileMatch `json:"candidates,omitempty"`
	Goals          []string              `json:"goals"`
	State          GuideState            `json:"state"`
	Steps          []GuideStep           `json:"steps"`
	Ready          bool                  `json:"ready"`
	Changes        bool                  `json:"changes"`
	RequiredInputs []string              `json:"required_inputs,omitempty"`
	Confirmation   *GuideConfirmation    `json:"confirmation,omitempty"`
	PlanDigest     string                `json:"plan_digest,omitempty"`
	Error          *device.Error         `json:"error,omitempty"`
}

// GuidePlan detects the router's state without changing it and plans only the missing steps.
func (s *Service) GuidePlan(ctx context.Context, request GuideRequest, events EventSink) (GuidePlanReport, error) {
	report, _, err := s.guidePlan(ctx, request, operationEmitter("guide plan", events))
	return report, err
}

func (s *Service) guidePlan(ctx context.Context, request GuideRequest, emit func(string, string)) (GuidePlanReport, device.Adapter, error) {
	report := GuidePlanReport{
		SchemaVersion: 1, Operation: "guide plan", Status: "running", Host: request.Target.Host,
		Goals: []string{}, State: GuideState{RootSSH: RootSSHNotChecked}, Steps: []GuideStep{},
	}
	facts := guideFacts{RootSSH: RootSSHNotChecked, Reset: request.ResetRootPassword, BackupManaged: s.backupRoot != ""}
	redactor := secret.NewRedactor(request.RootPassword.Expose())
	fail := func(err error, block string) (GuidePlanReport, device.Adapter, error) {
		normalized := normalizeError(err)
		redacted := *normalized
		redacted.Message = redactor.Text(redacted.Message)
		if block != "" {
			facts.Goals, facts.Block = report.Goals, block
			report.Steps = planSteps(facts)
			report.Ready, report.Changes, report.RequiredInputs = false, false, nil
		}
		report.Status = StatusFailed
		report.Error = &redacted
		emit("failed", StatusFailed)
		return report, device.Adapter{}, &redacted
	}

	emit("validate", "running")
	goals, err := normalizeGoals(request.Goals)
	if err != nil {
		return fail(err, "")
	}
	report.Goals = goals
	if err := request.Target.Validate(); err != nil {
		return fail(err, "")
	}
	hasRoot := request.RootPassword.Expose() != ""
	if hasRoot {
		if err := request.RootPassword.Validate(); err != nil {
			return fail(err, "")
		}
	}
	if err := s.checkProfileID(request.ProfileID); err != nil {
		return fail(err, "")
	}

	// A busy router or an unacknowledged uncertain change blocks the plan before any network access.
	release, err := s.locker.Acquire(request.Target)
	if err != nil {
		if hasCode(err, device.CodeOperationLocked) {
			report.State.Locked = true
			return fail(err, ReasonOperationLocked)
		}
		return fail(err, ReasonDetectionFailed)
	}
	_ = release()
	last, err := s.lastOutcome(request.Target)
	if err != nil {
		return fail(err, ReasonDetectionFailed)
	}
	report.State.Recovery = last
	if last != nil && last.Blocks() {
		report.State.RecoveryBlocked = true
		return fail(&device.Error{
			Code: device.CodeRecoveryReviewRequired,
			Message: fmt.Sprintf("the last %s on this router ended %s; check the router as RECOVERY.md describes and acknowledge before another change",
				last.Operation, last.Status),
		}, ReasonRecoveryReviewRequired)
	}

	emit("probe", "running")
	probe, err := s.discovery.Probe(ctx, request.Target)
	if err != nil {
		return fail(err, ReasonDetectionFailed)
	}
	report.Device = probe.Observation
	report.State.SSH, report.State.Telnet = probe.SSH, probe.Telnet
	facts.SSH = probe.SSH

	emit("compatibility", "running")
	adapter, err := s.resolveProfile(request.ProfileID, probe.Observation)
	if err != nil {
		report.Candidates = s.registry.Candidates(probe.Observation)
		return fail(err, ReasonIncompatible)
	}
	profile := adapter.Profile()
	report.Profile, facts.Profile = profile, profile

	// Root evidence comes from a password-authenticated SSH shell only; Telnet never counts.
	if hasRoot && probe.SSH == device.PortOpen && profile.Supports(device.CapabilityVerify) {
		verifier, ok := adapter.Operations.(RootVerifier)
		if !ok {
			return fail(&device.Error{Code: device.CodeUnsupportedCapability, Message: "root verification is unavailable"}, ReasonCapabilityMissing)
		}
		emit("root_verify", "running")
		result, err := verifier.VerifyRoot(ctx, request.Target, request.RootPassword, request.HostKeyPolicy, "ssh", false)
		switch {
		case err == nil && result.RootVerified && result.Transport == "ssh":
			facts.RootSSH = RootSSHVerified
		case err == nil:
			return fail(&device.Error{Code: device.CodeRootVerifyFailed, Message: "root verification did not succeed"}, ReasonDetectionFailed)
		case hasCode(err, device.CodeSSHAuthFailed):
			facts.RootSSH = RootSSHAuthFailed
		case hasCode(err, device.CodeHostKeyMismatch, device.CodeSSHHostKeyRejected):
			return fail(err, ReasonHostKeyRejected)
		default:
			return fail(err, ReasonDetectionFailed)
		}
	}
	report.State.RootSSH = facts.RootSSH
	if facts.RootSSH == RootSSHVerified && containsString(goals, GoalSoft) && profile.Supports(device.CapabilitySoftPersist) {
		inspector, ok := adapter.Operations.(SoftInspector)
		if !ok {
			return fail(&device.Error{Code: device.CodeUnsupportedCapability, Message: "soft persistence inspection is unavailable"}, ReasonCapabilityMissing)
		}
		emit("inspect", "running")
		soft, err := inspector.InspectSoft(ctx, request.Target, request.RootPassword, request.HostKeyPolicy)
		if err != nil {
			if hasCode(err, device.CodeHostKeyMismatch, device.CodeSSHHostKeyRejected) {
				return fail(err, ReasonHostKeyRejected)
			}
			return fail(err, ReasonDetectionFailed)
		}
		facts.Soft, report.State.Soft = soft, soft
	}
	if containsString(goals, GoalBackup) {
		report.State.Backup = s.latestManagedBackup(request.Target, probe.Observation, profile)
		facts.BackupFound = report.State.Backup != nil
	}

	facts.Goals = goals
	report.Steps = planSteps(facts)
	report.RequiredInputs = requiredInputs(report.Steps)
	report.Ready, report.Changes = planReadiness(report.Steps)
	if err := stepBlockError(report.Steps); err != nil {
		report.Status = StatusFailed
		report.Error = err
		emit("failed", StatusFailed)
		return report, adapter, err
	}
	if report.Ready {
		confirmation := GuideConfirmation{
			SchemaVersion: 1, Target: request.Target, Identity: confirmedIdentity(probe.Observation, profile),
			Goals: goals, ResetRootPassword: request.ResetRootPassword, Steps: confirmedSteps(report.Steps),
		}
		report.Confirmation = &confirmation
		report.PlanDigest = planDigest(confirmation)
	}
	report.Status = StatusSucceeded
	emit("completed", StatusSucceeded)
	return report, adapter, nil
}

// planReadiness: ready when no step needs input or is blocked or unsupported; changes when any step may write.
func planReadiness(steps []GuideStep) (ready, changes bool) {
	ready = true
	for _, step := range steps {
		switch step.Action {
		case ActionRun, ActionDecideAfter:
			changes = true
		case ActionNeedsInput, ActionBlocked, ActionUnsupported:
			ready = false
		}
	}
	return ready, changes
}

// stepBlockError reports the first step-level block, naming the separate maintenance it needs.
func stepBlockError(steps []GuideStep) *device.Error {
	for _, step := range steps {
		switch step.Action {
		case ActionBlocked:
			return softBlockError(step)
		case ActionUnsupported:
			return &device.Error{Code: device.CodeUnsupportedCapability, Message: fmt.Sprintf("the %s step is unavailable: %s", step.ID, step.Reason)}
		}
	}
	return nil
}

func softBlockError(step GuideStep) *device.Error {
	action := "review the router's firewall.auto_ssh configuration manually"
	if step.Maintenance == MaintenancePersistRemove {
		action = "remove the soft persistence first, then detect again"
	}
	return &device.Error{Code: device.CodeSoftConfigConflict, Message: fmt.Sprintf("soft persistence is in state %s; %s", step.Reason, action)}
}

func confirmedSteps(steps []GuideStep) []GuideConfirmedStep {
	confirmed := make([]GuideConfirmedStep, 0, len(steps))
	for _, step := range steps {
		confirmed = append(confirmed, GuideConfirmedStep{ID: step.ID, Action: step.Action, Effects: step.Effects})
	}
	return confirmed
}

func hasCode(err error, codes ...string) bool {
	var deviceErr *device.Error
	if !errors.As(err, &deviceErr) {
		return false
	}
	for _, code := range codes {
		if deviceErr.Code == code {
			return true
		}
	}
	return false
}

func containsString(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
