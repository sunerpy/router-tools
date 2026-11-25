// Package secret contains password validation and output redaction primitives.
package secret

import (
	"encoding/json"
	"unicode/utf8"

	"github.com/sunerpy/router-tools/internal/device"
)

// Password holds a validated secret. Expose is reserved for protocol boundaries.
type Password struct {
	value string
}

// NewPassword validates the exact input without trimming or Unicode normalization.
func NewPassword(raw string) (Password, error) {
	if err := ValidatePassword(raw); err != nil {
		return Password{}, err
	}
	return Password{value: raw}, nil
}

// ValidatePassword applies the shared admin/root password contract.
func ValidatePassword(raw string) error {
	if raw == "" {
		return &device.Error{Code: device.CodeInvalidPassword, Message: "password must not be empty"}
	}
	if !utf8.ValidString(raw) {
		return &device.Error{Code: device.CodeInvalidPassword, Message: "password must be valid UTF-8"}
	}
	if len(raw) > 1024 {
		return &device.Error{Code: device.CodeInvalidPassword, Message: "password exceeds 1024 UTF-8 bytes"}
	}
	for _, r := range raw {
		if r <= 0x1f || r == 0x7f {
			return &device.Error{Code: device.CodeInvalidPassword, Message: "password contains a control character"}
		}
	}
	return nil
}

// Validate detects zero-value or otherwise invalid Password values.
func (p Password) Validate() error { return ValidatePassword(p.value) }

// Expose returns the exact password for a protocol operation, never for logging.
func (p Password) Expose() string { return p.value }

// String prevents accidental direct formatting of the secret.
func (p Password) String() string { return "<redacted>" }

// MarshalJSON prevents passwords from entering JSON reports.
func (p Password) MarshalJSON() ([]byte, error) { return json.Marshal("<redacted>") }
