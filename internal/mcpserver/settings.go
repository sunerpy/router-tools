package mcpserver

import (
	"encoding/hex"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/sunerpy/router-tools/internal/device"
)

const (
	// DefaultPort is the desktop's default agent port.
	DefaultPort      = 18731
	settingsFile     = "settings.json"
	maxSettingsBytes = 16 << 10
)

// Settings is the desktop's agent access configuration. Only the token's SHA-256 is kept; the token itself is
// shown once when it is created.
type Settings struct {
	Enabled        bool      `json:"enabled"`
	ListenHost     string    `json:"listen_host"`
	Port           int       `json:"port"`
	AllowWrites    bool      `json:"allow_writes"`
	TokenSHA256    string    `json:"token_sha256,omitempty"`
	TokenCreatedAt time.Time `json:"token_created_at,omitzero"`
}

// DefaultSettings is the configuration of a new install: off, loopback, writes off, no token.
func DefaultSettings() Settings {
	return Settings{ListenHost: "127.0.0.1", Port: DefaultPort}
}

// Address is host:port for net.Listen.
func (s Settings) Address() string {
	return net.JoinHostPort(s.ListenHost, strconv.Itoa(s.Port))
}

// Validate checks a configuration the user is about to apply: the host is loopback, a wildcard, or one of
// this computer's current addresses, and the port is 1024-65535.
func (s Settings) Validate(local []netip.Addr) error {
	if err := s.validateStored(); err != nil {
		return err
	}
	addr := netip.MustParseAddr(s.ListenHost)
	if addr == netip.MustParseAddr("127.0.0.1") || addr == netip.IPv6Loopback() || addr.IsUnspecified() {
		return nil
	}
	for _, candidate := range local {
		if candidate.Unmap() == addr.Unmap() {
			return nil
		}
	}
	return invalidSettings("the listen address must be 127.0.0.1, ::1, 0.0.0.0, :: or an address of this computer")
}

// validateStored checks what a stored file may contain; whether an address still belongs to this computer
// is found out when the listener binds it.
func (s Settings) validateStored() error {
	addr, err := netip.ParseAddr(s.ListenHost)
	if err != nil || addr.Zone() != "" || addr.String() != s.ListenHost {
		return invalidSettings("the listen host must be an IP address without a zone")
	}
	if s.Port < 1024 || s.Port > 65535 {
		return invalidSettings("the port must be between 1024 and 65535")
	}
	if s.TokenSHA256 != "" {
		if decoded, err := hex.DecodeString(s.TokenSHA256); err != nil || len(decoded) != 32 {
			return invalidSettings("the stored token hash is not a SHA-256 value")
		}
	}
	return nil
}

func invalidSettings(message string) error {
	return &device.Error{Code: device.CodeInvalidArgument, Message: message}
}

// SettingsStore keeps Settings in Dir/settings.json (directory 0700, file 0600), replaced atomically.
type SettingsStore struct {
	Dir string
	// syncDir flushes the directory after the rename; tests replace it.
	syncDir func(string) error
}

// Load returns the stored settings, or the defaults when none are stored. The file is decoded exactly: unknown,
// duplicate or differently cased members, files over 16 KiB and invalid values are errors.
func (s SettingsStore) Load() (Settings, error) {
	file, err := os.Open(filepath.Join(s.Dir, settingsFile))
	if errors.Is(err, os.ErrNotExist) {
		return DefaultSettings(), nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("read the agent settings: %w", err)
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, maxSettingsBytes+1))
	if err != nil {
		return Settings{}, fmt.Errorf("read the agent settings: %w", err)
	}
	if len(data) > maxSettingsBytes {
		return Settings{}, invalidSettings("the agent settings file is larger than 16 KiB")
	}
	var settings Settings
	if err := jsonv2.Unmarshal(data, &settings, jsonv2.RejectUnknownMembers(true)); err != nil {
		return Settings{}, invalidSettings("the agent settings file is not valid")
	}
	if err := settings.validateStored(); err != nil {
		return Settings{}, err
	}
	return settings, nil
}

// Save validates and stores the settings through a synced temporary file and a rename.
func (s SettingsStore) Save(settings Settings) error {
	if err := settings.validateStored(); err != nil {
		return err
	}
	data, err := jsonv2.Marshal(settings)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return fmt.Errorf("create the agent settings directory: %w", err)
	}
	if err := os.Chmod(s.Dir, 0o700); err != nil {
		return fmt.Errorf("protect the agent settings directory: %w", err)
	}
	temp, err := os.CreateTemp(s.Dir, settingsFile+".*.tmp")
	if err != nil {
		return fmt.Errorf("write the agent settings: %w", err)
	}
	tempPath := temp.Name()
	defer func() { _ = os.Remove(tempPath) }()
	write := func() error {
		if err := temp.Chmod(0o600); err != nil {
			return err
		}
		if _, err := temp.Write(data); err != nil {
			return err
		}
		return temp.Sync()
	}
	if err := write(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write the agent settings: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("write the agent settings: %w", err)
	}
	if err := os.Rename(tempPath, filepath.Join(s.Dir, settingsFile)); err != nil {
		return fmt.Errorf("write the agent settings: %w", err)
	}
	// The new file is in place and is what Load reads. Flushing the directory only makes the rename survive a
	// crash, and some file systems (FUSE, SMB) refuse it: failing here would make the caller roll back settings
	// that already changed on disk.
	flush := s.syncDir
	if flush == nil {
		flush = syncDir
	}
	_ = flush(s.Dir)
	return nil
}
