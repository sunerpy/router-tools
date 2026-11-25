package app

// SoftResult distinguishes installation verification from reboot verification.
type SoftResult struct {
	Outcome        string   `json:"outcome"`
	Verified       bool     `json:"verified"`
	RebootVerified bool     `json:"reboot_verified"`
	Warnings       []string `json:"warnings,omitempty"`
}

// RebootResult records one verified boot transition and final SSH/soft evidence.
type RebootResult struct {
	OldBootID    string `json:"old_boot_id"`
	NewBootID    string `json:"new_boot_id"`
	SSHVerified  bool   `json:"ssh_verified"`
	SoftVerified bool   `json:"soft_verified"`
}
