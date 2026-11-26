package app_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/backup"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/secret"
)

// rd08Deep mirrors the RD08 1.1.96 profile's deep policy.
var rd08Deep = &device.DeepPolicy{Acknowledgement: "RD08-DEEP", RebootTransitions: 3}

type deepAppProber struct {
	fakeProber
	result app.DeepResult
	err    error
	params app.DeepParams
	calls  int
}

func (p *deepAppProber) DeepPersist(_ context.Context, params app.DeepParams) (app.DeepResult, error) {
	p.calls++
	p.params = params
	return p.result, p.err
}

func TestDeepRequiresDedicatedAcknowledgement(t *testing.T) {
	t.Parallel()

	prober := &deepAppProber{}
	deep := fakeDriver{profile: device.Profile{ID: "xiaomi-rd08-1.1.96", Capabilities: []device.Capability{device.CapabilityDeepPersist}, Deep: rd08Deep}}
	service, _ := app.NewService(newTestRegistry(t, deep, prober), prober)
	password, _ := secret.NewPassword("root")
	_, err := service.DeepPersist(t.Context(), app.DeepRequest{
		Target: device.DefaultTarget(), RootPassword: password, AllowTelnet: true, AckDeep: "yes",
	}, nil)
	assertCode(t, err, device.CodeDeepAckRequired)
	if prober.calls != 0 || prober.fakeProber.calls != 0 {
		t.Fatal("deep operation accessed network without RD08-DEEP")
	}

	// The registered phrase passes the offline check: the next gate (no backup) stops it, still offline.
	_, err = service.DeepPersist(t.Context(), app.DeepRequest{
		Target: device.DefaultTarget(), RootPassword: password, AllowTelnet: true, AckDeep: "RD08-DEEP",
	}, nil)
	assertCode(t, err, device.CodeBackupMissing)
	if prober.calls != 0 || prober.fakeProber.calls != 0 {
		t.Fatal("deep operation accessed network before its backup gate")
	}
}

func TestDeepSucceedsWithVerifiedBackupAndJournal(t *testing.T) {
	t.Parallel()

	const profileID = "xiaomi-rd08-1.1.96"
	backupDir := makeDeepBackupFixture(t, profileID)
	driver := fakeDriver{profile: device.Profile{ID: profileID, Capabilities: []device.Capability{device.CapabilityDeepPersist}, Deep: rd08Deep}, match: device.MatchResult{Level: device.ExactMatch}}
	prober := &deepAppProber{
		fakeProber: fakeProber{result: device.ProbeResult{Observation: device.Observation{DeviceID: "DEVICE", Hardware: "RD08", ROMVersion: "1.1.96"}}},
		result: app.DeepResult{
			Status:           app.StatusSucceeded,
			FinalSSHVerified: true,
			SoftVerified:     true,
			Transitions: []app.BootTransition{
				{OldBootID: "1", NewBootID: "2"}, {OldBootID: "2", NewBootID: "3"}, {OldBootID: "3", NewBootID: "4"},
			},
		},
	}
	service, _ := app.NewService(newTestRegistry(t, driver, prober), prober)
	password, _ := secret.NewPassword("root")
	report, err := service.DeepPersist(t.Context(), app.DeepRequest{
		Target: device.DefaultTarget(), RootPassword: password, AllowTelnet: true,
		AckDeep: "RD08-DEEP", BackupDir: backupDir, RunDir: t.TempDir(),
	}, nil)
	if err != nil {
		t.Fatalf("DeepPersist() error = %v", err)
	}
	if report.Status != app.StatusSucceeded || !report.Result.FinalSSHVerified || prober.calls != 1 {
		t.Fatalf("report = %+v", report)
	}
	if prober.params.Manifest.Device.Profile != profileID || prober.params.Observation.Hardware != "RD08" {
		t.Fatalf("adapter params = %+v", prober.params)
	}
	if _, err := os.Stat(report.JournalPath); err != nil {
		t.Fatalf("journal missing: %v", err)
	}
}

func TestDeepRejectsExistingJournalBeforeAdapter(t *testing.T) {
	t.Parallel()

	const profileID = "xiaomi-rd08-1.1.96"
	backupDir := makeDeepBackupFixture(t, profileID)
	driver := fakeDriver{profile: device.Profile{ID: profileID, Capabilities: []device.Capability{device.CapabilityDeepPersist}, Deep: rd08Deep}, match: device.MatchResult{Level: device.ExactMatch}}
	prober := &deepAppProber{fakeProber: fakeProber{result: device.ProbeResult{Observation: device.Observation{DeviceID: "DEVICE", Hardware: "RD08", ROMVersion: "1.1.96"}}}}
	service, _ := app.NewService(newTestRegistry(t, driver, prober), prober)
	password, _ := secret.NewPassword("root")
	runDir := t.TempDir()
	journalPath := filepath.Join(runDir, "192.168.31.1-deep.json")
	if err := os.WriteFile(journalPath, []byte("unfinished"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := service.DeepPersist(t.Context(), app.DeepRequest{
		Target: device.DefaultTarget(), RootPassword: password, AllowTelnet: true,
		AckDeep: "RD08-DEEP", BackupDir: backupDir, RunDir: runDir,
	}, nil)
	assertCode(t, err, device.CodeDeepJournalExists)
	if prober.calls != 0 {
		t.Fatal("deep adapter ran despite existing journal")
	}
}

func makeDeepBackupFixture(t *testing.T, profileID string) string {
	t.Helper()
	directory := t.TempDir()
	writeBackupAt(t, directory, backup.DeviceIdentity{DeviceID: "DEVICE", Hardware: "RD08", ROMVersion: "1.1.96", Profile: profileID})
	return directory
}

// writeBackupAt writes a complete backup that passes backup.Verify into directory, creating it when needed.
func writeBackupAt(t *testing.T, directory string, identity backup.DeviceIdentity) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(directory, "partitions"), 0o700); err != nil {
		t.Fatal(err)
	}
	mtdText := "dev: size erasesize name\nmtd0: 00000004 00020000 \"boot\"\n"
	partitionData := []byte{0, 1, 2, 3}
	files := map[string][]byte{
		"mtd.txt":                   []byte(mtdText),
		"partitions/mtd00_boot.bin": partitionData,
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(directory, filepath.FromSlash(name)), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	manifest := backup.Manifest{
		SchemaVersion: 1, ToolVersion: "test", CreatedAt: time.Now().UTC(), Complete: true,
		Device: identity,
		BootID: "00000000-0000-0000-0000-000000000001", MTDTextFile: "mtd.txt",
		Partitions: []backup.Partition{{Index: 0, Name: "boot", Size: 4, EraseSize: 0x20000, Source: "/dev/mtd0", File: "partitions/mtd00_boot.bin", SHA256: fmt.Sprintf("%x", sha256.Sum256(partitionData))}},
	}
	manifestData, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(directory, "manifest.json"), manifestData, 0o600); err != nil {
		t.Fatal(err)
	}
	files["manifest.json"] = manifestData
	var paths []string
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	var sums strings.Builder
	for _, path := range paths {
		fmt.Fprintf(&sums, "%x  %s\n", sha256.Sum256(files[path]), path)
	}
	if err := os.WriteFile(filepath.Join(directory, "SHA256SUMS"), []byte(sums.String()), 0o600); err != nil {
		t.Fatal(err)
	}
}
