package app

import (
	"context"

	"github.com/sunerpy/router-tools/internal/backup"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
	"github.com/sunerpy/router-tools/internal/secret"
)

// DeepJournal records stable high-risk stages and boot transitions.
type DeepJournal interface {
	Stage(string) error
	Transition(oldBootID, newBootID string) error
	Path() string
}

// BootTransition is one verified reboot in the final report.
type BootTransition struct {
	OldBootID string `json:"old_boot_id"`
	NewBootID string `json:"new_boot_id"`
}

// DeepParams contains verified local and online evidence for the adapter.
type DeepParams struct {
	Target        device.Target
	RootPassword  secret.Password
	HostKeyPolicy hostkey.Policy
	BackupDir     string
	Manifest      backup.Manifest
	Observation   device.Observation
	ProfileID     string
	Journal       DeepJournal
}

// DeepResult includes partial progress to prevent blind restart.
type DeepResult struct {
	Status           string           `json:"status"`
	AcceptedWrites   int              `json:"accepted_writes"`
	Transitions      []BootTransition `json:"transitions,omitempty"`
	FinalSSHVerified bool             `json:"final_ssh_verified"`
	SoftVerified     bool             `json:"soft_verified"`
	// WriteStarted is set before the first crash-partition write.
	WriteStarted bool `json:"write_started"`
}

// DeepPersister implements the exact RD08 three-reboot state machine.
type DeepPersister interface {
	DeepPersist(context.Context, DeepParams) (DeepResult, error)
}
