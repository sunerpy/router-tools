package cli_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/device"
)

type fakeVerifyService struct {
	fakeDoctorService
	report  app.VerifyReport
	err     error
	request app.VerifyRequest
}

func (s *fakeVerifyService) Verify(_ context.Context, request app.VerifyRequest, events app.EventSink) (app.VerifyReport, error) {
	s.request = request
	if events != nil {
		events.Emit(app.Event{Operation: "verify", Stage: "root_verify", Status: "running"})
	}
	return s.report, s.err
}

func TestVerifySSHCommand(t *testing.T) {
	t.Parallel()

	service := &fakeVerifyService{report: app.VerifyReport{
		SchemaVersion: 1,
		Operation:     "verify",
		Status:        app.StatusSucceeded,
		Transport:     "ssh",
		RootVerified:  true,
		SSHVerified:   true,
	}}
	code, stdout, stderr := executeWithInput(t, service, "root-secret\n",
		"--json", "--non-interactive", "--root-password-stdin", "--accept-new-host-key",
		"verify", "--transport", "ssh")
	if code != 0 {
		t.Fatalf("verify code = %d, stderr = %s", code, stderr)
	}
	if service.request.RootPassword.Expose() != "root-secret" || !service.request.HostKeyPolicy.AcceptNew {
		t.Fatalf("verify request = %+v", service.request)
	}
	if strings.Contains(stdout+stderr, "root-secret") {
		t.Fatal("verify output contains root password")
	}
	var report app.VerifyReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatal(err)
	}
	if !report.SSHVerified {
		t.Fatalf("report = %+v", report)
	}
}

func TestVerifyErrorExitCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code string
		want int
	}{
		{code: device.CodeSSHAuthFailed, want: 4},
		{code: device.CodeTelnetAuthFailed, want: 4},
		{code: device.CodeHostKeyMismatch, want: 5},
		{code: device.CodeSSHHostKeyRejected, want: 5},
		{code: device.CodeRemoteCommandFailed, want: 7},
		{code: device.CodeRootVerifyFailed, want: 8},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			operationErr := &device.Error{Code: tt.code, Message: "failed"}
			service := &fakeVerifyService{report: app.VerifyReport{Status: app.StatusFailed, Error: operationErr}, err: operationErr}
			code, _, _ := executeWithInput(t, service, "root\n", "--json", "--non-interactive", "--root-password-stdin", "verify")
			if code != tt.want {
				t.Fatalf("exit = %d, want %d", code, tt.want)
			}
		})
	}
}

func TestVerifyTelnetOutputDoesNotClaimSSH(t *testing.T) {
	t.Parallel()

	service := &fakeVerifyService{report: app.VerifyReport{
		Status: app.StatusSucceeded, Transport: "telnet", RootVerified: true, TelnetRootVerified: true,
	}}
	code, stdout, _ := executeWithInput(t, service, "root\n", "--allow-telnet", "verify", "--transport", "telnet")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(stdout, "not SSH success evidence") {
		t.Fatalf("stdout = %q", stdout)
	}
	if !service.request.AllowTelnet || service.request.Transport != "telnet" {
		t.Fatalf("request = %+v", service.request)
	}
}

func TestVerifyNonInteractiveRequiresRootPassword(t *testing.T) {
	t.Parallel()

	service := &fakeVerifyService{}
	code, _, _ := executeWithInput(t, service, "", "--non-interactive", "verify")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if service.request.RootPassword.Expose() != "" {
		t.Fatal("Verify was called without a password")
	}
}
