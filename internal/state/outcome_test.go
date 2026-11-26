package state_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/sunerpy/router-tools/internal/state"
)

const key = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestFileStoreRoundTripAndPermissions(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "state")
	store := state.FileStore{Dir: dir}
	if _, ok, err := store.Load(key); err != nil || ok {
		t.Fatalf("Load(missing) = %v, %v; want not found", ok, err)
	}
	started := time.Date(2026, 11, 26, 1, 2, 3, 0, time.UTC)
	outcome := state.Outcome{
		SchemaVersion: 1, Operation: "enable", Status: "in_progress", DeviceWritePossible: true,
		Host: "192.168.31.1", StartedAt: started,
	}
	if err := store.Save(key, outcome); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, ok, err := store.Load(key)
	if err != nil || !ok || got.Operation != "enable" || got.Status != "in_progress" || !got.DeviceWritePossible || !got.StartedAt.Equal(started) {
		t.Fatalf("Load() = %+v, %v, %v", got, ok, err)
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(filepath.Join(dir, key+".json"))
		dirInfo, _ := os.Stat(dir)
		if info.Mode().Perm() != 0o600 || dirInfo.Mode().Perm() != 0o700 {
			t.Fatalf("modes file=%o dir=%o, want 0600/0700", info.Mode().Perm(), dirInfo.Mode().Perm())
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("state dir holds %d entries, want only the record (no temp files)", len(entries))
	}
}

func TestFileStoreRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := state.FileStore{Dir: dir}
	if err := store.Save("../escape", state.Outcome{SchemaVersion: 1}); err == nil {
		t.Fatal("Save() accepted a key that is not a recovery key")
	}
	write := func(body string) {
		if err := os.WriteFile(filepath.Join(dir, key+".json"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range map[string]string{
		"unknown field":  `{"schema_version":1,"operation":"enable","status":"failed","password":"x"}`,
		"wrong schema":   `{"schema_version":2,"operation":"enable","status":"failed"}`,
		"trailing data":  `{"schema_version":1,"operation":"enable","status":"failed"} {}`,
		"unknown status": `{"schema_version":1,"operation":"enable","status":"maybe"}`,
		"oversized":      `{"schema_version":1,"operation":"` + strings.Repeat("x", 70000) + `","status":"failed"}`,
		"extra closer":   `{"schema_version":1,"operation":"enable","status":"failed","started_at":"2026-11-26T01:00:00Z"}}`,
	} {
		write(body)
		if _, _, err := store.Load(key); err == nil {
			t.Fatalf("%s: Load() accepted an invalid record", name)
		}
	}
}

func TestOutcomeBlocksUntilAcknowledged(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 11, 26, 1, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		outcome state.Outcome
		blocks  bool
	}{
		{"succeeded", state.Outcome{Status: "succeeded", StartedAt: at, FinishedAt: at}, false},
		{"failed without writes", state.Outcome{Status: "failed", StartedAt: at, FinishedAt: at}, false},
		{"failed after a write", state.Outcome{Status: "failed", DeviceWritePossible: true, StartedAt: at, FinishedAt: at}, true},
		{"partial", state.Outcome{Status: "partial", StartedAt: at, FinishedAt: at}, true},
		{"unknown", state.Outcome{Status: "unknown", StartedAt: at, FinishedAt: at}, true},
		{"in progress", state.Outcome{Status: "in_progress", DeviceWritePossible: true, StartedAt: at}, true},
		{"acknowledged after finishing", state.Outcome{Status: "partial", StartedAt: at, FinishedAt: at, AcknowledgedAt: at.Add(time.Second)}, false},
		{"acknowledged before finishing", state.Outcome{Status: "partial", StartedAt: at, FinishedAt: at.Add(time.Minute), AcknowledgedAt: at.Add(time.Second)}, true},
		{"acknowledged in progress", state.Outcome{Status: "in_progress", StartedAt: at, AcknowledgedAt: at.Add(time.Second)}, false},
	}
	for _, tc := range cases {
		if got := tc.outcome.Blocks(); got != tc.blocks {
			t.Fatalf("%s: Blocks() = %v, want %v", tc.name, got, tc.blocks)
		}
	}
}

// An unfinished, unacknowledged record carries no zero timestamps, so a consumer can test the keys.
func TestOutcomeOmitsUnsetTimes(t *testing.T) {
	t.Parallel()

	data, err := json.Marshal(state.Outcome{Operation: "enable", Status: state.StatusInProgress, StartedAt: time.Date(2026, 11, 26, 3, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"finished_at", "acknowledged_at", "0001-01-01"} {
		if strings.Contains(string(data), key) {
			t.Fatalf("record JSON %s contains %q", data, key)
		}
	}
}

func TestOutcomeEvidenceRoundTrip(t *testing.T) {
	t.Parallel()

	store := state.FileStore{Dir: filepath.Join(t.TempDir(), "state")}
	key := strings.Repeat("ab", 32)
	outcome := state.Outcome{
		Operation: "enable", Status: state.StatusPartial, DeviceWritePossible: true,
		StartedAt: time.Date(2026, 11, 26, 3, 0, 0, 0, time.UTC), FinishedAt: time.Date(2026, 11, 26, 3, 1, 0, 0, time.UTC),
		Evidence: state.WriteEvidence{WriteStarted: true, AcceptedSteps: 2, AttemptedMACs: []string{"0A:11:22:33:44:10"}, UncleanedMACs: []string{"0A:11:22:33:44:10"}},
	}
	if err := store.Save(key, outcome); err != nil {
		t.Fatal(err)
	}
	loaded, ok, err := store.Load(key)
	if err != nil || !ok || loaded.Evidence.AcceptedSteps != 2 || len(loaded.Evidence.UncleanedMACs) != 1 {
		t.Fatalf("Load() = %+v, %v, %v", loaded, ok, err)
	}
	data, _ := json.Marshal(state.Outcome{Operation: "persist soft", Status: state.StatusFailed, StartedAt: outcome.StartedAt})
	if strings.Contains(string(data), "write_evidence") || strings.Contains(string(data), "operation_id") {
		t.Fatalf("a record without evidence serializes %s", data)
	}
}
