package report_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/sunerpy/router-tools/internal/report"
)

func TestJournalStagesAndCompletion(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nested", "deep.json")
	journal, err := report.CreateJournal(path, "router.local", "/backup")
	if err != nil {
		t.Fatal(err)
	}
	for _, stage := range []string{"PRECHECK", "CRASH_WRITE_STARTED", "CRASH_WRITE_ACCEPTED"} {
		if err := journal.Stage(stage); err != nil {
			t.Fatal(err)
		}
	}
	if err := journal.Transition("old", "new"); err != nil {
		t.Fatal(err)
	}
	if err := journal.Stage("SUCCEEDED"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var decoded report.Data
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if !decoded.Complete || decoded.Status != "SUCCEEDED" || len(decoded.Stages) != 4 || len(decoded.Transitions) != 1 {
		t.Fatalf("journal = %+v", decoded)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// Windows has no POSIX permission bits: os.Stat reports 0666 for any writable file.
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("journal mode = %o", info.Mode().Perm())
	}
}

func TestJournalCreateRefusesExistingPath(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "deep.json")
	if _, err := report.CreateJournal(path, "host", "backup"); err != nil {
		t.Fatal(err)
	}
	if _, err := report.CreateJournal(path, "host", "backup"); err == nil {
		t.Fatal("CreateJournal() allowed overwriting an existing journal")
	}
}
