package app

import (
	"context"
	"errors"

	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
	"github.com/sunerpy/router-tools/internal/secret"
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
	Error         *device.Error      `json:"error,omitempty"`
}

// EnableSSH performs a confirmed, exact-profile temporary enable flow.
func (s *Service) EnableSSH(ctx context.Context, request EnableRequest, events EventSink) (EnableReport, error) {
	report := EnableReport{SchemaVersion: 1, Operation: "enable", Status: "running", Host: request.Target.Host}
	emit := operationEmitter("enable", events)
	redactor := secret.NewRedactor(request.AdminPassword.Expose(), request.RootPassword.Expose())
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
		emit("failed", report.Status)
		return report, &redacted
	}
	emit("validate", "running")
	if err := request.Target.Validate(); err != nil {
		return fail(err)
	}
	if err := request.AdminPassword.Validate(); err != nil {
		return fail(err)
	}
	if err := request.RootPassword.Validate(); err != nil {
		return fail(err)
	}
	if !request.Confirmed {
		return fail(&device.Error{Code: device.CodeConfirmationRequired, Message: "enable requires explicit confirmation"})
	}
	release, err := s.locker.Acquire(request.Target)
	if err != nil {
		return fail(err)
	}
	defer func() { _ = release() }()
	emit("probe", "running")
	probe, err := s.prober.Probe(ctx, request.Target)
	if err != nil {
		return fail(err)
	}
	report.Device = probe.Observation
	emit("compatibility", "running")
	driver, err := s.resolveProfile(request.ProfileID, probe.Observation)
	if err != nil {
		return fail(err)
	}
	report.Profile = driver.Profile()
	if !report.Profile.Supports(device.CapabilityEnableSSH) {
		return fail(&device.Error{Code: device.CodeUnsupportedCapability, Message: "the selected profile does not support SSH enable"})
	}
	enabler, ok := s.prober.(SSHEnabler)
	if !ok {
		return fail(&device.Error{Code: device.CodeUnsupportedCapability, Message: "the selected prober does not support SSH enable"})
	}
	emit("enable", "running")
	result, err := enabler.EnableSSH(ctx, request.Target, request.AdminPassword, request.RootPassword, request.HostKeyPolicy)
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
		return fail(err)
	}
	if !result.SSHVerified {
		if report.Status == StatusSucceeded {
			report.Status = StatusFailed
		}
		return fail(&device.Error{Code: device.CodeRootVerifyFailed, Message: "enable did not produce a verified SSH root login"})
	}
	if report.Status == "" {
		report.Status = StatusSucceeded
	}
	emit("completed", report.Status)
	return report, nil
}
