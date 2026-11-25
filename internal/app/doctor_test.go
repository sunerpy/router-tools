package app_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/device"
)

type fakeDriver struct {
	profile device.Profile
	match   device.MatchResult
}

func (d fakeDriver) Profile() device.Profile                     { return d.profile }
func (d fakeDriver) Match(device.Observation) device.MatchResult { return d.match }

type fakeProber struct {
	result device.ProbeResult
	err    error
	calls  int
}

func (p *fakeProber) Probe(context.Context, device.Target) (device.ProbeResult, error) {
	p.calls++
	return p.result, p.err
}

type eventRecorder struct {
	stages []string
}

func (r *eventRecorder) Emit(event app.Event) {
	r.stages = append(r.stages, event.Stage)
}

func TestDoctorSucceedsThroughFrontendIndependentService(t *testing.T) {
	t.Parallel()

	driver := fakeDriver{
		profile: device.Profile{ID: "test-rd08-1.1.96", Capabilities: []device.Capability{device.CapabilityDoctor}},
		match:   device.MatchResult{Level: device.ExactMatch},
	}
	prober := &fakeProber{result: device.ProbeResult{
		Observation: device.Observation{Hardware: "RD08", ROMVersion: "1.1.96"},
		SSH:         device.PortOpen,
		Telnet:      device.PortClosed,
	}}
	service, err := app.NewService(newTestRegistry(t, driver, prober), prober)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	events := &eventRecorder{}

	report, err := service.Doctor(t.Context(), app.DoctorRequest{
		Target: device.Target{Host: "192.168.31.1", WebPort: 80, SSHPort: 22, TelnetPort: 23},
	}, events)
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	if report.Status != app.StatusSucceeded || !report.Compatible {
		t.Fatalf("Doctor() report = %+v", report)
	}
	if report.Profile.ID != driver.profile.ID {
		t.Fatalf("profile ID = %q", report.Profile.ID)
	}
	if report.AdminAuth != app.CheckSkipped || report.RootVerify != app.CheckSkipped {
		t.Fatalf("optional checks were not marked skipped: %+v", report)
	}
	wantStages := []string{"validate", "probe", "compatibility", "completed"}
	if !reflect.DeepEqual(events.stages, wantStages) {
		t.Fatalf("event stages = %v, want %v", events.stages, wantStages)
	}
}

func TestDoctorFailsClosedAndReportsStableError(t *testing.T) {
	t.Parallel()

	driver := fakeDriver{
		profile: device.Profile{ID: "test-rd08-1.1.96"},
		match:   device.MatchResult{Level: device.DeviceMatch},
	}
	prober := &fakeProber{result: device.ProbeResult{Observation: device.Observation{Hardware: "RD08", ROMVersion: "9.9.9"}}}
	service, _ := app.NewService(newTestRegistry(t, driver, prober), prober)

	report, err := service.Doctor(t.Context(), app.DoctorRequest{
		Target: device.Target{Host: "192.168.31.1", WebPort: 80, SSHPort: 22, TelnetPort: 23},
	}, nil)
	assertCode(t, err, device.CodeUnsupportedFirmware)
	if report.Status != app.StatusFailed || report.Compatible || report.Error == nil {
		t.Fatalf("failure report = %+v", report)
	}
	if report.Error.Code != device.CodeUnsupportedFirmware {
		t.Fatalf("report error = %+v", report.Error)
	}
}

func TestDoctorRejectsInvalidTargetBeforeNetworkAccess(t *testing.T) {
	t.Parallel()

	prober := &fakeProber{}
	service, _ := app.NewService(newTestRegistry(t, fakeDriver{profile: device.Profile{ID: "test"}}, prober), prober)

	_, err := service.Doctor(t.Context(), app.DoctorRequest{}, nil)
	assertCode(t, err, device.CodeInvalidArgument)
	if prober.calls != 0 {
		t.Fatalf("prober calls = %d, want 0", prober.calls)
	}
}

func TestDoctorPreservesProbeErrorCode(t *testing.T) {
	t.Parallel()

	prober := &fakeProber{err: &device.Error{Code: device.CodeWebProtocolError, Message: "invalid response"}}
	service, _ := app.NewService(newTestRegistry(t, fakeDriver{profile: device.Profile{ID: "test"}}, prober), prober)

	report, err := service.Doctor(t.Context(), app.DoctorRequest{
		Target: device.Target{Host: "router.local", WebPort: 80, SSHPort: 22, TelnetPort: 23},
	}, nil)
	assertCode(t, err, device.CodeWebProtocolError)
	if report.Error == nil || report.Error.Code != device.CodeWebProtocolError {
		t.Fatalf("report error = %+v", report.Error)
	}
}

func TestDoctorPreservesCancellation(t *testing.T) {
	t.Parallel()

	prober := &fakeProber{err: context.Canceled}
	service, _ := app.NewService(newTestRegistry(t, fakeDriver{profile: device.Profile{ID: "test"}}, prober), prober)

	report, err := service.Doctor(t.Context(), app.DoctorRequest{
		Target: device.Target{Host: "router.local", WebPort: 80, SSHPort: 22, TelnetPort: 23},
	}, nil)
	assertCode(t, err, device.CodeCanceled)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Doctor() error = %v, want context.Canceled in chain", err)
	}
	if report.Error == nil || report.Error.Code != device.CodeCanceled {
		t.Fatalf("report error = %+v", report.Error)
	}
}

func assertCode(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want %s", want)
	}
	var deviceErr *device.Error
	if !errors.As(err, &deviceErr) {
		t.Fatalf("error = %T %v, want *device.Error", err, err)
	}
	if deviceErr.Code != want {
		t.Fatalf("error code = %q, want %q", deviceErr.Code, want)
	}
}

// newTestRegistry registers one fake driver served by operations; single-adapter tests also pass the same
// object to the service as its discovery prober, as they did before per-profile dispatch. Fixture IDs that
// are not already "model-firmware" are completed with firmware "1".
func newTestRegistry(t *testing.T, driver fakeDriver, operations any) *device.Registry {
	t.Helper()
	p := driver.profile
	if p.ModelID == "" {
		if i := strings.LastIndex(p.ID, "-"); i > 0 {
			p.ModelID, p.Firmware = p.ID[:i], p.ID[i+1:]
		} else {
			p.ModelID, p.Firmware, p.ID = p.ID, "1", p.ID+"-1"
		}
	}
	driver.profile = p
	registry, err := device.NewRegistry(device.Adapter{Driver: driver, Operations: operations})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	return registry
}
