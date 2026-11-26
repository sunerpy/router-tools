package cli_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/state"
)

// fakeGuideService returns its plans in order and records every request.
type fakeGuideService struct {
	fakeDoctorService
	plans         []app.GuidePlanReport
	planErrs      []error
	planRequests  []app.GuideRequest
	applyReport   app.GuideApplyReport
	applyErr      error
	applyRequests []app.GuideApplyRequest
	ackReport     app.RecoveryReport
	ackErr        error
	ackTargets    []device.Target
}

func (s *fakeGuideService) GuidePlan(_ context.Context, request app.GuideRequest, _ app.EventSink) (app.GuidePlanReport, error) {
	s.planRequests = append(s.planRequests, request)
	index := len(s.planRequests) - 1
	if index >= len(s.plans) {
		index = len(s.plans) - 1
	}
	var err error
	if index < len(s.planErrs) {
		err = s.planErrs[index]
	}
	return s.plans[index], err
}

func (s *fakeGuideService) GuideApply(_ context.Context, request app.GuideApplyRequest, _ app.EventSink) (app.GuideApplyReport, error) {
	s.applyRequests = append(s.applyRequests, request)
	if s.applyReport.Status == "" {
		s.applyReport.Status = app.StatusSucceeded
	}
	return s.applyReport, s.applyErr
}

func (s *fakeGuideService) AcknowledgeRecovery(_ context.Context, target device.Target) (app.RecoveryReport, error) {
	s.ackTargets = append(s.ackTargets, target)
	if s.ackReport.Status == "" {
		s.ackReport.Status = app.StatusSucceeded
	}
	return s.ackReport, s.ackErr
}

var enableEffects = []string{app.EffectTemporaryMACRules, app.EffectNVRAMCommit, app.EffectDropbearStart, app.EffectRootPasswordChange}

func freshPlan() app.GuidePlanReport {
	steps := []app.GuideStep{
		{ID: app.StepEnableSSH, Action: app.ActionRun, Reason: app.ReasonSSHClosed, Effects: enableEffects, Inputs: []string{app.InputAdminPassword, app.InputNewRootPassword}},
		{ID: app.StepBackup, Action: app.ActionRun, Reason: app.ReasonBackupMissing, Effects: []string{app.EffectLocalBackup}},
		{ID: app.StepSoftInstall, Action: app.ActionDecideAfter, Reason: app.ReasonAfterSSH, Effects: []string{app.EffectScriptWrite, app.EffectUCICommit}},
	}
	confirmation := &app.GuideConfirmation{
		SchemaVersion: 1, Target: device.DefaultTarget(),
		Identity: app.ConfirmedIdentity{DeviceID: "DEVICE", Hardware: "RD08", ROMVersion: "1.1.96", ProfileID: "xiaomi-rd08-1.1.96"},
		Goals:    []string{app.GoalSSH, app.GoalBackup, app.GoalSoft},
		Steps: []app.GuideConfirmedStep{
			{ID: app.StepEnableSSH, Action: app.ActionRun, Effects: enableEffects},
			{ID: app.StepBackup, Action: app.ActionRun, Effects: []string{app.EffectLocalBackup}},
			{ID: app.StepSoftInstall, Action: app.ActionDecideAfter, Effects: []string{app.EffectScriptWrite, app.EffectUCICommit}},
		},
	}
	return app.GuidePlanReport{
		SchemaVersion: 1, Operation: "guide plan", Status: app.StatusSucceeded, Host: "192.168.31.1",
		Profile: device.Profile{ID: "xiaomi-rd08-1.1.96"}, Goals: confirmation.Goals,
		State: app.GuideState{SSH: device.PortClosed, RootSSH: app.RootSSHNotChecked},
		Steps: steps, Ready: true, Changes: true, RequiredInputs: []string{app.InputAdminPassword, app.InputNewRootPassword},
		Confirmation: confirmation, PlanDigest: strings.Repeat("ab", 32),
	}
}

func needsRootPlan() app.GuidePlanReport {
	plan := freshPlan()
	plan.State.SSH = device.PortOpen
	plan.Steps[0] = app.GuideStep{ID: app.StepEnableSSH, Action: app.ActionNeedsInput, Reason: app.ReasonRootPasswordMissing, Inputs: []string{app.InputRootPassword, app.InputResetRootPassword}}
	plan.Ready, plan.Confirmation, plan.PlanDigest = false, nil, ""
	plan.RequiredInputs = []string{app.InputRootPassword, app.InputResetRootPassword}
	return plan
}

func nothingToDoPlan() app.GuidePlanReport {
	plan := freshPlan()
	plan.State = app.GuideState{SSH: device.PortOpen, RootSSH: app.RootSSHVerified, Soft: app.SoftInstalledVerified}
	for i := range plan.Steps {
		plan.Steps[i] = app.GuideStep{ID: plan.Steps[i].ID, Action: app.ActionSkip, Reason: "already"}
	}
	plan.Changes, plan.RequiredInputs = false, nil
	return plan
}

func TestSetupPlanOnlyRendersTheStepsWithoutApplying(t *testing.T) {
	t.Parallel()

	service := &fakeGuideService{plans: []app.GuidePlanReport{freshPlan()}}
	code, stdout, stderr := executeWithInput(t, service, "", "--non-interactive", "--run-dir", t.TempDir(), "setup", "--plan-only")
	if code != 0 {
		t.Fatalf("setup --plan-only exit = %d, stderr = %s", code, stderr)
	}
	for _, want := range []string{"enable_ssh", "run", "nvram_commit", "decide_after", "admin_password"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("plan output %q does not mention %q", stdout, want)
		}
	}
	if len(service.applyRequests) != 0 {
		t.Fatal("plan-only applied the plan")
	}
	if got := service.planRequests[0].Goals; strings.Join(got, ",") != "ssh,backup,soft" {
		t.Fatalf("default goals = %v, want ssh,backup,soft", got)
	}
}

func TestSetupPassesGoalsAndResetFlag(t *testing.T) {
	t.Parallel()

	service := &fakeGuideService{plans: []app.GuidePlanReport{freshPlan()}}
	code, _, stderr := executeWithInput(t, service, "", "--non-interactive", "--json", "--run-dir", t.TempDir(),
		"setup", "--plan-only", "--goal", "ssh,soft,reboot", "--reset-root-password")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}
	request := service.planRequests[0]
	if strings.Join(request.Goals, ",") != "ssh,soft,reboot" || !request.ResetRootPassword {
		t.Fatalf("plan request = goals %v reset %v", request.Goals, request.ResetRootPassword)
	}
}

func TestSetupAppliesTheConfirmedPlanWithStdinPasswords(t *testing.T) {
	t.Parallel()

	service := &fakeGuideService{plans: []app.GuidePlanReport{freshPlan()}}
	code, stdout, stderr := executeWithInput(t, service, "admin-secret\nroot-secret\n",
		"--json", "--non-interactive", "--admin-password-stdin", "--root-password-stdin", "--yes", "--run-dir", t.TempDir(), "setup")
	if code != 0 {
		t.Fatalf("setup exit = %d, stderr = %s", code, stderr)
	}
	if len(service.applyRequests) != 1 {
		t.Fatalf("apply calls = %d, want 1", len(service.applyRequests))
	}
	request := service.applyRequests[0]
	plan := freshPlan()
	if !request.Confirmed || request.PlanDigest != plan.PlanDigest || request.Confirmation.Identity != plan.Confirmation.Identity ||
		request.AdminPassword.Expose() != "admin-secret" || request.RootPassword.Expose() != "root-secret" || request.ToolVersion != "test" {
		t.Fatalf("apply request = %+v", request)
	}
	var report app.GuideApplyReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil || report.Status != app.StatusSucceeded {
		t.Fatalf("stdout must be one apply report JSON object: %v %q", err, stdout)
	}
	if strings.Contains(stdout+stderr, "admin-secret") || strings.Contains(stdout+stderr, "root-secret") {
		t.Fatal("a password reached the output")
	}
}

func TestSetupAsksForTheNewRootPasswordTwice(t *testing.T) {
	t.Parallel()

	service := &fakeGuideService{plans: []app.GuidePlanReport{freshPlan()}}
	code, _, stderr := executeWithInput(t, service, "admin-secret\nnew-secret\nnew-secret\nyes\n", "--run-dir", t.TempDir(), "setup")
	if code != 0 {
		t.Fatalf("setup exit = %d, stderr = %s", code, stderr)
	}
	if len(service.applyRequests) != 1 || service.applyRequests[0].RootPassword.Expose() != "new-secret" {
		t.Fatalf("apply requests = %+v", service.applyRequests)
	}
	for _, want := range []string{"New root password", "Repeat the new root password", "nvram"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("prompts %q do not contain %q", stderr, want)
		}
	}

	mismatch := &fakeGuideService{plans: []app.GuidePlanReport{freshPlan()}}
	code, _, _ = executeWithInput(t, mismatch, "admin-secret\nnew-secret\nother-secret\nyes\n", "--run-dir", t.TempDir(), "setup")
	if code != 2 || len(mismatch.applyRequests) != 0 {
		t.Fatalf("mismatched repeat: exit %d applies %d, want 2 and none", code, len(mismatch.applyRequests))
	}
}

func TestSetupAsksForTheCurrentRootPasswordAndDetectsAgain(t *testing.T) {
	t.Parallel()

	service := &fakeGuideService{plans: []app.GuidePlanReport{needsRootPlan(), nothingToDoPlan()}}
	code, stdout, stderr := executeWithInput(t, service, "root-secret\n", "--run-dir", t.TempDir(), "setup")
	if code != 0 {
		t.Fatalf("setup exit = %d, stderr = %s", code, stderr)
	}
	if len(service.planRequests) != 2 || service.planRequests[1].RootPassword.Expose() != "root-secret" {
		t.Fatalf("plan requests = %+v, want a second detection with the current root password", service.planRequests)
	}
	if !strings.Contains(stderr, "Current root password") || !strings.Contains(stdout, "Nothing to do") || len(service.applyRequests) != 0 {
		t.Fatalf("stdout %q stderr %q applies %d", stdout, stderr, len(service.applyRequests))
	}
}

func TestSetupNonInteractiveWithoutTheNeededPasswordFails(t *testing.T) {
	t.Parallel()

	service := &fakeGuideService{plans: []app.GuidePlanReport{needsRootPlan()}}
	code, _, stderr := executeWithInput(t, service, "", "--non-interactive", "--run-dir", t.TempDir(), "setup")
	if code != 2 || len(service.applyRequests) != 0 || !strings.Contains(stderr, "--root-password-stdin") {
		t.Fatalf("exit %d applies %d stderr %q", code, len(service.applyRequests), stderr)
	}
}

func TestSetupRequiresTypedConfirmation(t *testing.T) {
	t.Parallel()

	service := &fakeGuideService{plans: []app.GuidePlanReport{freshPlan()}}
	code, _, _ := executeWithInput(t, service, "admin-secret\nroot-secret\n", "--non-interactive",
		"--admin-password-stdin", "--root-password-stdin", "--run-dir", t.TempDir(), "setup")
	if code != 9 || len(service.applyRequests) != 0 {
		t.Fatalf("non-interactive without --yes: exit %d applies %d, want 9 and none", code, len(service.applyRequests))
	}
	declined := &fakeGuideService{plans: []app.GuidePlanReport{freshPlan()}}
	code, _, _ = executeWithInput(t, declined, "admin-secret\nnew-secret\nnew-secret\nno\n", "--run-dir", t.TempDir(), "setup")
	if code != 9 || len(declined.applyRequests) != 0 {
		t.Fatalf("declined: exit %d applies %d, want 9 and none", code, len(declined.applyRequests))
	}
}

func TestSetupWithNothingToDoNeverAsksToConfirm(t *testing.T) {
	t.Parallel()

	service := &fakeGuideService{plans: []app.GuidePlanReport{nothingToDoPlan()}}
	code, stdout, stderr := executeWithInput(t, service, "root-secret\n", "--non-interactive", "--root-password-stdin", "--run-dir", t.TempDir(), "setup")
	if code != 0 || len(service.applyRequests) != 0 || !strings.Contains(stdout, "Nothing to do") || strings.Contains(stderr, "Type yes") {
		t.Fatalf("exit %d applies %d stdout %q stderr %q", code, len(service.applyRequests), stdout, stderr)
	}
}

func TestSetupBlockedPlanUsesItsExitCode(t *testing.T) {
	t.Parallel()

	plan := freshPlan()
	plan.Status, plan.Ready, plan.Confirmation, plan.PlanDigest = app.StatusFailed, false, nil, ""
	for i := range plan.Steps {
		plan.Steps[i] = app.GuideStep{ID: plan.Steps[i].ID, Action: app.ActionBlocked, Reason: app.ReasonRecoveryReviewRequired}
	}
	plan.State.Recovery = &state.Outcome{Operation: "enable", Status: state.StatusUnknown}
	blockErr := &device.Error{Code: device.CodeRecoveryReviewRequired, Message: "the last enable on this router ended unknown"}
	plan.Error = blockErr
	service := &fakeGuideService{plans: []app.GuidePlanReport{plan}, planErrs: []error{blockErr}}
	code, stdout, _ := executeWithInput(t, service, "", "--non-interactive", "--run-dir", t.TempDir(), "setup")
	if code != 9 || len(service.applyRequests) != 0 || !strings.Contains(stdout, "recovery ack") {
		t.Fatalf("exit %d applies %d stdout %q; want 9, no apply, and the acknowledgement hint", code, len(service.applyRequests), stdout)
	}
}

func TestSetupStoppedRunRendersEveryStep(t *testing.T) {
	t.Parallel()

	stepErr := &device.Error{Code: device.CodeBackupIncomplete, Message: "short read"}
	service := &fakeGuideService{
		plans: []app.GuidePlanReport{freshPlan()},
		applyReport: app.GuideApplyReport{Status: app.StatusPartial, Steps: []app.GuideStepResult{
			{ID: app.StepEnableSSH, Action: app.ActionRun, Status: app.StatusSucceeded},
			{ID: app.StepBackup, Action: app.ActionRun, Status: app.StatusFailed, Error: stepErr},
			{ID: app.StepSoftInstall, Action: app.ActionNotRun, Status: app.StatusNotRun},
		}, Error: stepErr},
		applyErr: stepErr,
	}
	code, stdout, _ := executeWithInput(t, service, "admin-secret\nroot-secret\n",
		"--non-interactive", "--admin-password-stdin", "--root-password-stdin", "--yes", "--run-dir", t.TempDir(), "setup")
	if code != 11 {
		t.Fatalf("exit = %d, want the failing step's exit code 11", code)
	}
	for _, want := range []string{"enable_ssh", "succeeded", "BACKUP_INCOMPLETE", "not_run"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("result output %q does not mention %q", stdout, want)
		}
	}
}

func TestRecoveryAckRequiresConfirmationAndUsesTheTarget(t *testing.T) {
	t.Parallel()

	service := &fakeGuideService{ackReport: app.RecoveryReport{
		Acknowledged: true, Last: &state.Outcome{Operation: "enable", Status: state.StatusUnknown},
	}}
	code, _, _ := executeWithInput(t, service, "", "--non-interactive", "recovery", "ack")
	if code != 9 || len(service.ackTargets) != 0 {
		t.Fatalf("without --yes: exit %d acks %d, want 9 and none", code, len(service.ackTargets))
	}
	code, stdout, stderr := executeWithInput(t, service, "", "--non-interactive", "--host", "192.168.31.2", "--yes", "recovery", "ack")
	if code != 0 || len(service.ackTargets) != 1 || service.ackTargets[0].Host != "192.168.31.2" {
		t.Fatalf("exit %d targets %+v stderr %q", code, service.ackTargets, stderr)
	}
	if !strings.Contains(stdout, "enable") || !strings.Contains(stdout, "unknown") {
		t.Fatalf("ack output %q must name the acknowledged change", stdout)
	}
	code, _, _ = executeWithInput(t, service, "yes\n", "recovery", "ack")
	if code != 0 || len(service.ackTargets) != 2 {
		t.Fatalf("typed yes: exit %d acks %d", code, len(service.ackTargets))
	}
}

func TestRecoveryAndStateCodesHaveDocumentedExitCodes(t *testing.T) {
	t.Parallel()

	cases := map[string]int{device.CodeRecoveryReviewRequired: 9, device.CodeStateStoreFailed: 11}
	for code, want := range cases {
		service := &fakeGuideService{ackErr: &device.Error{Code: code, Message: "x"}}
		got, _, _ := executeWithInput(t, service, "", "--non-interactive", "--yes", "recovery", "ack")
		if got != want {
			t.Fatalf("%s exit = %d, want %d", code, got, want)
		}
	}
}
