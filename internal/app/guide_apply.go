package app

import (
	"context"
	"crypto/subtle"
	"slices"
	"time"

	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
	"github.com/sunerpy/router-tools/internal/secret"
)

// Step result statuses besides the use-case statuses succeeded, failed, partial and unknown.
const (
	StatusSkipped = "skipped"
	StatusNotRun  = "not_run"
)

// GuideApplyRequest applies a confirmed plan. Confirmation and PlanDigest are sent back exactly as the
// plan returned them; the passwords are the ones the plan asked for.
type GuideApplyRequest struct {
	Confirmation GuideConfirmation
	PlanDigest   string
	Confirmed    bool
	// AdminPassword is needed only when the enable step runs.
	AdminPassword secret.Password
	// RootPassword is the verified current root password, or the new one when the enable step runs.
	RootPassword  secret.Password
	HostKeyPolicy hostkey.Policy
	ToolVersion   string
	// Credentials may fill the admin password and, when SSH already works, the root password; the root
	// password the enable step sets is always typed.
	Credentials CredentialUse
}

// GuideStepResult is the outcome of one guided step together with its use-case report.
type GuideStepResult struct {
	ID          string              `json:"id"`
	Action      string              `json:"action"`
	Reason      string              `json:"reason,omitempty"`
	Status      string              `json:"status"`
	Effects     []string            `json:"effects,omitempty"`
	Maintenance string              `json:"maintenance,omitempty"`
	Enable      *EnableReport       `json:"enable,omitempty"`
	Backup      *BackupCreateReport `json:"backup,omitempty"`
	Soft        *SoftReport         `json:"soft,omitempty"`
	Reboot      *RebootReport       `json:"reboot,omitempty"`
	Error       *device.Error       `json:"error,omitempty"`
}

// GuideApplyReport is the result of one guided run.
type GuideApplyReport struct {
	SchemaVersion int                `json:"schema_version"`
	Operation     string             `json:"operation"`
	Status        string             `json:"status"`
	Host          string             `json:"host"`
	Device        device.Observation `json:"device"`
	Profile       device.Profile     `json:"profile"`
	PlanDigest    string             `json:"plan_digest"`
	Steps         []GuideStepResult  `json:"steps"`
	Error         *device.Error      `json:"error,omitempty"`
}

// GuideApply re-detects the router, requires the fresh plan to do no more than the confirmed one, then runs
// the steps in order through their own use cases and stops at the first step that does not succeed.
func (s *Service) GuideApply(ctx context.Context, request GuideApplyRequest, events EventSink) (GuideApplyReport, error) {
	confirmation := request.Confirmation
	report := GuideApplyReport{
		SchemaVersion: 1, Operation: "guide apply", Status: "running", Host: confirmation.Target.Host,
		PlanDigest: request.PlanDigest, Steps: []GuideStepResult{},
	}
	emit := operationEmitter("guide apply", events)
	redactor := secret.NewRedactor(request.AdminPassword.Expose(), request.RootPassword.Expose())
	fail := func(err error) (GuideApplyReport, error) {
		normalized := normalizeError(err)
		redacted := *normalized
		redacted.Message = redactor.Text(redacted.Message)
		if report.Status == "running" || report.Status == "" {
			report.Status = StatusFailed
		}
		report.Error = &redacted
		emit("failed", report.Status)
		return report, &redacted
	}

	emit("validate", "running")
	if !request.Confirmed {
		return fail(&device.Error{Code: device.CodeConfirmationRequired, Message: "the guided plan requires explicit confirmation"})
	}
	digest := planDigest(confirmation)
	if confirmation.SchemaVersion != 1 || request.PlanDigest == "" || subtle.ConstantTimeCompare([]byte(digest), []byte(request.PlanDigest)) != 1 {
		return fail(planChanged("the confirmation does not match its plan digest"))
	}
	if err := checkTypedPassword(request.RootPassword, request.Credentials); err != nil {
		return fail(err)
	}

	// Detect again with the confirmed profile, which never falls back to another one.
	fresh, adapter, err := s.guidePlan(ctx, GuideRequest{
		Target: confirmation.Target, ProfileID: confirmation.Identity.ProfileID, Goals: confirmation.Goals,
		RootPassword: request.RootPassword, ResetRootPassword: confirmation.ResetRootPassword,
		HostKeyPolicy: request.HostKeyPolicy,
		Credentials:   CredentialUse{UseSaved: request.Credentials.UseSaved, Redactor: request.Credentials.Redactor},
	}, operationEmitter("guide plan", stepSink(events, "detect")))
	report.Device, report.Profile = fresh.Device, fresh.Profile
	if err != nil {
		return fail(err)
	}
	if !slices.Equal(fresh.Goals, confirmation.Goals) {
		return fail(planChanged("the confirmed goals are not in canonical form"))
	}
	if confirmedIdentity(fresh.Device, fresh.Profile) != confirmation.Identity {
		return fail(planChanged("the device no longer matches the confirmed plan"))
	}
	steps, err := applicableSteps(confirmation.Steps, fresh.Steps)
	if err != nil {
		return fail(err)
	}
	for _, step := range steps {
		if step.ID == StepEnableSSH && step.Action == ActionRun {
			if err := checkTypedPassword(request.AdminPassword, request.Credentials); err != nil {
				return fail(err)
			}
			if err := request.RootPassword.Validate(); err != nil {
				return fail(err)
			}
		}
	}

	identity := confirmation.Identity
	var firstErr error
	var failing GuideStepResult
	wrote := false
	for index, step := range steps {
		if firstErr != nil {
			report.Steps = append(report.Steps, GuideStepResult{ID: step.ID, Action: ActionNotRun, Status: StatusNotRun, Effects: step.Effects})
			continue
		}
		sink := stepSink(events, step.ID)
		if step.Action == ActionDecideAfter {
			step, err = s.decideDeferred(ctx, adapter, confirmation, request, step, confirmation.Steps[index].Effects)
			if err != nil {
				result := GuideStepResult{ID: step.ID, Action: step.Action, Reason: step.Reason, Status: StatusFailed, Maintenance: step.Maintenance}
				result.Error = normalizeError(err)
				report.Steps = append(report.Steps, result)
				firstErr, failing = err, result
				continue
			}
		}
		result := GuideStepResult{ID: step.ID, Action: step.Action, Reason: step.Reason, Effects: step.Effects}
		if step.Action == ActionSkip {
			result.Status = StatusSkipped
			emitStep(events, step.ID, StatusSkipped)
			report.Steps = append(report.Steps, result)
			continue
		}
		emitStep(events, step.ID, "running")
		err := s.runGuideStep(ctx, step.ID, confirmation, request, &identity, sink, &result)
		if err != nil {
			result.Error = normalizeError(err)
			report.Steps = append(report.Steps, result)
			firstErr, failing = err, result
			emitStep(events, step.ID, result.Status)
			continue
		}
		wrote = true
		emitStep(events, step.ID, result.Status)
		report.Steps = append(report.Steps, result)
	}
	if firstErr != nil {
		report.Status = guideFailureStatus(failing.Status, wrote)
		return fail(firstErr)
	}
	report.Status = StatusSucceeded
	emit("completed", StatusSucceeded)
	return report, nil
}

// runGuideStep calls the step's own use case with the confirmation and the confirmed identity.
func (s *Service) runGuideStep(ctx context.Context, id string, confirmation GuideConfirmation, request GuideApplyRequest, identity *ConfirmedIdentity, sink EventSink, result *GuideStepResult) error {
	target, profileID := confirmation.Target, confirmation.Identity.ProfileID
	soft := SoftRequest{
		Target: target, ProfileID: profileID, RootPassword: request.RootPassword, HostKeyPolicy: request.HostKeyPolicy,
		Confirmed: true, ExpectedIdentity: identity, Credentials: request.Credentials,
	}
	switch id {
	case StepEnableSSH:
		enable, err := s.EnableSSH(ctx, EnableRequest{
			Target: target, ProfileID: profileID, AdminPassword: request.AdminPassword, RootPassword: request.RootPassword,
			HostKeyPolicy: request.HostKeyPolicy, Confirmed: true, ExpectedIdentity: identity, Credentials: request.Credentials,
		}, sink)
		result.Enable, result.Status = &enable, enable.Status
		return err
	case StepBackup:
		created, err := s.BackupCreate(ctx, BackupCreateRequest{
			Target: target, ProfileID: profileID, RootPassword: request.RootPassword, HostKeyPolicy: request.HostKeyPolicy,
			ToolVersion: request.ToolVersion, ExpectedIdentity: identity, Credentials: request.Credentials,
		}, sink)
		result.Backup, result.Status = &created, created.Status
		return err
	case StepSoftInstall:
		installed, err := s.PersistSoft(ctx, soft, sink)
		result.Soft, result.Status = &installed, installed.Status
		return err
	case StepRebootVerify:
		rebooted, err := s.RebootVerify(ctx, soft, sink)
		result.Reboot, result.Status = &rebooted, rebooted.Status
		return err
	}
	result.Status = StatusFailed
	return &device.Error{Code: device.CodeInternalError, Message: "unknown guided step"}
}

// decideDeferred re-inspects a decide_after step once the steps before it succeeded. It may only become
// skip, or run with exactly the confirmed effects; any other state stops the run.
func (s *Service) decideDeferred(ctx context.Context, adapter device.Adapter, confirmation GuideConfirmation, request GuideApplyRequest, step GuideStep, confirmedEffects []string) (GuideStep, error) {
	if step.ID != StepSoftInstall {
		return step, planChanged("only soft persistence can be decided after SSH works")
	}
	inspector, ok := adapter.Operations.(SoftInspector)
	if !ok {
		return step, &device.Error{Code: device.CodeUnsupportedCapability, Message: "soft persistence inspection is unavailable"}
	}
	use := CredentialUse{UseSaved: request.Credentials.UseSaved, Redactor: request.Credentials.Redactor}
	creds := s.credentialSession(use, confirmation.Target, adapter.Profile(), secret.NewRedactor())
	root, err := creds.root(request.RootPassword, false)
	if err != nil {
		return step, err
	}
	soft, err := inspector.InspectSoft(ctx, confirmation.Target, root, creds.hostKeys(request.HostKeyPolicy))
	creds.observe(err)
	if err != nil {
		return step, err
	}
	decided := softStep(soft)
	switch {
	case decided.Action == ActionSkip:
		return decided, nil
	case decided.Action == ActionRun && slices.Equal(decided.Effects, confirmedEffects):
		return decided, nil
	case decided.Action == ActionBlocked:
		return decided, softBlockError(decided)
	}
	return decided, planChanged("soft persistence needs a change that was not confirmed")
}

// applicableSteps allows a fresh step only to keep its confirmed action or to do less: run -> run|skip,
// decide_after -> decide_after|run|skip, skip -> skip, always with the confirmed effects.
func applicableSteps(confirmed []GuideConfirmedStep, fresh []GuideStep) ([]GuideStep, error) {
	if len(confirmed) != len(fresh) {
		return nil, planChanged("the plan has different steps")
	}
	for i, step := range fresh {
		want := confirmed[i]
		if step.ID != want.ID {
			return nil, planChanged("the plan has different steps")
		}
		allowed := false
		switch step.Action {
		case ActionSkip:
			allowed = want.Action == ActionSkip || want.Action == ActionRun || want.Action == ActionDecideAfter
		case ActionRun:
			allowed = (want.Action == ActionRun || want.Action == ActionDecideAfter) && slices.Equal(step.Effects, want.Effects)
		case ActionDecideAfter:
			allowed = want.Action == ActionDecideAfter && slices.Equal(step.Effects, want.Effects)
		}
		if !allowed {
			return nil, planChanged("the " + step.ID + " step changed from " + want.Action + " to " + step.Action)
		}
	}
	return fresh, nil
}

// guideFailureStatus: unknown and partial step results carry over; a failure after earlier successful
// writes, or a store failure after a successful step, leaves the router partially configured.
func guideFailureStatus(stepStatus string, wroteBefore bool) string {
	switch {
	case stepStatus == StatusUnknown:
		return StatusUnknown
	case stepStatus == StatusPartial || stepStatus == StatusSucceeded || wroteBefore:
		return StatusPartial
	}
	return StatusFailed
}

func planChanged(reason string) error {
	return &device.Error{Code: device.CodeConfirmationRequired, Message: reason + "; detect again and confirm the new plan"}
}

// stepSink tags every event of a guided step's use case with the step.
func stepSink(events EventSink, step string) EventSink {
	if events == nil {
		return nil
	}
	return EventSinkFunc(func(event Event) {
		event.Step = step
		events.Emit(event)
	})
}

func emitStep(events EventSink, step, status string) {
	if events != nil {
		events.Emit(Event{SchemaVersion: 1, Operation: "guide apply", Stage: step, Status: status, Step: step, Time: time.Now().UTC()})
	}
}
