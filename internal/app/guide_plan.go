package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/sunerpy/router-tools/internal/device"
)

// Guided goals. SSH is always part of a guided run; reboot verification requires soft persistence.
const (
	GoalSSH    = "ssh"
	GoalBackup = "backup"
	GoalSoft   = "soft"
	GoalReboot = "reboot"
)

// Guided steps, always planned and executed in this order.
const (
	StepEnableSSH    = "enable_ssh"
	StepBackup       = "backup"
	StepSoftInstall  = "soft_install"
	StepRebootVerify = "reboot_verify"
)

// Step actions. not_run is used only in apply results for steps after the first one that did not succeed.
const (
	ActionRun         = "run"
	ActionSkip        = "skip"
	ActionDecideAfter = "decide_after"
	ActionNeedsInput  = "needs_input"
	ActionBlocked     = "blocked"
	ActionUnsupported = "unsupported"
	ActionNotRun      = "not_run"
)

// Effects a run step has on the router or on local disk. Frontends translate them for the confirmation.
const (
	EffectTemporaryMACRules  = "temporary_mac_rules"
	EffectNVRAMCommit        = "nvram_commit"
	EffectDropbearStart      = "dropbear_start"
	EffectRootPasswordChange = "root_password_change"
	EffectLocalBackup        = "local_backup"
	EffectScriptWrite        = "script_write"
	EffectUCICommit          = "uci_commit"
	EffectReboot             = "reboot"
)

// Inputs a plan asks for. new_root_password is the password the enable flow sets; frontends ask for it twice.
const (
	InputAdminPassword     = "admin_password"
	InputRootPassword      = "root_password"
	InputNewRootPassword   = "new_root_password"
	InputResetRootPassword = "reset_root_password"
)

// Reason codes explain each step's action.
const (
	ReasonRootSSHVerified        = "root_ssh_verified"
	ReasonSSHClosed              = "ssh_closed"
	ReasonRootPasswordMissing    = "root_password_missing"
	ReasonSSHAuthFailed          = "ssh_auth_failed"
	ReasonResetRootPassword      = "reset_root_password"
	ReasonBackupVerified         = "backup_verified"
	ReasonBackupMissing          = "backup_missing"
	ReasonBackupRootMissing      = "backup_root_missing"
	ReasonAfterSSH               = "after_ssh"
	ReasonSoftInstalledVerified  = "soft_installed_verified"
	ReasonSoftNotInstalled       = "soft_not_installed"
	ReasonSoftFileDrift          = "soft_file_drift"
	ReasonSoftModeDrift          = "soft_mode_drift"
	ReasonSoftUCIDrift           = "soft_uci_drift"
	ReasonSoftConflict           = "soft_conflict"
	ReasonSoftOrphanFile         = "soft_orphan_file"
	ReasonRebootSelected         = "reboot_selected"
	ReasonCapabilityMissing      = "capability_missing"
	ReasonIncompatible           = "incompatible"
	ReasonHostKeyRejected        = "host_key_rejected"
	ReasonRecoveryReviewRequired = "recovery_review_required"
	ReasonOperationLocked        = "operation_locked"
	ReasonDetectionFailed        = "detection_failed"
)

// Maintenance actions named by a blocked soft step; they are never folded into a guided run.
const (
	MaintenancePersistRemove = "persist_remove"
	MaintenanceManualReview  = "manual_review"
)

// Root SSH evidence collected while planning.
const (
	RootSSHVerified   = "verified"
	RootSSHAuthFailed = "auth_failed"
	RootSSHNotChecked = "not_checked"
)

var (
	enableStepEffects = []string{EffectTemporaryMACRules, EffectNVRAMCommit, EffectDropbearStart, EffectRootPasswordChange}
	softStepEffects   = []string{EffectScriptWrite, EffectUCICommit}
)

// GuideStep is one planned step.
type GuideStep struct {
	ID          string   `json:"id"`
	Action      string   `json:"action"`
	Reason      string   `json:"reason"`
	Effects     []string `json:"effects,omitempty"`
	Inputs      []string `json:"inputs,omitempty"`
	Maintenance string   `json:"maintenance,omitempty"`
}

// guideFacts is everything the planner decides from; it contains no secret.
type guideFacts struct {
	Goals         []string
	Profile       device.Profile
	SSH           device.PortState
	RootSSH       string
	Soft          SoftState
	BackupFound   bool
	BackupManaged bool
	Reset         bool
	// Block is a plan-level reason (incompatible, host key, recovery review, lock) that blocks every step.
	Block string
}

// normalizeGoals returns the goals in step order with ssh always first.
func normalizeGoals(goals []string) ([]string, error) {
	selected := map[string]bool{GoalSSH: true}
	for _, goal := range goals {
		switch goal {
		case GoalSSH, GoalBackup, GoalSoft, GoalReboot:
			selected[goal] = true
		default:
			return nil, &device.Error{Code: device.CodeInvalidArgument, Message: "goals must be ssh, backup, soft, or reboot"}
		}
	}
	if selected[GoalReboot] && !selected[GoalSoft] {
		return nil, &device.Error{Code: device.CodeInvalidArgument, Message: "reboot verification requires the soft goal"}
	}
	var ordered []string
	for _, goal := range []string{GoalSSH, GoalBackup, GoalSoft, GoalReboot} {
		if selected[goal] {
			ordered = append(ordered, goal)
		}
	}
	return ordered, nil
}

// planSteps is the pure planner: the ordered steps of the selected goals for the observed state.
func planSteps(facts guideFacts) []GuideStep {
	selected := make(map[string]bool, len(facts.Goals))
	for _, goal := range facts.Goals {
		selected[goal] = true
	}
	var ids []string
	for _, entry := range []struct{ goal, step string }{
		{GoalSSH, StepEnableSSH}, {GoalBackup, StepBackup}, {GoalSoft, StepSoftInstall}, {GoalReboot, StepRebootVerify},
	} {
		if selected[entry.goal] {
			ids = append(ids, entry.step)
		}
	}
	steps := make([]GuideStep, 0, len(ids))
	for _, id := range ids {
		if facts.Block != "" {
			steps = append(steps, GuideStep{ID: id, Action: ActionBlocked, Reason: facts.Block})
			continue
		}
		steps = append(steps, planStep(id, facts))
	}
	return steps
}

func planStep(id string, facts guideFacts) GuideStep {
	unsupported := GuideStep{ID: id, Action: ActionUnsupported, Reason: ReasonCapabilityMissing}
	if !facts.Profile.Supports(device.CapabilityVerify) {
		return unsupported
	}
	verified := facts.RootSSH == RootSSHVerified
	switch id {
	case StepEnableSSH:
		if verified {
			return GuideStep{ID: id, Action: ActionSkip, Reason: ReasonRootSSHVerified}
		}
		reason := ""
		switch {
		case facts.SSH != device.PortOpen:
			reason = ReasonSSHClosed
		case facts.Reset:
			reason = ReasonResetRootPassword
		case facts.RootSSH == RootSSHAuthFailed:
			return GuideStep{ID: id, Action: ActionNeedsInput, Reason: ReasonSSHAuthFailed, Inputs: []string{InputRootPassword, InputResetRootPassword}}
		default:
			return GuideStep{ID: id, Action: ActionNeedsInput, Reason: ReasonRootPasswordMissing, Inputs: []string{InputRootPassword, InputResetRootPassword}}
		}
		if !facts.Profile.Supports(device.CapabilityEnableSSH) {
			return unsupported
		}
		return GuideStep{ID: id, Action: ActionRun, Reason: reason, Effects: enableStepEffects, Inputs: []string{InputAdminPassword, InputNewRootPassword}}
	case StepBackup:
		switch {
		case !facts.Profile.Supports(device.CapabilityBackup):
			return unsupported
		case !facts.BackupManaged:
			return GuideStep{ID: id, Action: ActionUnsupported, Reason: ReasonBackupRootMissing}
		case facts.BackupFound:
			return GuideStep{ID: id, Action: ActionSkip, Reason: ReasonBackupVerified}
		default:
			return GuideStep{ID: id, Action: ActionRun, Reason: ReasonBackupMissing, Effects: []string{EffectLocalBackup}}
		}
	case StepSoftInstall:
		if !facts.Profile.Supports(device.CapabilitySoftPersist) {
			return unsupported
		}
		if !verified {
			return GuideStep{ID: id, Action: ActionDecideAfter, Reason: ReasonAfterSSH, Effects: softStepEffects}
		}
		return softStep(facts.Soft)
	case StepRebootVerify:
		if !facts.Profile.Supports(device.CapabilitySoftPersist) {
			return unsupported
		}
		return GuideStep{ID: id, Action: ActionRun, Reason: ReasonRebootSelected, Effects: []string{EffectReboot}}
	}
	return unsupported
}

// softStep maps an inspected soft-persistence state to the soft step.
func softStep(soft SoftState) GuideStep {
	id := StepSoftInstall
	switch soft {
	case SoftInstalledVerified:
		return GuideStep{ID: id, Action: ActionSkip, Reason: ReasonSoftInstalledVerified}
	case SoftNotInstalled:
		return GuideStep{ID: id, Action: ActionRun, Reason: ReasonSoftNotInstalled, Effects: softStepEffects}
	case SoftFileDrift:
		return GuideStep{ID: id, Action: ActionRun, Reason: ReasonSoftFileDrift, Effects: softStepEffects}
	case SoftModeDrift:
		return GuideStep{ID: id, Action: ActionBlocked, Reason: ReasonSoftModeDrift, Maintenance: MaintenancePersistRemove}
	case SoftUCIDrift:
		return GuideStep{ID: id, Action: ActionBlocked, Reason: ReasonSoftUCIDrift, Maintenance: MaintenancePersistRemove}
	case SoftOrphanFile:
		return GuideStep{ID: id, Action: ActionBlocked, Reason: ReasonSoftOrphanFile, Maintenance: MaintenanceManualReview}
	default:
		return GuideStep{ID: id, Action: ActionBlocked, Reason: ReasonSoftConflict, Maintenance: MaintenanceManualReview}
	}
}

// requiredInputs lists what the frontend must supply: the unresolved inputs while any step needs input,
// otherwise the passwords the confirmed run uses.
func requiredInputs(steps []GuideStep) []string {
	var asked []string
	enableRuns, rootNeeded := false, false
	for _, step := range steps {
		switch step.Action {
		case ActionNeedsInput:
			asked = appendMissing(asked, step.Inputs...)
		case ActionRun, ActionDecideAfter:
			if step.ID == StepEnableSSH {
				enableRuns = true
			} else {
				rootNeeded = true
			}
		}
	}
	switch {
	case len(asked) > 0:
		return asked
	case enableRuns:
		return []string{InputAdminPassword, InputNewRootPassword}
	case rootNeeded:
		return []string{InputRootPassword}
	}
	return nil
}

func appendMissing(list []string, values ...string) []string {
	for _, value := range values {
		found := false
		for _, existing := range list {
			found = found || existing == value
		}
		if !found {
			list = append(list, value)
		}
	}
	return list
}

// ConfirmedIdentity is the device identity a confirmation binds; every guided write checks it again after
// taking the target lock.
type ConfirmedIdentity struct {
	DeviceID    string `json:"device_id"`
	Hardware    string `json:"hardware"`
	DisplayName string `json:"display_name,omitempty"`
	ROMVersion  string `json:"rom_version"`
	ProfileID   string `json:"profile_id"`
}

// GuideConfirmedStep is the part of a step the user confirms.
type GuideConfirmedStep struct {
	ID      string   `json:"id"`
	Action  string   `json:"action"`
	Effects []string `json:"effects,omitempty"`
}

// GuideConfirmation is shown to the user and sent back unchanged with its digest to apply the plan. It
// holds no secret.
type GuideConfirmation struct {
	SchemaVersion     int                  `json:"schema_version"`
	Target            device.Target        `json:"target"`
	Identity          ConfirmedIdentity    `json:"identity"`
	Goals             []string             `json:"goals"`
	ResetRootPassword bool                 `json:"reset_root_password"`
	Steps             []GuideConfirmedStep `json:"steps"`
}

// planDigest is the hex SHA-256 of the confirmation's canonical JSON encoding.
func planDigest(confirmation GuideConfirmation) string {
	data, err := json.Marshal(confirmation)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func confirmedIdentity(observation device.Observation, profile device.Profile) ConfirmedIdentity {
	identity := backupIdentity(observation, profile)
	return ConfirmedIdentity{
		DeviceID: identity.DeviceID, Hardware: identity.Hardware, DisplayName: observation.DisplayName,
		ROMVersion: identity.ROMVersion, ProfileID: profile.ID,
	}
}

// checkExpectedIdentity runs after the lock and resolution and before any write of a guided step.
func checkExpectedIdentity(expected *ConfirmedIdentity, observation device.Observation, profile device.Profile) error {
	if expected == nil || confirmedIdentity(observation, profile) == *expected {
		return nil
	}
	return &device.Error{Code: device.CodeConfirmationRequired, Message: "the device no longer matches the confirmed plan; detect again"}
}
