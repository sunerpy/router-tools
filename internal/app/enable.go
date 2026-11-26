package app

import (
	"context"
	"errors"

	"github.com/sunerpy/router-tools/internal/credentials"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
	"github.com/sunerpy/router-tools/internal/secret"
	"github.com/sunerpy/router-tools/internal/state"
)

const (
	StatusPartial          = "partial"
	StatusUnknown          = "unknown"
	OutcomeAlreadyVerified = "already_verified"
)

// CleanupResult records every attempted MAC deletion.
type CleanupResult struct {
	MAC      string `json:"mac"`
	Attempts int    `json:"attempts"`
	Success  bool   `json:"success"`
	Error    string `json:"error,omitempty"`
}

// EnableResult is the adapter's outcome, including partial evidence on failure.
type EnableResult struct {
	Status        string          `json:"status"`
	Outcome       string          `json:"outcome,omitempty"`
	AcceptedSteps int             `json:"accepted_steps"`
	SSHVerified   bool            `json:"ssh_verified"`
	AttemptedMACs []string        `json:"attempted_macs,omitempty"`
	Cleanup       []CleanupResult `json:"cleanup,omitempty"`
	Warnings      []string        `json:"warnings,omitempty"`
}

// SSHEnabler owns the adapter-specific fixed command flow.
type SSHEnabler interface {
	EnableSSH(context.Context, device.Target, secret.Password, secret.Password, hostkey.Policy) (EnableResult, error)
}

// EnableRequest contains both passwords and an explicit ordinary confirmation.
type EnableRequest struct {
	Target        device.Target
	ProfileID     string
	AdminPassword secret.Password
	RootPassword  secret.Password
	HostKeyPolicy hostkey.Policy
	Confirmed     bool
	// ExpectedIdentity, set by a guided run, must match the device after the lock is taken.
	ExpectedIdentity *ConfirmedIdentity
	// Credentials may fill the admin password; the root password the flow sets is always typed.
	Credentials CredentialUse
}

// EnableReport is safe for CLI/GUI rendering after error redaction.
type EnableReport struct {
	SchemaVersion int                `json:"schema_version"`
	Operation     string             `json:"operation"`
	Status        string             `json:"status"`
	Outcome       string             `json:"outcome,omitempty"`
	Host          string             `json:"host"`
	Device        device.Observation `json:"device"`
	Profile       device.Profile     `json:"profile"`
	AcceptedSteps int                `json:"accepted_steps"`
	SSHVerified   bool               `json:"ssh_verified"`
	AttemptedMACs []string           `json:"attempted_macs,omitempty"`
	Cleanup       []CleanupResult    `json:"cleanup,omitempty"`
	Warnings      []string           `json:"warnings,omitempty"`
	Credentials   []CredentialNote   `json:"credentials,omitempty"`
	Error         *device.Error      `json:"error,omitempty"`
}

// EnableSSH performs a confirmed, exact-profile temporary enable flow.
func (s *Service) EnableSSH(ctx context.Context, request EnableRequest, events EventSink) (EnableReport, error) {
	report := EnableReport{SchemaVersion: 1, Operation: "enable", Status: "running", Host: request.Target.Host}
	emit := operationEmitter("enable", events)
	redactor := secret.NewRedactor(request.AdminPassword.Expose(), request.RootPassword.Expose())
	var creds *credentialSession
	fail := func(err error) (EnableReport, error) {
		normalized := normalizeError(err)
		redacted := *normalized
		redacted.Message = redactor.Text(redacted.Message)
		if report.Status == "running" || report.Status == "" {
			report.Status = StatusFailed
		}
		report.Error = &redacted
		for i := range report.Cleanup {
			report.Cleanup[i].Error = redactor.Text(report.Cleanup[i].Error)
		}
		for i := range report.Warnings {
			report.Warnings[i] = redactor.Text(report.Warnings[i])
		}
		report.Credentials = creds.report()
		emit("failed", report.Status)
		return report, &redacted
	}
	emit("validate", "running")
	if err := request.Target.Validate(); err != nil {
		return fail(err)
	}
	if err := checkTypedPassword(request.AdminPassword, request.Credentials); err != nil {
		return fail(err)
	}
	if err := request.RootPassword.Validate(); err != nil {
		return fail(err)
	}
	if !request.Confirmed {
		return fail(&device.Error{Code: device.CodeConfirmationRequired, Message: "enable requires explicit confirmation"})
	}
	if err := s.checkProfileID(request.ProfileID); err != nil {
		return fail(err)
	}
	release, err := s.locker.Acquire(request.Target)
	if err != nil {
		return fail(err)
	}
	defer func() { _ = release() }()
	emit("probe", "running")
	probe, err := s.discovery.Probe(ctx, request.Target)
	if err != nil {
		return fail(err)
	}
	report.Device = probe.Observation
	emit("compatibility", "running")
	adapter, err := s.resolveProfile(request.ProfileID, probe.Observation)
	if err != nil {
		return fail(err)
	}
	report.Profile = adapter.Profile()
	if !report.Profile.Supports(device.CapabilityEnableSSH) {
		return fail(&device.Error{Code: device.CodeUnsupportedCapability, Message: "the selected profile does not support SSH enable"})
	}
	enabler, ok := adapter.Operations.(SSHEnabler)
	if !ok {
		return fail(&device.Error{Code: device.CodeUnsupportedCapability, Message: "the selected prober does not support SSH enable"})
	}
	if err := checkExpectedIdentity(request.ExpectedIdentity, probe.Observation, report.Profile); err != nil {
		return fail(err)
	}
	creds = s.credentialSession(request.Credentials, request.Target, report.Profile, redactor)
	admin, err := creds.admin(request.AdminPassword)
	if err != nil {
		return fail(err)
	}
	root, err := creds.root(request.RootPassword, false)
	if err != nil {
		return fail(err)
	}
	guard, err := s.beginWrite(request.Target, "enable", state.WriteEvidence{})
	if err != nil {
		return fail(err)
	}
	emit("enable", "running")
	result, err := enabler.EnableSSH(ctx, request.Target, admin, root, creds.hostKeys(request.HostKeyPolicy))
	creds.observe(err)
	finish := func(final EnableReport, finalErr error) (EnableReport, error) {
		writePossible := result.AcceptedSteps > 0 || len(result.AttemptedMACs) > 0 ||
			final.Status == StatusPartial || final.Status == StatusUnknown
		evidence := state.WriteEvidence{
			WriteStarted: result.AcceptedSteps > 0 || len(result.AttemptedMACs) > 0, AcceptedSteps: result.AcceptedSteps,
			AttemptedMACs: result.AttemptedMACs, UncleanedMACs: uncleanedMACs(result.Cleanup),
		}
		return settleReport(guard, &final, final.Status, writePossible, evidence, finalErr, func(e *device.Error) { final.Error = e })
	}
	report.Status = result.Status
	report.Outcome = result.Outcome
	report.AcceptedSteps = result.AcceptedSteps
	report.SSHVerified = result.SSHVerified
	report.AttemptedMACs = result.AttemptedMACs
	report.Cleanup = result.Cleanup
	report.Warnings = result.Warnings
	if err != nil {
		var deviceErr *device.Error
		if errors.As(err, &deviceErr) && deviceErr.Code == device.CodeRemoteResultUnknown {
			report.Status = StatusUnknown
		} else if report.Status == "" || report.Status == "running" {
			if result.AcceptedSteps > 0 {
				report.Status = StatusPartial
			} else {
				report.Status = StatusFailed
			}
		}
		return finish(fail(err))
	}
	if !result.SSHVerified {
		if report.Status == StatusSucceeded {
			report.Status = StatusFailed
		}
		return finish(fail(&device.Error{Code: device.CodeRootVerifyFailed, Message: "enable did not produce a verified SSH root login"}))
	}
	if report.Status == "" {
		report.Status = StatusSucceeded
	}
	if report.Status == StatusSucceeded {
		creds.remember(credentials.KindWebAdmin, credentials.KindRoot)
	}
	report.Credentials = creds.report()
	emit("completed", report.Status)
	return finish(report, nil)
}

// uncleanedMACs lists the temporary MAC rules whose removal did not succeed.
func uncleanedMACs(cleanup []CleanupResult) []string {
	var macs []string
	for _, entry := range cleanup {
		if !entry.Success {
			macs = append(macs, entry.MAC)
		}
	}
	return macs
}
