// Package wiring is the shared composition root of routerctl and the desktop GUI: the compiled-in
// adapters, the discovery prober, the cross-process lock, the recovery outcome store and the managed
// directories. Both mains call NewService, so the production graph cannot differ between them.
package wiring

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/credentials"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/devices/builtin"
	"github.com/sunerpy/router-tools/internal/safety/oplock"
	"github.com/sunerpy/router-tools/internal/state"
)

// Paths are the per-user directories the service writes to.
type Paths struct {
	Locks       string // <UserConfigDir>/routerctl/locks
	State       string // <UserConfigDir>/routerctl/state: write-ahead outcome records
	DeepJournal string // <UserConfigDir>/routerctl/deep: managed deep journals
	Backups     string // <UserHomeDir>/RouterTools/backups: managed partition backups (not roaming)
	Credentials string // <UserConfigDir>/routerctl/credentials: the encrypted password vault
}

// DefaultPaths resolves the managed directories for the current user. It fails instead of falling back to
// the working directory.
func DefaultPaths() (Paths, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, fmt.Errorf("locate the user configuration directory: %w", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("locate the user home directory: %w", err)
	}
	base := filepath.Join(configDir, "routerctl")
	return Paths{
		Locks:       filepath.Join(base, "locks"),
		State:       filepath.Join(base, "state"),
		DeepJournal: filepath.Join(base, "deep"),
		Backups:     filepath.Join(home, "RouterTools", "backups"),
		Credentials: filepath.Join(base, "credentials"),
	}, nil
}

// NewService builds the production application service. It always carries the recovery outcome store.
func NewService(paths Paths) (*app.Service, error) {
	if paths.Locks == "" || paths.State == "" || paths.DeepJournal == "" || paths.Backups == "" || paths.Credentials == "" {
		return nil, fmt.Errorf("every managed directory must be set")
	}
	registry, err := device.NewRegistry(builtin.Adapters()...)
	if err != nil {
		return nil, fmt.Errorf("initialize device registry: %w", err)
	}
	return app.NewServiceWithOptions(registry, builtin.Discovery(), app.Options{
		Locker:      oplock.FlockLocker{Dir: paths.Locks},
		Outcomes:    state.FileStore{Dir: paths.State},
		Credentials: credentials.FileVault{Dir: paths.Credentials},
		BackupRoot:  paths.Backups,
		JournalRoot: paths.DeepJournal,
	})
}
