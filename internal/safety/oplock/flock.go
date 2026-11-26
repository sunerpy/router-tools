// Package oplock implements per-router non-blocking operation locks.
package oplock

import (
	"os"
	"path/filepath"

	"github.com/gofrs/flock"

	"github.com/sunerpy/router-tools/internal/device"
)

// FlockLocker coordinates independent CLI/GUI processes.
type FlockLocker struct {
	Dir string
}

// Acquire takes the target lock or fails immediately without queueing a write.
func (l FlockLocker) Acquire(target device.Target) (func() error, error) {
	if err := os.MkdirAll(l.Dir, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(l.Dir, device.RecoveryKey(target)+".lock")
	lock := flock.New(path)
	acquired, err := lock.TryLock()
	if err != nil {
		return nil, err
	}
	if !acquired {
		return nil, &device.Error{Code: device.CodeOperationLocked, Message: "another operation already holds this target lock"}
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = lock.Unlock()
		return nil, err
	}
	return lock.Unlock, nil
}
