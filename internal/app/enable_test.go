package app_test

import (
	"context"
	"testing"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
	"github.com/sunerpy/router-tools/internal/secret"
)

type enableProber struct {
	fakeProber
	result app.EnableResult
	err    error
	calls  int
}

func (p *enableProber) EnableSSH(context.Context, device.Target, secret.Password, secret.Password, hostkey.Policy) (app.EnableResult, error) {
	p.calls++
	return p.result, p.err
}

func TestEnableRequiresConfirmationBeforeNetworkAccess(t *testing.T) {
	t.Parallel()

	prober := &enableProber{}
	service, _ := app.NewService(newTestRegistry(t, fakeDriver{profile: device.Profile{ID: "test"}}, prober), prober)
	admin, _ := secret.NewPassword("admin")
	root, _ := secret.NewPassword("root")
	_, err := service.EnableSSH(t.Context(), app.EnableRequest{
		Target: device.DefaultTarget(), AdminPassword: admin, RootPassword: root,
	}, nil)
	assertCode(t, err, device.CodeConfirmationRequired)
	if prober.calls != 0 || prober.fakeProber.calls != 0 {
		t.Fatalf("network calls = %d/%d", prober.fakeProber.calls, prober.calls)
	}
}

func TestEnableReturnsPartialReport(t *testing.T) {
	t.Parallel()

	driver := fakeDriver{profile: device.Profile{ID: "test", Capabilities: []device.Capability{device.CapabilityEnableSSH}}, match: device.MatchResult{Level: device.ExactMatch}}
	operationErr := &device.Error{Code: device.CodeMACCleanupFailed, Message: "cleanup failed"}
	prober := &enableProber{result: app.EnableResult{Status: app.StatusPartial, AcceptedSteps: 3, SSHVerified: true}, err: operationErr}
	service, _ := app.NewService(newTestRegistry(t, driver, prober), prober)
	admin, _ := secret.NewPassword("admin")
	root, _ := secret.NewPassword("root")
	report, err := service.EnableSSH(t.Context(), app.EnableRequest{
		Target: device.DefaultTarget(), AdminPassword: admin, RootPassword: root, Confirmed: true,
	}, nil)
	assertCode(t, err, device.CodeMACCleanupFailed)
	if report.Status != app.StatusPartial || !report.SSHVerified {
		t.Fatalf("report = %+v", report)
	}
}
