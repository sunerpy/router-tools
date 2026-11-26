//go:build !windows

package credentials

// protectKey stores the raw key on macOS and Linux, where it relies on the 0600 file mode alone; any
// program running as the same user can read it.
func protectKey(raw []byte) ([]byte, error) { return append([]byte(nil), raw...), nil }

func unprotectKey(blob []byte) ([]byte, error) { return append([]byte(nil), blob...), nil }
