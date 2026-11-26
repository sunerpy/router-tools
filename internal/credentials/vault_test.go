package credentials_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sunerpy/router-tools/internal/credentials"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/secret"
)

var savedAt = time.Date(2026, 11, 26, 4, 0, 0, 0, time.UTC)

func password(t *testing.T, raw string) secret.Password {
	t.Helper()
	value, err := secret.NewPassword(raw)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func assertCode(t *testing.T, err error, want string) {
	t.Helper()
	var deviceErr *device.Error
	if !errors.As(err, &deviceErr) || deviceErr.Code != want {
		t.Fatalf("error = %v, want %s", err, want)
	}
}

func newVault(t *testing.T) (credentials.FileVault, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "credentials")
	return credentials.FileVault{Dir: dir}, dir
}

func TestNewKeyUsesTheCanonicalHost(t *testing.T) {
	t.Parallel()

	if got := credentials.NewKey(credentials.KindRoot, "xiaomi-rd08", "ROUTER.lan", 22); got.Host != "router.lan" {
		t.Fatalf("host = %q, want the canonical spelling", got.Host)
	}
	if got := credentials.NewKey(credentials.KindRoot, "xiaomi-rd08", "2001:DB8:0:0:0:0:0:1", 22); got.Host != "2001:db8::1" {
		t.Fatalf("host = %q, want the canonical IPv6 form", got.Host)
	}
}

func TestVaultRoundTripKeepsOnlyCiphertextOnDisk(t *testing.T) {
	t.Parallel()

	vault, dir := newVault(t)
	key := credentials.NewKey(credentials.KindRoot, "xiaomi-rd08", "192.168.31.1", 22)
	admin := credentials.NewKey(credentials.KindWebAdmin, "xiaomi-rd08", "192.168.31.1", 80)
	if _, _, ok, err := vault.Get(key); err != nil || ok {
		t.Fatalf("Get() on an empty vault = %v, %v; want not saved", ok, err)
	}
	if err := vault.Put(key, password(t, "root-secret-value"), savedAt); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if err := vault.Put(admin, password(t, "admin-secret-value"), savedAt); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	got, entry, ok, err := vault.Get(key)
	if err != nil || !ok || got.Expose() != "root-secret-value" || !entry.SavedAt.Equal(savedAt) || entry.Rejected() {
		t.Fatalf("Get() = %q %+v %v %v", got.Expose(), entry, ok, err)
	}
	entries, err := vault.List()
	if err != nil || len(entries) != 2 {
		t.Fatalf("List() = %+v, %v", entries, err)
	}
	for _, name := range []string{"vault.json", "vault.key"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "root-secret-value") || strings.Contains(string(data), "admin-secret-value") {
			t.Fatalf("%s contains a plaintext password", name)
		}
		if runtime.GOOS != "windows" {
			if info, _ := os.Stat(filepath.Join(dir, name)); info.Mode().Perm() != 0o600 {
				t.Fatalf("%s mode = %o, want 0600", name, info.Mode().Perm())
			}
		}
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(dir); info.Mode().Perm() != 0o700 {
			t.Fatalf("vault directory mode = %o, want 0700", info.Mode().Perm())
		}
	}
}

func TestVaultRejectsTamperingAndAnotherKey(t *testing.T) {
	t.Parallel()

	tamper := map[string]func(t *testing.T, dir string){
		"ciphertext": func(t *testing.T, dir string) {
			replaceJSONField(t, dir, "ciphertext", func(v string) string { return flipBase64(v) })
		},
		"nonce": func(t *testing.T, dir string) {
			replaceJSONField(t, dir, "nonce", func(v string) string { return flipBase64(v) })
		},
		"entry moved to another router": func(t *testing.T, dir string) {
			replaceJSONField(t, dir, "host", func(string) string { return "192.168.31.2" })
		},
		"another vault key": func(t *testing.T, dir string) {
			other := credentials.FileVault{Dir: filepath.Join(t.TempDir(), "other")}
			if err := other.Put(credentials.NewKey(credentials.KindRoot, "xiaomi-rd08", "10.0.0.1", 22), password(t, "x"), savedAt); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(other.Dir, "vault.key"))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "vault.key"), data, 0o600); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, change := range tamper {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			vault, dir := newVault(t)
			key := credentials.NewKey(credentials.KindRoot, "xiaomi-rd08", "192.168.31.1", 22)
			if err := vault.Put(key, password(t, "root-secret-value"), savedAt); err != nil {
				t.Fatal(err)
			}
			change(t, dir)
			lookup := key
			if name == "entry moved to another router" {
				lookup = credentials.NewKey(credentials.KindRoot, "xiaomi-rd08", "192.168.31.2", 22)
			}
			value, _, ok, err := vault.Get(lookup)
			assertCode(t, err, device.CodeCredentialStoreFailed)
			if ok || value.Expose() != "" {
				t.Fatalf("Get() returned a value from a tampered vault")
			}
		})
	}
}

func TestVaultDecodesStrictly(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"unknown field":   `{"schema_version":1,"entries":[],"password":"x"}`,
		"wrong schema":    `{"schema_version":2,"entries":[]}`,
		"trailing data":   `{"schema_version":1,"entries":[]} {}`,
		"unknown kind":    `{"schema_version":1,"entries":[{"kind":"telnet","scope":"xiaomi-rd08","host":"192.168.31.1","port":22,"saved_at":"2026-11-26T04:00:00Z","nonce":"AAAAAAAAAAAAAAAA","ciphertext":"AAAA"}]}`,
		"bad port":        `{"schema_version":1,"entries":[{"kind":"root","scope":"xiaomi-rd08","host":"192.168.31.1","port":0,"saved_at":"2026-11-26T04:00:00Z","nonce":"AAAAAAAAAAAAAAAA","ciphertext":"AAAA"}]}`,
		"non-canonical":   `{"schema_version":1,"entries":[{"kind":"root","scope":"xiaomi-rd08","host":"ROUTER.lan","port":22,"saved_at":"2026-11-26T04:00:00Z","nonce":"AAAAAAAAAAAAAAAA","ciphertext":"AAAA"}]}`,
		"short nonce":     `{"schema_version":1,"entries":[{"kind":"root","scope":"xiaomi-rd08","host":"192.168.31.1","port":22,"saved_at":"2026-11-26T04:00:00Z","nonce":"AAAA","ciphertext":"AAAA"}]}`,
		"duplicate entry": `{"schema_version":1,"entries":[{"kind":"root","scope":"xiaomi-rd08","host":"192.168.31.1","port":22,"saved_at":"2026-11-26T04:00:00Z","nonce":"AAAAAAAAAAAAAAAA","ciphertext":"AAAA"},{"kind":"root","scope":"xiaomi-rd08","host":"192.168.31.1","port":22,"saved_at":"2026-11-26T04:00:00Z","nonce":"AAAAAAAAAAAAAAAA","ciphertext":"AAAA"}]}`,
		"oversized":       `{"schema_version":1,"entries":[],"x":"` + strings.Repeat("x", 2<<20) + `"}`,
		"extra closer":    `{"schema_version":1,"entries":[]}}`,
		"extra bracket":   `{"schema_version":1,"entries":[]}]`,
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			vault, dir := newVault(t)
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "vault.json"), []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := vault.List()
			assertCode(t, err, device.CodeCredentialStoreFailed)
		})
	}
}

// Entries without their key are reported, and the vault never creates a new key over them.
func TestVaultNeverReplacesAMissingKeyOverExistingEntries(t *testing.T) {
	t.Parallel()

	vault, dir := newVault(t)
	key := credentials.NewKey(credentials.KindRoot, "xiaomi-rd08", "192.168.31.1", 22)
	if err := vault.Put(key, password(t, "root-secret-value"), savedAt); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "vault.key")); err != nil {
		t.Fatal(err)
	}
	_, _, _, err := vault.Get(key)
	assertCode(t, err, device.CodeCredentialStoreFailed)
	err = vault.Put(credentials.NewKey(credentials.KindWebAdmin, "xiaomi-rd08", "192.168.31.1", 80), password(t, "admin"), savedAt)
	assertCode(t, err, device.CodeCredentialStoreFailed)
	if _, err := os.Stat(filepath.Join(dir, "vault.key")); !os.IsNotExist(err) {
		t.Fatalf("a new vault key was created over existing entries: %v", err)
	}
	if entries, err := vault.List(); err != nil || len(entries) != 1 {
		t.Fatalf("List() = %+v, %v; metadata must stay listable so the entry can be forgotten", entries, err)
	}
}

func TestVaultRejectionForgetAndForgetAll(t *testing.T) {
	t.Parallel()

	vault, dir := newVault(t)
	root := credentials.NewKey(credentials.KindRoot, "xiaomi-rd08", "192.168.31.1", 22)
	admin := credentials.NewKey(credentials.KindWebAdmin, "xiaomi-rd08", "192.168.31.1", 80)
	other := credentials.NewKey(credentials.KindRoot, "xiaomi-rd08", "192.168.31.2", 22)
	for _, key := range []credentials.Key{root, admin, other} {
		if err := vault.Put(key, password(t, "secret-"+string(key.Kind)), savedAt); err != nil {
			t.Fatal(err)
		}
	}
	if err := vault.MarkRejected(root, savedAt, savedAt.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, entry, ok, _ := vault.Get(root); !ok || !entry.Rejected() {
		t.Fatalf("rejected entry = %+v, %v", entry, ok)
	}
	if err := vault.Put(root, password(t, "new-root"), savedAt.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if value, entry, _, _ := vault.Get(root); entry.Rejected() || value.Expose() != "new-root" {
		t.Fatalf("a verified save must replace the value and clear the rejection: %+v", entry)
	}

	deleted, err := vault.Delete(credentials.Filter{Host: "192.168.31.1"})
	if err != nil || len(deleted) != 2 {
		t.Fatalf("Delete(host) = %+v, %v; want both entries of that router", deleted, err)
	}
	if entries, _ := vault.List(); len(entries) != 1 || entries[0].Key != other {
		t.Fatalf("remaining = %+v", entries)
	}
	if _, err := vault.Delete(credentials.Filter{}); err == nil {
		t.Fatal("Delete() without a filter must be refused; use DeleteAll")
	}
	if err := vault.DeleteAll(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"vault.json", "vault.key"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("%s still exists after DeleteAll: %v", name, err)
		}
	}
	if entries, err := vault.List(); err != nil || len(entries) != 0 {
		t.Fatalf("List() after DeleteAll = %+v, %v", entries, err)
	}
}

// Separate vault instances, as separate processes would be, serialize their read-modify-write.
func TestConcurrentWritersKeepEveryEntry(t *testing.T) {
	t.Parallel()

	_, dir := newVault(t)
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			key := credentials.NewKey(credentials.KindRoot, "xiaomi-rd08", "192.168.31.1", 2000+i)
			errs <- credentials.FileVault{Dir: dir}.Put(key, password(t, "secret"), savedAt)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("Put() error = %v", err)
		}
	}
	if entries, err := (credentials.FileVault{Dir: dir}).List(); err != nil || len(entries) != 20 {
		t.Fatalf("List() = %d entries, %v; want 20", len(entries), err)
	}
}

func replaceJSONField(t *testing.T, dir, field string, change func(string) string) {
	t.Helper()
	path := filepath.Join(dir, "vault.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	marker := `"` + field + `": "`
	start := strings.Index(text, marker)
	if start < 0 {
		t.Fatalf("field %s not found in %s", field, text)
	}
	start += len(marker)
	end := start + strings.Index(text[start:], `"`)
	text = text[:start] + change(text[start:end]) + text[end:]
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func flipBase64(value string) string {
	if strings.HasPrefix(value, "A") {
		return "B" + value[1:]
	}
	return "A" + value[1:]
}

// A rejection only marks the value that was refused: a newer value saved meanwhile stays usable.
func TestMarkRejectedOnlyMarksTheRefusedValue(t *testing.T) {
	t.Parallel()

	for name, store := range map[string]credentials.Store{"file": credentials.FileVault{Dir: filepath.Join(t.TempDir(), "v")}, "memory": credentials.NewMemory()} {
		key := credentials.NewKey(credentials.KindRoot, "xiaomi-rd08", "192.168.31.1", 22)
		if err := store.Put(key, password(t, "old"), savedAt); err != nil {
			t.Fatal(err)
		}
		_, refused, _, _ := store.Get(key)
		if err := store.Put(key, password(t, "new"), savedAt.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		if err := store.MarkRejected(key, refused.SavedAt, savedAt.Add(2*time.Minute)); err != nil {
			t.Fatal(err)
		}
		if value, entry, _, _ := store.Get(key); entry.Rejected() || value.Expose() != "new" {
			t.Fatalf("%s: the newer value was marked rejected by a stale refusal", name)
		}
	}
}
