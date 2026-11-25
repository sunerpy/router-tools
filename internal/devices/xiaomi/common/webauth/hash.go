// Package webauth implements Xiaomi Web login primitives without transport policy.
package webauth

import (
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"math/big"
	"time"
)

// EncryptionMode selects the digest required by init_info.newEncryptMode.
type EncryptionMode int

const (
	EncryptionSHA1 EncryptionMode = iota
	EncryptionSHA256
)

// Nonce constructs the Xiaomi login nonce. The random integer range is part of the wire contract.
func Nonce(deviceID string, unixSeconds int64, randomInteger int) (string, error) {
	if deviceID == "" {
		return "", fmt.Errorf("device ID is empty")
	}
	if randomInteger < 1000 || randomInteger > 10000 {
		return "", fmt.Errorf("nonce random integer %d is outside [1000, 10000]", randomInteger)
	}
	return fmt.Sprintf("0_%s_%d_%d", deviceID, unixSeconds, randomInteger), nil
}

// GenerateNonce creates a nonce using Unix seconds and crypto/rand in [1000, 10000].
func GenerateNonce(deviceID string, now time.Time, random io.Reader) (string, error) {
	if random == nil {
		return "", fmt.Errorf("nonce random source is nil")
	}
	const randomRange = 10000 - 1000 + 1
	value, err := rand.Int(random, big.NewInt(randomRange))
	if err != nil {
		return "", fmt.Errorf("generate nonce random integer: %w", err)
	}
	return Nonce(deviceID, now.Unix(), int(value.Int64())+1000)
}

// PasswordHash derives H(nonce + H(password + key)) over UTF-8 bytes.
func PasswordHash(password, key, nonce string, mode EncryptionMode) (string, error) {
	var newHash func() hash.Hash
	switch mode {
	case EncryptionSHA1:
		newHash = sha1.New
	case EncryptionSHA256:
		newHash = sha256.New
	default:
		return "", fmt.Errorf("unsupported Xiaomi encryption mode %d", mode)
	}

	inner := newHash()
	_, _ = inner.Write([]byte(password + key))
	innerHex := hex.EncodeToString(inner.Sum(nil))

	outer := newHash()
	_, _ = outer.Write([]byte(nonce + innerHex))
	return hex.EncodeToString(outer.Sum(nil)), nil
}
