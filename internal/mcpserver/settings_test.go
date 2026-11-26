package mcpserver_test

import (
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/sunerpy/router-tools/internal/mcpserver"
)

func TestSettingsDefaultWhenMissing(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "mcp")
	settings, err := mcpserver.SettingsStore{Dir: dir}.Load()
	if err != nil || settings != mcpserver.DefaultSettings() {
		t.Fatalf("Load() = %+v, %v; want the defaults", settings, err)
	}
	want := mcpserver.Settings{Enabled: false, ListenHost: "127.0.0.1", Port: 18731}
	if settings != want {
		t.Fatalf("defaults = %+v, want %+v", settings, want)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("Load created %s: %v", dir, err)
	}
}

func TestSettingsRoundTripPrivately(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "mcp")
	store := mcpserver.SettingsStore{Dir: dir}
	want := mcpserver.Settings{
		Enabled: true, ListenHost: "0.0.0.0", Port: 18800, AllowWrites: true,
		TokenSHA256: strings.Repeat("ab", 32), TokenCreatedAt: time.Date(2026, 11, 27, 8, 0, 0, 0, time.UTC),
	}
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(want); err != nil {
		t.Fatalf("replacing the file: %v", err)
	}
	got, err := store.Load()
	if err != nil || got != want {
		t.Fatalf("Load() = %+v, %v; want %+v", got, err, want)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "settings.json" {
		t.Fatalf("directory holds %v (%v); want only settings.json", entries, err)
	}
	if runtime.GOOS != "windows" {
		for path, mode := range map[string]os.FileMode{dir: 0o700, filepath.Join(dir, "settings.json"): 0o600} {
			info, err := os.Stat(path)
			if err != nil || info.Mode().Perm() != mode {
				t.Fatalf("%s mode = %v (%v), want %v", path, info.Mode().Perm(), err, mode)
			}
		}
	}
}

// The file is decoded exactly: unknown, duplicate or differently cased members, oversized files and invalid
// values are refused rather than half-applied.
func TestSettingsLoadIsStrict(t *testing.T) {
	t.Parallel()
	valid := `{"enabled":false,"listen_host":"127.0.0.1","port":18731,"allow_writes":false}`
	for name, content := range map[string]string{
		"unknown member":   `{"enabled":false,"listen_host":"127.0.0.1","port":18731,"allow_writes":false,"token":"x"}`,
		"duplicate member": `{"enabled":false,"enabled":true,"listen_host":"127.0.0.1","port":18731,"allow_writes":false}`,
		"other case":       `{"Enabled":true,"listen_host":"127.0.0.1","port":18731,"allow_writes":false}`,
		"not json":         `enabled=true`,
		"low port":         strings.Replace(valid, "18731", "80", 1),
		"high port":        strings.Replace(valid, "18731", "70000", 1),
		"host name":        strings.Replace(valid, "127.0.0.1", "localhost", 1),
		"token hash":       strings.Replace(valid, `"allow_writes":false`, `"allow_writes":false,"token_sha256":"xyz"`, 1),
		"oversized":        strings.Replace(valid, "{", "{"+strings.Repeat(" ", 17<<10), 1),
		// The first 16 KiB are a valid object; only the size check refuses the file.
		"oversized after the object": valid + strings.Repeat(" ", 17<<10),
		"zone":                       strings.Replace(valid, "127.0.0.1", "fe80::1%eth0", 1),
	} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if settings, err := (mcpserver.SettingsStore{Dir: dir}).Load(); err == nil {
			t.Fatalf("%s: loaded %+v", name, settings)
		}
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(valid), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := (mcpserver.SettingsStore{Dir: dir}).Load(); err != nil {
		t.Fatalf("valid file: %v", err)
	}
}

// A listen host is loopback, a wildcard or an address of this computer; ports below 1024 are refused.
func TestSettingsValidate(t *testing.T) {
	t.Parallel()
	local := []netip.Addr{netip.MustParseAddr("192.168.31.20"), netip.MustParseAddr("fd00::20")}
	base := mcpserver.DefaultSettings()
	for _, host := range []string{"127.0.0.1", "::1", "0.0.0.0", "::", "192.168.31.20", "fd00::20"} {
		settings := base
		settings.ListenHost = host
		if err := settings.Validate(local); err != nil {
			t.Fatalf("%s refused: %v", host, err)
		}
	}
	for _, host := range []string{"", "localhost", "8.8.8.8", "192.168.31.21", "127.0.0.2", "fe80::1%eth0", "not an address"} {
		settings := base
		settings.ListenHost = host
		if err := settings.Validate(local); err == nil {
			t.Fatalf("%q accepted", host)
		}
	}
	for port, ok := range map[int]bool{1023: false, 1024: true, 18731: true, 65535: true, 65536: false, 0: false} {
		settings := base
		settings.Port = port
		if err := settings.Validate(local); (err == nil) != ok {
			t.Fatalf("port %d: %v", port, err)
		}
	}
	if got := (mcpserver.Settings{ListenHost: "::1", Port: 18731}).Address(); got != "[::1]:18731" {
		t.Fatalf("Address() = %s", got)
	}
}

func TestSettingsSaveFailureKeepsTheOldFile(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	blocker := filepath.Join(parent, "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := (mcpserver.SettingsStore{Dir: filepath.Join(blocker, "mcp")}).Save(mcpserver.DefaultSettings()); err == nil {
		t.Fatal("saved below a regular file")
	}
	if err := (mcpserver.SettingsStore{Dir: parent}).Save(mcpserver.Settings{ListenHost: "127.0.0.1", Port: 80}); err == nil {
		t.Fatal("saved invalid settings")
	}
}
