package app_test

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/backup"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
	"github.com/sunerpy/router-tools/internal/secret"
)

// recordingBackups records the options the adapter receives.
type recordingBackups struct {
	*allOps
	options backup.CreateOptions
}

func (r *recordingBackups) CreateBackup(ctx context.Context, t device.Target, p secret.Password, h hostkey.Policy, o backup.CreateOptions) (backup.Manifest, error) {
	r.options = o
	return r.allOps.CreateBackup(ctx, t, p, h, o)
}

// displayDriver matches like the RD08 driver: hardware first, display name only when hardware is empty,
// firmware compared after trimming surrounding spaces.
type displayDriver struct{ p device.Profile }

func (d displayDriver) Profile() device.Profile { return d.p }
func (d displayDriver) Match(o device.Observation) device.MatchResult {
	known := strings.EqualFold(o.Hardware, d.p.Hardware) || (o.Hardware == "" && strings.Contains(o.DisplayName, d.p.Model))
	switch {
	case !known:
		return device.MatchResult{Level: device.NoMatch}
	case strings.TrimSpace(o.ROMVersion) != d.p.Firmware:
		return device.MatchResult{Level: device.DeviceMatch}
	default:
		return device.MatchResult{Level: device.ExactMatch}
	}
}

func managedService(t *testing.T, ops any, options app.Options) *app.Service {
	t.Helper()
	registry, err := device.NewRegistry(device.Adapter{Driver: displayDriver{p: fullProfile("1.0")}, Operations: ops})
	if err != nil {
		t.Fatal(err)
	}
	// No hardware string: the device matched through its display name, and the firmware has spaces.
	discovery := &fakeProber{result: device.ProbeResult{Observation: device.Observation{DeviceID: "DEVICE", DisplayName: "Acme R1", ROMVersion: " 1.0 "}}}
	service, err := app.NewServiceWithOptions(registry, discovery, options)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func TestBackupCreateUsesTheManagedDirectoryWhenNoneIsGiven(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ops := &recordingBackups{allOps: newAllOps()}
	service := managedService(t, ops, app.Options{BackupRoot: root})
	password, _ := secret.NewPassword("root")
	report, _ := service.BackupCreate(t.Context(), app.BackupCreateRequest{Target: device.DefaultTarget(), RootPassword: password}, nil)

	dir := ops.options.OutputDir
	prefix := filepath.Join(root, "acme-r1", "192.168.31.1-"+device.RecoveryKey(device.DefaultTarget())[:12]) + string(filepath.Separator)
	if !strings.HasPrefix(dir, prefix) || !regexp.MustCompile(`^\d{8}T\d{6}Z$`).MatchString(filepath.Base(dir)) {
		t.Fatalf("managed backup dir = %q, want %s<UTC stamp>", dir, prefix)
	}
	if report.OutputDir != dir {
		t.Fatalf("report output dir = %q, want %q", report.OutputDir, dir)
	}
	if ops.options.Device.ROMVersion != "1.0" || ops.options.Device.Hardware != "R1" || ops.options.Device.Profile != "acme-r1-1.0" {
		t.Fatalf("manifest identity = %+v, want the trimmed backup identity", ops.options.Device)
	}

	unmanaged := managedService(t, &recordingBackups{allOps: newAllOps()}, app.Options{})
	_, err := unmanaged.BackupCreate(t.Context(), app.BackupCreateRequest{Target: device.DefaultTarget(), RootPassword: password}, nil)
	assertCode(t, err, device.CodeInvalidArgument)
}

// Without a run directory, deep persistence uses one managed journal directory per router, so changing a
// port or retrying with an empty directory still meets the existing journal.
func TestDeepUsesTheManagedJournalDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ops := newAllOps()
	service := managedService(t, ops, app.Options{JournalRoot: root})
	password, _ := secret.NewPassword("root")
	request := app.DeepRequest{
		Target: device.DefaultTarget(), RootPassword: password, AllowTelnet: true, AckDeep: "ACME-DEEP",
		BackupDir: makeDeepBackupFixture(t, "acme-r1-1.0"),
	}
	report, _ := service.DeepPersist(t.Context(), request, nil)
	want := filepath.Join(root, "acme-r1", "192.168.31.1-"+device.RecoveryKey(device.DefaultTarget())[:12], "192.168.31.1-deep.json")
	if report.JournalPath != want {
		t.Fatalf("journal = %q, want %q", report.JournalPath, want)
	}

	request.Target.WebPort, request.Target.SSHPort = 8080, 2222
	_, err := service.DeepPersist(t.Context(), request, nil)
	assertCode(t, err, device.CodeDeepJournalExists)
	if ops.calls["DeepPersist"] != 1 {
		t.Fatalf("deep adapter calls = %d, want exactly the first run", ops.calls["DeepPersist"])
	}
}

// The managed directories and the managed journal file name follow the canonical host, so another spelling
// of the same router (host-name case, an IPv6 form) meets the same journal and backups.
func TestManagedPathsFollowTheCanonicalHost(t *testing.T) {
	t.Parallel()

	cases := map[string][2]string{
		"host-name case": {"ROUTER.lan", "router.LAN"},
		"IPv6 spelling":  {"2001:DB8:0:0:0:0:0:1", "2001:db8::1"},
	}
	for name, hosts := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			ops := newAllOps()
			service := managedService(t, ops, app.Options{JournalRoot: root})
			password, _ := secret.NewPassword("root")
			request := app.DeepRequest{
				Target: device.Target{Host: hosts[0], WebPort: 80, SSHPort: 22, TelnetPort: 23}, RootPassword: password,
				AllowTelnet: true, AckDeep: "ACME-DEEP", BackupDir: makeDeepBackupFixture(t, "acme-r1-1.0"),
			}
			first, _ := service.DeepPersist(t.Context(), request, nil)
			request.Target.Host = hosts[1]
			second, err := service.DeepPersist(t.Context(), request, nil)
			assertCode(t, err, device.CodeDeepJournalExists)
			if first.JournalPath == "" || second.JournalPath != first.JournalPath || ops.calls["DeepPersist"] != 1 {
				t.Fatalf("journals %q and %q, adapter calls %d; want one journal and one run", first.JournalPath, second.JournalPath, ops.calls["DeepPersist"])
			}
		})
	}
}

// The guided backup scan applies the same identity fallback as backup creation: a device known only by
// its display name matches a manifest that recorded the profile's hardware, and a foreign device's backup
// never counts.
func TestGuidePlanFindsManagedBackupsByTheBackupIdentity(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		identity backup.DeviceIdentity
		action   string
	}{
		"display-name device": {backup.DeviceIdentity{DeviceID: "DEVICE", Hardware: "R1", ROMVersion: "1.0", Profile: "acme-r1-1.0"}, app.ActionSkip},
		"another device":      {backup.DeviceIdentity{DeviceID: "OTHER", Hardware: "R1", ROMVersion: "1.0", Profile: "acme-r1-1.0"}, app.ActionRun},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			target := device.DefaultTarget()
			writeBackupAt(t, filepath.Join(root, "acme-r1", "192.168.31.1-"+device.RecoveryKey(target)[:12], "20261126T010000Z"), tc.identity)
			service := managedService(t, newAllOps(), app.Options{BackupRoot: root})
			plan, err := service.GuidePlan(t.Context(), app.GuideRequest{Target: target, Goals: []string{app.GoalSSH, app.GoalBackup}}, nil)
			if err != nil {
				t.Fatalf("GuidePlan() error = %v", err)
			}
			if plan.Steps[1].ID != app.StepBackup || plan.Steps[1].Action != tc.action {
				t.Fatalf("backup step = %+v, want %s", plan.Steps[1], tc.action)
			}
		})
	}
}
