package backup_test

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/sunerpy/router-tools/internal/backup"
	"github.com/sunerpy/router-tools/internal/device"
)

func TestVerifyValidBackup(t *testing.T) {
	t.Parallel()

	directory := makeBackupFixture(t)
	if err := backup.Verify(directory); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
}

func TestVerifyDetectsTampering(t *testing.T) {
	t.Parallel()

	directory := makeBackupFixture(t)
	file := filepath.Join(directory, "partitions", "mtd00_0_BOOT.bin")
	if err := os.WriteFile(file, []byte{1, 2, 3, 4}, 0o600); err != nil {
		t.Fatal(err)
	}
	assertBackupCode(t, backup.Verify(directory), device.CodeBackupHashMismatch)
}

func TestVerifyRejectsUnsafeManifest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*backup.Manifest)
	}{
		{name: "incomplete", mutate: func(m *backup.Manifest) { m.Complete = false }},
		{name: "wrong schema", mutate: func(m *backup.Manifest) { m.SchemaVersion = 2 }},
		{name: "absolute path", mutate: func(m *backup.Manifest) { m.Partitions[0].File = "/etc/passwd" }},
		{name: "parent traversal", mutate: func(m *backup.Manifest) { m.MTDTextFile = "../mtd.txt" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			directory := makeBackupFixture(t)
			manifest := readManifestFixture(t, directory)
			tt.mutate(&manifest)
			writeJSONFixture(t, filepath.Join(directory, "manifest.json"), manifest)
			if err := backup.Verify(directory); err == nil {
				t.Fatal("Verify() error = nil")
			}
		})
	}
}

func TestVerifyRejectsMissingSumsCoverage(t *testing.T) {
	t.Parallel()

	directory := makeBackupFixture(t)
	sums, err := os.ReadFile(filepath.Join(directory, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(sums)), "\n")
	if err := os.WriteFile(filepath.Join(directory, "SHA256SUMS"), []byte(lines[0]+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	assertBackupCode(t, backup.Verify(directory), device.CodeBackupLayoutMismatch)
}

func TestVerifyRejectsSymlink(t *testing.T) {
	t.Parallel()

	directory := makeBackupFixture(t)
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(directory, "partitions", "mtd00_0_BOOT.bin")
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := backup.Verify(directory); err == nil {
		t.Fatal("Verify() accepted a symlink partition")
	}
}

func makeBackupFixture(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	if err := os.Mkdir(filepath.Join(directory, "partitions"), 0o700); err != nil {
		t.Fatal(err)
	}
	mtdText := "dev:    size   erasesize  name\nmtd0: 00000004 00020000 \"0:BOOT\"\n"
	partitionData := []byte{0, 255, 1, 2}
	// Backups record slash-separated paths on every OS, as Create does (verify rejects "\\").
	partitionPath := "partitions/mtd00_0_BOOT.bin"
	writeFixture(t, filepath.Join(directory, filepath.FromSlash(partitionPath)), partitionData)
	writeFixture(t, filepath.Join(directory, "mtd.txt"), []byte(mtdText))
	manifest := backup.Manifest{
		SchemaVersion: 1,
		ToolVersion:   "test",
		CreatedAt:     time.Now().UTC(),
		Complete:      true,
		Device:        backup.DeviceIdentity{DeviceID: "DEVICE", Hardware: "RD08", ROMVersion: "1.1.96", Profile: "xiaomi-rd08-1.1.96"},
		BootID:        "00000000-0000-0000-0000-000000000001",
		MTDTextFile:   "mtd.txt",
		Partitions: []backup.Partition{{
			Index: 0, Name: "0:BOOT", Size: 4, EraseSize: 0x20000,
			Source: "/dev/mtd0", File: partitionPath, SHA256: hashFixture(partitionData),
		}},
	}
	writeJSONFixture(t, filepath.Join(directory, "manifest.json"), manifest)
	files := map[string][]byte{
		partitionPath:   partitionData,
		"mtd.txt":       []byte(mtdText),
		"manifest.json": readFileFixture(t, filepath.Join(directory, "manifest.json")),
	}
	var paths []string
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	var sums strings.Builder
	for _, path := range paths {
		fmt.Fprintf(&sums, "%s  %s\n", hashFixture(files[path]), path)
	}
	writeFixture(t, filepath.Join(directory, "SHA256SUMS"), []byte(sums.String()))
	return directory
}

func readManifestFixture(t *testing.T, directory string) backup.Manifest {
	t.Helper()
	var manifest backup.Manifest
	if err := json.Unmarshal(readFileFixture(t, filepath.Join(directory, "manifest.json")), &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func writeJSONFixture(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, path, data)
}

func writeFixture(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func readFileFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func hashFixture(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }

func assertBackupCode(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want %s", want)
	}
	var deviceErr *device.Error
	if !errors.As(err, &deviceErr) || deviceErr.Code != want {
		t.Fatalf("error = %T %v, want %s", err, err, want)
	}
}
