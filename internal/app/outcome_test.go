package app_test

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
	"github.com/sunerpy/router-tools/internal/secret"
	"github.com/sunerpy/router-tools/internal/state"
)

// memoryOutcomes is an in-memory app.OutcomeStore with failure injection.
type memoryOutcomes struct {
	mu        sync.Mutex
	records   map[string]state.Outcome
	saves     int
	failSaveN int // fail the Nth save (1-based); 0 never fails
}

func newMemoryOutcomes() *memoryOutcomes {
	return &memoryOutcomes{records: make(map[string]state.Outcome)}
}

func (m *memoryOutcomes) Load(key string) (state.Outcome, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	outcome, ok := m.records[key]
	return outcome, ok, nil
}

func (m *memoryOutcomes) Save(key string, outcome state.Outcome) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saves++
	if m.failSaveN != 0 && m.saves == m.failSaveN {
		return errors.New("disk full")
	}
	outcome.SchemaVersion = 1
	m.records[key] = outcome
	return nil
}

func (m *memoryOutcomes) last(target device.Target) state.Outcome {
	outcome, _, _ := m.Load(device.RecoveryKey(target))
	return outcome
}

// guardedOps records which adapter methods ran and what the store held when they started.
type guardedOps struct {
	*allOps
	store      *memoryOutcomes
	target     device.Target
	seenStatus string
	soft       app.SoftResult
	reboot     app.RebootResult
	enable     app.EnableResult
	deep       app.DeepResult
	err        error
	onDeep     func()
}

func (g *guardedOps) observe() { g.seenStatus = g.store.last(g.target).Status }

func (g *guardedOps) EnableSSH(ctx context.Context, t device.Target, a, r secret.Password, p hostkey.Policy) (app.EnableResult, error) {
	g.observe()
	_, _ = g.allOps.EnableSSH(ctx, t, a, r, p)
	return g.enable, g.err
}

func (g *guardedOps) InstallSoft(ctx context.Context, t device.Target, r secret.Password, p hostkey.Policy, telnet bool) (app.SoftResult, error) {
	g.observe()
	_, _ = g.allOps.InstallSoft(ctx, t, r, p, telnet)
	return g.soft, g.err
}

func (g *guardedOps) RemoveSoft(ctx context.Context, t device.Target, r secret.Password, p hostkey.Policy, telnet bool) (app.SoftResult, error) {
	g.observe()
	_, _ = g.allOps.RemoveSoft(ctx, t, r, p, telnet)
	return g.soft, g.err
}

func (g *guardedOps) RebootVerify(ctx context.Context, t device.Target, r secret.Password, p hostkey.Policy, telnet bool) (app.RebootResult, error) {
	g.observe()
	_, _ = g.allOps.RebootVerify(ctx, t, r, p, telnet)
	return g.reboot, g.err
}

func (g *guardedOps) DeepPersist(ctx context.Context, params app.DeepParams) (app.DeepResult, error) {
	g.observe()
	if g.onDeep != nil {
		g.onDeep()
	}
	_, _ = g.allOps.DeepPersist(ctx, params)
	return g.deep, g.err
}

type guardFixture struct {
	service *app.Service
	ops     *guardedOps
	store   *memoryOutcomes
	target  device.Target
	clock   *time.Time
}

func newGuardFixture(t *testing.T) *guardFixture {
	t.Helper()
	store := newMemoryOutcomes()
	target := device.DefaultTarget()
	ops := &guardedOps{allOps: newAllOps(), store: store, target: target}
	registry, err := device.NewRegistry(device.Adapter{Driver: modelDriver{p: fullProfile("1.0")}, Operations: ops})
	if err != nil {
		t.Fatal(err)
	}
	discovery := &fakeProber{result: device.ProbeResult{Observation: device.Observation{DeviceID: "DEVICE", Hardware: "R1", ROMVersion: "1.0"}}}
	clock := time.Date(2026, 11, 26, 2, 0, 0, 0, time.UTC)
	fixture := &guardFixture{ops: ops, store: store, target: target, clock: &clock}
	service, err := app.NewServiceWithOptions(registry, discovery, app.Options{
		Outcomes: store,
		Clock: func() time.Time {
			*fixture.clock = fixture.clock.Add(time.Second)
			return *fixture.clock
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	fixture.service = service
	return fixture
}

type mutation struct {
	method string
	call   func(f *guardFixture, target device.Target) error
}

func mutations(t *testing.T) map[string]mutation {
	password, _ := secret.NewPassword("root")
	soft := func(target device.Target) app.SoftRequest {
		return app.SoftRequest{Target: target, RootPassword: password, Confirmed: true}
	}
	return map[string]mutation{
		"enable": {"EnableSSH", func(f *guardFixture, target device.Target) error {
			_, err := f.service.EnableSSH(t.Context(), app.EnableRequest{Target: target, AdminPassword: password, RootPassword: password, Confirmed: true}, nil)
			return err
		}},
		"persist soft": {"InstallSoft", func(f *guardFixture, target device.Target) error {
			_, err := f.service.PersistSoft(t.Context(), soft(target), nil)
			return err
		}},
		"persist remove": {"RemoveSoft", func(f *guardFixture, target device.Target) error {
			_, err := f.service.PersistRemove(t.Context(), soft(target), nil)
			return err
		}},
		"reboot verify": {"RebootVerify", func(f *guardFixture, target device.Target) error {
			_, err := f.service.RebootVerify(t.Context(), soft(target), nil)
			return err
		}},
		"deep persist": {"DeepPersist", func(f *guardFixture, target device.Target) error {
			_, err := f.service.DeepPersist(t.Context(), app.DeepRequest{
				Target: target, RootPassword: password, AllowTelnet: true, AckDeep: "ACME-DEEP",
				BackupDir: makeDeepBackupFixture(t, "acme-r1-1.0"), RunDir: t.TempDir(),
			}, nil)
			return err
		}},
	}
}

// Every mutating use case, called directly, refuses after an unacknowledged uncertain change, even when
// the caller changes a management port.
func TestMutationsRefuseAfterAnUncertainChange(t *testing.T) {
	t.Parallel()

	for name, m := range mutations(t) {
		t.Run(name, func(t *testing.T) {
			f := newGuardFixture(t)
			_ = f.store.Save(device.RecoveryKey(f.target), state.Outcome{
				Operation: "enable", Status: state.StatusPartial, DeviceWritePossible: true,
				StartedAt: time.Date(2026, 11, 26, 1, 0, 0, 0, time.UTC), FinishedAt: time.Date(2026, 11, 26, 1, 1, 0, 0, time.UTC),
			})
			otherPorts := f.target
			otherPorts.WebPort, otherPorts.SSHPort = 8080, 2222
			assertCode(t, m.call(f, otherPorts), device.CodeRecoveryReviewRequired)
			if f.ops.total() != 0 {
				t.Fatalf("adapter calls = %v, want none while recovery review is pending", f.ops.calls)
			}

			if _, err := f.service.AcknowledgeRecovery(t.Context(), f.target); err != nil {
				t.Fatalf("AcknowledgeRecovery() error = %v", err)
			}
			if err := m.call(f, f.target); err != nil && errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			if f.ops.calls[m.method] != 1 {
				t.Fatalf("after acknowledgement %s calls = %d, want 1", m.method, f.ops.calls[m.method])
			}
		})
	}
}

// The in_progress record is durable before the adapter runs, and a write that cannot be recorded never
// reaches the adapter.
func TestMutationsRecordInProgressBeforeTheAdapter(t *testing.T) {
	t.Parallel()

	for name, m := range mutations(t) {
		t.Run(name, func(t *testing.T) {
			f := newGuardFixture(t)
			_ = m.call(f, f.target)
			if f.ops.seenStatus != state.StatusInProgress {
				t.Fatalf("record when the adapter started = %q, want in_progress", f.ops.seenStatus)
			}

			g := newGuardFixture(t)
			g.store.failSaveN = 1
			assertCode(t, m.call(g, g.target), device.CodeStateStoreFailed)
			if g.ops.total() != 0 {
				t.Fatal("the adapter ran although the in_progress record could not be stored")
			}
		})
	}
}

func TestFailedSettleLeavesTheRouterBlocked(t *testing.T) {
	t.Parallel()

	f := newGuardFixture(t)
	f.ops.soft = app.SoftResult{Outcome: "installed", Verified: true, WriteStarted: true}
	f.store.failSaveN = 2 // begin succeeds, settle fails
	password, _ := secret.NewPassword("root")
	report, err := f.service.PersistSoft(t.Context(), app.SoftRequest{Target: f.target, RootPassword: password, Confirmed: true}, nil)
	assertCode(t, err, device.CodeStateStoreFailed)
	if report.Status != app.StatusSucceeded {
		t.Fatalf("report status = %q, want the device result to stay succeeded", report.Status)
	}
	if got := f.store.last(f.target); got.Status != state.StatusInProgress || !got.Blocks() {
		t.Fatalf("record after a failed settle = %+v, want a blocking in_progress record", got)
	}
}

// Only adapter stage evidence lowers write possibility; an error code alone never does.
func TestSettleUsesAdapterWriteEvidence(t *testing.T) {
	t.Parallel()

	password, _ := secret.NewPassword("root")
	authFailed := &device.Error{Code: device.CodeSSHAuthFailed, Message: "denied"}
	hostKey := &device.Error{Code: device.CodeHostKeyMismatch, Message: "changed"}
	cases := []struct {
		name   string
		setup  func(*guardedOps)
		call   func(*guardFixture) error
		blocks bool
	}{
		{"soft failed before any write", func(o *guardedOps) { o.err = authFailed }, func(f *guardFixture) error {
			_, err := f.service.PersistSoft(t.Context(), app.SoftRequest{Target: f.target, RootPassword: password, Confirmed: true}, nil)
			return err
		}, false},
		{"soft failed after a write", func(o *guardedOps) {
			o.soft, o.err = app.SoftResult{WriteStarted: true}, &device.Error{Code: device.CodeRemoteCommandFailed, Message: "mv failed"}
		}, func(f *guardFixture) error {
			_, err := f.service.PersistSoft(t.Context(), app.SoftRequest{Target: f.target, RootPassword: password, Confirmed: true}, nil)
			return err
		}, true},
		{"reboot submitted then host key mismatch", func(o *guardedOps) {
			o.reboot, o.err = app.RebootResult{WriteStarted: true}, hostKey
		}, func(f *guardFixture) error {
			_, err := f.service.RebootVerify(t.Context(), app.SoftRequest{Target: f.target, RootPassword: password, Confirmed: true}, nil)
			return err
		}, true},
		{"enable failed before any MAC", func(o *guardedOps) {
			o.enable, o.err = app.EnableResult{Status: app.StatusFailed}, &device.Error{Code: device.CodeWebAuthFailed, Message: "denied"}
		}, func(f *guardFixture) error {
			_, err := f.service.EnableSSH(t.Context(), app.EnableRequest{Target: f.target, AdminPassword: password, RootPassword: password, Confirmed: true}, nil)
			return err
		}, false},
		{"enable rejected after a MAC was attempted", func(o *guardedOps) {
			o.enable, o.err = app.EnableResult{Status: app.StatusFailed, AttemptedMACs: []string{"0A:11:22:33:44:01"}}, &device.Error{Code: device.CodeRemoteAPIRejected, Message: "rejected"}
		}, func(f *guardFixture) error {
			_, err := f.service.EnableSSH(t.Context(), app.EnableRequest{Target: f.target, AdminPassword: password, RootPassword: password, Confirmed: true}, nil)
			return err
		}, true},
		{"enable already verified", func(o *guardedOps) {
			o.enable = app.EnableResult{Status: app.StatusSucceeded, Outcome: app.OutcomeAlreadyVerified, SSHVerified: true}
		}, func(f *guardFixture) error {
			_, err := f.service.EnableSSH(t.Context(), app.EnableRequest{Target: f.target, AdminPassword: password, RootPassword: password, Confirmed: true}, nil)
			return err
		}, false},
		{"deep failed before the crash write", func(o *guardedOps) {
			o.deep, o.err = app.DeepResult{Status: app.StatusFailed}, &device.Error{Code: device.CodeBackupLayoutMismatch, Message: "layout"}
		}, func(f *guardFixture) error {
			_, err := f.service.DeepPersist(t.Context(), app.DeepRequest{Target: f.target, RootPassword: password, AllowTelnet: true, AckDeep: "ACME-DEEP", BackupDir: makeDeepBackupFixture(t, "acme-r1-1.0"), RunDir: t.TempDir()}, nil)
			return err
		}, false},
		{"deep unknown crash write", func(o *guardedOps) {
			o.deep, o.err = app.DeepResult{Status: app.StatusUnknown, WriteStarted: true}, &device.Error{Code: device.CodeRemoteResultUnknown, Message: "unknown"}
		}, func(f *guardFixture) error {
			_, err := f.service.DeepPersist(t.Context(), app.DeepRequest{Target: f.target, RootPassword: password, AllowTelnet: true, AckDeep: "ACME-DEEP", BackupDir: makeDeepBackupFixture(t, "acme-r1-1.0"), RunDir: t.TempDir()}, nil)
			return err
		}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newGuardFixture(t)
			tc.setup(f.ops)
			_ = tc.call(f)
			got := f.store.last(f.target)
			if got.Status == state.StatusInProgress || got.FinishedAt.IsZero() {
				t.Fatalf("record was not settled: %+v", got)
			}
			if got.Blocks() != tc.blocks {
				t.Fatalf("record %+v blocks = %v, want %v", got, got.Blocks(), tc.blocks)
			}
		})
	}
}

// The record keeps the adapter's stage evidence, so the operator can still see what may have changed after
// the report itself is gone (desktop history lives in memory).
func TestRecordKeepsTheWriteEvidence(t *testing.T) {
	t.Parallel()

	password, _ := secret.NewPassword("root")
	t.Run("enable", func(t *testing.T) {
		f := newGuardFixture(t)
		f.ops.enable = app.EnableResult{
			Status: app.StatusPartial, AcceptedSteps: 2, AttemptedMACs: []string{"0A:11:22:33:44:10", "0A:11:22:33:44:11"},
			Cleanup: []app.CleanupResult{{MAC: "0A:11:22:33:44:10", Success: true}, {MAC: "0A:11:22:33:44:11", Error: "timeout"}},
		}
		f.ops.err = &device.Error{Code: device.CodeMACCleanupFailed, Message: "cleanup"}
		_, _ = f.service.EnableSSH(t.Context(), app.EnableRequest{Target: f.target, AdminPassword: password, RootPassword: password, Confirmed: true}, nil)
		want := state.WriteEvidence{
			WriteStarted: true, AcceptedSteps: 2,
			AttemptedMACs: []string{"0A:11:22:33:44:10", "0A:11:22:33:44:11"}, UncleanedMACs: []string{"0A:11:22:33:44:11"},
		}
		if got := f.store.last(f.target).Evidence; !reflect.DeepEqual(got, want) {
			t.Fatalf("evidence = %+v, want %+v", got, want)
		}
	})
	t.Run("deep records its journal before the first write", func(t *testing.T) {
		f := newGuardFixture(t)
		runDir := t.TempDir()
		var inProgress state.Outcome
		f.ops.onDeep = func() { inProgress = f.store.last(f.target) }
		f.ops.deep = app.DeepResult{Status: app.StatusUnknown, WriteStarted: true, AcceptedWrites: 1}
		f.ops.err = &device.Error{Code: device.CodeRemoteResultUnknown, Message: "unknown"}
		report, _ := f.service.DeepPersist(t.Context(), app.DeepRequest{
			Target: f.target, RootPassword: password, AllowTelnet: true, AckDeep: "ACME-DEEP",
			BackupDir: makeDeepBackupFixture(t, "acme-r1-1.0"), RunDir: runDir,
		}, nil)
		if inProgress.Status != state.StatusInProgress || inProgress.Evidence.JournalPath != report.JournalPath || report.JournalPath == "" {
			t.Fatalf("in-progress record = %+v, journal %q; want the journal named before the adapter runs", inProgress, report.JournalPath)
		}
		final := f.store.last(f.target).Evidence
		if !reflect.DeepEqual(final, state.WriteEvidence{WriteStarted: true, AcceptedSteps: 1, JournalPath: report.JournalPath}) {
			t.Fatalf("final evidence = %+v", final)
		}
	})
	t.Run("soft", func(t *testing.T) {
		f := newGuardFixture(t)
		f.ops.soft, f.ops.err = app.SoftResult{WriteStarted: true}, &device.Error{Code: device.CodeRemoteCommandFailed, Message: "mv"}
		_, _ = f.service.PersistSoft(t.Context(), app.SoftRequest{Target: f.target, RootPassword: password, Confirmed: true}, nil)
		if got := f.store.last(f.target).Evidence; !got.WriteStarted || got.AcceptedSteps != 0 || got.AttemptedMACs != nil {
			t.Fatalf("evidence = %+v", got)
		}
	})
}
