package app_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/backup"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
	"github.com/sunerpy/router-tools/internal/secret"
)

type onlineBackupProber struct {
	fakeProber
	err error
}

func (p *onlineBackupProber) VerifyBackupOnline(context.Context, device.Target, secret.Password, hostkey.Policy, backup.Manifest, device.Observation, string) error {
	return p.err
}

func TestBackupVerifyOnlineFailureIsRedacted(t *testing.T) {
	t.Parallel()

	driver := fakeDriver{
		profile: device.Profile{ID: "test", Hardware: "RD08", Capabilities: []device.Capability{device.CapabilityBackup}},
		match:   device.MatchResult{Level: device.ExactMatch},
	}
	verifierErr := &device.Error{Code: device.CodeBackupLayoutMismatch, Message: "layout differs after login with root-secret"}
	prober := &onlineBackupProber{
		fakeProber: fakeProber{result: device.ProbeResult{Observation: device.Observation{Hardware: "RD08", ROMVersion: "1.1.96"}}},
		err:        verifierErr,
	}
	service, _ := app.NewService(newTestRegistry(t, driver, prober), prober)
	password, _ := secret.NewPassword("root-secret")

	report, err := service.BackupVerifyOnline(t.Context(), app.BackupOnlineVerifyRequest{
		BackupCreateRequest: app.BackupCreateRequest{Target: device.DefaultTarget(), RootPassword: password},
		BackupDir:           makeDeepBackupFixture(t, "test"),
	}, nil)
	assertCode(t, err, device.CodeBackupLayoutMismatch)
	if report.Status != app.StatusFailed || report.Error == nil || report.Error.Code != device.CodeBackupLayoutMismatch {
		t.Fatalf("report = %+v", report)
	}
	encoded, _ := json.Marshal(report)
	for _, output := range []string{err.Error(), string(encoded)} {
		if strings.Contains(output, "root-secret") {
			t.Fatalf("output contains the root password: %s", output)
		}
	}
	if verifierErr.Message != "layout differs after login with root-secret" {
		t.Fatalf("adapter error was mutated in place: %q", verifierErr.Message)
	}
}

type rebootRedactionProber struct {
	fakeProber
	err error
}

func (p *rebootRedactionProber) RebootVerify(context.Context, device.Target, secret.Password, hostkey.Policy, bool) (app.RebootResult, error) {
	return app.RebootResult{}, p.err
}

type backupCreateRedactionProber struct {
	fakeProber
	err error
}

func (p *backupCreateRedactionProber) CreateBackup(context.Context, device.Target, secret.Password, hostkey.Policy, backup.CreateOptions) (backup.Manifest, error) {
	return backup.Manifest{}, p.err
}

func TestRootPasswordFailuresAreRedacted(t *testing.T) {
	t.Parallel()

	const message = "remote output echoed root-secret"
	observation := device.ProbeResult{Observation: device.Observation{Hardware: "RD08", ROMVersion: "1.1.96"}}
	cases := map[string]func(t *testing.T, adapterErr *device.Error) (any, error){
		"reboot-verify": func(t *testing.T, adapterErr *device.Error) (any, error) {
			adapterProber := &rebootRedactionProber{fakeProber: fakeProber{result: observation}, err: adapterErr}
			service, _ := app.NewService(newTestRegistry(t, fakeDriver{
				profile: device.Profile{ID: "test", Capabilities: []device.Capability{device.CapabilitySoftPersist}},
				match:   device.MatchResult{Level: device.ExactMatch},
			}, adapterProber), adapterProber)
			password, _ := secret.NewPassword("root-secret")
			return service.RebootVerify(t.Context(), app.SoftRequest{Target: device.DefaultTarget(), RootPassword: password, Confirmed: true}, nil)
		},
		"backup-create": func(t *testing.T, adapterErr *device.Error) (any, error) {
			adapterProber := &backupCreateRedactionProber{fakeProber: fakeProber{result: observation}, err: adapterErr}
			service, _ := app.NewService(newTestRegistry(t, fakeDriver{
				profile: device.Profile{ID: "test", Capabilities: []device.Capability{device.CapabilityBackup}},
				match:   device.MatchResult{Level: device.ExactMatch},
			}, adapterProber), adapterProber)
			password, _ := secret.NewPassword("root-secret")
			return service.BackupCreate(t.Context(), app.BackupCreateRequest{Target: device.DefaultTarget(), RootPassword: password, OutputDir: t.TempDir() + "/new"}, nil)
		},
	}
	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			adapterErr := &device.Error{Code: device.CodeRemoteCommandFailed, Message: message}
			report, err := run(t, adapterErr)
			assertCode(t, err, device.CodeRemoteCommandFailed)
			encoded, _ := json.Marshal(report)
			for _, output := range []string{err.Error(), string(encoded)} {
				if strings.Contains(output, "root-secret") {
					t.Fatalf("output contains the root password: %s", output)
				}
			}
			if adapterErr.Message != message {
				t.Fatalf("adapter error was mutated in place: %q", adapterErr.Message)
			}
		})
	}
}
