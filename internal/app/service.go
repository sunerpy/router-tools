// Package app provides frontend-independent application use cases.
package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sunerpy/router-tools/internal/credentials"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
	"github.com/sunerpy/router-tools/internal/secret"
)

const (
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
	CheckSkipped    = "skipped"
	CheckPending    = "pending"
	CheckVerified   = "verified"
	CheckFailed     = "failed"
)

// Event is emitted for CLI rendering and future GUI progress bindings.
type Event struct {
	SchemaVersion int    `json:"schema_version"`
	Operation     string `json:"operation"`
	Stage         string `json:"stage"`
	Status        string `json:"status"`
	// Step names the guided step an event belongs to; empty outside a guided run.
	Step string    `json:"step,omitempty"`
	Time time.Time `json:"time"`
}

// EventSink receives structured progress. Implementations must not log secrets.
type EventSink interface {
	Emit(event Event)
}

// EventSinkFunc adapts a function into an EventSink.
type EventSinkFunc func(event Event)

func (f EventSinkFunc) Emit(event Event) { f(event) }

// AdminVerifier validates Web credentials without exposing a token to the application.
type AdminVerifier interface {
	VerifyAdmin(ctx context.Context, target device.Target, password secret.Password) error
}

// Service owns application flow while protocols remain behind interfaces. The discovery prober only
// identifies devices; every capability is served by the resolved adapter's own implementation.
type Service struct {
	registry    *device.Registry
	discovery   device.Prober
	locker      OperationLocker
	outcomes    OutcomeStore
	credentials credentials.Store
	clock       func() time.Time
	backupRoot  string
	journalRoot string
}

// NewService wires frontend-independent dependencies.
func NewService(registry *device.Registry, prober device.Prober) (*Service, error) {
	return NewServiceWithLocker(registry, prober, nil)
}

// NewServiceWithLocker wires a cross-process locker in production. prober is the discovery prober: it is
// only asked to Probe, never type-asserted for capabilities.
func NewServiceWithLocker(registry *device.Registry, prober device.Prober, locker OperationLocker) (*Service, error) {
	return NewServiceWithOptions(registry, prober, Options{Locker: locker})
}

// Options carries the production stores and managed directories. A nil Outcomes store disables the
// recovery guard and is meant for tests only; both mains always pass one. A nil Credentials store means no
// password is ever saved or filled.
type Options struct {
	Locker      OperationLocker
	Outcomes    OutcomeStore
	Credentials credentials.Store
	BackupRoot  string
	JournalRoot string
	Clock       func() time.Time
}

// NewServiceWithOptions wires the service with explicit stores. prober is the discovery prober.
func NewServiceWithOptions(registry *device.Registry, prober device.Prober, options Options) (*Service, error) {
	if registry == nil || prober == nil {
		return nil, fmt.Errorf("app service requires a registry and prober")
	}
	if options.Locker == nil {
		options.Locker = newLocalLocker()
	}
	if options.Clock == nil {
		options.Clock = func() time.Time { return time.Now().UTC() }
	}
	return &Service{
		registry: registry, discovery: prober, locker: options.Locker, outcomes: options.Outcomes,
		credentials: options.Credentials, clock: options.Clock, backupRoot: options.BackupRoot, journalRoot: options.JournalRoot,
	}, nil
}

// Catalog lists the registered device models and their exact firmware profiles. It needs no network.
func (s *Service) Catalog() []device.CatalogEntry {
	return s.registry.Catalog()
}

// checkProfileID rejects an unregistered explicit profile before any lock or network access.
func (s *Service) checkProfileID(profileID string) error {
	if profileID == "" || profileID == "auto" {
		return nil
	}
	if _, ok := s.registry.Lookup(profileID); !ok {
		return &device.Error{Code: device.CodeUnsupportedProfile, Message: "the requested profile is not registered; list profiles with routerctl profiles"}
	}
	return nil
}

// DoctorRequest controls a read-only preflight.
type DoctorRequest struct {
	Target         device.Target
	ProfileID      string
	WithAdminAuth  bool
	AdminPassword  secret.Password
	WithRootVerify bool
	RootPassword   secret.Password
	Transport      string
	AllowTelnet    bool
	HostKeyPolicy  hostkey.Policy
	Credentials    CredentialUse
}

// DoctorReport is the stable result rendered by CLI and future GUI clients.
type DoctorReport struct {
	SchemaVersion      int                   `json:"schema_version"`
	Operation          string                `json:"operation"`
	Status             string                `json:"status"`
	Host               string                `json:"host"`
	Device             device.Observation    `json:"device"`
	Profile            device.Profile        `json:"profile"`
	Candidates         []device.ProfileMatch `json:"candidates,omitempty"`
	Compatible         bool                  `json:"compatible"`
	SSH                device.PortState      `json:"ssh"`
	Telnet             device.PortState      `json:"telnet"`
	AdminAuth          string                `json:"admin_auth"`
	RootVerify         string                `json:"root_verify"`
	Transport          string                `json:"transport,omitempty"`
	SSHVerified        bool                  `json:"ssh_verified"`
	TelnetRootVerified bool                  `json:"telnet_root_verified"`
	BootID             string                `json:"boot_id,omitempty"`
	NextSteps          []string              `json:"next_steps,omitempty"`
	Credentials        []CredentialNote      `json:"credentials,omitempty"`
	Error              *device.Error         `json:"error,omitempty"`
}

// Doctor executes no device writes. Open ports are never reported as authenticated shells.
func (s *Service) Doctor(ctx context.Context, request DoctorRequest, events EventSink) (DoctorReport, error) {
	report := DoctorReport{
		SchemaVersion: 1,
		Operation:     "doctor",
		Status:        "running",
		Host:          request.Target.Host,
		AdminAuth:     CheckSkipped,
		RootVerify:    CheckSkipped,
	}
	emit := func(stage, status string) {
		if events != nil {
			events.Emit(Event{
				SchemaVersion: 1,
				Operation:     "doctor",
				Stage:         stage,
				Status:        status,
				Time:          time.Now().UTC(),
			})
		}
	}
	if request.WithAdminAuth {
		report.AdminAuth = CheckPending
	}
	if request.WithRootVerify {
		report.RootVerify = CheckPending
		if request.Transport == "" {
			request.Transport = "ssh"
		}
		report.Transport = request.Transport
	}
	redactor := secret.NewRedactor()
	if request.WithAdminAuth {
		redactor.Add(request.AdminPassword.Expose())
	}
	if request.WithRootVerify {
		redactor.Add(request.RootPassword.Expose())
	}
	var creds *credentialSession
	fail := func(err error) (DoctorReport, error) {
		normalized := normalizeError(err)
		if request.WithAdminAuth && report.AdminAuth != CheckVerified {
			report.AdminAuth = CheckFailed
		}
		if request.WithRootVerify && report.RootVerify != CheckVerified {
			report.RootVerify = CheckFailed
		}
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
	if err := s.checkProfileID(request.ProfileID); err != nil {
		return fail(err)
	}
	if request.WithAdminAuth {
		if err := checkTypedPassword(request.AdminPassword, request.Credentials); err != nil {
			return fail(err)
		}
	}
	if request.WithRootVerify {
		if err := checkTypedPassword(request.RootPassword, request.Credentials); err != nil {
			return fail(err)
		}
		if request.Transport == "telnet" && !request.AllowTelnet {
			return fail(&device.Error{Code: device.CodeUnsupportedCapability, Message: "Telnet requires explicit permission"})
		}
	}
	emit("probe", "running")
	probe, err := s.discovery.Probe(ctx, request.Target)
	if err != nil {
		return fail(err)
	}
	report.Device = probe.Observation
	report.SSH = probe.SSH
	report.Telnet = probe.Telnet

	emit("compatibility", "running")
	adapter, err := s.resolveProfile(request.ProfileID, probe.Observation)
	if err != nil {
		report.Candidates = s.registry.Candidates(probe.Observation)
		return fail(err)
	}
	report.Profile = adapter.Profile()
	report.Compatible = true
	creds = s.credentialSession(request.Credentials, request.Target, report.Profile, redactor)
	if request.WithAdminAuth {
		emit("admin_auth", "running")
		if !report.Profile.Supports(device.CapabilityWebAuth) {
			return fail(&device.Error{Code: device.CodeUnsupportedCapability, Message: "the selected profile does not support Web authentication"})
		}
		verifier, ok := adapter.Operations.(AdminVerifier)
		if !ok {
			return fail(&device.Error{Code: device.CodeUnsupportedCapability, Message: "the selected prober does not support Web authentication"})
		}
		admin, err := creds.admin(request.AdminPassword)
		if err != nil {
			return fail(err)
		}
		if err := verifier.VerifyAdmin(ctx, request.Target, admin); err != nil {
			creds.observe(err)
			return fail(err)
		}
		report.AdminAuth = CheckVerified
		creds.remember(credentials.KindWebAdmin)
	}
	if request.WithRootVerify {
		emit("root_verify", "running")
		if !report.Profile.Supports(device.CapabilityVerify) {
			return fail(&device.Error{Code: device.CodeUnsupportedCapability, Message: "the selected profile does not support root verification"})
		}
		verifier, ok := adapter.Operations.(RootVerifier)
		if !ok {
			return fail(&device.Error{Code: device.CodeUnsupportedCapability, Message: "root verification is unavailable"})
		}
		root, err := creds.root(request.RootPassword, request.AllowTelnet)
		if err != nil {
			return fail(err)
		}
		result, err := verifier.VerifyRoot(ctx, request.Target, root, creds.hostKeys(request.HostKeyPolicy), request.Transport, request.AllowTelnet)
		if err != nil {
			creds.observe(err)
			return fail(err)
		}
		report.RootVerify = CheckVerified
		report.Transport = result.Transport
		report.SSHVerified = result.RootVerified && result.Transport == "ssh"
		report.TelnetRootVerified = result.RootVerified && result.Transport == "telnet"
		report.BootID = result.BootID
		if report.SSHVerified {
			creds.remember(credentials.KindRoot)
		}
	}
	report.Credentials = creds.report()
	report.Status = StatusSucceeded
	report.NextSteps = []string{
		"Port state is connectivity evidence only; verify root authentication before treating SSH as available",
	}
	emit("completed", StatusSucceeded)
	return report, nil
}

func normalizeError(err error) *device.Error {
	if errors.Is(err, context.Canceled) {
		return device.WrapError(device.CodeCanceled, "operation canceled", context.Canceled)
	}
	var deviceErr *device.Error
	if errors.As(err, &deviceErr) {
		return deviceErr
	}
	return &device.Error{Code: device.CodeInternalError, Message: err.Error()}
}
