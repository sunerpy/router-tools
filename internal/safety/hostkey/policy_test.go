package hostkey_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
)

func TestPolicyTOFUAndPinning(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nested", "known_hosts")
	address := (&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 22}).String()
	key := testPublicKey(t)
	otherKey := testPublicKey(t)

	rejectCallback, err := (hostkey.Policy{KnownHostsPath: path}).Callback(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	err = rejectCallback(address, &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 22}, key)
	assertCode(t, err, device.CodeSSHHostKeyRejected)

	accept := hostkey.Policy{KnownHostsPath: path, AcceptNew: true}
	acceptCallback, err := accept.Callback(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := acceptCallback(address, &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 22}, key); err != nil {
		t.Fatalf("accept new key: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// Windows has no POSIX permission bits: os.Stat reports 0666 for any writable file.
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("known_hosts mode = %o", info.Mode().Perm())
	}

	trusted, err := (hostkey.Policy{KnownHostsPath: path}).Callback(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := trusted(address, &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 22}, key); err != nil {
		t.Fatalf("trusted key rejected: %v", err)
	}
	if err := trusted(address, &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 22}, otherKey); err == nil {
		t.Fatal("changed host key was accepted")
	} else {
		assertCode(t, err, device.CodeHostKeyMismatch)
	}
}

func TestPolicyPinOverridesStoredKey(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "known_hosts")
	key := testPublicKey(t)
	otherKey := testPublicKey(t)
	address := "127.0.0.1:22"
	remote := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 22}

	callback, err := (hostkey.Policy{KnownHostsPath: path, PinSHA256: ssh.FingerprintSHA256(key)}).Callback(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := callback(address, remote, key); err != nil {
		t.Fatalf("matching pin rejected: %v", err)
	}
	assertCode(t, callback(address, remote, otherKey), device.CodeHostKeyMismatch)
}

func TestPolicyInteractiveConfirmation(t *testing.T) {
	t.Parallel()

	key := testPublicKey(t)
	var prompt hostkey.Prompt
	policy := hostkey.Policy{
		KnownHostsPath: filepath.Join(t.TempDir(), "known_hosts"),
		Confirm: func(got hostkey.Prompt) bool {
			prompt = got
			return true
		},
	}
	callback, err := policy.Callback(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := callback("router.local:22", &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 22}, key); err != nil {
		t.Fatal(err)
	}
	if prompt.Fingerprint != ssh.FingerprintSHA256(key) || prompt.Algorithm != key.Type() {
		t.Fatalf("prompt = %+v", prompt)
	}
}

func testPublicKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, err := ssh.NewPublicKey(privateKey.Public())
	if err != nil {
		t.Fatal(err)
	}
	return publicKey
}

func assertCode(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want %s", want)
	}
	var deviceErr *device.Error
	if !errors.As(err, &deviceErr) {
		t.Fatalf("error = %T %v", err, err)
	}
	if deviceErr.Code != want {
		t.Fatalf("code = %q, want %q", deviceErr.Code, want)
	}
}
