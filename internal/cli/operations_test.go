package cli_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/backup"
	"github.com/sunerpy/router-tools/internal/device"
)

type fakeAllService struct {
	fakeVerifyService
	enableRequest      app.EnableRequest
	softRequest        app.SoftRequest
	backupRequest      app.BackupCreateRequest
	deepRequest        app.DeepRequest
	enableReport       app.EnableReport
	softReport         app.SoftReport
	rebootReport       app.RebootReport
	backupReport       app.BackupCreateReport
	backupVerifyReport app.BackupVerifyReport
	deepReport         app.DeepReport
	operationErr       error
}

func (s *fakeAllService) EnableSSH(_ context.Context, request app.EnableRequest, _ app.EventSink) (app.EnableReport, error) {
	s.enableRequest = request
	if s.enableReport.Status == "" {
		s.enableReport.Status = app.StatusSucceeded
		s.enableReport.SSHVerified = true
	}
	return s.enableReport, s.operationErr
}

func (s *fakeAllService) PersistSoft(_ context.Context, request app.SoftRequest, _ app.EventSink) (app.SoftReport, error) {
	s.softRequest = request
	s.softReport.Status = app.StatusSucceeded
	s.softReport.Result.Verified = true
	return s.softReport, s.operationErr
}

func (s *fakeAllService) PersistRemove(_ context.Context, request app.SoftRequest, _ app.EventSink) (app.SoftReport, error) {
	return s.PersistSoft(context.Background(), request, nil)
}

func (s *fakeAllService) RebootVerify(_ context.Context, request app.SoftRequest, _ app.EventSink) (app.RebootReport, error) {
	s.softRequest = request
	s.rebootReport.Status = app.StatusSucceeded
	s.rebootReport.Result.OldBootID = "1"
	s.rebootReport.Result.NewBootID = "2"
	s.rebootReport.Result.SSHVerified = true
	s.rebootReport.Result.SoftVerified = true
	return s.rebootReport, s.operationErr
}

func (s *fakeAllService) BackupCreate(_ context.Context, request app.BackupCreateRequest, _ app.EventSink) (app.BackupCreateReport, error) {
	s.backupRequest = request
	s.backupReport.Status = app.StatusSucceeded
	s.backupReport.Manifest = &backup.Manifest{Complete: true}
	return s.backupReport, s.operationErr
}

func (s *fakeAllService) BackupVerify(context.Context, string, app.EventSink) (app.BackupVerifyReport, error) {
	s.backupVerifyReport.Status = app.StatusSucceeded
	s.backupVerifyReport.Valid = true
	return s.backupVerifyReport, s.operationErr
}

func (s *fakeAllService) DeepPersist(_ context.Context, request app.DeepRequest, _ app.EventSink) (app.DeepReport, error) {
	s.deepRequest = request
	s.deepReport.Status = app.StatusSucceeded
	s.deepReport.Result.FinalSSHVerified = true
	s.deepReport.Result.SoftVerified = true
	s.deepReport.Result.Transitions = []app.BootTransition{{}, {}, {}}
	return s.deepReport, s.operationErr
}

func TestEnableReadsTwoPasswordLinesAndConfirmedFlag(t *testing.T) {
	t.Parallel()

	service := &fakeAllService{}
	code, _, stderr := executeWithInput(t, service, "admin\r\nroot\r\n",
		"--json", "--non-interactive", "--admin-password-stdin", "--root-password-stdin", "--yes", "enable")
	if code != 0 {
		t.Fatalf("enable exit = %d, stderr = %s", code, stderr)
	}
	if service.enableRequest.AdminPassword.Expose() != "admin" || service.enableRequest.RootPassword.Expose() != "root" || !service.enableRequest.Confirmed {
		t.Fatalf("enable request = %+v", service.enableRequest)
	}
}

func TestOperationWritesPrivateRunReport(t *testing.T) {
	t.Parallel()

	service := &fakeAllService{}
	runDir := t.TempDir()
	code, _, _ := executeWithInput(t, service, "admin\nroot\n",
		"--json", "--non-interactive", "--admin-password-stdin", "--root-password-stdin", "--yes", "--run-dir", runDir, "enable")
	if code != 0 {
		t.Fatalf("enable exit = %d", code)
	}
	entries, err := os.ReadDir(runDir)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		found = true
		info, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}
		// Windows has no POSIX permission bits: os.Stat reports 0666 for any writable file.
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
			t.Fatalf("report mode = %o", info.Mode().Perm())
		}
	}
	if !found {
		t.Fatal("no JSON run report was written")
	}
}

func TestPartialOperationUsesExitCodeTen(t *testing.T) {
	t.Parallel()

	service := &fakeAllService{enableReport: app.EnableReport{Status: app.StatusPartial}, operationErr: &device.Error{Code: device.CodeMACCleanupFailed}}
	code, _, _ := executeWithInput(t, service, "admin\nroot\n",
		"--json", "--non-interactive", "--admin-password-stdin", "--root-password-stdin", "--yes", "enable")
	if code != 10 {
		t.Fatalf("partial exit = %d, want 10", code)
	}
}

func TestConfigDefaultsAndExplicitFlagPrecedence(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.json")
	data := []byte(`{"schema_version":1,"host":"config-router","web_port":8081,"ssh_port":22,"telnet_port":23,"profile":"auto","log_level":"info"}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	service := &fakeAllService{fakeVerifyService: fakeVerifyService{fakeDoctorService: fakeDoctorService{report: app.DoctorReport{Status: app.StatusSucceeded}}}}
	code, _, _ := executeWithInput(t, service, "", "--config", path, "--json", "doctor", "--web-port", "9090")
	if code != 0 {
		t.Fatalf("doctor exit = %d", code)
	}
	doctorRequest := service.fakeDoctorService.request
	if doctorRequest.Target.Host != "config-router" || doctorRequest.Target.WebPort != 9090 {
		t.Fatalf("target = %+v", doctorRequest.Target)
	}
}

func TestBackupAndDeepFlagsReachApplication(t *testing.T) {
	t.Parallel()

	service := &fakeAllService{}
	runDir := t.TempDir()
	code, _, stderr := executeWithInput(t, service, "root\n",
		"--json", "--non-interactive", "--root-password-stdin", "--allow-telnet", "--ack-deep", "RD08-DEEP",
		"--run-dir", runDir, "deep-persist", "--backup-dir", "/backup")
	if code != 0 {
		t.Fatalf("deep exit = %d, stderr = %s", code, stderr)
	}
	if service.deepRequest.BackupDir != "/backup" || service.deepRequest.RunDir != runDir || service.deepRequest.AckDeep != "RD08-DEEP" || !service.deepRequest.AllowTelnet {
		t.Fatalf("deep request = %+v", service.deepRequest)
	}

	service = &fakeAllService{}
	code, _, stderr = executeWithInput(t, service, "root\n", "--json", "--non-interactive", "--root-password-stdin", "backup", "create", "--output", "/new-backup")
	if code != 0 {
		t.Fatalf("backup exit = %d, stderr = %s", code, stderr)
	}
	if service.backupRequest.OutputDir != "/new-backup" {
		t.Fatalf("backup request = %+v", service.backupRequest)
	}
}
