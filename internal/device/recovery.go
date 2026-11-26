package device

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"strings"
)

// RecoveryKey identifies one router for locks, recovery records and managed directories. It depends only
// on the canonical host, never on management ports, so changing a port cannot select another lock,
// outcome record or journal directory for the same router.
func RecoveryKey(target Target) string {
	sum := sha256.Sum256([]byte("router-tools/recovery/v1\x00" + CanonicalHost(target.Host)))
	return hex.EncodeToString(sum[:])
}

// CanonicalHost is the host spelling every per-router name derives from: IP literals in Go's canonical
// form, host names in lower case without a trailing dot.
func CanonicalHost(host string) string {
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}
	return strings.TrimSuffix(strings.ToLower(host), ".")
}
