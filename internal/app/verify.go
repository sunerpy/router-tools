package app

import (
	"context"
	"time"

	"github.com/sunerpy/router-tools/internal/credentials"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
	"github.com/sunerpy/router-tools/internal/secret"
)

// RootVerifier establishes an authenticated root shell for an exact adapter.
type RootVerifier interface {
	VerifyRoot(ctx context.Context, target device.Target, password secret.Password, policy hostkey.Policy, transport string, allowTelnet bool) (RootVerification, error)
}

// RootVerification contains shell evidence without credentials.
type RootVerification struct {
	Transport          string `json:"transport"`
	RootVerified       bool   `json:"root_verified"`
	SSHVerified        bool   `json:"ssh_verified"`
	TelnetRootVerified bool   `json:"telnet_root_verified"`
	BootID             string `json:"boot_id,omitempty"`
}

// VerifyRequest controls root shell verification.
type VerifyRequest struct {
	Target        device.Target
	ProfileID     string
	RootPassword  secret.Password
	Transport     string
	AllowTelnet   bool
	HostKeyPolicy hostkey.Policy
	Credentials   CredentialUse
}

// VerifyReport distinguishes SSH success from Telnet-only success.
type VerifyReport struct {
	SchemaVersion      int                `json:"schema_version"`
	Operation          string             `json:"operation"`
	Status             string             `json:"status"`
	Host               string             `json:"host"`
	Device             device.Observation `json:"device"`
	Profile            device.Profile     `json:"profile"`
	Transport          string             `json:"transport"`
	RootVerified       bool               `json:"root_verified"`
	SSHVerified        bool               `json:"ssh_verified"`
	TelnetRootVerified bool               `json:"telnet_root_verified"`
	BootID             string             `json:"boot_id,omitempty"`
	Credentials        []CredentialNote   `json:"credentials,omitempty"`
	Error              *device.Error      `json:"error,omitempty"`
}

// Verify resolves the exact profile before attempting any shell authentication.
func (s *Service) Verify(ctx context.Context, request VerifyRequest, events EventSink) (VerifyReport, error) {
	if request.Transport == "" {
		request.Transport = "ssh"
	}
	report := VerifyReport{SchemaVersion: 1, Operation: "verify", Status: "running", Host: request.Target.Host, Transport: request.Transport}
	emit := operationEmitter("verify", events)
	redactor := secret.NewRedactor(request.RootPassword.Expose())
	var creds *credentialSession
	fail := func(err error) (VerifyReport, error) {
		normalized := normalizeError(err)
		redacted := *normalized
		redacted.Message = redactor.Text(redacted.Message)
		report.Status = StatusFailed
		report.Error = &redacted
		report.Credentials = creds.report()
		emit("failed", StatusFailed)
		return report, &redacted
	}
	emit("validate", "running")
	if err := request.Target.Validate(); err != nil {
		return fail(err)
	}
	if err := checkTypedPassword(request.RootPassword, request.Credentials); err != nil {
		return fail(err)
	}
	if request.Transport != "ssh" && request.Transport != "telnet" && request.Transport != "auto" {
		return fail(&device.Error{Code: device.CodeInvalidArgument, Message: "transport must be ssh, telnet, or auto"})
	}
	if request.Transport == "telnet" && !request.AllowTelnet {
		return fail(&device.Error{Code: device.CodeUnsupportedCapability, Message: "Telnet requires explicit --allow-telnet"})
	}
	if err := s.checkProfileID(request.ProfileID); err != nil {
		return fail(err)
	}
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
	if !report.Profile.Supports(device.CapabilityVerify) {
		return fail(&device.Error{Code: device.CodeUnsupportedCapability, Message: "the selected profile does not support root verification"})
	}
	verifier, ok := adapter.Operations.(RootVerifier)
	if !ok {
		return fail(&device.Error{Code: device.CodeUnsupportedCapability, Message: "the selected prober does not support root verification"})
	}
	creds = s.credentialSession(request.Credentials, request.Target, report.Profile, redactor)
	root, err := creds.root(request.RootPassword, request.AllowTelnet)
	if err != nil {
		return fail(err)
	}
	emit("root_verify", "running")
	result, err := verifier.VerifyRoot(ctx, request.Target, root, creds.hostKeys(request.HostKeyPolicy), request.Transport, request.AllowTelnet)
	if err != nil {
		creds.observe(err)
		return fail(err)
	}
	report.Transport = result.Transport
	report.RootVerified = result.RootVerified
	report.SSHVerified = result.RootVerified && result.Transport == "ssh"
	report.TelnetRootVerified = result.RootVerified && result.Transport == "telnet"
	report.BootID = result.BootID
	if !report.RootVerified {
		return fail(&device.Error{Code: device.CodeRootVerifyFailed, Message: "root verification did not succeed"})
	}
	if report.SSHVerified {
		creds.remember(credentials.KindRoot)
	}
	report.Credentials = creds.report()
	report.Status = StatusSucceeded
	emit("completed", StatusSucceeded)
	return report, nil
}

// resolveProfile returns the exact adapter for the observation; an explicit profile never falls back.
func (s *Service) resolveProfile(profileID string, observation device.Observation) (device.Adapter, error) {
	if profileID != "" && profileID != "auto" {
		return s.registry.ResolveProfile(profileID, observation)
	}
	return s.registry.Resolve(observation)
}

func operationEmitter(operation string, events EventSink) func(string, string) {
	return func(stage, status string) {
		if events != nil {
			events.Emit(Event{SchemaVersion: 1, Operation: operation, Stage: stage, Status: status, Time: time.Now().UTC()})
		}
	}
}
