// Package oplock implements per-target non-blocking operation locks.
package oplock

import (
	"crypto/sha256"
	"fmt"
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
	key := fmt.Sprintf("%s|%d|%d|%d", target.Host, target.WebPort, target.SSHPort, target.TelnetPort)
	name := fmt.Sprintf("%x.lock", sha256.Sum256([]byte(key)))
	path := filepath.Join(l.Dir, name)
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
