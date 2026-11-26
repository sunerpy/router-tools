package cli_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/credentials"
	"github.com/sunerpy/router-tools/internal/device"
)

// fakeCredentialService adds the saved-password backends to every operation fake.
type fakeCredentialService struct {
	fakeAllService
	entries   []credentials.Entry
	filters   []credentials.Filter
	forgotAll int
	listErr   error
}

func (s *fakeCredentialService) ListCredentials() (app.CredentialReport, error) {
	return app.CredentialReport{Operation: "credentials list", Status: app.StatusSucceeded, Entries: s.entries}, s.listErr
}

func (s *fakeCredentialService) ForgetCredentials(filter credentials.Filter) (app.CredentialReport, error) {
	s.filters = append(s.filters, filter)
	return app.CredentialReport{Operation: "credentials forget", Status: app.StatusSucceeded, Entries: s.entries[:1]}, nil
}

func (s *fakeCredentialService) ForgetAllCredentials() (app.CredentialReport, error) {
	s.forgotAll++
	return app.CredentialReport{Operation: "credentials forget --all", Status: app.StatusSucceeded}, nil
}

func savedRootEntry(port int, rejected bool) credentials.Entry {
	entry := credentials.Entry{Key: credentials.NewKey(credentials.KindRoot, "xiaomi-rd08", "192.168.31.1", port), SavedAt: time.Date(2026, 11, 26, 5, 0, 0, 0, time.UTC)}
	if rejected {
		entry.RejectedAt = entry.SavedAt.Add(time.Hour)
	}
	return entry
}

func TestSavedPasswordFlagsReachTheApplicationWithoutReadingStdin(t *testing.T) {
	t.Parallel()

	service := &fakeCredentialService{}
	service.report = app.VerifyReport{Status: app.StatusSucceeded, RootVerified: true, SSHVerified: true}
	code, _, stderr := executeWithInput(t, service, "must-not-be-read\n",
		"--non-interactive", "--use-saved-password", "--remember-password", "--run-dir", t.TempDir(), "verify")
	if code != 0 {
		t.Fatalf("verify exit = %d, stderr = %s", code, stderr)
	}
	request := service.request
	if request.RootPassword.Expose() != "" || !request.Credentials.UseSaved || !request.Credentials.Remember {
		t.Fatalf("verify request = password %q credentials %+v; want the vault to fill it", request.RootPassword.Expose(), request.Credentials)
	}
}

func TestInteractiveRunsPromptOnlyWithoutAUsableSavedPassword(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		entries []credentials.Entry
		prompt  bool
	}{
		"saved":         {[]credentials.Entry{savedRootEntry(22, false)}, false},
		"another port":  {[]credentials.Entry{savedRootEntry(2222, false)}, true},
		"rejected":      {[]credentials.Entry{savedRootEntry(22, true)}, true},
		"nothing saved": {nil, true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			service := &fakeCredentialService{entries: tc.entries}
			service.report = app.VerifyReport{Status: app.StatusSucceeded, RootVerified: true, SSHVerified: true}
			code, _, stderr := executeWithInput(t, service, "typed-root\n", "--use-saved-password", "--run-dir", t.TempDir(), "verify")
			if code != 0 {
				t.Fatalf("verify exit = %d, stderr = %s", code, stderr)
			}
			prompted := strings.Contains(stderr, "Root password")
			typed := service.request.RootPassword.Expose()
			if prompted != tc.prompt || (tc.prompt && typed != "typed-root") || (!tc.prompt && typed != "") {
				t.Fatalf("prompted %v typed %q, want prompt=%v", prompted, typed, tc.prompt)
			}
		})
	}
}

func TestEnableAlwaysAsksForTheRootPasswordItSets(t *testing.T) {
	t.Parallel()

	service := &fakeCredentialService{}
	code, _, stderr := executeWithInput(t, service, "new-root\n",
		"--non-interactive", "--use-saved-password", "--root-password-stdin", "--yes", "--run-dir", t.TempDir(), "enable")
	if code != 0 {
		t.Fatalf("enable exit = %d, stderr = %s", code, stderr)
	}
	request := service.enableRequest
	if request.AdminPassword.Expose() != "" || request.RootPassword.Expose() != "new-root" || !request.Credentials.UseSaved {
		t.Fatalf("enable request = admin %q root %q credentials %+v", request.AdminPassword.Expose(), request.RootPassword.Expose(), request.Credentials)
	}
}

func TestDeepPersistRefusesSavedPasswords(t *testing.T) {
	t.Parallel()

	for _, flag := range []string{"--use-saved-password", "--remember-password"} {
		service := &fakeCredentialService{}
		code, _, stderr := executeWithInput(t, service, "root\n", "--non-interactive", flag, "--root-password-stdin",
			"--allow-telnet", "--ack-deep", "RD08-DEEP", "--run-dir", t.TempDir(), "deep-persist", "--backup-dir", "/backup")
		if code != 2 || service.deepRequest.AckDeep != "" || !strings.Contains(stderr, "saved") {
			t.Fatalf("%s: exit %d deep called %v stderr %q; want 2 before the operation", flag, code, service.deepRequest.AckDeep != "", stderr)
		}
	}
}

func TestCredentialsListAndForget(t *testing.T) {
	t.Parallel()

	service := &fakeCredentialService{entries: []credentials.Entry{savedRootEntry(22, false), savedRootEntry(2222, true)}}
	code, stdout, _ := executeWithInput(t, service, "", "--json", "credentials", "list")
	var report app.CredentialReport
	if code != 0 || json.Unmarshal([]byte(stdout), &report) != nil || len(report.Entries) != 2 {
		t.Fatalf("list --json exit %d stdout %q", code, stdout)
	}
	code, stdout, _ = executeWithInput(t, service, "", "credentials", "list")
	if code != 0 || !strings.Contains(stdout, "root") || !strings.Contains(stdout, "2222") || !strings.Contains(stdout, "rejected") {
		t.Fatalf("list exit %d stdout %q", code, stdout)
	}

	code, _, _ = executeWithInput(t, service, "", "credentials", "forget")
	if code != 2 || len(service.filters) != 0 {
		t.Fatalf("forget without a filter: exit %d filters %v; want 2 and nothing deleted", code, service.filters)
	}
	code, stdout, _ = executeWithInput(t, service, "", "credentials", "forget", "--kind", "root", "--host", "ROUTER.lan", "--port", "22", "--scope", "xiaomi-rd08")
	want := credentials.Filter{Kind: credentials.KindRoot, Scope: "xiaomi-rd08", Host: "ROUTER.lan", Port: 22}
	if code != 0 || len(service.filters) != 1 || service.filters[0] != want || !strings.Contains(stdout, "192.168.31.1") {
		t.Fatalf("forget exit %d filters %+v stdout %q", code, service.filters, stdout)
	}
	code, _, _ = executeWithInput(t, service, "", "credentials", "forget", "--all", "--host", "192.168.31.1")
	if code != 2 || service.forgotAll != 0 {
		t.Fatalf("--all with a filter: exit %d forgot-all %d; want 2", code, service.forgotAll)
	}
	code, _, _ = executeWithInput(t, service, "", "credentials", "forget", "--all")
	if code != 0 || service.forgotAll != 1 {
		t.Fatalf("--all: exit %d forgot-all %d", code, service.forgotAll)
	}
}

func TestCredentialErrorCodesHaveDocumentedExitCodes(t *testing.T) {
	t.Parallel()

	cases := map[string]int{device.CodeCredentialNotFound: 2, device.CodeCredentialStoreFailed: 11}
	for code, want := range cases {
		service := &fakeCredentialService{listErr: &device.Error{Code: code, Message: "x"}}
		if got, _, _ := executeWithInput(t, service, "", "credentials", "list"); got != want {
			t.Fatalf("%s exit = %d, want %d", code, got, want)
		}
	}
}

func TestSetupUsesSavedPasswordsTheirPlanReports(t *testing.T) {
	t.Parallel()

	plan := nothingToDoPlan()
	plan.Steps[2] = app.GuideStep{ID: app.StepSoftInstall, Action: app.ActionRun, Reason: app.ReasonSoftNotInstalled, Effects: []string{app.EffectScriptWrite, app.EffectUCICommit}}
	plan.Confirmation.Steps[2] = app.GuideConfirmedStep{ID: app.StepSoftInstall, Action: app.ActionRun, Effects: []string{app.EffectScriptWrite, app.EffectUCICommit}}
	plan.Changes, plan.State.SavedRoot = true, true
	service := &fakeGuideService{plans: []app.GuidePlanReport{plan}}
	code, _, stderr := executeWithInput(t, service, "yes\n", "--use-saved-password", "--run-dir", t.TempDir(), "setup")
	if code != 0 || strings.Contains(stderr, "Root password") {
		t.Fatalf("setup exit %d stderr %q; want no root prompt when the plan uses the saved one", code, stderr)
	}
	if !service.planRequests[0].Credentials.UseSaved || len(service.applyRequests) != 1 || !service.applyRequests[0].Credentials.UseSaved {
		t.Fatalf("plan %+v apply %+v; want saved passwords requested in both", service.planRequests[0].Credentials, service.applyRequests)
	}
}

// A Telnet-capable run can never use a saved root password, so an interactive run asks for it instead of
// failing later.
func TestInteractiveTelnetRunsAskForTheRootPassword(t *testing.T) {
	t.Parallel()

	service := &fakeCredentialService{entries: []credentials.Entry{savedRootEntry(22, false)}}
	code, _, stderr := executeWithInput(t, service, "typed-root\nyes\n", "--use-saved-password", "--allow-telnet", "--run-dir", t.TempDir(), "persist", "soft")
	if code != 0 || !strings.Contains(stderr, "Root password") || service.softRequest.RootPassword.Expose() != "typed-root" {
		t.Fatalf("exit %d, stderr %q, root %q; want a prompt", code, stderr, service.softRequest.RootPassword.Expose())
	}
}

func TestCredentialUseCarriesTheCLIRedactor(t *testing.T) {
	t.Parallel()

	service := &fakeCredentialService{}
	service.report = app.VerifyReport{Status: app.StatusSucceeded, RootVerified: true, SSHVerified: true}
	if code, _, _ := executeWithInput(t, service, "", "--non-interactive", "--use-saved-password", "--run-dir", t.TempDir(), "verify"); code != 0 {
		t.Fatalf("verify exit = %d", code)
	}
	if service.request.Credentials.Redactor == nil {
		t.Fatal("the CLI must pass its redactor so filled saved values are hidden in its own output")
	}
}

// Human output tells the user what happened to saved passwords, not only --json.
func TestHumanOutputShowsCredentialNotes(t *testing.T) {
	t.Parallel()

	service := &fakeCredentialService{}
	service.report = app.VerifyReport{Status: app.StatusSucceeded, RootVerified: true, SSHVerified: true, Credentials: []app.CredentialNote{
		{Kind: "root", Action: app.CredentialUsedSaved},
		{Kind: "web-admin", Action: app.CredentialSaveFailed, Error: &device.Error{Code: device.CodeCredentialStoreFailed, Message: "disk full"}},
	}}
	code, stdout, _ := executeWithInput(t, service, "", "--non-interactive", "--use-saved-password", "--run-dir", t.TempDir(), "verify")
	if code != 0 || !strings.Contains(stdout, "Saved password (root): used") || !strings.Contains(stdout, "CREDENTIAL_STORE_FAILED: disk full") {
		t.Fatalf("exit %d stdout %q", code, stdout)
	}
	service.enableReport = app.EnableReport{Status: app.StatusSucceeded, SSHVerified: true, Credentials: []app.CredentialNote{{Kind: "root", Action: app.CredentialSaved}}}
	code, stdout, _ = executeWithInput(t, service, "admin-secret\nroot-secret\n", "--non-interactive", "--admin-password-stdin", "--root-password-stdin", "--yes", "--run-dir", t.TempDir(), "enable")
	if code != 0 || !strings.Contains(stdout, "Saved password (root): saved") {
		t.Fatalf("enable exit %d stdout %q", code, stdout)
	}
}
