package webauth_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/sunerpy/router-tools/internal/devices/xiaomi/common/webauth"
)

func TestPasswordHashFixedVectors(t *testing.T) {
	t.Parallel()

	const (
		deviceID = "AABBCCDDEEFF"
		key      = "test-key"
		password = "admin"
		nonce    = "0_AABBCCDDEEFF_1700000000_4321"
	)

	tests := []struct {
		name string
		mode webauth.EncryptionMode
		want string
	}{
		{
			name: "SHA-1",
			mode: webauth.EncryptionSHA1,
			want: "27b55f701d7b83763e1f5a77d31ab9a5b3fb347c",
		},
		{
			name: "SHA-256",
			mode: webauth.EncryptionSHA256,
			want: "21cf7a9e37d16fa8b58bbaca7ff6bba339c269535a879ceae7a3df736d00b5e4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNonce, err := webauth.Nonce(deviceID, 1700000000, 4321)
			if err != nil {
				t.Fatalf("Nonce() error = %v", err)
			}
			if gotNonce != nonce {
				t.Fatalf("Nonce() = %q, want %q", gotNonce, nonce)
			}
			got, err := webauth.PasswordHash(password, key, gotNonce, tt.mode)
			if err != nil {
				t.Fatalf("PasswordHash() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("PasswordHash() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerateNonceWithDeterministicRandom(t *testing.T) {
	t.Parallel()

	got, err := webauth.GenerateNonce(
		"AABBCCDDEEFF",
		time.Unix(1700000000, 999),
		bytes.NewReader([]byte{0x0c, 0xf9}),
	)
	if err != nil {
		t.Fatalf("GenerateNonce() error = %v", err)
	}
	const want = "0_AABBCCDDEEFF_1700000000_4321"
	if got != want {
		t.Fatalf("GenerateNonce() = %q, want %q", got, want)
	}
}

func TestNonceRejectsOutOfRangeRandomInteger(t *testing.T) {
	t.Parallel()

	for _, random := range []int{999, 10001} {
		if _, err := webauth.Nonce("device", 1700000000, random); err == nil {
			t.Fatalf("Nonce(random=%d) error = nil", random)
		}
	}
}

func TestPasswordHashRejectsUnknownEncryptionMode(t *testing.T) {
	t.Parallel()

	_, err := webauth.PasswordHash("admin", "key", "nonce", webauth.EncryptionMode(99))
	if err == nil {
		t.Fatal("PasswordHash() unknown mode error = nil")
	}
}
