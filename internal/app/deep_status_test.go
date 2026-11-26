package app_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/backup"
	"github.com/sunerpy/router-tools/internal/credentials"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
	"github.com/sunerpy/router-tools/internal/secret"
	"github.com/sunerpy/router-tools/internal/state"
)

// inspectingOps adds the read-only deep inspection to every capability fake.
type inspectingOps struct {
	*allOps
	inspection app.DeepInspection
	err        error
	passwords  []string
}

func (o *inspectingOps) InspectDeep(_ context.Context, _ device.Target, password secret.Password, _ hostkey.Policy) (app.DeepInspection, error) {
	o.calls["InspectDeep"]++
	o.passwords = append(o.passwords, password.Expose())
	return o.inspection, o.err
}

type deepStatusFixture struct {
	service     *app.Service
	ops         *inspectingOps
	outcomes    *memoryOutcomes
	vault       *credentials.Memory
	backupRoot  string
	journalRoot string
	target      device.Target
	discovery   *fakeProber
}

func newDeepStatusFixture(t *testing.T) *deepStatusFixture {
	t.Helper()
	ops := &inspectingOps{allOps: newAllOps(), inspection: app.DeepInspection{Flags: app.DeepFlagsNone, CrashPartition: true, CrashReadable: true}}
	registry, err := device.NewRegistry(device.Adapter{Driver: modelDriver{p: fullProfile("1.0")}, Operations: ops})
	if err != nil {
		t.Fatal(err)
	}
	f := &deepStatusFixture{
		ops: ops, outcomes: newMemoryOutcomes(), vault: credentials.NewMemory(),
		backupRoot: t.TempDir(), journalRoot: t.TempDir(), target: device.DefaultTarget(),
	}
	discovery := &fakeProber{result: device.ProbeResult{
		Observation: device.Observation{DeviceID: "DEVICE", Hardware: "R1", ROMVersion: "1.0"}, SSH: device.PortOpen,
	}}
	f.discovery = discovery
	service, err := app.NewServiceWithOptions(registry, discovery, app.Options{
		Outcomes: f.outcomes, Credentials: f.vault, BackupRoot: f.backupRoot, JournalRoot: f.journalRoot,
	})
	if err != nil {
		t.Fatal(err)
	}
	f.service = service
	return f
}

func (f *deepStatusFixture) managedDir(root string) string {
	return filepath.Join(root, "acme-r1", "192.168.31.1-"+device.RecoveryKey(f.target)[:12])
}

// The deep page learns the resolved profile's policy, the managed journal and backups, and the read-only
// device state, without any write.
func TestDeepStatusReportsTheManagedPathsAndTheReadOnlyState(t *testing.T) {
	t.Parallel()

	f := newDeepStatusFixture(t)
	backupDir := filepath.Join(f.managedDir(f.backupRoot), "20261126T010000Z")
	writeBackupAt(t, backupDir, backup.DeviceIdentity{DeviceID: "DEVICE", Hardware: "R1", ROMVersion: "1.0", Profile: "acme-r1-1.0"})
	report, err := f.service.DeepStatus(t.Context(), app.DeepStatusRequest{Target: f.target, RootPassword: mustPassword(t, "root")}, nil)
	if err != nil {
		t.Fatalf("DeepStatus() error = %v", err)
	}
	if report.Profile.Deep == nil || report.Profile.Deep.Acknowledgement != "ACME-DEEP" || report.Profile.Deep.RebootTransitions != 1 {
		t.Fatalf("profile = %+v, want the resolved profile's deep policy", report.Profile)
	}
	wantJournal := filepath.Join(f.managedDir(f.journalRoot), "192.168.31.1-deep.json")
	if report.JournalPath != wantJournal || report.JournalExists {
		t.Fatalf("journal %q exists %v, want %q absent", report.JournalPath, report.JournalExists, wantJournal)
	}
	if len(report.Backups) != 1 || report.Backups[0].Path != backupDir {
		t.Fatalf("backups = %+v, want the managed backup", report.Backups)
	}
	if report.RootSSH != app.RootSSHVerified || report.Inspection == nil || report.Inspection.Flags != app.DeepFlagsNone || !report.Ready {
		t.Fatalf("root %q inspection %+v ready %v", report.RootSSH, report.Inspection, report.Ready)
	}
	for _, method := range []string{"DeepPersist", "EnableSSH", "InstallSoft", "RemoveSoft", "RebootVerify", "CreateBackup"} {
		if f.ops.calls[method] != 0 {
			t.Fatalf("DeepStatus called the write method %s", method)
		}
	}
	if f.outcomes.saves != 0 {
		t.Fatal("DeepStatus wrote a recovery record")
	}
}

func TestDeepStatusNamesWhatBlocksAnotherRun(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		setup  func(*testing.T, *deepStatusFixture)
		reason string
	}{
		"existing journal": {func(t *testing.T, f *deepStatusFixture) {
			dir := f.managedDir(f.journalRoot)
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "192.168.31.1-deep.json"), []byte("{}"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, app.DeepBlockedJournal},
		"flags already set": {func(_ *testing.T, f *deepStatusFixture) {
			f.ops.inspection.Flags = app.DeepFlagsAllSet
		}, app.DeepBlockedAlreadySet},
		"mixed flags": {func(_ *testing.T, f *deepStatusFixture) {
			f.ops.inspection.Flags = app.DeepFlagsMixed
		}, app.DeepBlockedMixedFlags},
		"crash partition missing": {func(_ *testing.T, f *deepStatusFixture) {
			f.ops.inspection.CrashPartition, f.ops.inspection.CrashReadable = false, false
		}, app.DeepBlockedCrashPartition},
		"no verified backup": {func(*testing.T, *deepStatusFixture) {}, app.DeepBlockedNoBackup},
		"unacknowledged change": {func(_ *testing.T, f *deepStatusFixture) {
			_ = f.outcomes.Save(device.RecoveryKey(f.target), state.Outcome{Operation: "enable", Status: state.StatusUnknown, DeviceWritePossible: true, StartedAt: time.Now()})
		}, app.DeepBlockedRecovery},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newDeepStatusFixture(t)
			if name != "no verified backup" {
				writeBackupAt(t, filepath.Join(f.managedDir(f.backupRoot), "20261126T010000Z"),
					backup.DeviceIdentity{DeviceID: "DEVICE", Hardware: "R1", ROMVersion: "1.0", Profile: "acme-r1-1.0"})
			}
			tc.setup(t, f)
			report, err := f.service.DeepStatus(t.Context(), app.DeepStatusRequest{Target: f.target, RootPassword: mustPassword(t, "root")}, nil)
			if err != nil {
				t.Fatalf("DeepStatus() error = %v", err)
			}
			if report.Ready || !containsReason(report.Blocked, tc.reason) {
				t.Fatalf("ready %v blocked %v, want %s", report.Ready, report.Blocked, tc.reason)
			}
		})
	}
}

func TestDeepStatusWithoutARootPasswordSkipsInspection(t *testing.T) {
	t.Parallel()

	f := newDeepStatusFixture(t)
	report, err := f.service.DeepStatus(t.Context(), app.DeepStatusRequest{Target: f.target}, nil)
	if err != nil || report.RootSSH != app.RootSSHNotChecked || report.Inspection != nil || f.ops.calls["InspectDeep"] != 0 {
		t.Fatalf("DeepStatus() = %+v, %v", report, err)
	}

	// A saved root password may be used for this read-only SSH check.
	_ = f.vault.Put(credentials.NewKey(credentials.KindRoot, "acme-r1", "192.168.31.1", 22), mustPassword(t, "saved-root"), time.Now())
	report, err = f.service.DeepStatus(t.Context(), app.DeepStatusRequest{Target: f.target, Credentials: app.CredentialUse{UseSaved: true}}, nil)
	if err != nil || report.RootSSH != app.RootSSHVerified || f.ops.passwords[0] != "saved-root" {
		t.Fatalf("DeepStatus() = root %q, %v; inspector got %v", report.RootSSH, err, f.ops.passwords)
	}
}

func TestDeepStatusReportsAnUnsupportedProfileWithCandidates(t *testing.T) {
	t.Parallel()

	f := newDeepStatusFixture(t)
	_, err := f.service.DeepStatus(t.Context(), app.DeepStatusRequest{Target: f.target, ProfileID: "acme-r1-9.9"}, nil)
	assertCode(t, err, device.CodeUnsupportedProfile)
}

func containsReason(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

// The page tells a closed SSH port apart from a missing or refused root password.
func TestDeepStatusReportsTheSSHPortWithoutInspectingAClosedOne(t *testing.T) {
	t.Parallel()

	f := newDeepStatusFixture(t)
	f.discovery.result.SSH = device.PortClosed
	report, err := f.service.DeepStatus(t.Context(), app.DeepStatusRequest{Target: f.target, RootPassword: mustPassword(t, "root")}, nil)
	if err != nil || report.SSH != device.PortClosed || report.RootSSH != app.RootSSHNotChecked || report.Inspection != nil || report.Ready {
		t.Fatalf("DeepStatus() = ssh %q root %q inspection %+v ready %v, %v", report.SSH, report.RootSSH, report.Inspection, report.Ready, err)
	}
	if f.ops.calls["VerifyRoot"] != 0 || f.ops.calls["InspectDeep"] != 0 {
		t.Fatalf("DeepStatus used SSH on a closed port: %v", f.ops.calls)
	}
}
