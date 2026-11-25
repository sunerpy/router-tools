package device

import "context"

// PortState is connectivity evidence only, never proof of authentication.
type PortState string

const (
	PortOpen        PortState = "open"
	PortClosed      PortState = "closed"
	PortUnreachable PortState = "unreachable"
)

// ProbeResult contains read-only device and port observations.
type ProbeResult struct {
	Observation Observation `json:"observation"`
	SSH         PortState   `json:"ssh"`
	Telnet      PortState   `json:"telnet"`
}

// Prober collects read-only evidence for compatibility resolution.
type Prober interface {
	Probe(ctx context.Context, target Target) (ProbeResult, error)
}
