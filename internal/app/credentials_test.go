package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/backup"
	"github.com/sunerpy/router-tools/internal/credentials"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
	"github.com/sunerpy/router-tools/internal/secret"
)

// passwordOps records the passwords and host-key policies each adapter method received.
type passwordOps struct {
	*allOps
	t           *testing.T
	admin, root []string
	policies    []hostkey.Policy
	err         error
	enable      app.EnableResult
}

func (o *passwordOps) record(admin, root secret.Password, policy hostkey.Policy) {
	if admin.Expose() != "" {
		o.admin = append(o.admin, admin.Expose())
	}
	o.root = append(o.root, root.Expose())
	o.policies = append(o.policies, policy)
}

func (o *passwordOps) VerifyAdmin(ctx context.Context, t device.Target, p secret.Password) error {
	o.admin = append(o.admin, p.Expose())
	_ = o.allOps.VerifyAdmin(ctx, t, p)
	return o.err
}

func (o *passwordOps) VerifyRoot(ctx context.Context, t device.Target, p secret.Password, h hostkey.Policy, transport string, telnet bool) (app.RootVerification, error) {
	o.record(secret.Password{}, p, h)
	result, _ := o.allOps.VerifyRoot(ctx, t, p, h, transport, telnet)
	if o.err != nil {
		return app.RootVerification{}, o.err
	}
	return result, nil
}

func (o *passwordOps) EnableSSH(ctx context.Context, t device.Target, a, r secret.Password, h hostkey.Policy) (app.EnableResult, error) {
	o.record(a, r, h)
	_, _ = o.allOps.EnableSSH(ctx, t, a, r, h)
	if o.enable.Status == "" {
		o.enable = app.EnableResult{Status: app.StatusSucceeded, SSHVerified: true, AcceptedSteps: 3}
	}
	return o.enable, o.err
}

func (o *passwordOps) InstallSoft(ctx context.Context, t device.Target, r secret.Password, h hostkey.Policy, telnet bool) (app.SoftResult, error) {
	o.record(secret.Password{}, r, h)
	_, _ = o.allOps.InstallSoft(ctx, t, r, h, telnet)
	return app.SoftResult{Verified: true, WriteStarted: o.err != nil}, o.err
}

func (o *passwordOps) RemoveSoft(ctx context.Context, t device.Target, r secret.Password, h hostkey.Policy, telnet bool) (app.SoftResult, error) {
	o.record(secret.Password{}, r, h)
	_, _ = o.allOps.RemoveSoft(ctx, t, r, h, telnet)
	return app.SoftResult{Verified: true}, o.err
}

func (o *passwordOps) RebootVerify(ctx context.Context, t device.Target, r secret.Password, h hostkey.Policy, telnet bool) (app.RebootResult, error) {
	o.record(secret.Password{}, r, h)
	_, _ = o.allOps.RebootVerify(ctx, t, r, h, telnet)
	return app.RebootResult{OldBootID: "1", NewBootID: "2", SSHVerified: true, SoftVerified: true}, o.err
}

func (o *passwordOps) CreateBackup(ctx context.Context, t device.Target, r secret.Password, h hostkey.Policy, options backup.CreateOptions) (backup.Manifest, error) {
	o.record(secret.Password{}, r, h)
	_, _ = o.allOps.CreateBackup(ctx, t, r, h, options)
	if o.err != nil {
		return backup.Manifest{}, o.err
	}
	writeBackupAt(o.t, options.OutputDir, options.Device)
	return backup.LoadManifest(options.OutputDir)
}

func (o *passwordOps) VerifyBackupOnline(ctx context.Context, t device.Target, r secret.Password, h hostkey.Policy, m backup.Manifest, obs device.Observation, profile string) error {
	o.record(secret.Password{}, r, h)
	_ = o.allOps.VerifyBackupOnline(ctx, t, r, h, m, obs, profile)
	return o.err
}

type credentialFixture struct {
	service *app.Service
	ops     *passwordOps
	store   *credentials.Memory
	target  device.Target
}

var (
	rootKey  = credentials.NewKey(credentials.KindRoot, "acme-r1", "192.168.31.1", 22)
	adminKey = credentials.NewKey(credentials.KindWebAdmin, "acme-r1", "192.168.31.1", 80)
	savedAt  = time.Date(2026, 11, 26, 5, 0, 0, 0, time.UTC)
)

func newCredentialFixture(t *testing.T) *credentialFixture {
	t.Helper()
	ops := &passwordOps{allOps: newAllOps(), t: t}
	store := credentials.NewMemory()
	registry, err := device.NewRegistry(device.Adapter{Driver: modelDriver{p: fullProfile("1.0")}, Operations: ops})
	if err != nil {
		t.Fatal(err)
	}
	discovery := &fakeProber{result: device.ProbeResult{
		Observation: device.Observation{DeviceID: "DEVICE", Hardware: "R1", ROMVersion: "1.0"}, SSH: device.PortOpen,
	}}
	service, err := app.NewServiceWithOptions(registry, discovery, app.Options{
		Credentials: store, BackupRoot: t.TempDir(), Clock: func() time.Time { return savedAt },
	})
	if err != nil {
		t.Fatal(err)
	}
	return &credentialFixture{service: service, ops: ops, store: store, target: device.DefaultTarget()}
}

func (f *credentialFixture) save(t *testing.T, key credentials.Key, raw string) {
	t.Helper()
	if err := f.store.Put(key, mustPassword(t, raw), savedAt); err != nil {
		t.Fatal(err)
	}
}

func saved(t *testing.T, store *credentials.Memory, key credentials.Key) (string, credentials.Entry, bool) {
	t.Helper()
	value, entry, ok, err := store.Get(key)
	if err != nil {
		t.Fatal(err)
	}
	return value.Expose(), entry, ok
}

func hasNote(notes []app.CredentialNote, kind, action string) bool {
	for _, note := range notes {
		if note.Kind == kind && note.Action == action {
			return true
		}
	}
	return false
}

func TestVerifyFillsTheSavedRootPasswordAfterResolution(t *testing.T) {
	t.Parallel()

	f := newCredentialFixture(t)
	f.save(t, rootKey, "saved-root")
	report, err := f.service.Verify(t.Context(), app.VerifyRequest{
		Target: f.target, Credentials: app.CredentialUse{UseSaved: true},
		HostKeyPolicy: hostkey.Policy{AcceptNew: true},
	}, nil)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if len(f.ops.root) != 1 || f.ops.root[0] != "saved-root" {
		t.Fatalf("adapter received %v, want the saved root password", f.ops.root)
	}
	if f.ops.policies[0].AcceptNew {
		t.Fatal("a saved root password must never be sent to an automatically accepted new host key")
	}
	if !hasNote(report.Credentials, "root", app.CredentialUsedSaved) {
		t.Fatalf("notes = %+v, want used_saved", report.Credentials)
	}
}

func TestTypedPasswordsWinOverSavedOnes(t *testing.T) {
	t.Parallel()

	f := newCredentialFixture(t)
	f.save(t, rootKey, "saved-root")
	_, err := f.service.Verify(t.Context(), app.VerifyRequest{
		Target: f.target, RootPassword: mustPassword(t, "typed-root"), Credentials: app.CredentialUse{UseSaved: true},
		HostKeyPolicy: hostkey.Policy{AcceptNew: true},
	}, nil)
	if err != nil || f.ops.root[0] != "typed-root" || !f.ops.policies[0].AcceptNew {
		t.Fatalf("Verify() = %v; adapter got %v accept-new %v; want the typed password and the caller's policy", err, f.ops.root, f.ops.policies[0].AcceptNew)
	}
}

func TestSavedPasswordsThatCannotBeUsedFailBeforeTheAdapter(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		setup   func(*testing.T, *credentialFixture)
		request app.VerifyRequest
		code    string
	}{
		"nothing saved": {func(*testing.T, *credentialFixture) {}, app.VerifyRequest{}, device.CodeCredentialNotFound},
		"rejected entry": {func(t *testing.T, f *credentialFixture) {
			f.save(t, rootKey, "saved-root")
			_ = f.store.MarkRejected(rootKey, savedAt, savedAt)
		}, app.VerifyRequest{}, device.CodeCredentialNotFound},
		"Telnet allowed": {
			func(t *testing.T, f *credentialFixture) { f.save(t, rootKey, "saved-root") },
			app.VerifyRequest{Transport: "auto", AllowTelnet: true},
			device.CodeInvalidArgument,
		},
		"another model's entry": {func(t *testing.T, f *credentialFixture) {
			f.save(t, credentials.NewKey(credentials.KindRoot, "other-model", "192.168.31.1", 22), "saved-root")
		}, app.VerifyRequest{}, device.CodeCredentialNotFound},
		"another SSH port": {func(t *testing.T, f *credentialFixture) {
			f.save(t, credentials.NewKey(credentials.KindRoot, "acme-r1", "192.168.31.1", 2222), "saved-root")
		}, app.VerifyRequest{}, device.CodeCredentialNotFound},
		"broken vault": {func(_ *testing.T, f *credentialFixture) {
			f.store.Fail = &device.Error{Code: device.CodeCredentialStoreFailed, Message: "cannot decrypt"}
		}, app.VerifyRequest{}, device.CodeCredentialStoreFailed},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newCredentialFixture(t)
			tc.setup(t, f)
			request := tc.request
			request.Target, request.Credentials = f.target, app.CredentialUse{UseSaved: true}
			_, err := f.service.Verify(t.Context(), request, nil)
			assertCode(t, err, tc.code)
			if len(f.ops.root) != 0 {
				t.Fatalf("the adapter ran with %v", f.ops.root)
			}
		})
	}

	// Without UseSaved an empty password still fails before any network access.
	f := newCredentialFixture(t)
	f.save(t, rootKey, "saved-root")
	_, err := f.service.Verify(t.Context(), app.VerifyRequest{Target: f.target}, nil)
	assertCode(t, err, device.CodeInvalidPassword)
}

func TestARejectedSavedPasswordIsMarkedAndNotRetried(t *testing.T) {
	t.Parallel()

	f := newCredentialFixture(t)
	f.save(t, rootKey, "saved-root")
	f.ops.err = &device.Error{Code: device.CodeSSHAuthFailed, Message: "denied for saved-root"}
	report, err := f.service.Verify(t.Context(), app.VerifyRequest{Target: f.target, Credentials: app.CredentialUse{UseSaved: true}}, nil)
	assertCode(t, err, device.CodeSSHAuthFailed)
	if _, entry, _ := saved(t, f.store, rootKey); !entry.Rejected() || len(f.ops.root) != 1 {
		t.Fatalf("entry %+v after %d attempts; want it marked rejected after exactly one", entry, len(f.ops.root))
	}
	if !hasNote(report.Credentials, "root", app.CredentialRejected) {
		t.Fatalf("notes = %+v", report.Credentials)
	}
	if strings.Contains(report.Error.Message, "saved-root") {
		t.Fatalf("the error message leaks the saved password: %q", report.Error.Message)
	}
}

func TestRememberSavesOnlyPasswordsTheDeviceAccepted(t *testing.T) {
	t.Parallel()

	t.Run("accepted", func(t *testing.T) {
		t.Parallel()
		f := newCredentialFixture(t)
		report, err := f.service.Verify(t.Context(), app.VerifyRequest{
			Target: f.target, RootPassword: mustPassword(t, "typed-root"), Credentials: app.CredentialUse{Remember: true},
		}, nil)
		if value, _, ok := saved(t, f.store, rootKey); err != nil || !ok || value != "typed-root" || !hasNote(report.Credentials, "root", app.CredentialSaved) {
			t.Fatalf("Verify() = %v; saved %q %v; notes %+v", err, value, ok, report.Credentials)
		}
	})
	t.Run("rejected", func(t *testing.T) {
		t.Parallel()
		f := newCredentialFixture(t)
		f.ops.err = &device.Error{Code: device.CodeSSHAuthFailed, Message: "denied"}
		_, _ = f.service.Verify(t.Context(), app.VerifyRequest{
			Target: f.target, RootPassword: mustPassword(t, "typed-root"), Credentials: app.CredentialUse{Remember: true},
		}, nil)
		if _, _, ok := saved(t, f.store, rootKey); ok {
			t.Fatal("a password the device refused was saved")
		}
	})
	t.Run("saving fails", func(t *testing.T) {
		t.Parallel()
		f := newCredentialFixture(t)
		f.store.Fail = errors.New("disk full")
		report, err := f.service.Verify(t.Context(), app.VerifyRequest{
			Target: f.target, RootPassword: mustPassword(t, "typed-root"), Credentials: app.CredentialUse{Remember: true},
		}, nil)
		if err != nil || report.Status != app.StatusSucceeded || !hasNote(report.Credentials, "root", app.CredentialSaveFailed) {
			t.Fatalf("Verify() = %q, %v; notes %+v; want success with a save_failed note", report.Status, err, report.Credentials)
		}
	})
	t.Run("a verified enable with uncleaned MAC rules is partial and saves nothing", func(t *testing.T) {
		t.Parallel()
		f := newCredentialFixture(t)
		f.ops.enable = app.EnableResult{Status: app.StatusPartial, AcceptedSteps: 3, SSHVerified: true}
		_, _ = f.service.EnableSSH(t.Context(), app.EnableRequest{
			Target: f.target, AdminPassword: mustPassword(t, "new-admin"), RootPassword: mustPassword(t, "new-root"),
			Confirmed: true, Credentials: app.CredentialUse{Remember: true},
		}, nil)
		if _, _, ok := saved(t, f.store, adminKey); ok {
			t.Fatal("a partial enable saved the admin password")
		}
	})
	t.Run("a partial enable never overwrites", func(t *testing.T) {
		t.Parallel()
		f := newCredentialFixture(t)
		f.save(t, adminKey, "old-admin")
		f.ops.enable = app.EnableResult{Status: app.StatusPartial, AcceptedSteps: 2}
		f.ops.err = &device.Error{Code: device.CodeMACCleanupFailed, Message: "cleanup"}
		_, _ = f.service.EnableSSH(t.Context(), app.EnableRequest{
			Target: f.target, AdminPassword: mustPassword(t, "new-admin"), RootPassword: mustPassword(t, "new-root"),
			Confirmed: true, Credentials: app.CredentialUse{Remember: true},
		}, nil)
		if value, _, _ := saved(t, f.store, adminKey); value != "old-admin" {
			t.Fatalf("saved admin = %q, want the earlier value kept", value)
		}
		if _, _, ok := saved(t, f.store, rootKey); ok {
			t.Fatal("the root password of a partial enable was saved")
		}
	})
}

// Every use case that takes a root password fills it from the vault, and remembers a typed one only after
// the device accepted it over SSH.
func TestEveryRootPasswordUseCaseFillsAndRemembers(t *testing.T) {
	t.Parallel()

	calls := map[string]func(*credentialFixture, secret.Password, app.CredentialUse) ([]app.CredentialNote, error){
		"doctor": func(f *credentialFixture, root secret.Password, use app.CredentialUse) ([]app.CredentialNote, error) {
			r, err := f.service.Doctor(context.Background(), app.DoctorRequest{Target: f.target, WithRootVerify: true, RootPassword: root, Credentials: use}, nil)
			return r.Credentials, err
		},
		"persist soft": func(f *credentialFixture, root secret.Password, use app.CredentialUse) ([]app.CredentialNote, error) {
			r, err := f.service.PersistSoft(context.Background(), app.SoftRequest{Target: f.target, RootPassword: root, Confirmed: true, Credentials: use}, nil)
			return r.Credentials, err
		},
		"persist remove": func(f *credentialFixture, root secret.Password, use app.CredentialUse) ([]app.CredentialNote, error) {
			r, err := f.service.PersistRemove(context.Background(), app.SoftRequest{Target: f.target, RootPassword: root, Confirmed: true, Credentials: use}, nil)
			return r.Credentials, err
		},
		"reboot verify": func(f *credentialFixture, root secret.Password, use app.CredentialUse) ([]app.CredentialNote, error) {
			r, err := f.service.RebootVerify(context.Background(), app.SoftRequest{Target: f.target, RootPassword: root, Confirmed: true, Credentials: use}, nil)
			return r.Credentials, err
		},
		"backup create": func(f *credentialFixture, root secret.Password, use app.CredentialUse) ([]app.CredentialNote, error) {
			r, err := f.service.BackupCreate(context.Background(), app.BackupCreateRequest{Target: f.target, RootPassword: root, Credentials: use}, nil)
			return r.Credentials, err
		},
		"backup verify online": func(f *credentialFixture, root secret.Password, use app.CredentialUse) ([]app.CredentialNote, error) {
			r, err := f.service.BackupVerifyOnline(context.Background(), app.BackupOnlineVerifyRequest{
				BackupCreateRequest: app.BackupCreateRequest{Target: f.target, RootPassword: root, Credentials: use},
				BackupDir:           makeDeepBackupFixture(f.ops.t, "acme-r1-1.0"),
			}, nil)
			return r.Credentials, err
		},
	}
	for name, call := range calls {
		t.Run(name+" fills", func(t *testing.T) {
			t.Parallel()
			f := newCredentialFixture(t)
			f.save(t, rootKey, "saved-root")
			notes, err := call(f, secret.Password{}, app.CredentialUse{UseSaved: true})
			if err != nil || len(f.ops.root) == 0 || f.ops.root[0] != "saved-root" || !hasNote(notes, "root", app.CredentialUsedSaved) {
				t.Fatalf("err %v; adapter got %v; notes %+v", err, f.ops.root, notes)
			}
		})
		t.Run(name+" remembers", func(t *testing.T) {
			t.Parallel()
			f := newCredentialFixture(t)
			if _, err := call(f, mustPassword(t, "typed-root"), app.CredentialUse{Remember: true}); err != nil {
				t.Fatalf("%s error = %v", name, err)
			}
			value, _, ok := saved(t, f.store, rootKey)
			if !ok || value != "typed-root" {
				t.Fatalf("saved %q %v, want the accepted root password", value, ok)
			}
		})
	}
}

func TestDoctorAndEnableHandleTheAdminPassword(t *testing.T) {
	t.Parallel()

	f := newCredentialFixture(t)
	f.save(t, adminKey, "saved-admin")
	report, err := f.service.Doctor(t.Context(), app.DoctorRequest{Target: f.target, WithAdminAuth: true, Credentials: app.CredentialUse{UseSaved: true}}, nil)
	if err != nil || len(f.ops.admin) != 1 || f.ops.admin[0] != "saved-admin" || !hasNote(report.Credentials, "web-admin", app.CredentialUsedSaved) {
		t.Fatalf("Doctor() = %v; admin %v; notes %+v", err, f.ops.admin, report.Credentials)
	}

	// The enable flow may fill the admin password, but the root password it sets is always typed.
	_, err = f.service.EnableSSH(t.Context(), app.EnableRequest{Target: f.target, Confirmed: true, Credentials: app.CredentialUse{UseSaved: true}}, nil)
	assertCode(t, err, device.CodeInvalidPassword)
	f.ops.admin = nil
	enable, err := f.service.EnableSSH(t.Context(), app.EnableRequest{
		Target: f.target, RootPassword: mustPassword(t, "new-root"), Confirmed: true,
		Credentials: app.CredentialUse{UseSaved: true, Remember: true},
	}, nil)
	if err != nil || f.ops.admin[0] != "saved-admin" {
		t.Fatalf("EnableSSH() = %v; admin %v", err, f.ops.admin)
	}
	if value, _, ok := saved(t, f.store, rootKey); !ok || value != "new-root" || !hasNote(enable.Credentials, "root", app.CredentialSaved) {
		t.Fatalf("root saved %q %v; notes %+v", value, ok, enable.Credentials)
	}

	f.ops.err = &device.Error{Code: device.CodeWebAuthFailed, Message: "denied"}
	_, err = f.service.Doctor(t.Context(), app.DoctorRequest{Target: f.target, WithAdminAuth: true, Credentials: app.CredentialUse{UseSaved: true}}, nil)
	assertCode(t, err, device.CodeWebAuthFailed)
	if _, entry, _ := saved(t, f.store, adminKey); !entry.Rejected() {
		t.Fatal("a refused saved admin password was not marked rejected")
	}
}

func TestListAndForgetSavedPasswordsNeverReturnValues(t *testing.T) {
	t.Parallel()

	f := newCredentialFixture(t)
	f.save(t, rootKey, "saved-root")
	f.save(t, adminKey, "saved-admin")
	f.save(t, credentials.NewKey(credentials.KindRoot, "acme-r1", "192.168.31.2", 22), "other-root")
	list, err := f.service.ListCredentials()
	if err != nil || len(list.Entries) != 3 || list.Status != app.StatusSucceeded {
		t.Fatalf("ListCredentials() = %+v, %v", list, err)
	}
	if data, _ := jsonMarshal(list); strings.Contains(data, "saved-root") || strings.Contains(data, "saved-admin") {
		t.Fatal("the list contains a password")
	}
	_, err = f.service.ForgetCredentials(credentials.Filter{})
	assertCode(t, err, device.CodeInvalidArgument)
	_, err = f.service.ForgetCredentials(credentials.Filter{Kind: "telnet"})
	assertCode(t, err, device.CodeInvalidArgument)
	forgotten, err := f.service.ForgetCredentials(credentials.Filter{Host: "192.168.31.1"})
	if err != nil || len(forgotten.Entries) != 2 {
		t.Fatalf("ForgetCredentials(host) = %+v, %v", forgotten, err)
	}
	if _, err := f.service.ForgetAllCredentials(); err != nil {
		t.Fatal(err)
	}
	if list, _ := f.service.ListCredentials(); len(list.Entries) != 0 {
		t.Fatalf("entries after forgetting all = %+v", list.Entries)
	}
}

func jsonMarshal(value any) (string, error) {
	data, err := json.Marshal(value)
	return string(data), err
}

// A run that may have used Telnet never saves the root password, because success is not SSH evidence.
func TestTelnetCapableRunsNeverRememberTheRootPassword(t *testing.T) {
	t.Parallel()

	f := newCredentialFixture(t)
	if _, err := f.service.PersistSoft(t.Context(), app.SoftRequest{
		Target: f.target, RootPassword: mustPassword(t, "typed-root"), AllowTelnet: true, Confirmed: true,
		Credentials: app.CredentialUse{Remember: true},
	}, nil); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := saved(t, f.store, rootKey); ok {
		t.Fatal("a Telnet-capable soft install remembered the root password")
	}
}

// The caller's redactor learns every saved value the application filled in, so a frontend's own output
// pass also hides it.
func TestFilledSavedValuesReachTheCallersRedactor(t *testing.T) {
	t.Parallel()

	f := newCredentialFixture(t)
	f.save(t, rootKey, "saved-root-value")
	redactor := secret.NewRedactor()
	if _, err := f.service.Verify(t.Context(), app.VerifyRequest{
		Target: f.target, Credentials: app.CredentialUse{UseSaved: true, Redactor: redactor},
	}, nil); err != nil {
		t.Fatal(err)
	}
	if got := redactor.Text("output with saved-root-value"); strings.Contains(got, "saved-root-value") {
		t.Fatalf("redacted text = %q", got)
	}
}
