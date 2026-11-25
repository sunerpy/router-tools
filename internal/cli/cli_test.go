package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/buildinfo"
	"github.com/sunerpy/router-tools/internal/cli"
	"github.com/sunerpy/router-tools/internal/device"
)

type fakeDoctorService struct {
	report  app.DoctorReport
	err     error
	request app.DoctorRequest
}

func (s *fakeDoctorService) Doctor(_ context.Context, request app.DoctorRequest, events app.EventSink) (app.DoctorReport, error) {
	s.request = request
	if events != nil {
		events.Emit(app.Event{Operation: "doctor", Stage: "probe", Status: "running"})
	}
	return s.report, s.err
}

func TestVersionCommand(t *testing.T) {
	t.Parallel()

	code, stdout, stderr := execute(t, nil, buildinfo.Info{
		Version: "v1.2.3", CommitID: "abc123", BuildTime: "2026-01-02T03:04:05Z",
	}, "version")
	if code != 0 {
		t.Fatalf("Execute() code = %d, stderr = %s", code, stderr)
	}
	for _, want := range []string{"v1.2.3", "abc123", "2026-01-02T03:04:05Z"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("version stdout %q does not contain %q", stdout, want)
		}
	}
}

func TestDoctorJSONIsSingleObjectOnStdout(t *testing.T) {
	t.Parallel()

	service := &fakeDoctorService{report: app.DoctorReport{
		SchemaVersion: 1,
		Operation:     "doctor",
		Status:        app.StatusSucceeded,
		Host:          "192.168.31.1",
		Compatible:    true,
		Profile:       device.Profile{ID: "xiaomi-rd08-1.1.96"},
		AdminAuth:     app.CheckSkipped,
		RootVerify:    app.CheckSkipped,
	}}
	code, stdout, stderr := execute(t, service, testBuildInfo(),
		"--json", "doctor", "--host", "192.168.31.1", "--web-port", "8080")
	if code != 0 {
		t.Fatalf("Execute() code = %d, stderr = %s", code, stderr)
	}
	decoder := json.NewDecoder(strings.NewReader(stdout))
	var report app.DoctorReport
	if err := decoder.Decode(&report); err != nil {
		t.Fatalf("stdout is not JSON: %v; stdout = %q", err, stdout)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("stdout contains data after report: %v", err)
	}
	if report.Profile.ID != "xiaomi-rd08-1.1.96" {
		t.Fatalf("JSON profile = %q", report.Profile.ID)
	}
	if service.request.Target.WebPort != 8080 || service.request.ProfileID != "auto" {
		t.Fatalf("Doctor request = %+v", service.request)
	}
	if !strings.Contains(stderr, "probe") {
		t.Fatalf("progress stderr = %q, want probe stage", stderr)
	}
}

func TestDoctorFailurePrintsReportAndUsesCompatibilityExitCode(t *testing.T) {
	t.Parallel()

	operationErr := &device.Error{Code: device.CodeUnsupportedFirmware, Message: "unsupported firmware"}
	service := &fakeDoctorService{
		report: app.DoctorReport{
			SchemaVersion: 1,
			Operation:     "doctor",
			Status:        app.StatusFailed,
			Error:         operationErr,
		},
		err: operationErr,
	}
	code, stdout, _ := execute(t, service, testBuildInfo(), "--json", "doctor")
	if code != 5 {
		t.Fatalf("Execute() code = %d, want 5", code)
	}
	var report app.DoctorReport
	if err := json.Unmarshal(bytes.TrimSpace([]byte(stdout)), &report); err != nil {
		t.Fatalf("failed operation did not print a JSON report: %v", err)
	}
	if report.Error == nil || report.Error.Code != device.CodeUnsupportedFirmware {
		t.Fatalf("report error = %+v", report.Error)
	}
}

func TestCLIArgumentFailureUsesExitCodeTwo(t *testing.T) {
	t.Parallel()

	code, _, _ := execute(t, &fakeDoctorService{}, testBuildInfo(), "doctor", "--web-port", "invalid")
	if code != 2 {
		t.Fatalf("Execute() code = %d, want 2", code)
	}
}

func TestDoctorHumanOutputMarksAuthenticationAsUnverified(t *testing.T) {
	t.Parallel()

	service := &fakeDoctorService{report: app.DoctorReport{
		SchemaVersion: 1,
		Operation:     "doctor",
		Status:        app.StatusSucceeded,
		Compatible:    true,
		Profile:       device.Profile{ID: "xiaomi-rd08-1.1.96"},
		Device:        device.Observation{Hardware: "RD08", ROMVersion: "1.1.96"},
		SSH:           device.PortOpen,
		AdminAuth:     app.CheckSkipped,
		RootVerify:    app.CheckSkipped,
	}}
	code, stdout, _ := execute(t, service, testBuildInfo(), "doctor")
	if code != 0 {
		t.Fatalf("Execute() code = %d", code)
	}
	for _, want := range []string{"RD08", "1.1.96", "open", "not authentication proof", "skipped"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("doctor stdout %q does not contain %q", stdout, want)
		}
	}
}

func TestCancellationUsesExitCode130(t *testing.T) {
	t.Parallel()

	for _, operationErr := range []error{
		context.Canceled,
		device.WrapError(device.CodeCanceled, "operation canceled", context.Canceled),
	} {
		service := &fakeDoctorService{err: operationErr, report: app.DoctorReport{Status: app.StatusFailed}}
		code, _, _ := execute(t, service, testBuildInfo(), "--json", "doctor")
		if code != 130 {
			t.Fatalf("Execute(%T) code = %d, want 130", operationErr, code)
		}
	}
}

func execute(t *testing.T, service cli.DoctorService, info buildinfo.Info, args ...string) (int, string, string) {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cli.Execute(t.Context(), args, &stdout, &stderr, service, info)
	return code, stdout.String(), stderr.String()
}

func testBuildInfo() buildinfo.Info {
	return buildinfo.Info{Version: "test", CommitID: "test", BuildTime: "test"}
}
