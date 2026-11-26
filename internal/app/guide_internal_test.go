package app

import (
	"reflect"
	"testing"

	"github.com/sunerpy/router-tools/internal/device"
)

func guideTestProfile(capabilities ...device.Capability) device.Profile {
	if len(capabilities) == 0 {
		capabilities = []device.Capability{
			device.CapabilityDoctor, device.CapabilityVerify, device.CapabilityEnableSSH,
			device.CapabilitySoftPersist, device.CapabilityBackup,
		}
	}
	return device.Profile{ID: "acme-r1-1.0", ModelID: "acme-r1", Hardware: "R1", Firmware: "1.0", Capabilities: capabilities}
}

var enableEffects = []string{EffectTemporaryMACRules, EffectNVRAMCommit, EffectDropbearStart, EffectRootPasswordChange}

func TestPlanStepsRules(t *testing.T) {
	t.Parallel()

	allGoals := []string{GoalSSH, GoalBackup, GoalSoft}
	base := func(mutate func(*guideFacts)) guideFacts {
		facts := guideFacts{
			Goals: allGoals, Profile: guideTestProfile(), SSH: device.PortOpen, RootSSH: RootSSHVerified,
			Soft: SoftInstalledVerified, BackupFound: true, BackupManaged: true,
		}
		mutate(&facts)
		return facts
	}
	step := func(id, action, reason string, effects, inputs []string) GuideStep {
		return GuideStep{ID: id, Action: action, Reason: reason, Effects: effects, Inputs: inputs}
	}
	enableRun := func(reason string) GuideStep {
		return step(StepEnableSSH, ActionRun, reason, enableEffects, []string{InputAdminPassword, InputNewRootPassword})
	}
	enableAsk := func(reason string) GuideStep {
		return step(StepEnableSSH, ActionNeedsInput, reason, nil, []string{InputRootPassword, InputResetRootPassword})
	}
	enableSkip := step(StepEnableSSH, ActionSkip, ReasonRootSSHVerified, nil, nil)
	backupSkip := step(StepBackup, ActionSkip, ReasonBackupVerified, nil, nil)
	backupRun := step(StepBackup, ActionRun, ReasonBackupMissing, []string{EffectLocalBackup}, nil)
	softEffects := []string{EffectScriptWrite, EffectUCICommit}
	softSkip := step(StepSoftInstall, ActionSkip, ReasonSoftInstalledVerified, nil, nil)
	softLater := step(StepSoftInstall, ActionDecideAfter, ReasonAfterSSH, softEffects, nil)
	softBlocked := func(reason, maintenance string) GuideStep {
		s := step(StepSoftInstall, ActionBlocked, reason, nil, nil)
		s.Maintenance = maintenance
		return s
	}

	cases := []struct {
		name   string
		facts  guideFacts
		steps  []GuideStep
		inputs []string
	}{
		{
			name: "fresh device with SSH closed",
			facts: base(func(f *guideFacts) {
				f.SSH, f.RootSSH, f.Soft, f.BackupFound = device.PortClosed, RootSSHNotChecked, "", false
			}),
			steps:  []GuideStep{enableRun(ReasonSSHClosed), backupRun, softLater},
			inputs: []string{InputAdminPassword, InputNewRootPassword},
		},
		{
			name:  "unreachable SSH port is treated as closed",
			facts: base(func(f *guideFacts) { f.SSH, f.RootSSH, f.Soft = device.PortUnreachable, RootSSHNotChecked, "" }),
			steps: []GuideStep{enableRun(ReasonSSHClosed), backupSkip, softLater}, inputs: []string{InputAdminPassword, InputNewRootPassword},
		},
		{
			name:  "already configured: nothing to do",
			facts: base(func(*guideFacts) {}),
			steps: []GuideStep{enableSkip, backupSkip, softSkip},
		},
		{
			name:  "SSH open without a root password asks for it",
			facts: base(func(f *guideFacts) { f.RootSSH, f.Soft = RootSSHNotChecked, "" }),
			steps: []GuideStep{enableAsk(ReasonRootPasswordMissing), backupSkip, softLater}, inputs: []string{InputRootPassword, InputResetRootPassword},
		},
		{
			name:  "SSH open with a rejected root password asks again",
			facts: base(func(f *guideFacts) { f.RootSSH, f.Soft = RootSSHAuthFailed, "" }),
			steps: []GuideStep{enableAsk(ReasonSSHAuthFailed), backupSkip, softLater}, inputs: []string{InputRootPassword, InputResetRootPassword},
		},
		{
			name:  "explicit reset runs the enable flow and changes the root password",
			facts: base(func(f *guideFacts) { f.RootSSH, f.Soft, f.Reset = RootSSHAuthFailed, "", true }),
			steps: []GuideStep{enableRun(ReasonResetRootPassword), backupSkip, softLater}, inputs: []string{InputAdminPassword, InputNewRootPassword},
		},
		{
			name:  "reset is ignored while root SSH already works",
			facts: base(func(f *guideFacts) { f.Reset = true }),
			steps: []GuideStep{enableSkip, backupSkip, softSkip},
		},
		{
			name:  "missing backup and soft persistence run over the verified shell",
			facts: base(func(f *guideFacts) { f.Soft, f.BackupFound = SoftNotInstalled, false }),
			steps: []GuideStep{
				enableSkip, backupRun,
				step(StepSoftInstall, ActionRun, ReasonSoftNotInstalled, softEffects, nil),
			},
			inputs: []string{InputRootPassword},
		},
		{
			name:  "file drift is repaired by an install",
			facts: base(func(f *guideFacts) { f.Soft = SoftFileDrift }),
			steps: []GuideStep{enableSkip, backupSkip, step(StepSoftInstall, ActionRun, ReasonSoftFileDrift, softEffects, nil)}, inputs: []string{InputRootPassword},
		},
		{
			name:  "mode drift needs a separate remove",
			facts: base(func(f *guideFacts) { f.Soft = SoftModeDrift }),
			steps: []GuideStep{enableSkip, backupSkip, softBlocked(ReasonSoftModeDrift, MaintenancePersistRemove)},
		},
		{
			name:  "UCI drift needs a separate remove",
			facts: base(func(f *guideFacts) { f.Soft = SoftUCIDrift }),
			steps: []GuideStep{enableSkip, backupSkip, softBlocked(ReasonSoftUCIDrift, MaintenancePersistRemove)},
		},
		{
			name:  "a foreign configuration is never touched",
			facts: base(func(f *guideFacts) { f.Soft = SoftConflict }),
			steps: []GuideStep{enableSkip, backupSkip, softBlocked(ReasonSoftConflict, MaintenanceManualReview)},
		},
		{
			name:  "an orphan script needs a manual review",
			facts: base(func(f *guideFacts) { f.Soft = SoftOrphanFile }),
			steps: []GuideStep{enableSkip, backupSkip, softBlocked(ReasonSoftOrphanFile, MaintenanceManualReview)},
		},
		{
			name:  "reboot verification always runs when selected",
			facts: base(func(f *guideFacts) { f.Goals = []string{GoalSSH, GoalBackup, GoalSoft, GoalReboot} }),
			steps: []GuideStep{
				enableSkip, backupSkip, softSkip,
				step(StepRebootVerify, ActionRun, ReasonRebootSelected, []string{EffectReboot}, nil),
			},
			inputs: []string{InputRootPassword},
		},
		{
			name:  "only the selected goals have steps",
			facts: base(func(f *guideFacts) { f.Goals = []string{GoalSSH} }),
			steps: []GuideStep{enableSkip},
		},
		{
			name: "a plan-level block blocks every step and plans no write",
			facts: base(func(f *guideFacts) {
				f.SSH, f.RootSSH, f.Soft, f.BackupFound, f.Block = device.PortClosed, RootSSHNotChecked, "", false, ReasonRecoveryReviewRequired
			}),
			steps: []GuideStep{
				step(StepEnableSSH, ActionBlocked, ReasonRecoveryReviewRequired, nil, nil),
				step(StepBackup, ActionBlocked, ReasonRecoveryReviewRequired, nil, nil),
				step(StepSoftInstall, ActionBlocked, ReasonRecoveryReviewRequired, nil, nil),
			},
		},
		{
			name: "capabilities the profile lacks are unsupported",
			facts: base(func(f *guideFacts) {
				f.Profile = guideTestProfile(device.CapabilityVerify)
				f.SSH, f.RootSSH, f.Soft, f.BackupFound = device.PortClosed, RootSSHNotChecked, "", false
			}),
			steps: []GuideStep{
				step(StepEnableSSH, ActionUnsupported, ReasonCapabilityMissing, nil, nil),
				step(StepBackup, ActionUnsupported, ReasonCapabilityMissing, nil, nil),
				step(StepSoftInstall, ActionUnsupported, ReasonCapabilityMissing, nil, nil),
			},
		},
		{
			name: "an unneeded enable is skipped even without the capability",
			facts: base(func(f *guideFacts) {
				f.Profile = guideTestProfile(device.CapabilityVerify, device.CapabilitySoftPersist, device.CapabilityBackup)
			}),
			steps: []GuideStep{enableSkip, backupSkip, softSkip},
		},
		{
			name:  "without a managed backup root the backup step cannot run",
			facts: base(func(f *guideFacts) { f.BackupFound, f.BackupManaged = false, false }),
			steps: []GuideStep{enableSkip, step(StepBackup, ActionUnsupported, ReasonBackupRootMissing, nil, nil), softSkip},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			steps := planSteps(tc.facts)
			if !reflect.DeepEqual(steps, tc.steps) {
				t.Fatalf("planSteps() =\n%+v\nwant\n%+v", steps, tc.steps)
			}
			if inputs := requiredInputs(steps); !reflect.DeepEqual(inputs, tc.inputs) {
				t.Fatalf("requiredInputs() = %v, want %v", inputs, tc.inputs)
			}
		})
	}
}

func TestNormalizeGoals(t *testing.T) {
	t.Parallel()

	got, err := normalizeGoals([]string{GoalSoft, GoalBackup, GoalSoft})
	if err != nil || !reflect.DeepEqual(got, []string{GoalSSH, GoalBackup, GoalSoft}) {
		t.Fatalf("normalizeGoals() = %v, %v; want ssh first, canonical order, no duplicates", got, err)
	}
	if got, err := normalizeGoals(nil); err != nil || !reflect.DeepEqual(got, []string{GoalSSH}) {
		t.Fatalf("normalizeGoals(nil) = %v, %v; want ssh only", got, err)
	}
	for _, goals := range [][]string{{GoalReboot}, {"deep"}, {""}} {
		if _, err := normalizeGoals(goals); err == nil {
			t.Fatalf("normalizeGoals(%v) succeeded, want INVALID_ARGUMENT", goals)
		}
	}
}

func TestPlanDigestIsStableAndCoversEveryConfirmedField(t *testing.T) {
	t.Parallel()

	confirmation := func() GuideConfirmation {
		return GuideConfirmation{
			SchemaVersion: 1,
			Target:        device.DefaultTarget(),
			Identity:      ConfirmedIdentity{DeviceID: "DEVICE", Hardware: "R1", DisplayName: "Acme R1", ROMVersion: "1.0", ProfileID: "acme-r1-1.0"},
			Goals:         []string{GoalSSH, GoalBackup, GoalSoft},
			Steps: []GuideConfirmedStep{
				{ID: StepEnableSSH, Action: ActionRun, Effects: enableEffects},
				{ID: StepBackup, Action: ActionRun, Effects: []string{EffectLocalBackup}},
				{ID: StepSoftInstall, Action: ActionDecideAfter, Effects: []string{EffectScriptWrite, EffectUCICommit}},
			},
		}
	}
	digest := planDigest(confirmation())
	if digest != planDigest(confirmation()) || len(digest) != 64 {
		t.Fatalf("digest %q is not a stable SHA-256 hex", digest)
	}
	mutations := map[string]func(*GuideConfirmation){
		"host":        func(c *GuideConfirmation) { c.Target.Host = "192.168.31.2" },
		"ssh port":    func(c *GuideConfirmation) { c.Target.SSHPort = 2222 },
		"device id":   func(c *GuideConfirmation) { c.Identity.DeviceID = "OTHER" },
		"hardware":    func(c *GuideConfirmation) { c.Identity.Hardware = "R2" },
		"display":     func(c *GuideConfirmation) { c.Identity.DisplayName = "Acme R2" },
		"firmware":    func(c *GuideConfirmation) { c.Identity.ROMVersion = "1.1" },
		"profile":     func(c *GuideConfirmation) { c.Identity.ProfileID = "acme-r1-1.1" },
		"goals":       func(c *GuideConfirmation) { c.Goals = c.Goals[:2] },
		"reset":       func(c *GuideConfirmation) { c.ResetRootPassword = true },
		"step action": func(c *GuideConfirmation) { c.Steps[1].Action = ActionSkip },
		"effects":     func(c *GuideConfirmation) { c.Steps[0].Effects = c.Steps[0].Effects[:3] },
		"step added": func(c *GuideConfirmation) {
			c.Steps = append(c.Steps, GuideConfirmedStep{ID: StepRebootVerify, Action: ActionRun, Effects: []string{EffectReboot}})
		},
	}
	for name, mutate := range mutations {
		changed := confirmation()
		mutate(&changed)
		if planDigest(changed) == digest {
			t.Fatalf("changing %s kept the digest", name)
		}
	}
}
