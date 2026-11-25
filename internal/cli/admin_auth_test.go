package cli_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/buildinfo"
	"github.com/sunerpy/router-tools/internal/cli"
	"github.com/sunerpy/router-tools/internal/device"
)

func TestDoctorAdminPasswordFromStdin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantPass string
	}{
		{name: "LF", input: " admin \n", wantPass: " admin "},
		{name: "CRLF", input: " admin \r\n", wantPass: " admin "},
		{name: "EOF without newline", input: " admin ", wantPass: " admin "},
		{name: "Unicode", input: "密码🔐\n", wantPass: "密码🔐"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &fakeDoctorService{report: app.DoctorReport{
				SchemaVersion: 1,
				Status:        app.StatusSucceeded,
				AdminAuth:     app.CheckVerified,
			}}
			code, stdout, stderr := executeWithInput(t, service, tt.input,
				"--json", "--non-interactive", "--admin-password-stdin", "doctor", "--with-admin-auth")
			if code != 0 {
				t.Fatalf("ExecuteWithInput() code = %d, stderr = %s", code, stderr)
			}
			if !service.request.WithAdminAuth || service.request.AdminPassword.Expose() != tt.wantPass {
				t.Fatalf("admin request = %+v, password = %q", service.request, service.request.AdminPassword.Expose())
			}
			if strings.Contains(stdout+stderr, tt.wantPass) {
				t.Fatalf("output contains password: %s%s", stdout, stderr)
			}
			var report app.DoctorReport
			if err := json.Unmarshal(bytes.TrimSpace([]byte(stdout)), &report); err != nil {
				t.Fatalf("stdout JSON error = %v", err)
			}
			if report.AdminAuth != app.CheckVerified {
				t.Fatalf("AdminAuth = %q", report.AdminAuth)
			}
		})
	}
}

func TestDoctorWithRootVerifyReadsSecondPasswordLine(t *testing.T) {
	t.Parallel()

	service := &fakeDoctorService{report: app.DoctorReport{Status: app.StatusSucceeded, RootVerify: app.CheckVerified}}
	code, _, stderr := executeWithInput(t, service, "admin\nroot\n",
		"--json", "--non-interactive", "--admin-password-stdin", "--root-password-stdin",
		"doctor", "--with-admin-auth", "--with-root-verify")
	if code != 0 {
		t.Fatalf("doctor exit = %d, stderr = %s", code, stderr)
	}
	if service.request.AdminPassword.Expose() != "admin" || service.request.RootPassword.Expose() != "root" || !service.request.WithRootVerify {
		t.Fatalf("doctor request = %+v", service.request)
	}
}

func TestDoctorNonInteractiveRequiresAdminPasswordStdin(t *testing.T) {
	t.Parallel()

	service := &fakeDoctorService{}
	code, _, _ := executeWithInput(t, service, "", "--non-interactive", "doctor", "--with-admin-auth")
	if code != 2 {
		t.Fatalf("ExecuteWithInput() code = %d, want 2", code)
	}
	if service.request.WithAdminAuth {
		t.Fatal("Doctor service was called without a password")
	}
}

func TestDoctorAdminAuthenticationFailureUsesExitCodeFourAndRedactsOutput(t *testing.T) {
	t.Parallel()

	operationErr := &device.Error{
		Code:    device.CodeWebAuthFailed,
		Message: "admin-secret rejected at ;stok=token-secret/api",
	}
	service := &fakeDoctorService{
		report: app.DoctorReport{
			SchemaVersion: 1,
			Status:        app.StatusFailed,
			AdminAuth:     app.CheckFailed,
			Error:         operationErr,
		},
		err: operationErr,
	}
	code, stdout, stderr := executeWithInput(t, service, "admin-secret\n",
		"--json", "--non-interactive", "--admin-password-stdin", "doctor", "--with-admin-auth")
	if code != 4 {
		t.Fatalf("ExecuteWithInput() code = %d, want 4", code)
	}
	for _, hidden := range []string{"admin-secret", "token-secret"} {
		if strings.Contains(stdout+stderr, hidden) {
			t.Fatalf("output contains %q: %s%s", hidden, stdout, stderr)
		}
	}
}

func TestDoctorInteractiveNonTTYWarnsBeforeVisibleInput(t *testing.T) {
	t.Parallel()

	service := &fakeDoctorService{report: app.DoctorReport{Status: app.StatusSucceeded, AdminAuth: app.CheckVerified}}
	code, _, stderr := executeWithInput(t, service, "admin\n", "doctor", "--with-admin-auth")
	if code != 0 {
		t.Fatalf("ExecuteWithInput() code = %d, stderr = %s", code, stderr)
	}
	if !strings.Contains(stderr, "not a TTY") {
		t.Fatalf("stderr = %q, want non-TTY warning", stderr)
	}
	if service.request.AdminPassword.Expose() != "admin" {
		t.Fatalf("password = %q", service.request.AdminPassword.Expose())
	}
}

func TestDoctorRejectsEmptyAdminPasswordBeforeServiceCall(t *testing.T) {
	t.Parallel()

	service := &fakeDoctorService{}
	code, _, _ := executeWithInput(t, service, "\n", "--non-interactive", "--admin-password-stdin", "doctor", "--with-admin-auth")
	if code != 2 {
		t.Fatalf("ExecuteWithInput() code = %d, want 2", code)
	}
	if service.request.WithAdminAuth {
		t.Fatal("Doctor service was called with an empty password")
	}
}

func executeWithInput(t *testing.T, service cli.DoctorService, input string, args ...string) (int, string, string) {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cli.ExecuteWithInput(
		t.Context(),
		args,
		strings.NewReader(input),
		&stdout,
		&stderr,
		service,
		buildinfo.Info{Version: "test"},
	)
	return code, stdout.String(), stderr.String()
}
