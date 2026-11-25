// Package app provides frontend-independent application use cases.
package app

import (
	"context"
	"errors"
	"fmt"
	"time"

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
	SchemaVersion int       `json:"schema_version"`
	Operation     string    `json:"operation"`
	Stage         string    `json:"stage"`
	Status        string    `json:"status"`
	Time          time.Time `json:"time"`
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

// Service owns application flow while protocols remain behind interfaces.
type Service struct {
	registry *device.Registry
	prober   device.Prober
	locker   OperationLocker
}

// NewService wires frontend-independent dependencies.
func NewService(registry *device.Registry, prober device.Prober) (*Service, error) {
	return NewServiceWithLocker(registry, prober, nil)
}

// NewServiceWithLocker wires a cross-process locker in production.
func NewServiceWithLocker(registry *device.Registry, prober device.Prober, locker OperationLocker) (*Service, error) {
	if registry == nil || prober == nil {
		return nil, fmt.Errorf("app service requires a registry and prober")
	}
	if locker == nil {
		locker = newLocalLocker()
	}
	return &Service{registry: registry, prober: prober, locker: locker}, nil
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
}

// DoctorReport is the stable result rendered by CLI and future GUI clients.
type DoctorReport struct {
	SchemaVersion      int                `json:"schema_version"`
	Operation          string             `json:"operation"`
	Status             string             `json:"status"`
	Host               string             `json:"host"`
	Device             device.Observation `json:"device"`
	Profile            device.Profile     `json:"profile"`
	Compatible         bool               `json:"compatible"`
	SSH                device.PortState   `json:"ssh"`
	Telnet             device.PortState   `json:"telnet"`
	AdminAuth          string             `json:"admin_auth"`
	RootVerify         string             `json:"root_verify"`
	Transport          string             `json:"transport,omitempty"`
	SSHVerified        bool               `json:"ssh_verified"`
	TelnetRootVerified bool               `json:"telnet_root_verified"`
	BootID             string             `json:"boot_id,omitempty"`
	NextSteps          []string           `json:"next_steps,omitempty"`
	Error              *device.Error      `json:"error,omitempty"`
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
	fail := func(err error) (DoctorReport, error) {
		normalized := normalizeError(err)
		redactor := secret.NewRedactor()
		if request.WithAdminAuth {
			redactor.Add(request.AdminPassword.Expose())
			if report.AdminAuth != CheckVerified {
				report.AdminAuth = CheckFailed
			}
		}
		if request.WithRootVerify {
			redactor.Add(request.RootPassword.Expose())
			if report.RootVerify != CheckVerified {
				report.RootVerify = CheckFailed
			}
		}
		redacted := *normalized
		redacted.Message = redactor.Text(redacted.Message)
		report.Status = StatusFailed
		report.Error = &redacted
		emit("failed", StatusFailed)
		return report, &redacted
	}

	emit("validate", "running")
	if err := request.Target.Validate(); err != nil {
		return fail(err)
	}
	if request.WithAdminAuth {
		if err := request.AdminPassword.Validate(); err != nil {
			return fail(err)
		}
	}
	if request.WithRootVerify {
		if err := request.RootPassword.Validate(); err != nil {
			return fail(err)
		}
		if request.Transport == "telnet" && !request.AllowTelnet {
			return fail(&device.Error{Code: device.CodeUnsupportedCapability, Message: "Telnet requires explicit permission"})
		}
	}
	emit("probe", "running")
	probe, err := s.prober.Probe(ctx, request.Target)
	if err != nil {
		return fail(err)
	}
	report.Device = probe.Observation
	report.SSH = probe.SSH
	report.Telnet = probe.Telnet

	emit("compatibility", "running")
	var driver device.Driver
	if request.ProfileID != "" && request.ProfileID != "auto" {
		driver, err = s.registry.ResolveProfile(request.ProfileID, probe.Observation)
	} else {
		driver, err = s.registry.Resolve(probe.Observation)
	}
	if err != nil {
		return fail(err)
	}
	report.Profile = driver.Profile()
	report.Compatible = true
	if request.WithAdminAuth {
		emit("admin_auth", "running")
		if !report.Profile.Supports(device.CapabilityWebAuth) {
			return fail(&device.Error{Code: device.CodeUnsupportedCapability, Message: "the selected profile does not support Web authentication"})
		}
		verifier, ok := s.prober.(AdminVerifier)
		if !ok {
			return fail(&device.Error{Code: device.CodeUnsupportedCapability, Message: "the selected prober does not support Web authentication"})
		}
		if err := verifier.VerifyAdmin(ctx, request.Target, request.AdminPassword); err != nil {
			return fail(err)
		}
		report.AdminAuth = CheckVerified
	}
	if request.WithRootVerify {
		emit("root_verify", "running")
		if !report.Profile.Supports(device.CapabilityVerify) {
			return fail(&device.Error{Code: device.CodeUnsupportedCapability, Message: "the selected profile does not support root verification"})
		}
		verifier, ok := s.prober.(RootVerifier)
		if !ok {
			return fail(&device.Error{Code: device.CodeUnsupportedCapability, Message: "root verification is unavailable"})
		}
		result, err := verifier.VerifyRoot(ctx, request.Target, request.RootPassword, request.HostKeyPolicy, request.Transport, request.AllowTelnet)
		if err != nil {
			return fail(err)
		}
		report.RootVerify = CheckVerified
		report.Transport = result.Transport
		report.SSHVerified = result.RootVerified && result.Transport == "ssh"
		report.TelnetRootVerified = result.RootVerified && result.Transport == "telnet"
		report.BootID = result.BootID
	}
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
