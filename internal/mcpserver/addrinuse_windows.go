//go:build windows

package mcpserver

import "syscall"

// errAddressInUse is WSAEADDRINUSE, the bind error for a port another socket holds.
var errAddressInUse error = syscall.Errno(10048)
