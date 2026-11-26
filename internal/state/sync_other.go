//go:build !windows

package state

import "os"

// syncDir makes the rename durable on POSIX file systems.
func syncDir(dir string) error {
	handle, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer func() { _ = handle.Close() }()
	return handle.Sync()
}
