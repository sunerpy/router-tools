// Package hostkey implements SSH host-key pinning and TOFU persistence.
package hostkey

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/sunerpy/router-tools/internal/device"
)

// Prompt contains only public host-key evidence.
type Prompt struct {
	Address     string
	Algorithm   string
	Fingerprint string
}

// ConfirmFunc decides whether a previously unknown public key may be stored.
type ConfirmFunc func(prompt Prompt) bool

// Policy defines fail-closed host-key verification.
type Policy struct {
	KnownHostsPath string
	PinSHA256      string
	AcceptNew      bool
	Confirm        ConfirmFunc
}

// Callback prepares known_hosts with restrictive permissions and returns an SSH callback.
func (p Policy) Callback(_ context.Context) (ssh.HostKeyCallback, error) {
	if p.KnownHostsPath == "" {
		return nil, fmt.Errorf("known_hosts path is required")
	}
	if err := os.MkdirAll(filepath.Dir(p.KnownHostsPath), 0o700); err != nil {
		return nil, fmt.Errorf("create known_hosts directory: %w", err)
	}
	file, err := os.OpenFile(p.KnownHostsPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create known_hosts: %w", err)
	}
	_ = file.Close()
	if err := os.Chmod(p.KnownHostsPath, 0o600); err != nil {
		return nil, fmt.Errorf("secure known_hosts: %w", err)
	}
	trusted, err := knownhosts.New(p.KnownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("load known_hosts: %w", err)
	}

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		fingerprint := ssh.FingerprintSHA256(key)
		if p.PinSHA256 != "" {
			if fingerprint != p.PinSHA256 {
				return &device.Error{Code: device.CodeHostKeyMismatch, Message: "SSH host key does not match the pinned fingerprint"}
			}
			return nil
		}
		err := trusted(hostname, remote, key)
		if err == nil {
			return nil
		}
		var keyError *knownhosts.KeyError
		if !errors.As(err, &keyError) {
			return &device.Error{Code: device.CodeSSHHostKeyRejected, Message: "SSH host key validation failed"}
		}
		if len(keyError.Want) != 0 {
			return &device.Error{Code: device.CodeHostKeyMismatch, Message: "SSH host key changed; refusing to replace the stored key"}
		}
		allowed := p.AcceptNew
		if !allowed && p.Confirm != nil {
			allowed = p.Confirm(Prompt{Address: hostname, Algorithm: key.Type(), Fingerprint: fingerprint})
		}
		if !allowed {
			return &device.Error{Code: device.CodeSSHHostKeyRejected, Message: "unknown SSH host key was not accepted"}
		}
		if err := appendKey(p.KnownHostsPath, hostname, key); err != nil {
			return fmt.Errorf("store SSH host key: %w", err)
		}
		return nil
	}, nil
}

func appendKey(path, hostname string, key ssh.PublicKey) error {
	line := knownhosts.Line([]string{knownhosts.Normalize(hostname)}, key)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.WriteString(line + "\n"); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
