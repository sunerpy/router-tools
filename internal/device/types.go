package device

// Capability identifies an operation implemented by a device adapter.
type Capability string

const (
	CapabilityDoctor      Capability = "doctor"
	CapabilityWebAuth     Capability = "web_auth"
	CapabilityVerify      Capability = "verify"
	CapabilityEnableSSH   Capability = "enable_ssh"
	CapabilitySoftPersist Capability = "soft_persist"
	CapabilityBackup      Capability = "backup"
	CapabilityDeepPersist Capability = "deep_persist"
)

// Profile describes one exact model/firmware adapter. ID is always ModelID + "-" + Firmware.
type Profile struct {
	ID           string       `json:"id"`
	ModelID      string       `json:"model_id"`
	Vendor       string       `json:"vendor"`
	Model        string       `json:"model"`
	Hardware     string       `json:"hardware"`
	Firmware     string       `json:"firmware"`
	Capabilities []Capability `json:"capabilities"`
	Deep         *DeepPolicy  `json:"deep,omitempty"`
}

// DeepPolicy holds the deep-persistence safety parameters of one profile. It is present exactly when the
// profile declares CapabilityDeepPersist.
type DeepPolicy struct {
	// Acknowledgement is the phrase the operator must type; interfaces display it but never prefill it.
	Acknowledgement string `json:"acknowledgement"`
	// RebootTransitions is the exact number of observed reboots a successful run requires.
	RebootTransitions int `json:"reboot_transitions"`
}

// Supports reports whether the adapter implements a capability.
func (p Profile) Supports(capability Capability) bool {
	for _, candidate := range p.Capabilities {
		if candidate == capability {
			return true
		}
	}
	return false
}

// Observation contains only read-only evidence collected from a device.
type Observation struct {
	DeviceID       string `json:"device_id"`
	Hardware       string `json:"hardware,omitempty"`
	DisplayName    string `json:"display_name,omitempty"`
	ROMVersion     string `json:"rom_version"`
	NewEncryptMode int    `json:"new_encrypt_mode"`
}

// MatchLevel describes how closely an adapter matches an observation.
type MatchLevel int

const (
	NoMatch MatchLevel = iota
	DeviceMatch
	ExactMatch
)

// MatchResult is the adapter's compatibility decision.
type MatchResult struct {
	Level  MatchLevel
	Reason string
}

// Driver owns compatibility policy for one model and firmware.
type Driver interface {
	Profile() Profile
	Match(observation Observation) MatchResult
}
