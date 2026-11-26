//go:build windows

package state

// syncDir is a no-op on Windows, where directory handles cannot be flushed; the renamed file itself was
// synced before the rename.
func syncDir(string) error { return nil }
