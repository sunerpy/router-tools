package app_test

import (
	"context"
	"testing"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
	"github.com/sunerpy/router-tools/internal/secret"
)

type rootProber struct {
	fakeProber
	result    app.RootVerification
	err       error
	calls     int
	transport string
}

func (p *rootProber) VerifyRoot(_ context.Context, _ device.Target, _ secret.Password, _ hostkey.Policy, transport string, _ bool) (app.RootVerification, error) {
	p.calls++
	p.transport = transport
	return p.result, p.err
}

func TestVerifySSHRoot(t *testing.T) {
	t.Parallel()

	driver := fakeDriver{profile: device.Profile{ID: "test", Capabilities: []device.Capability{device.CapabilityVerify}}, match: device.MatchResult{Level: device.ExactMatch}}
	prober := &rootProber{
		fakeProber: fakeProber{result: device.ProbeResult{Observation: device.Observation{Hardware: "RD08", ROMVersion: "1.1.96"}}},
		result:     app.RootVerification{Transport: "ssh", RootVerified: true, BootID: "00000000-0000-0000-0000-000000000001"},
	}
	service, _ := app.NewService(newTestRegistry(t, driver, prober), prober)
	password, _ := secret.NewPassword("root-password")

	report, err := service.Verify(t.Context(), app.VerifyRequest{
		Target:       device.Target{Host: "router.local", WebPort: 80, SSHPort: 22, TelnetPort: 23},
		RootPassword: password,
		Transport:    "ssh",
	}, nil)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if !report.SSHVerified || report.TelnetRootVerified || report.Status != app.StatusSucceeded {
		t.Fatalf("Verify() report = %+v", report)
	}
	if prober.calls != 1 || prober.transport != "ssh" {
		t.Fatalf("VerifyRoot calls = %d transport = %q", prober.calls, prober.transport)
	}
}

func TestVerifyTelnetDoesNotProveSSH(t *testing.T) {
	t.Parallel()

	driver := fakeDriver{profile: device.Profile{ID: "test", Capabilities: []device.Capability{device.CapabilityVerify}}, match: device.MatchResult{Level: device.ExactMatch}}
	prober := &rootProber{result: app.RootVerification{Transport: "telnet", RootVerified: true}}
	service, _ := app.NewService(newTestRegistry(t, driver, prober), prober)
	password, _ := secret.NewPassword("root-password")

	report, err := service.Verify(t.Context(), app.VerifyRequest{
		Target:       device.Target{Host: "router.local", WebPort: 80, SSHPort: 22, TelnetPort: 23},
		RootPassword: password,
		Transport:    "telnet",
		AllowTelnet:  true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.SSHVerified || !report.TelnetRootVerified {
		t.Fatalf("Verify() report = %+v", report)
	}
}

func TestDoctorWithRootVerify(t *testing.T) {
	t.Parallel()

	driver := fakeDriver{profile: device.Profile{ID: "test", Capabilities: []device.Capability{device.CapabilityVerify}}, match: device.MatchResult{Level: device.ExactMatch}}
	prober := &rootProber{result: app.RootVerification{Transport: "ssh", RootVerified: true, BootID: "00000000-0000-0000-0000-000000000001"}}
	service, _ := app.NewService(newTestRegistry(t, driver, prober), prober)
	password, _ := secret.NewPassword("root")
	report, err := service.Doctor(t.Context(), app.DoctorRequest{
		Target: device.DefaultTarget(), WithRootVerify: true, RootPassword: password, Transport: "ssh",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.RootVerify != app.CheckVerified || !report.SSHVerified || report.TelnetRootVerified {
		t.Fatalf("doctor report = %+v", report)
	}
}

func TestVerifyRejectsPasswordBeforeNetworkAccess(t *testing.T) {
	t.Parallel()

	prober := &rootProber{}
	service, _ := app.NewService(newTestRegistry(t, fakeDriver{profile: device.Profile{ID: "test"}}, prober), prober)
	_, err := service.Verify(t.Context(), app.VerifyRequest{
		Target: device.Target{Host: "router.local", WebPort: 80, SSHPort: 22, TelnetPort: 23},
	}, nil)
	assertCode(t, err, device.CodeInvalidPassword)
	if prober.calls != 0 || prober.fakeProber.calls != 0 {
		t.Fatalf("network calls = %d/%d", prober.fakeProber.calls, prober.calls)
	}
}
