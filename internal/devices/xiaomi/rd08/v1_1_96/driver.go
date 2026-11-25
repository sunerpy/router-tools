// Package rd08v1196 implements the Xiaomi RD08 firmware 1.1.96 adapter.
package rd08v1196

import (
	"strings"

	"github.com/sunerpy/router-tools/internal/device"
)

const (
	profileID = "xiaomi-rd08-1.1.96"
	firmware  = "1.1.96"
)

// Driver implements RD08 1.1.96 compatibility policy.
type Driver struct{}

// New constructs the RD08 1.1.96 adapter.
func New() *Driver { return &Driver{} }

// Profile returns immutable adapter metadata.
func (d *Driver) Profile() device.Profile {
	return device.Profile{
		ID:       profileID,
		Vendor:   "Xiaomi",
		Model:    "BE6500 Pro",
		Hardware: "RD08",
		Firmware: firmware,
		Capabilities: []device.Capability{
			device.CapabilityDoctor,
			device.CapabilityWebAuth,
			device.CapabilityVerify,
			device.CapabilityEnableSSH,
			device.CapabilitySoftPersist,
			device.CapabilityBackup,
			device.CapabilityDeepPersist,
		},
	}
}

// Match follows the fail-closed RD08 identity rules from the implementation specification.
func (d *Driver) Match(observation device.Observation) device.MatchResult {
	hardware := strings.TrimSpace(observation.Hardware)
	deviceMatched := false
	switch {
	case hardware != "":
		deviceMatched = strings.EqualFold(hardware, "RD08")
	case strings.TrimSpace(observation.DisplayName) != "":
		deviceMatched = strings.Contains(
			strings.ToLower(observation.DisplayName),
			"be6500 pro",
		)
	}
	if !deviceMatched {
		return device.MatchResult{Level: device.NoMatch, Reason: "hardware or display name does not identify RD08"}
	}
	if strings.TrimSpace(observation.ROMVersion) != firmware {
		return device.MatchResult{Level: device.DeviceMatch, Reason: "RD08 firmware is not exactly 1.1.96"}
	}
	return device.MatchResult{Level: device.ExactMatch}
}
