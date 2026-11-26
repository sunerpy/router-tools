//go:build windows

package credentials

import (
	"bytes"
	"testing"
)

// The vault key on disk is a DPAPI blob for the current user, never the raw key.
func TestDPAPIProtectsTheVaultKey(t *testing.T) {
	t.Parallel()

	raw := bytes.Repeat([]byte{0x5a}, keySize)
	blob, err := protectKey(raw)
	if err != nil {
		t.Fatalf("protectKey() error = %v", err)
	}
	if bytes.Contains(blob, raw) {
		t.Fatal("the protected key contains the raw key")
	}
	back, err := unprotectKey(blob)
	if err != nil || !bytes.Equal(back, raw) {
		t.Fatalf("unprotectKey() = %x, %v", back, err)
	}
	blob[len(blob)-1] ^= 0xff
	if _, err := unprotectKey(blob); err == nil {
		t.Fatal("a tampered DPAPI blob was accepted")
	}
}
