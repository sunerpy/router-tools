package app_test

import (
	"context"
	"testing"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/backup"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
	"github.com/sunerpy/router-tools/internal/secret"
)

type operationProber struct {
	fakeProber
	softResult   app.SoftResult
	rebootResult app.RebootResult
	manifest     backup.Manifest
	err          error
	calls        int
}

func (p *operationProber) InstallSoft(context.Context, device.Target, secret.Password, hostkey.Policy, bool) (app.SoftResult, error) {
	p.calls++
	return p.softResult, p.err
}

func (p *operationProber) RemoveSoft(context.Context, device.Target, secret.Password, hostkey.Policy, bool) (app.SoftResult, error) {
	p.calls++
	return p.softResult, p.err
}

func (p *operationProber) RebootVerify(context.Context, device.Target, secret.Password, hostkey.Policy, bool) (app.RebootResult, error) {
	p.calls++
	return p.rebootResult, p.err
}

func (p *operationProber) CreateBackup(_ context.Context, _ device.Target, _ secret.Password, _ hostkey.Policy, options backup.CreateOptions) (backup.Manifest, error) {
	p.calls++
	return p.manifest, p.err
}

func TestSoftPersistRequiresConfirmation(t *testing.T) {
	t.Parallel()

	registry, _ := device.NewRegistry(fakeDriver{profile: device.Profile{ID: "test"}})
	prober := &operationProber{}
	service, _ := app.NewService(registry, prober)
	password, _ := secret.NewPassword("root")
	_, err := service.PersistSoft(t.Context(), app.SoftRequest{Target: device.DefaultTarget(), RootPassword: password}, nil)
	assertCode(t, err, device.CodeConfirmationRequired)
	if prober.calls != 0 || prober.fakeProber.calls != 0 {
		t.Fatal("soft persist accessed the network before confirmation")
	}
}

func TestRebootVerifyReport(t *testing.T) {
	t.Parallel()

	driver := fakeDriver{profile: device.Profile{ID: "test", Capabilities: []device.Capability{device.CapabilitySoftPersist}}, match: device.MatchResult{Level: device.ExactMatch}}
	registry, _ := device.NewRegistry(driver)
	prober := &operationProber{rebootResult: app.RebootResult{
		OldBootID:    "00000000-0000-0000-0000-000000000001",
		NewBootID:    "00000000-0000-0000-0000-000000000002",
		SSHVerified:  true,
		SoftVerified: true,
	}}
	service, _ := app.NewService(registry, prober)
	password, _ := secret.NewPassword("root")
	report, err := service.RebootVerify(t.Context(), app.SoftRequest{Target: device.DefaultTarget(), RootPassword: password, Confirmed: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != app.StatusSucceeded || !report.Result.SSHVerified || !report.Result.SoftVerified {
		t.Fatalf("report = %+v", report)
	}
}

func TestBackupCreateValidatesPasswordBeforeNetwork(t *testing.T) {
	t.Parallel()

	registry, _ := device.NewRegistry(fakeDriver{profile: device.Profile{ID: "test"}})
	prober := &operationProber{}
	service, _ := app.NewService(registry, prober)
	_, err := service.BackupCreate(t.Context(), app.BackupCreateRequest{Target: device.DefaultTarget(), OutputDir: t.TempDir()}, nil)
	assertCode(t, err, device.CodeInvalidPassword)
	if prober.calls != 0 || prober.fakeProber.calls != 0 {
		t.Fatal("backup accessed the network with an invalid password")
	}
}
