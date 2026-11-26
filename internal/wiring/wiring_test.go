package wiring_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/cli"
	"github.com/sunerpy/router-tools/internal/credentials"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/guiapi"
	"github.com/sunerpy/router-tools/internal/secret"
	"github.com/sunerpy/router-tools/internal/state"
	"github.com/sunerpy/router-tools/internal/wiring"
)

// Every CLI command finds its backend by a type assertion, so the production service must satisfy all of
// them; otherwise a command would only report that it is not configured.
var (
	_ cli.DoctorService       = (*app.Service)(nil)
	_ cli.CatalogService      = (*app.Service)(nil)
	_ cli.VerifyService       = (*app.Service)(nil)
	_ cli.EnableService       = (*app.Service)(nil)
	_ cli.SoftService         = (*app.Service)(nil)
	_ cli.BackupService       = (*app.Service)(nil)
	_ cli.OnlineBackupService = (*app.Service)(nil)
	_ cli.DeepService         = (*app.Service)(nil)
	_ cli.GuideService        = (*app.Service)(nil)
	_ cli.RecoveryService     = (*app.Service)(nil)
	// routerctl mcp composes guiapi over the application service.
	_ guiapi.Application    = (*app.Service)(nil)
	_ cli.CredentialService = (*app.Service)(nil)
)

func isolateUserDirs(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("AppData", filepath.Join(home, "appdata"))
	return home
}

func TestPathsAreManagedUnderTheUserDirectories(t *testing.T) {
	home := isolateUserDirs(t)
	configDir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	paths, err := wiring.DefaultPaths()
	if err != nil {
		t.Fatalf("DefaultPaths() error = %v", err)
	}
	want := wiring.Paths{
		Locks:       filepath.Join(configDir, "routerctl", "locks"),
		State:       filepath.Join(configDir, "routerctl", "state"),
		DeepJournal: filepath.Join(configDir, "routerctl", "deep"),
		Backups:     filepath.Join(home, "RouterTools", "backups"),
		Credentials: filepath.Join(configDir, "routerctl", "credentials"),
	}
	if paths != want {
		t.Fatalf("DefaultPaths() = %+v, want %+v", paths, want)
	}
}

// The production service always carries the recovery guard: a record in the managed state directory is
// what AcknowledgeRecovery finds and acknowledges.
func TestProductionServiceUsesTheOutcomeStore(t *testing.T) {
	isolateUserDirs(t)
	paths, err := wiring.DefaultPaths()
	if err != nil {
		t.Fatal(err)
	}
	service, err := wiring.NewService(paths)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	target := device.DefaultTarget()
	store := state.FileStore{Dir: paths.State}
	at := time.Date(2026, 11, 26, 1, 0, 0, 0, time.UTC)
	if err := store.Save(device.RecoveryKey(target), state.Outcome{Operation: "enable", Status: state.StatusPartial, StartedAt: at, FinishedAt: at}); err != nil {
		t.Fatal(err)
	}
	report, err := service.AcknowledgeRecovery(t.Context(), target)
	if err != nil || !report.Acknowledged {
		t.Fatalf("AcknowledgeRecovery() = %+v, %v; want the managed record acknowledged", report, err)
	}
	if len(service.Catalog()) == 0 {
		t.Fatal("the production service has no registered profiles")
	}
	vault := credentials.FileVault{Dir: paths.Credentials}
	root, _ := secret.NewPassword("root-secret")
	if err := vault.Put(credentials.NewKey(credentials.KindRoot, "xiaomi-rd08", "192.168.31.1", 22), root, at); err != nil {
		t.Fatal(err)
	}
	if list, err := service.ListCredentials(); err != nil || len(list.Entries) != 1 {
		t.Fatalf("ListCredentials() = %+v, %v; want the managed vault", list, err)
	}
}
