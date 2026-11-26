//go:build !windows

package mcpserver

import "syscall"

// errAddressInUse is the bind error for a port another socket holds.
var errAddressInUse error = syscall.EADDRINUSE
