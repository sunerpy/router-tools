package app_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/secret"
)

type adminProber struct {
	fakeProber
	verifyErr      error
	verifyCalls    int
	verifyPassword string
}

func (p *adminProber) VerifyAdmin(_ context.Context, _ device.Target, password secret.Password) error {
	p.verifyCalls++
	p.verifyPassword = password.Expose()
	return p.verifyErr
}

func TestDoctorWithAdminAuthVerifiesWithoutReportingToken(t *testing.T) {
	t.Parallel()

	driver := fakeDriver{
		profile: device.Profile{
			ID:           "test-rd08-1.1.96",
			Capabilities: []device.Capability{device.CapabilityDoctor, device.CapabilityWebAuth},
		},
		match: device.MatchResult{Level: device.ExactMatch},
	}
	prober := &adminProber{fakeProber: fakeProber{result: device.ProbeResult{
		Observation: device.Observation{Hardware: "RD08", ROMVersion: "1.1.96"},
	}}}
	service, _ := app.NewService(newTestRegistry(t, driver, prober), prober)
	password, _ := secret.NewPassword(" admin-password ")
	events := &eventRecorder{}

	report, err := service.Doctor(t.Context(), app.DoctorRequest{
		Target:        device.Target{Host: "router.local", WebPort: 80, SSHPort: 22, TelnetPort: 23},
		WithAdminAuth: true,
		AdminPassword: password,
	}, events)
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	if report.AdminAuth != app.CheckVerified || prober.verifyCalls != 1 {
		t.Fatalf("admin auth report = %q, calls = %d", report.AdminAuth, prober.verifyCalls)
	}
	if prober.verifyPassword != " admin-password " {
		t.Fatalf("password was changed: %q", prober.verifyPassword)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	for _, hidden := range []string{" admin-password ", "stok", "token"} {
		if strings.Contains(string(encoded), hidden) {
			t.Fatalf("report contains %q: %s", hidden, encoded)
		}
	}
	wantStages := []string{"validate", "probe", "compatibility", "admin_auth", "completed"}
	if !equalStrings(events.stages, wantStages) {
		t.Fatalf("event stages = %v, want %v", events.stages, wantStages)
	}
}

func TestDoctorAdminAuthFailureIsRedacted(t *testing.T) {
	t.Parallel()

	driver := fakeDriver{
		profile: device.Profile{ID: "test", Capabilities: []device.Capability{device.CapabilityWebAuth}},
		match:   device.MatchResult{Level: device.ExactMatch},
	}
	prober := &adminProber{
		fakeProber: fakeProber{result: device.ProbeResult{Observation: device.Observation{Hardware: "RD08"}}},
		verifyErr: &device.Error{
			Code:    device.CodeWebAuthFailed,
			Message: "password admin-secret failed at ;stok=token-secret/api",
		},
	}
	service, _ := app.NewService(newTestRegistry(t, driver, prober), prober)
	password, _ := secret.NewPassword("admin-secret")

	report, err := service.Doctor(t.Context(), app.DoctorRequest{
		Target:        device.Target{Host: "router.local", WebPort: 80, SSHPort: 22, TelnetPort: 23},
		WithAdminAuth: true,
		AdminPassword: password,
	}, nil)
	assertCode(t, err, device.CodeWebAuthFailed)
	if report.AdminAuth != app.CheckFailed {
		t.Fatalf("AdminAuth = %q, want failed", report.AdminAuth)
	}
	encoded, _ := json.Marshal(report)
	for _, output := range []string{err.Error(), string(encoded)} {
		for _, hidden := range []string{"admin-secret", "token-secret"} {
			if strings.Contains(output, hidden) {
				t.Fatalf("output contains %q: %s", hidden, output)
			}
		}
	}
}

func TestDoctorAdminAuthRejectsMissingCapability(t *testing.T) {
	t.Parallel()

	driver := fakeDriver{
		profile: device.Profile{ID: "test", Capabilities: []device.Capability{device.CapabilityDoctor}},
		match:   device.MatchResult{Level: device.ExactMatch},
	}
	prober := &adminProber{fakeProber: fakeProber{result: device.ProbeResult{Observation: device.Observation{Hardware: "RD08"}}}}
	service, _ := app.NewService(newTestRegistry(t, driver, prober), prober)
	password, _ := secret.NewPassword("admin")

	report, err := service.Doctor(t.Context(), app.DoctorRequest{
		Target:        device.Target{Host: "router.local", WebPort: 80, SSHPort: 22, TelnetPort: 23},
		WithAdminAuth: true,
		AdminPassword: password,
	}, nil)
	assertCode(t, err, device.CodeUnsupportedCapability)
	if prober.verifyCalls != 0 {
		t.Fatalf("VerifyAdmin calls = %d, want 0", prober.verifyCalls)
	}
	if report.AdminAuth != app.CheckFailed {
		t.Fatalf("AdminAuth = %q", report.AdminAuth)
	}
}

func TestDoctorAdminPasswordIsValidatedBeforeNetworkAccess(t *testing.T) {
	t.Parallel()

	prober := &adminProber{}
	service, _ := app.NewService(newTestRegistry(t, fakeDriver{profile: device.Profile{ID: "test"}}, prober), prober)

	_, err := service.Doctor(t.Context(), app.DoctorRequest{
		Target:        device.Target{Host: "router.local", WebPort: 80, SSHPort: 22, TelnetPort: 23},
		WithAdminAuth: true,
		AdminPassword: secret.Password{},
	}, nil)
	assertCode(t, err, device.CodeInvalidPassword)
	if prober.calls != 0 || prober.verifyCalls != 0 {
		t.Fatalf("network calls = probe:%d verify:%d, want zero", prober.calls, prober.verifyCalls)
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
