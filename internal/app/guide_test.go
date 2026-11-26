package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/backup"
	"github.com/sunerpy/router-tools/internal/credentials"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
	"github.com/sunerpy/router-tools/internal/secret"
	"github.com/sunerpy/router-tools/internal/state"
)

// simRouter is a fake router whose state follows the guided writes, so a plan and its apply see the same
// device evolve.
type simRouter struct {
	mu           sync.Mutex
	t            *testing.T
	observation  device.Observation
	sshOpen      bool
	rootPassword string
	soft         app.SoftState
	bootID       string
	probes       int
	calls        map[string]int
	writes       []string
	enableErr    error
	backupErr    error
	sshErr       error         // returned by every SSH login, e.g. a host-key mismatch
	afterEnable  app.SoftState // soft state the router reports once SSH was enabled; "" keeps soft
	onProbe      func(*simRouter)
}

func newSimRouter(t *testing.T) *simRouter {
	return &simRouter{
		t:           t,
		observation: device.Observation{DeviceID: "DEVICE", Hardware: "R1", DisplayName: "Acme R1", ROMVersion: "1.0"},
		soft:        app.SoftNotInstalled, bootID: "00000000-0000-0000-0000-000000000001",
		calls: make(map[string]int),
	}
}

func (r *simRouter) count(name string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls[name]
}

func (r *simRouter) recordWrite(name string) {
	r.calls[name]++
	r.writes = append(r.writes, name)
}

func (r *simRouter) login(password secret.Password) error {
	switch {
	case !r.sshOpen:
		return &device.Error{Code: device.CodeSSHUnreachable, Message: "connection refused"}
	case r.sshErr != nil:
		return r.sshErr
	case password.Expose() != r.rootPassword:
		return &device.Error{Code: device.CodeSSHAuthFailed, Message: "permission denied"}
	}
	return nil
}

// simProbe is the discovery prober: it only reads the observation and port states.
type simProbe struct{ r *simRouter }

func (p simProbe) Probe(context.Context, device.Target) (device.ProbeResult, error) {
	p.r.mu.Lock()
	defer p.r.mu.Unlock()
	p.r.probes++
	if p.r.onProbe != nil {
		p.r.onProbe(p.r)
	}
	ssh := device.PortClosed
	if p.r.sshOpen {
		ssh = device.PortOpen
	}
	return device.ProbeResult{Observation: p.r.observation, SSH: ssh, Telnet: device.PortClosed}, nil
}

// simOps serves the resolved profile's capabilities against the simulated router.
type simOps struct{ r *simRouter }

func (o simOps) VerifyRoot(_ context.Context, _ device.Target, password secret.Password, _ hostkey.Policy, transport string, _ bool) (app.RootVerification, error) {
	o.r.mu.Lock()
	defer o.r.mu.Unlock()
	o.r.calls["VerifyRoot"]++
	if transport != "ssh" {
		o.r.t.Errorf("guided root verification used transport %q, want ssh only", transport)
	}
	if err := o.r.login(password); err != nil {
		return app.RootVerification{}, err
	}
	return app.RootVerification{Transport: "ssh", RootVerified: true, SSHVerified: true, BootID: o.r.bootID}, nil
}

func (o simOps) InspectSoft(_ context.Context, _ device.Target, password secret.Password, _ hostkey.Policy) (app.SoftState, error) {
	o.r.mu.Lock()
	defer o.r.mu.Unlock()
	o.r.calls["InspectSoft"]++
	if err := o.r.login(password); err != nil {
		return "", err
	}
	return o.r.soft, nil
}

func (o simOps) EnableSSH(_ context.Context, _ device.Target, _, root secret.Password, _ hostkey.Policy) (app.EnableResult, error) {
	o.r.mu.Lock()
	defer o.r.mu.Unlock()
	o.r.recordWrite("EnableSSH")
	if o.r.enableErr != nil {
		return app.EnableResult{Status: app.StatusFailed}, o.r.enableErr
	}
	o.r.sshOpen, o.r.rootPassword = true, root.Expose()
	if o.r.afterEnable != "" {
		o.r.soft = o.r.afterEnable
	}
	return app.EnableResult{Status: app.StatusSucceeded, AcceptedSteps: 3, SSHVerified: true}, nil
}

func (o simOps) InstallSoft(_ context.Context, _ device.Target, password secret.Password, _ hostkey.Policy, telnet bool) (app.SoftResult, error) {
	o.r.mu.Lock()
	defer o.r.mu.Unlock()
	o.r.recordWrite("InstallSoft")
	if telnet {
		o.r.t.Error("guided soft install allowed Telnet")
	}
	if err := o.r.login(password); err != nil {
		return app.SoftResult{}, err
	}
	o.r.soft = app.SoftInstalledVerified
	return app.SoftResult{Outcome: "installed", Verified: true, WriteStarted: true}, nil
}

func (o simOps) RemoveSoft(context.Context, device.Target, secret.Password, hostkey.Policy, bool) (app.SoftResult, error) {
	o.r.mu.Lock()
	defer o.r.mu.Unlock()
	o.r.recordWrite("RemoveSoft")
	return app.SoftResult{}, errors.New("guided runs never remove soft persistence")
}

func (o simOps) RebootVerify(_ context.Context, _ device.Target, password secret.Password, _ hostkey.Policy, telnet bool) (app.RebootResult, error) {
	o.r.mu.Lock()
	defer o.r.mu.Unlock()
	o.r.recordWrite("RebootVerify")
	if telnet {
		o.r.t.Error("guided reboot verification allowed Telnet")
	}
	old := o.r.bootID
	o.r.bootID = "00000000-0000-0000-0000-000000000002"
	if err := o.r.login(password); err != nil {
		return app.RebootResult{OldBootID: old, WriteStarted: true}, err
	}
	return app.RebootResult{OldBootID: old, NewBootID: o.r.bootID, SSHVerified: true, SoftVerified: o.r.soft == app.SoftInstalledVerified, WriteStarted: true}, nil
}

func (o simOps) CreateBackup(_ context.Context, _ device.Target, password secret.Password, _ hostkey.Policy, options backup.CreateOptions) (backup.Manifest, error) {
	o.r.mu.Lock()
	defer o.r.mu.Unlock()
	o.r.recordWrite("CreateBackup")
	if o.r.backupErr != nil {
		return backup.Manifest{}, o.r.backupErr
	}
	if err := o.r.login(password); err != nil {
		return backup.Manifest{}, err
	}
	writeBackupAt(o.r.t, options.OutputDir, options.Device)
	return backup.LoadManifest(options.OutputDir)
}

type guideFixture struct {
	service    *app.Service
	router     *simRouter
	store      *memoryOutcomes
	vault      *credentials.Memory
	backupRoot string
	target     device.Target
}

func newGuideFixture(t *testing.T) *guideFixture {
	t.Helper()
	router := newSimRouter(t)
	registry, err := device.NewRegistry(device.Adapter{Driver: modelDriver{p: fullProfile("1.0")}, Operations: simOps{r: router}})
	if err != nil {
		t.Fatal(err)
	}
	fixture := &guideFixture{router: router, store: newMemoryOutcomes(), vault: credentials.NewMemory(), backupRoot: t.TempDir(), target: device.DefaultTarget()}
	clock := time.Date(2026, 11, 26, 3, 0, 0, 0, time.UTC)
	service, err := app.NewServiceWithOptions(registry, simProbe{r: router}, app.Options{
		Outcomes: fixture.store, BackupRoot: fixture.backupRoot, Credentials: fixture.vault,
		Clock: func() time.Time { clock = clock.Add(time.Second); return clock },
	})
	if err != nil {
		t.Fatal(err)
	}
	fixture.service = service
	return fixture
}

// seedBackup stores a verified managed backup of the simulated router.
func (f *guideFixture) seedBackup(t *testing.T) {
	t.Helper()
	dir := filepath.Join(f.backupRoot, "acme-r1", "192.168.31.1-"+device.RecoveryKey(f.target)[:12], "20261126T010000Z")
	writeBackupAt(t, dir, backup.DeviceIdentity{DeviceID: "DEVICE", Hardware: "R1", ROMVersion: "1.0", Profile: "acme-r1-1.0"})
}

func (f *guideFixture) configured(t *testing.T) {
	t.Helper()
	f.router.sshOpen, f.router.rootPassword, f.router.soft = true, "root-secret", app.SoftInstalledVerified
	f.seedBackup(t)
}

func mustPassword(t *testing.T, raw string) secret.Password {
	t.Helper()
	password, err := secret.NewPassword(raw)
	if err != nil {
		t.Fatal(err)
	}
	return password
}

func defaultGoals() []string { return []string{app.GoalSSH, app.GoalBackup, app.GoalSoft} }

func actions(steps []app.GuideStep) map[string]string {
	result := make(map[string]string, len(steps))
	for _, step := range steps {
		result[step.ID] = step.Action
	}
	return result
}

func resultStatuses(steps []app.GuideStepResult) map[string]string {
	result := make(map[string]string, len(steps))
	for _, step := range steps {
		result[step.ID] = step.Status
	}
	return result
}

func (f *guideFixture) writeCount() int {
	f.router.mu.Lock()
	defer f.router.mu.Unlock()
	return len(f.router.writes)
}

func TestGuidePlanFreshDeviceIsReadOnly(t *testing.T) {
	t.Parallel()

	f := newGuideFixture(t)
	report, err := f.service.GuidePlan(t.Context(), app.GuideRequest{Target: f.target, Goals: defaultGoals()}, nil)
	if err != nil {
		t.Fatalf("GuidePlan() error = %v", err)
	}
	want := map[string]string{app.StepEnableSSH: app.ActionRun, app.StepBackup: app.ActionRun, app.StepSoftInstall: app.ActionDecideAfter}
	if !reflect.DeepEqual(actions(report.Steps), want) {
		t.Fatalf("actions = %v, want %v", actions(report.Steps), want)
	}
	if report.Status != app.StatusSucceeded || !report.Ready || !report.Changes || report.Confirmation == nil || len(report.PlanDigest) != 64 {
		t.Fatalf("report = status %q ready %v changes %v confirmation %v digest %q", report.Status, report.Ready, report.Changes, report.Confirmation, report.PlanDigest)
	}
	if !reflect.DeepEqual(report.RequiredInputs, []string{app.InputAdminPassword, app.InputNewRootPassword}) {
		t.Fatalf("required inputs = %v", report.RequiredInputs)
	}
	wantIdentity := app.ConfirmedIdentity{DeviceID: "DEVICE", Hardware: "R1", DisplayName: "Acme R1", ROMVersion: "1.0", ProfileID: "acme-r1-1.0"}
	if report.Confirmation.Identity != wantIdentity || report.Confirmation.Target != f.target {
		t.Fatalf("confirmation identity %+v target %+v", report.Confirmation.Identity, report.Confirmation.Target)
	}
	if f.writeCount() != 0 || f.router.count("VerifyRoot") != 0 {
		t.Fatalf("plan wrote %v and verified root %d times; want a read-only plan without an SSH login on a closed port",
			f.router.writes, f.router.count("VerifyRoot"))
	}
	if report.State.SSH != device.PortClosed || report.State.RootSSH != app.RootSSHNotChecked || report.State.Backup != nil {
		t.Fatalf("state = %+v", report.State)
	}
}

func TestGuidePlanAlreadyConfiguredHasNothingToDo(t *testing.T) {
	t.Parallel()

	f := newGuideFixture(t)
	f.configured(t)
	report, err := f.service.GuidePlan(t.Context(), app.GuideRequest{
		Target: f.target, Goals: defaultGoals(), RootPassword: mustPassword(t, "root-secret"),
	}, nil)
	if err != nil {
		t.Fatalf("GuidePlan() error = %v", err)
	}
	for _, step := range report.Steps {
		if step.Action != app.ActionSkip {
			t.Fatalf("step %s = %s (%s), want skip", step.ID, step.Action, step.Reason)
		}
	}
	if !report.Ready || report.Changes || report.RequiredInputs != nil || report.State.RootSSH != app.RootSSHVerified ||
		report.State.Soft != app.SoftInstalledVerified || report.State.Backup == nil {
		t.Fatalf("report = %+v", report)
	}
	if f.writeCount() != 0 {
		t.Fatalf("plan wrote %v", f.router.writes)
	}
}

func TestGuidePlanAsksForTheCurrentRootPasswordInsteadOfResettingIt(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		password string
		reset    bool
		action   string
		reason   string
	}{
		{name: "no password", action: app.ActionNeedsInput, reason: app.ReasonRootPasswordMissing},
		{name: "rejected password", password: "wrong", action: app.ActionNeedsInput, reason: app.ReasonSSHAuthFailed},
		{name: "explicit reset", password: "new-secret", reset: true, action: app.ActionRun, reason: app.ReasonResetRootPassword},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newGuideFixture(t)
			f.router.sshOpen, f.router.rootPassword = true, "root-secret"
			request := app.GuideRequest{Target: f.target, Goals: defaultGoals(), ResetRootPassword: tc.reset}
			if tc.password != "" {
				request.RootPassword = mustPassword(t, tc.password)
			}
			report, err := f.service.GuidePlan(t.Context(), request, nil)
			if err != nil {
				t.Fatalf("GuidePlan() error = %v", err)
			}
			enable := report.Steps[0]
			if enable.ID != app.StepEnableSSH || enable.Action != tc.action || enable.Reason != tc.reason {
				t.Fatalf("enable step = %+v, want %s/%s", enable, tc.action, tc.reason)
			}
			if tc.action == app.ActionNeedsInput && (report.Ready || report.Confirmation != nil || report.PlanDigest != "") {
				t.Fatalf("a plan that needs input must not be confirmable: %+v", report)
			}
			if tc.reset && !reflect.DeepEqual(enable.Effects, []string{app.EffectTemporaryMACRules, app.EffectNVRAMCommit, app.EffectDropbearStart, app.EffectRootPasswordChange}) {
				t.Fatalf("reset effects = %v, want the root password change listed", enable.Effects)
			}
			if f.writeCount() != 0 {
				t.Fatalf("plan wrote %v", f.router.writes)
			}
		})
	}
}

func TestGuidePlanBlocksWithoutPlanningWrites(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		setup  func(*testing.T, *guideFixture) (release func())
		code   string
		reason string
		probes bool
	}{
		{
			name: "unacknowledged uncertain change",
			setup: func(_ *testing.T, f *guideFixture) func() {
				_ = f.store.Save(device.RecoveryKey(f.target), state.Outcome{Operation: "enable", Status: state.StatusUnknown, DeviceWritePossible: true, StartedAt: time.Now()})
				return nil
			},
			code: device.CodeRecoveryReviewRequired, reason: app.ReasonRecoveryReviewRequired,
		},
		{
			name: "another operation holds the router",
			setup: func(t *testing.T, f *guideFixture) func() {
				release, err := f.service.LockForTest(f.target)
				if err != nil {
					t.Fatal(err)
				}
				return func() { _ = release() }
			},
			code: device.CodeOperationLocked, reason: app.ReasonOperationLocked,
		},
		{
			name: "unsupported firmware",
			setup: func(_ *testing.T, f *guideFixture) func() {
				f.router.observation.ROMVersion = "2.0"
				return nil
			},
			code: device.CodeUnsupportedFirmware, reason: app.ReasonIncompatible, probes: true,
		},
		{
			name: "host key mismatch",
			setup: func(_ *testing.T, f *guideFixture) func() {
				f.router.sshOpen, f.router.rootPassword = true, "root-secret"
				f.router.sshErr = &device.Error{Code: device.CodeHostKeyMismatch, Message: "changed"}
				return nil
			},
			code: device.CodeHostKeyMismatch, reason: app.ReasonHostKeyRejected, probes: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newGuideFixture(t)
			if release := tc.setup(t, f); release != nil {
				defer release()
			}
			report, err := f.service.GuidePlan(t.Context(), app.GuideRequest{
				Target: f.target, Goals: defaultGoals(), RootPassword: mustPassword(t, "root-secret"),
			}, nil)
			assertCode(t, err, tc.code)
			if report.Status != app.StatusFailed || report.Ready || report.Confirmation != nil || len(report.Steps) != 3 {
				t.Fatalf("report = %+v", report)
			}
			for _, step := range report.Steps {
				if step.Action != app.ActionBlocked || step.Reason != tc.reason || step.Effects != nil {
					t.Fatalf("step %+v, want blocked by %s without effects", step, tc.reason)
				}
			}
			if f.writeCount() != 0 || (f.router.probes != 0) != tc.probes {
				t.Fatalf("writes %v probes %d", f.router.writes, f.router.probes)
			}
		})
	}
}

func TestGuidePlanReportsCandidatesForUnsupportedFirmware(t *testing.T) {
	t.Parallel()

	f := newGuideFixture(t)
	f.router.observation.ROMVersion = "2.0"
	report, err := f.service.GuidePlan(t.Context(), app.GuideRequest{Target: f.target, Goals: defaultGoals()}, nil)
	assertCode(t, err, device.CodeUnsupportedFirmware)
	if len(report.Candidates) != 1 || report.Candidates[0].Profile.ID != "acme-r1-1.0" {
		t.Fatalf("candidates = %+v, want the registered firmware of the same model", report.Candidates)
	}
}

func TestGuidePlanNamesTheMaintenanceForSoftDrift(t *testing.T) {
	t.Parallel()

	f := newGuideFixture(t)
	f.configured(t)
	f.router.soft = app.SoftUCIDrift
	report, err := f.service.GuidePlan(t.Context(), app.GuideRequest{
		Target: f.target, Goals: defaultGoals(), RootPassword: mustPassword(t, "root-secret"),
	}, nil)
	assertCode(t, err, device.CodeSoftConfigConflict)
	soft := report.Steps[2]
	if soft.Action != app.ActionBlocked || soft.Maintenance != app.MaintenancePersistRemove || report.Ready || report.Confirmation != nil {
		t.Fatalf("soft step = %+v ready %v", soft, report.Ready)
	}
	if report.Steps[0].Action != app.ActionSkip || report.Steps[1].Action != app.ActionSkip {
		t.Fatalf("the other steps must still be planned: %+v", report.Steps)
	}
}

func TestGuidePlanRejectsInvalidGoalsBeforeAnyAccess(t *testing.T) {
	t.Parallel()

	for _, goals := range [][]string{{app.GoalReboot}, {"deep"}} {
		f := newGuideFixture(t)
		_, err := f.service.GuidePlan(t.Context(), app.GuideRequest{Target: f.target, Goals: goals}, nil)
		assertCode(t, err, device.CodeInvalidArgument)
		if f.router.probes != 0 {
			t.Fatalf("goals %v probed the router", goals)
		}
	}
}

func (f *guideFixture) plan(t *testing.T, request app.GuideRequest) app.GuidePlanReport {
	t.Helper()
	if request.Target == (device.Target{}) {
		request.Target = f.target
	}
	if request.Goals == nil {
		request.Goals = defaultGoals()
	}
	report, err := f.service.GuidePlan(t.Context(), request, nil)
	if err != nil || !report.Ready {
		t.Fatalf("GuidePlan() = ready %v, %v", report.Ready, err)
	}
	return report
}

func applyRequest(t *testing.T, plan app.GuidePlanReport) app.GuideApplyRequest {
	return app.GuideApplyRequest{
		Confirmation: *plan.Confirmation, PlanDigest: plan.PlanDigest, Confirmed: true,
		AdminPassword: mustPassword(t, "admin-secret"), RootPassword: mustPassword(t, "root-secret"),
	}
}

func TestGuideApplyFreshDeviceRunsEveryMissingStepInOrder(t *testing.T) {
	t.Parallel()

	f := newGuideFixture(t)
	plan := f.plan(t, app.GuideRequest{})
	report, err := f.service.GuideApply(t.Context(), applyRequest(t, plan), nil)
	if err != nil {
		t.Fatalf("GuideApply() error = %v (%+v)", err, report)
	}
	if !reflect.DeepEqual(f.router.writes, []string{"EnableSSH", "CreateBackup", "InstallSoft"}) {
		t.Fatalf("writes = %v, want enable, backup, soft in order", f.router.writes)
	}
	want := map[string]string{app.StepEnableSSH: app.StatusSucceeded, app.StepBackup: app.StatusSucceeded, app.StepSoftInstall: app.StatusSucceeded}
	if report.Status != app.StatusSucceeded || !reflect.DeepEqual(resultStatuses(report.Steps), want) {
		t.Fatalf("report status %q steps %v", report.Status, resultStatuses(report.Steps))
	}
	if report.Steps[0].Enable == nil || report.Steps[1].Backup == nil || report.Steps[2].Soft == nil {
		t.Fatalf("each run step must carry its use-case report: %+v", report.Steps)
	}
	if last := f.store.last(f.target); last.Operation != "persist soft" || last.Status != state.StatusSucceeded {
		t.Fatalf("last recorded change = %+v, want the soft install recorded as succeeded", last)
	}
}

func TestGuideApplyRefusesAChangedOrTamperedPlanBeforeAnyWrite(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*testing.T, *guideFixture, *app.GuideApplyRequest){
		"not confirmed": func(_ *testing.T, _ *guideFixture, r *app.GuideApplyRequest) { r.Confirmed = false },
		"digest mismatch": func(_ *testing.T, _ *guideFixture, r *app.GuideApplyRequest) {
			r.Confirmation.Steps[0].Effects = r.Confirmation.Steps[0].Effects[1:]
		},
		"device changed": func(_ *testing.T, f *guideFixture, _ *app.GuideApplyRequest) {
			f.router.observation.DeviceID = "OTHER"
		},
		"confirmed effects differ from the fresh plan": func(_ *testing.T, _ *guideFixture, r *app.GuideApplyRequest) {
			r.Confirmation.Steps[0].Effects = r.Confirmation.Steps[0].Effects[1:]
			r.PlanDigest = app.PlanDigestForTest(r.Confirmation)
		},
		"a skipped step would now run": func(_ *testing.T, f *guideFixture, r *app.GuideApplyRequest) {
			r.Confirmation.Steps[1].Action, r.Confirmation.Steps[1].Effects = app.ActionSkip, nil
			r.PlanDigest = app.PlanDigestForTest(r.Confirmation)
		},
		"SSH opened with an unknown root password": func(_ *testing.T, f *guideFixture, _ *app.GuideApplyRequest) {
			f.router.sshOpen, f.router.rootPassword = true, "someone-else"
		},
		"a confirmation of a plan that was not ready": func(_ *testing.T, f *guideFixture, r *app.GuideApplyRequest) {
			f.router.sshOpen, f.router.rootPassword = true, "root-secret"
			r.Confirmation.Steps[0].Action, r.Confirmation.Steps[0].Effects = app.ActionNeedsInput, nil
			r.PlanDigest = app.PlanDigestForTest(r.Confirmation)
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newGuideFixture(t)
			plan := f.plan(t, app.GuideRequest{})
			request := applyRequest(t, plan)
			mutate(t, f, &request)
			report, err := f.service.GuideApply(t.Context(), request, nil)
			assertCode(t, err, device.CodeConfirmationRequired)
			if len(report.Steps) != 0 || f.writeCount() != 0 || f.store.saves != 0 {
				t.Fatalf("steps %+v writes %v outcome saves %d; want a refusal before any step", report.Steps, f.router.writes, f.store.saves)
			}
		})
	}
}

func TestGuideApplyAllowsFewerWritesButNeverMore(t *testing.T) {
	t.Parallel()

	f := newGuideFixture(t)
	plan := f.plan(t, app.GuideRequest{})
	f.seedBackup(t) // a backup appeared after the plan: run -> skip is allowed
	report, err := f.service.GuideApply(t.Context(), applyRequest(t, plan), nil)
	if err != nil {
		t.Fatalf("GuideApply() error = %v", err)
	}
	if f.router.count("CreateBackup") != 0 || report.Steps[1].Status != app.StatusSkipped || report.Steps[1].Action != app.ActionSkip {
		t.Fatalf("backup step = %+v, CreateBackup calls %d; want skipped", report.Steps[1], f.router.count("CreateBackup"))
	}
}

func TestGuideApplyStopsAtTheFirstStepThatDoesNotSucceed(t *testing.T) {
	t.Parallel()

	t.Run("enable fails before any write", func(t *testing.T) {
		t.Parallel()
		f := newGuideFixture(t)
		plan := f.plan(t, app.GuideRequest{})
		f.router.enableErr = &device.Error{Code: device.CodeWebAuthFailed, Message: "bad password"}
		report, err := f.service.GuideApply(t.Context(), applyRequest(t, plan), nil)
		assertCode(t, err, device.CodeWebAuthFailed)
		want := map[string]string{app.StepEnableSSH: app.StatusFailed, app.StepBackup: app.StatusNotRun, app.StepSoftInstall: app.StatusNotRun}
		if report.Status != app.StatusFailed || !reflect.DeepEqual(resultStatuses(report.Steps), want) {
			t.Fatalf("status %q steps %v", report.Status, resultStatuses(report.Steps))
		}
		if f.router.count("CreateBackup")+f.router.count("InstallSoft") != 0 {
			t.Fatalf("later steps ran: %v", f.router.writes)
		}
	})
	t.Run("backup fails after SSH was enabled", func(t *testing.T) {
		t.Parallel()
		f := newGuideFixture(t)
		plan := f.plan(t, app.GuideRequest{})
		f.router.backupErr = &device.Error{Code: device.CodeBackupIncomplete, Message: "short read"}
		report, err := f.service.GuideApply(t.Context(), applyRequest(t, plan), nil)
		assertCode(t, err, device.CodeBackupIncomplete)
		if report.Status != app.StatusPartial || report.Steps[2].Status != app.StatusNotRun || f.router.count("InstallSoft") != 0 {
			t.Fatalf("status %q steps %v writes %v", report.Status, resultStatuses(report.Steps), f.router.writes)
		}
	})
}

func TestGuideApplyDecidesDeferredStepsAfterSSHWorks(t *testing.T) {
	t.Parallel()

	t.Run("already installed is skipped", func(t *testing.T) {
		t.Parallel()
		f := newGuideFixture(t)
		f.router.afterEnable = app.SoftInstalledVerified
		plan := f.plan(t, app.GuideRequest{})
		report, err := f.service.GuideApply(t.Context(), applyRequest(t, plan), nil)
		if err != nil {
			t.Fatalf("GuideApply() error = %v", err)
		}
		if f.router.count("InstallSoft") != 0 || report.Steps[2].Status != app.StatusSkipped || report.Steps[2].Reason != app.ReasonSoftInstalledVerified {
			t.Fatalf("soft step = %+v", report.Steps[2])
		}
	})
	t.Run("a conflict stops the run", func(t *testing.T) {
		t.Parallel()
		f := newGuideFixture(t)
		f.router.afterEnable = app.SoftConflict
		plan := f.plan(t, app.GuideRequest{})
		report, err := f.service.GuideApply(t.Context(), applyRequest(t, plan), nil)
		assertCode(t, err, device.CodeSoftConfigConflict)
		soft := report.Steps[2]
		if f.router.count("InstallSoft") != 0 || soft.Action != app.ActionBlocked || soft.Status != app.StatusFailed || report.Status != app.StatusPartial {
			t.Fatalf("soft step = %+v report status %q", soft, report.Status)
		}
	})
}

// With nothing to do apply still re-detects with the root password, but needs no admin password and writes
// nothing.
func TestGuideApplyWithNothingToDoWritesNothing(t *testing.T) {
	t.Parallel()

	f := newGuideFixture(t)
	f.configured(t)
	plan := f.plan(t, app.GuideRequest{RootPassword: mustPassword(t, "root-secret")})
	report, err := f.service.GuideApply(t.Context(), app.GuideApplyRequest{
		Confirmation: *plan.Confirmation, PlanDigest: plan.PlanDigest, Confirmed: true,
		RootPassword: mustPassword(t, "root-secret"),
	}, nil)
	if err != nil || report.Status != app.StatusSucceeded || f.writeCount() != 0 {
		t.Fatalf("GuideApply() = %q, %v; writes %v", report.Status, err, f.router.writes)
	}
}

func TestGuideApplyTagsUseCaseEventsWithTheirStep(t *testing.T) {
	t.Parallel()

	f := newGuideFixture(t)
	plan := f.plan(t, app.GuideRequest{})
	var mu sync.Mutex
	steps := map[string]map[string]bool{}
	sink := app.EventSinkFunc(func(event app.Event) {
		mu.Lock()
		defer mu.Unlock()
		if steps[event.Step] == nil {
			steps[event.Step] = map[string]bool{}
		}
		steps[event.Step][event.Operation] = true
	})
	if _, err := f.service.GuideApply(t.Context(), applyRequest(t, plan), sink); err != nil {
		t.Fatal(err)
	}
	if !steps[app.StepEnableSSH]["enable"] || !steps[app.StepBackup]["backup create"] || !steps[app.StepSoftInstall]["persist soft"] {
		t.Fatalf("events per step = %v, want each use case's events tagged with its step", steps)
	}
}

// Every write a guided step dispatches checks the confirmed identity again after taking the lock.
func TestGuidedWritesCheckTheConfirmedIdentity(t *testing.T) {
	t.Parallel()

	stale := &app.ConfirmedIdentity{DeviceID: "OTHER", Hardware: "R1", DisplayName: "Acme R1", ROMVersion: "1.0", ProfileID: "acme-r1-1.0"}
	root, admin := mustPassword(t, "root-secret"), mustPassword(t, "admin-secret")
	calls := map[string]func(*guideFixture) error{
		"enable": func(f *guideFixture) error {
			_, err := f.service.EnableSSH(t.Context(), app.EnableRequest{Target: f.target, AdminPassword: admin, RootPassword: root, Confirmed: true, ExpectedIdentity: stale}, nil)
			return err
		},
		"backup": func(f *guideFixture) error {
			_, err := f.service.BackupCreate(t.Context(), app.BackupCreateRequest{Target: f.target, RootPassword: root, ExpectedIdentity: stale}, nil)
			return err
		},
		"soft": func(f *guideFixture) error {
			_, err := f.service.PersistSoft(t.Context(), app.SoftRequest{Target: f.target, RootPassword: root, Confirmed: true, ExpectedIdentity: stale}, nil)
			return err
		},
		"reboot": func(f *guideFixture) error {
			_, err := f.service.RebootVerify(t.Context(), app.SoftRequest{Target: f.target, RootPassword: root, Confirmed: true, ExpectedIdentity: stale}, nil)
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newGuideFixture(t)
			f.configured(t)
			assertCode(t, call(f), device.CodeConfirmationRequired)
			if f.writeCount() != 0 || f.store.saves != 0 {
				t.Fatalf("writes %v outcome saves %d, want none", f.router.writes, f.store.saves)
			}
		})
	}
}

// Failed plans and refused runs still carry JSON arrays, never null, as the frontend types promise.
func TestGuideReportsUseEmptyListsWhenNothingWasPlanned(t *testing.T) {
	t.Parallel()

	f := newGuideFixture(t)
	plan, err := f.service.GuidePlan(t.Context(), app.GuideRequest{Target: device.Target{Host: "bad/host"}, Goals: []string{"deep"}}, nil)
	assertCode(t, err, device.CodeInvalidArgument)
	data, _ := json.Marshal(plan)
	if !strings.Contains(string(data), `"steps":[]`) || !strings.Contains(string(data), `"goals":[]`) {
		t.Fatalf("failed plan JSON = %s, want empty steps and goals lists", data)
	}
	run, err := f.service.GuideApply(t.Context(), app.GuideApplyRequest{}, nil)
	assertCode(t, err, device.CodeConfirmationRequired)
	data, _ = json.Marshal(run)
	if !strings.Contains(string(data), `"steps":[]`) {
		t.Fatalf("refused run JSON = %s, want an empty steps list", data)
	}
}

// Reboot verification runs on every guided run that selects it, over SSH only.
func TestGuideApplyRunsTheSelectedRebootVerification(t *testing.T) {
	t.Parallel()

	f := newGuideFixture(t)
	f.configured(t)
	plan := f.plan(t, app.GuideRequest{Goals: []string{app.GoalSSH, app.GoalBackup, app.GoalSoft, app.GoalReboot}, RootPassword: mustPassword(t, "root-secret")})
	if got := actions(plan.Steps); got[app.StepRebootVerify] != app.ActionRun || got[app.StepSoftInstall] != app.ActionSkip {
		t.Fatalf("plan = %v, want only the reboot verification to run", got)
	}
	report, err := f.service.GuideApply(t.Context(), applyRequest(t, plan), nil)
	if err != nil || report.Status != app.StatusSucceeded {
		t.Fatalf("GuideApply() = %q, %v", report.Status, err)
	}
	if !reflect.DeepEqual(f.router.writes, []string{"RebootVerify"}) || report.Steps[3].Reboot == nil {
		t.Fatalf("writes = %v, reboot report %v; want exactly one reboot verification", f.router.writes, report.Steps[3].Reboot)
	}
}

var (
	guideRootKey  = credentials.NewKey(credentials.KindRoot, "acme-r1", "192.168.31.1", 22)
	guideAdminKey = credentials.NewKey(credentials.KindWebAdmin, "acme-r1", "192.168.31.1", 80)
)

func TestGuidePlanUsesASavedRootPassword(t *testing.T) {
	t.Parallel()

	f := newGuideFixture(t)
	f.configured(t)
	f.router.soft = app.SoftNotInstalled
	_ = f.vault.Put(guideRootKey, mustPassword(t, "root-secret"), time.Now())
	plan, err := f.service.GuidePlan(t.Context(), app.GuideRequest{Target: f.target, Goals: defaultGoals(), Credentials: app.CredentialUse{UseSaved: true}}, nil)
	if err != nil || !plan.Ready {
		t.Fatalf("GuidePlan() = ready %v, %v", plan.Ready, err)
	}
	if plan.State.RootSSH != app.RootSSHVerified || !plan.State.SavedRoot || plan.RequiredInputs != nil {
		t.Fatalf("state %+v, inputs %v; want root verified with the saved password and nothing to ask", plan.State, plan.RequiredInputs)
	}
	if !hasNote(plan.Credentials, "root", app.CredentialUsedSaved) {
		t.Fatalf("notes = %+v", plan.Credentials)
	}

	// The run fills the saved password into each step; nothing was typed.
	report, err := f.service.GuideApply(t.Context(), app.GuideApplyRequest{
		Confirmation: *plan.Confirmation, PlanDigest: plan.PlanDigest, Confirmed: true, Credentials: app.CredentialUse{UseSaved: true},
	}, nil)
	if err != nil || report.Status != app.StatusSucceeded || f.router.count("InstallSoft") != 1 {
		t.Fatalf("GuideApply() = %q, %v; writes %v", report.Status, err, f.router.writes)
	}
}

func TestGuidePlanMarksARejectedSavedRootPassword(t *testing.T) {
	t.Parallel()

	f := newGuideFixture(t)
	f.router.sshOpen, f.router.rootPassword = true, "changed-meanwhile"
	_ = f.vault.Put(guideRootKey, mustPassword(t, "root-secret"), time.Now())
	plan, err := f.service.GuidePlan(t.Context(), app.GuideRequest{Target: f.target, Goals: defaultGoals(), Credentials: app.CredentialUse{UseSaved: true}}, nil)
	if err != nil || plan.Steps[0].Action != app.ActionNeedsInput || plan.Steps[0].Reason != app.ReasonSSHAuthFailed {
		t.Fatalf("GuidePlan() = %+v, %v; want the current root password asked for", plan.Steps, err)
	}
	if _, entry, _, _ := f.vault.Get(guideRootKey); !entry.Rejected() || plan.State.SavedRoot {
		t.Fatalf("entry %+v saved-root %v; want the refused password marked rejected", entry, plan.State.SavedRoot)
	}
}

func TestGuidePlanCountsASavedAdminPasswordAsProvided(t *testing.T) {
	t.Parallel()

	f := newGuideFixture(t)
	_ = f.vault.Put(guideAdminKey, mustPassword(t, "admin-secret"), time.Now())
	plan, err := f.service.GuidePlan(t.Context(), app.GuideRequest{Target: f.target, Goals: defaultGoals(), Credentials: app.CredentialUse{UseSaved: true}}, nil)
	if err != nil || !plan.State.SavedAdmin || !reflect.DeepEqual(plan.RequiredInputs, []string{app.InputNewRootPassword}) {
		t.Fatalf("GuidePlan() = saved-admin %v inputs %v, %v; want only the new root password asked for", plan.State.SavedAdmin, plan.RequiredInputs, err)
	}

	// The enable step may use the saved admin password, but the new root password must be typed.
	request := app.GuideApplyRequest{Confirmation: *plan.Confirmation, PlanDigest: plan.PlanDigest, Confirmed: true, Credentials: app.CredentialUse{UseSaved: true}}
	_, err = f.service.GuideApply(t.Context(), request, nil)
	assertCode(t, err, device.CodeInvalidPassword)
	if f.writeCount() != 0 {
		t.Fatalf("writes %v before the new root password was typed", f.router.writes)
	}
	request.RootPassword = mustPassword(t, "new-root")
	request.Credentials.Remember = true
	report, err := f.service.GuideApply(t.Context(), request, nil)
	if err != nil || report.Status != app.StatusSucceeded {
		t.Fatalf("GuideApply() = %q, %v", report.Status, err)
	}
	if value, _, ok, _ := f.vault.Get(guideRootKey); !ok || value.Expose() != "new-root" {
		t.Fatalf("saved root %q %v, want the new root password remembered after the verified enable", value.Expose(), ok)
	}
}

func TestGuidePlanRemembersATypedRootPasswordOnlyAfterVerification(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		typed string
		saved bool
	}{{"root-secret", true}, {"wrong", false}} {
		f := newGuideFixture(t)
		f.configured(t)
		_, _ = f.service.GuidePlan(t.Context(), app.GuideRequest{
			Target: f.target, Goals: defaultGoals(), RootPassword: mustPassword(t, tc.typed),
			Credentials: app.CredentialUse{Remember: true},
		}, nil)
		value, _, ok, _ := f.vault.Get(guideRootKey)
		if ok != tc.saved || (ok && value.Expose() != tc.typed) {
			t.Fatalf("typed %q: saved %q %v, want saved=%v", tc.typed, value.Expose(), ok, tc.saved)
		}
	}
}
