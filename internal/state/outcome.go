// Package state persists the write-ahead outcome record of the last device change per router. It holds
// metadata only: never a password, command, token or report body.
package state

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

const (
	schemaVersion = 1
	maxRecordSize = 64 << 10
)

// Status values of an outcome record.
const (
	StatusInProgress = "in_progress"
	StatusSucceeded  = "succeeded"
	StatusFailed     = "failed"
	StatusPartial    = "partial"
	StatusUnknown    = "unknown"
)

var recoveryKey = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Outcome is the last device change recorded for one router.
type Outcome struct {
	SchemaVersion       int           `json:"schema_version"`
	Operation           string        `json:"operation"`
	Host                string        `json:"host,omitempty"`
	Status              string        `json:"status"`
	ErrorCode           string        `json:"error_code,omitempty"`
	DeviceWritePossible bool          `json:"device_write_possible"`
	Evidence            WriteEvidence `json:"write_evidence,omitzero"`
	StartedAt           time.Time     `json:"started_at"`
	FinishedAt          time.Time     `json:"finished_at,omitzero"`
	AcknowledgedAt      time.Time     `json:"acknowledged_at,omitzero"`
}

// WriteEvidence is the adapter's stage evidence kept with the record, so the operator can review what may
// have changed after the report itself is gone. It never holds a secret.
type WriteEvidence struct {
	// WriteStarted: the adapter reached its first device write.
	WriteStarted bool `json:"write_started,omitzero"`
	// AcceptedSteps: enable commands the router accepted, or deep writes that were accepted.
	AcceptedSteps int `json:"accepted_steps,omitzero"`
	// AttemptedMACs are the temporary MAC rules an enable run tried to add; UncleanedMACs could not be
	// removed again and must be checked in the router's Web UI.
	AttemptedMACs []string `json:"attempted_macs,omitempty"`
	UncleanedMACs []string `json:"uncleaned_macs,omitempty"`
	// JournalPath is the deep journal to inspect.
	JournalPath string `json:"journal_path,omitempty"`
}

// Blocks reports whether the record forbids another device write until someone acknowledges that the
// router was checked: the change is still in progress (or the process died), ended partial or unknown,
// or failed after a write may have been submitted.
func (o Outcome) Blocks() bool {
	uncertain := false
	switch o.Status {
	case StatusInProgress, StatusPartial, StatusUnknown:
		uncertain = true
	case StatusFailed:
		uncertain = o.DeviceWritePossible
	}
	if !uncertain {
		return false
	}
	reference := o.FinishedAt
	if reference.IsZero() {
		reference = o.StartedAt
	}
	return o.AcknowledgedAt.IsZero() || !o.AcknowledgedAt.After(reference)
}

// FileStore keeps one JSON record per recovery key in Dir.
type FileStore struct {
	Dir string
}

// Load returns the record for key; ok is false when none exists.
func (s FileStore) Load(key string) (Outcome, bool, error) {
	path, err := s.path(key)
	if err != nil {
		return Outcome{}, false, err
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return Outcome{}, false, nil
	}
	if err != nil {
		return Outcome{}, false, err
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, maxRecordSize+1))
	if err != nil {
		return Outcome{}, false, err
	}
	if len(data) > maxRecordSize {
		return Outcome{}, false, fmt.Errorf("outcome record exceeds %d bytes", maxRecordSize)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var outcome Outcome
	if err := decoder.Decode(&outcome); err != nil {
		return Outcome{}, false, fmt.Errorf("decode outcome record: %w", err)
	}
	if decoder.More() {
		return Outcome{}, false, fmt.Errorf("outcome record has trailing data")
	}
	if err := validate(outcome); err != nil {
		return Outcome{}, false, err
	}
	return outcome, true, nil
}

// Save atomically replaces the record for key and syncs it before returning.
func (s FileStore) Save(key string, outcome Outcome) error {
	path, err := s.path(key)
	if err != nil {
		return err
	}
	outcome.SchemaVersion = schemaVersion
	if err := validate(outcome); err != nil {
		return err
	}
	data, err := json.MarshalIndent(outcome, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(s.Dir, 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(s.Dir, key+".*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(append(data, '\n')); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}
	return syncDir(s.Dir)
}

func (s FileStore) path(key string) (string, error) {
	if s.Dir == "" {
		return "", fmt.Errorf("outcome store directory is not configured")
	}
	if !recoveryKey.MatchString(key) {
		return "", fmt.Errorf("outcome key is not a recovery key")
	}
	return filepath.Join(s.Dir, key+".json"), nil
}

func validate(outcome Outcome) error {
	if outcome.SchemaVersion != schemaVersion {
		return fmt.Errorf("unsupported outcome schema version %d", outcome.SchemaVersion)
	}
	switch outcome.Status {
	case StatusInProgress, StatusSucceeded, StatusFailed, StatusPartial, StatusUnknown:
	default:
		return fmt.Errorf("unknown outcome status %q", outcome.Status)
	}
	if outcome.Operation == "" || len(outcome.Operation) > 64 {
		return fmt.Errorf("invalid outcome operation")
	}
	return nil
}
