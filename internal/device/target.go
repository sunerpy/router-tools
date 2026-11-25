package device

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// Target identifies one router and its management ports.
type Target struct {
	Host       string `json:"host"`
	WebPort    int    `json:"web_port"`
	SSHPort    int    `json:"ssh_port"`
	TelnetPort int    `json:"telnet_port"`
}

// DefaultTarget returns the RD08 defaults without device-specific behavior.
func DefaultTarget() Target {
	return Target{Host: "192.168.31.1", WebPort: 80, SSHPort: 22, TelnetPort: 23}
}

// Validate rejects malformed targets before any network access.
func (t Target) Validate() error {
	if !validHost(t.Host) {
		return &Error{Code: CodeInvalidArgument, Message: "host must be an IP address or hostname without a scheme or path"}
	}
	for name, port := range map[string]int{"Web": t.WebPort, "SSH": t.SSHPort, "Telnet": t.TelnetPort} {
		if port < 1 || port > 65535 {
			return &Error{Code: CodeInvalidArgument, Message: fmt.Sprintf("%s port must be in [1, 65535]", name)}
		}
	}
	return nil
}

// WebBaseURL constructs the fixed plain-HTTP management origin.
func (t Target) WebBaseURL() (string, error) {
	if err := t.Validate(); err != nil {
		return "", err
	}
	return (&url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(t.Host, strconv.Itoa(t.WebPort)),
	}).String(), nil
}

func validHost(host string) bool {
	if host == "" || strings.TrimSpace(host) != host || strings.ContainsAny(host, "/\\?#@") {
		return false
	}
	if net.ParseIP(host) != nil {
		return true
	}
	if strings.Contains(host, ":") || len(host) > 253 || strings.Contains(host, "..") {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' {
				return false
			}
		}
	}
	return true
}
