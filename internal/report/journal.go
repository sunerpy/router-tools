// Package report implements atomic local run journals.
package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// StageRecord is one stable deep-persist state transition.
type StageRecord struct {
	Stage string    `json:"stage"`
	Time  time.Time `json:"time"`
}

// BootTransition records old and new canonical boot IDs.
type BootTransition struct {
	OldBootID string    `json:"old_boot_id"`
	NewBootID string    `json:"new_boot_id"`
	Time      time.Time `json:"time"`
}

// Data is the on-disk journal schema.
type Data struct {
	SchemaVersion int              `json:"schema_version"`
	Host          string           `json:"host"`
	BackupDir     string           `json:"backup_dir"`
	Complete      bool             `json:"complete"`
	Status        string           `json:"status"`
	Stages        []StageRecord    `json:"stages"`
	Transitions   []BootTransition `json:"transitions,omitempty"`
}

// Journal serializes atomic updates.
type Journal struct {
	mu   sync.Mutex
	path string
	data Data
}

// CreateJournal creates a new file without overwriting any previous evidence.
func CreateJournal(path, host, backupDir string) (*Journal, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	journal := &Journal{path: path, data: Data{SchemaVersion: 1, Host: host, BackupDir: backupDir, Status: "PRECHECK"}}
	data, err := json.MarshalIndent(journal.data, "", "  ")
	if err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return nil, err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	return journal, nil
}

// Path returns the local evidence path.
func (j *Journal) Path() string { return j.path }

// Stage appends a stable stage and atomically persists it.
func (j *Journal) Stage(stage string) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.data.Stages = append(j.data.Stages, StageRecord{Stage: stage, Time: time.Now().UTC()})
	j.data.Status = stage
	if stage == "SUCCEEDED" || stage == "FAILED" || stage == "UNKNOWN" {
		j.data.Complete = true
	}
	return j.writeLocked()
}

// Transition records one verified reboot transition.
func (j *Journal) Transition(oldBootID, newBootID string) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.data.Transitions = append(j.data.Transitions, BootTransition{OldBootID: oldBootID, NewBootID: newBootID, Time: time.Now().UTC()})
	return j.writeLocked()
}

func (j *Journal) writeLocked() error {
	data, err := json.MarshalIndent(j.data, "", "  ")
	if err != nil {
		return err
	}
	temporary := fmt.Sprintf("%s.tmp", j.path)
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(temporary, j.path)
}
