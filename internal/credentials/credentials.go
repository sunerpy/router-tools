// Package credentials keeps passwords the user chose to remember, encrypted at rest with AES-256-GCM. It is
// the only place a password may be persisted, and no method except Get ever returns a password.
package credentials

import (
	"time"

	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/secret"
)

// Kind is the password a saved entry holds.
type Kind string

const (
	KindWebAdmin Kind = "web-admin"
	KindRoot     Kind = "root"
)

// Key identifies one saved password: its kind, the device model it belongs to, the canonical host, and the
// port the password is used on (the Web port for the admin password, the SSH port for root).
type Key struct {
	Kind  Kind   `json:"kind"`
	Scope string `json:"scope"`
	Host  string `json:"host"`
	Port  int    `json:"port"`
}

// NewKey builds a key with the canonical host spelling.
func NewKey(kind Kind, scope, host string, port int) Key {
	return Key{Kind: kind, Scope: scope, Host: device.CanonicalHost(host), Port: port}
}

// Entry is the public part of a saved password.
type Entry struct {
	Key
	SavedAt    time.Time `json:"saved_at"`
	RejectedAt time.Time `json:"rejected_at,omitzero"`
}

// Rejected reports whether the device refused the saved value since it was saved; such an entry is never
// filled in automatically.
func (e Entry) Rejected() bool { return !e.RejectedAt.IsZero() }

// Filter selects entries; empty fields match every entry. A filter with no field set selects nothing.
type Filter struct {
	Kind  Kind
	Scope string
	Host  string
	Port  int
}

// Empty reports whether no field is set.
func (f Filter) Empty() bool { return f == Filter{} }

// Matches reports whether key satisfies every set field. Host compares canonical spellings.
func (f Filter) Matches(key Key) bool {
	return (f.Kind == "" || f.Kind == key.Kind) && (f.Scope == "" || f.Scope == key.Scope) &&
		(f.Host == "" || device.CanonicalHost(f.Host) == key.Host) && (f.Port == 0 || f.Port == key.Port)
}

// Store keeps saved passwords.
type Store interface {
	// List returns every entry's metadata, never a password.
	List() ([]Entry, error)
	// Get returns the saved password for key; ok is false when none is saved.
	Get(key Key) (secret.Password, Entry, bool, error)
	// Put saves or replaces the password for key and clears any rejection.
	Put(key Key, password secret.Password, at time.Time) error
	// MarkRejected records that the device refused the saved password. It changes the entry only while it
	// still holds the value saved at savedAt, so a newer value saved meanwhile stays usable.
	MarkRejected(key Key, savedAt, at time.Time) error
	// Delete removes the entries matching a non-empty filter and returns them.
	Delete(filter Filter) ([]Entry, error)
	// DeleteAll removes every entry and the vault key.
	DeleteAll() error
}

func storeError(message string, cause error) error {
	return device.WrapError(device.CodeCredentialStoreFailed, message, cause)
}
