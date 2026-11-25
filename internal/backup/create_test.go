package backup_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sunerpy/router-tools/internal/backup"
	"github.com/sunerpy/router-tools/internal/transport/shell"
)

type fakeBackupShell struct {
	mtdText     string
	data        map[string][]byte
	streamCalls int
	shortFirst  bool
	commands    []string
}

func (s *fakeBackupShell) Run(_ context.Context, command string, _ shell.RunOptions) (shell.CommandResult, error) {
	s.commands = append(s.commands, command)
	if command == "cat /proc/mtd" {
		return shell.CommandResult{Stdout: []byte(s.mtdText), Started: true}, nil
	}
	if strings.HasPrefix(command, "test -r ") {
		node := strings.TrimPrefix(command, "test -r ")
		if _, ok := s.data[node]; ok {
			return shell.CommandResult{Started: true}, nil
		}
		result := shell.CommandResult{Started: true, ExitCode: 1}
		return result, &shell.RemoteCommandError{Result: result}
	}
	result := shell.CommandResult{Started: true, ExitCode: 1}
	return result, &shell.RemoteCommandError{Result: result}
}
func (s *fakeBackupShell) Close() error { return nil }
func (s *fakeBackupShell) Stream(_ context.Context, command string, destination io.Writer, _ int64) (shell.CommandResult, error) {
	s.commands = append(s.commands, command)
	s.streamCalls++
	node := strings.TrimPrefix(command, "cat ")
	data := s.data[node]
	if s.shortFirst && s.streamCalls == 1 {
		data = data[:len(data)-1]
	}
	_, _ = destination.Write(data)
	return shell.CommandResult{Started: true}, nil
}

func TestCreateAndVerifyBackup(t *testing.T) {
	t.Parallel()

	remote := &fakeBackupShell{
		mtdText: "dev:    size   erasesize  name\nmtd0: 00000004 00020000 \"0:BOOT\"\nmtd1: 00000002 00020000 \"crash\"\n",
		data: map[string][]byte{
			"/dev/mtd0": {0, 255, 1, 2},
			"/dev/mtd1": {3, 4},
		},
		shortFirst: true,
	}
	output := filepath.Join(t.TempDir(), "new-backup")
	manifest, err := backup.Create(t.Context(), remote, backup.CreateOptions{
		OutputDir:   output,
		ToolVersion: "test",
		Device:      backup.DeviceIdentity{DeviceID: "DEVICE", Hardware: "RD08", ROMVersion: "1.1.96", Profile: "xiaomi-rd08-1.1.96"},
		BootID:      "00000000-0000-0000-0000-000000000001",
		Now:         func() time.Time { return time.Unix(1700000000, 0).UTC() },
		FreeSpace:   func(string) (uint64, error) { return 1 << 20, nil },
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if !manifest.Complete || len(manifest.Partitions) != 2 || remote.streamCalls != 3 {
		t.Fatalf("manifest = %+v, stream calls = %d", manifest, remote.streamCalls)
	}
	if err := backup.Verify(output); err != nil {
		t.Fatalf("Verify(Create()) error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(output, "partitions", "mtd00_0_BOOT.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string([]byte{0, 255, 1, 2}) {
		t.Fatalf("binary data changed: %v", data)
	}
	for _, command := range remote.commands {
		if strings.Contains(command, "mtd write") || strings.Contains(command, "mtd erase") {
			t.Fatalf("backup wrote MTD: %q", command)
		}
	}
}

func TestCreateChecksSpaceBeforePartitionReads(t *testing.T) {
	t.Parallel()

	remote := &fakeBackupShell{
		mtdText: "dev: size erasesize name\nmtd0: 00000100 00020000 \"boot\"\n",
		data:    map[string][]byte{"/dev/mtd0": make([]byte, 256)},
	}
	_, err := backup.Create(t.Context(), remote, backup.CreateOptions{
		OutputDir: filepath.Join(t.TempDir(), "backup"),
		FreeSpace: func(string) (uint64, error) { return 10, nil },
	})
	if err == nil {
		t.Fatal("Create() space error = nil")
	}
	if remote.streamCalls != 0 {
		t.Fatalf("stream calls = %d, want 0", remote.streamCalls)
	}
}

func TestCreateShortReadLeavesNoFormalManifest(t *testing.T) {
	t.Parallel()

	remote := &fakeBackupShell{
		mtdText: "dev: size erasesize name\nmtd0: 00000004 00020000 \"boot\"\n",
		data:    map[string][]byte{"/dev/mtd0": {1, 2}},
	}
	output := filepath.Join(t.TempDir(), "backup")
	_, err := backup.Create(t.Context(), remote, backup.CreateOptions{OutputDir: output, FreeSpace: func(string) (uint64, error) { return 100, nil }})
	if err == nil {
		t.Fatal("Create() short-read error = nil")
	}
	if _, statErr := os.Stat(filepath.Join(output, "manifest.json")); !os.IsNotExist(statErr) {
		t.Fatalf("formal manifest exists after failure: %v", statErr)
	}
	matches, globErr := filepath.Glob(filepath.Join(output, "partitions", "*.partial"))
	if globErr != nil || len(matches) == 0 {
		t.Fatalf("partial evidence missing: %v %v", matches, globErr)
	}
}
