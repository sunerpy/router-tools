package guiapi_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/credentials"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/guiapi"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
)

type fakeApplication struct {
	doctorRequest     app.DoctorRequest
	verifyRequest     app.VerifyRequest
	deepRequest       app.DeepRequest
	doctorReport      app.DoctorReport
	enableReport      app.EnableReport
	enableErr         error
	events            app.EventSink
	ctx               context.Context
	started           chan struct{}
	guideRequest      app.GuideRequest
	planReport        app.GuidePlanReport
	planErr           error
	applyRequest      app.GuideApplyRequest
	applyReport       app.GuideApplyReport
	applyErr          error
	ackTarget         device.Target
	credentialEntries []credentials.Entry
	deepStatusRequest app.DeepStatusRequest
	forgetFilter      credentials.Filter
	forgotAll         bool
}

func (f *fakeApplication) Doctor(ctx context.Context, request app.DoctorRequest, events app.EventSink) (app.DoctorReport, error) {
	f.doctorRequest = request
	f.events = events
	f.ctx = ctx
	if f.started != nil {
		close(f.started)
		<-ctx.Done()
		return f.doctorReport, ctx.Err()
	}
	f.doctorReport.Status = app.StatusSucceeded
	return f.doctorReport, nil
}

func (f *fakeApplication) Verify(_ context.Context, request app.VerifyRequest, _ app.EventSink) (app.VerifyReport, error) {
	f.verifyRequest = request
	return app.VerifyReport{Status: app.StatusSucceeded, RootVerified: true, SSHVerified: true}, nil
}

func (f *fakeApplication) EnableSSH(context.Context, app.EnableRequest, app.EventSink) (app.EnableReport, error) {
	return f.enableReport, f.enableErr
}

func (f *fakeApplication) PersistSoft(context.Context, app.SoftRequest, app.EventSink) (app.SoftReport, error) {
	return app.SoftReport{}, nil
}

func (f *fakeApplication) PersistRemove(context.Context, app.SoftRequest, app.EventSink) (app.SoftReport, error) {
	return app.SoftReport{}, nil
}

func (f *fakeApplication) RebootVerify(context.Context, app.SoftRequest, app.EventSink) (app.RebootReport, error) {
	return app.RebootReport{}, nil
}

func (f *fakeApplication) BackupCreate(context.Context, app.BackupCreateRequest, app.EventSink) (app.BackupCreateReport, error) {
	return app.BackupCreateReport{}, nil
}

func (f *fakeApplication) BackupVerify(context.Context, string, app.EventSink) (app.BackupVerifyReport, error) {
	return app.BackupVerifyReport{}, nil
}

func (f *fakeApplication) BackupVerifyOnline(context.Context, app.BackupOnlineVerifyRequest, app.EventSink) (app.BackupVerifyReport, error) {
	return app.BackupVerifyReport{}, nil
}

func (f *fakeApplication) DeepPersist(_ context.Context, request app.DeepRequest, _ app.EventSink) (app.DeepReport, error) {
	f.deepRequest = request
	return app.DeepReport{Status: app.StatusSucceeded}, nil
}

func (f *fakeApplication) GuidePlan(_ context.Context, request app.GuideRequest, events app.EventSink) (app.GuidePlanReport, error) {
	f.guideRequest = request
	if events != nil {
		events.Emit(app.Event{Operation: "guide plan", Stage: "probe", Status: "running"})
	}
	return f.planReport, f.planErr
}

func (f *fakeApplication) GuideApply(_ context.Context, request app.GuideApplyRequest, events app.EventSink) (app.GuideApplyReport, error) {
	f.applyRequest = request
	if events != nil {
		events.Emit(app.Event{Operation: "enable", Stage: "probe", Status: "running", Step: app.StepEnableSSH})
	}
	return f.applyReport, f.applyErr
}

func (f *fakeApplication) AcknowledgeRecovery(_ context.Context, target device.Target) (app.RecoveryReport, error) {
	f.ackTarget = target
	return app.RecoveryReport{Status: app.StatusSucceeded, Acknowledged: true}, nil
}

func (f *fakeApplication) DeepStatus(_ context.Context, request app.DeepStatusRequest, _ app.EventSink) (app.DeepStatusReport, error) {
	f.deepStatusRequest = request
	return app.DeepStatusReport{Operation: "deep status", Status: app.StatusSucceeded, Ready: true}, nil
}

func (f *fakeApplication) ListCredentials() (app.CredentialReport, error) {
	return app.CredentialReport{Operation: "credentials list", Status: app.StatusSucceeded, Entries: f.credentialEntries}, nil
}

func (f *fakeApplication) ForgetCredentials(filter credentials.Filter) (app.CredentialReport, error) {
	f.forgetFilter = filter
	return app.CredentialReport{Operation: "credentials forget", Status: app.StatusSucceeded}, nil
}

func (f *fakeApplication) ForgetAllCredentials() (app.CredentialReport, error) {
	f.forgotAll = true
	return app.CredentialReport{Operation: "credentials forget --all", Status: app.StatusSucceeded}, nil
}

func (f *fakeApplication) Catalog() []device.CatalogEntry {
	return []device.CatalogEntry{{ModelID: "xiaomi-rd08", Profiles: []device.Profile{{ID: "xiaomi-rd08-1.1.96"}}}}
}

func TestCatalogPassesThroughWithoutHistory(t *testing.T) {
	t.Parallel()

	service := guiapi.New(&fakeApplication{}, nil, nil)
	catalog := service.Catalog()
	if len(catalog) != 1 || catalog[0].Profiles[0].ID != "xiaomi-rd08-1.1.96" {
		t.Fatalf("Catalog() = %+v", catalog)
	}
	if len(service.ListReports()) != 0 {
		t.Fatal("Catalog() recorded a history entry")
	}
}

type eventRecorder struct {
	mu       sync.Mutex
	ids      []string
	messages []string
}

func (r *eventRecorder) Emit(id string, event app.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ids = append(r.ids, id)
	r.messages = append(r.messages, event.Operation+":"+event.Stage)
}

type confirmer struct{ called bool }

func (c *confirmer) Confirm(_ string, _ hostkey.Prompt) bool { c.called = true; return true }

func TestDoctorConvertsSecretsAndEmitsOperationEvents(t *testing.T) {
	t.Parallel()

	application := &fakeApplication{}
	events := &eventRecorder{}
	service := guiapi.New(application, events, &confirmer{})
	_, err := service.Doctor(t.Context(), guiapi.OperationRequest{
		OperationID:    "op-1",
		Target:         device.DefaultTarget(),
		WithAdminAuth:  true,
		AdminPassword:  "admin-secret",
		WithRootVerify: true,
		RootPassword:   "root-secret",
		Transport:      "ssh",
	})
	if err != nil {
		t.Fatal(err)
	}
	if application.doctorRequest.AdminPassword.Expose() != "admin-secret" || application.doctorRequest.RootPassword.Expose() != "root-secret" {
		t.Fatalf("doctor request = %+v", application.doctorRequest)
	}
	application.events.Emit(app.Event{Operation: "doctor", Stage: "completed"})
	if len(events.ids) != 1 || events.ids[0] != "op-1" {
		t.Fatalf("event IDs = %v", events.ids)
	}
	for _, message := range events.messages {
		if strings.Contains(message, "secret") {
			t.Fatalf("event contains secret: %s", message)
		}
	}
	if len(service.ListReports()) != 1 || service.ListReports()[0].Report == nil {
		t.Fatal("operation report history was not recorded with its report")
	}
}

func TestCancelOperation(t *testing.T) {
	t.Parallel()

	application := &fakeApplication{started: make(chan struct{})}
	service := guiapi.New(application, &eventRecorder{}, nil)
	done := make(chan error, 1)
	go func() {
		_, err := service.Doctor(t.Context(), guiapi.OperationRequest{OperationID: "cancel-me", Target: device.DefaultTarget()})
		done <- err
	}()
	<-application.started
	if !service.Cancel("cancel-me") {
		t.Fatal("Cancel() = false")
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("Doctor() error = %v, want cancellation", err)
	}
}

func TestDeepRequestKeepsDedicatedSafetyInputs(t *testing.T) {
	t.Parallel()

	application := &fakeApplication{}
	service := guiapi.New(application, &eventRecorder{}, nil)
	_, err := service.DeepPersist(t.Context(), guiapi.OperationRequest{
		OperationID:  "deep-1",
		Target:       device.DefaultTarget(),
		RootPassword: "root",
		AllowTelnet:  true,
		AckDeep:      "RD08-DEEP",
		BackupDir:    "/backup",
		RunDir:       "/run",
	})
	if err != nil {
		t.Fatal(err)
	}
	if application.deepRequest.AckDeep != "RD08-DEEP" || application.deepRequest.BackupDir != "/backup" || application.deepRequest.RunDir != "/run" {
		t.Fatalf("deep request = %+v", application.deepRequest)
	}
}

func TestVerifyUsesGUIHostKeyConfirmer(t *testing.T) {
	t.Parallel()

	application := &fakeApplication{}
	confirm := &confirmer{}
	service := guiapi.New(application, &eventRecorder{}, confirm)
	_, err := service.Verify(t.Context(), guiapi.OperationRequest{OperationID: "verify-1", Target: device.DefaultTarget(), RootPassword: "root"})
	if err != nil {
		t.Fatal(err)
	}
	if application.verifyRequest.HostKeyPolicy.Confirm == nil {
		t.Fatal("host-key Confirm was not wired")
	}
	if !application.verifyRequest.HostKeyPolicy.Confirm(hostkey.Prompt{}) || !confirm.called {
		t.Fatal("GUI host-key confirmation was not called")
	}
}

// Wails rejects the promise and discards the value whenever a bound method
// returns a non-nil error, so terminal reports must travel as the value.
func TestTerminalFailureReportIsDeliveredWithoutBindingError(t *testing.T) {
	t.Parallel()

	for _, status := range []string{app.StatusFailed, app.StatusPartial, app.StatusUnknown} {
		t.Run(status, func(t *testing.T) {
			t.Parallel()

			reportErr := &device.Error{Code: device.CodeMACCleanupFailed, Message: "cleanup failed"}
			application := &fakeApplication{
				enableReport: app.EnableReport{SchemaVersion: 1, Operation: "enable", Status: status, Error: reportErr},
				enableErr:    reportErr,
			}
			service := guiapi.New(application, &eventRecorder{}, nil)
			report, err := service.EnableSSH(t.Context(), guiapi.OperationRequest{OperationID: "enable-" + status, Target: device.DefaultTarget()})
			if err != nil {
				t.Fatalf("EnableSSH() error = %v, want the report delivered with a nil binding error", err)
			}
			if report.Status != status || report.Error == nil || report.Error.Code != device.CodeMACCleanupFailed {
				t.Fatalf("report = %+v", report)
			}
			history := service.ListReports()
			if len(history) != 1 {
				t.Fatalf("history length = %d, want 1", len(history))
			}
			recorded, ok := history[0].Report.(app.EnableReport)
			if !ok || recorded.Status != status {
				t.Fatalf("recorded report = %#v", history[0].Report)
			}
		})
	}
}

func TestNonTerminalReportKeepsBindingError(t *testing.T) {
	t.Parallel()

	for _, status := range []string{"", "running", app.StatusSucceeded} {
		t.Run("status="+status, func(t *testing.T) {
			t.Parallel()

			reportErr := &device.Error{Code: device.CodeInternalError, Message: "inconsistent report"}
			application := &fakeApplication{
				enableReport: app.EnableReport{Status: status},
				enableErr:    reportErr,
			}
			service := guiapi.New(application, &eventRecorder{}, nil)
			_, err := service.EnableSSH(t.Context(), guiapi.OperationRequest{OperationID: "enable-x", Target: device.DefaultTarget()})
			if !errors.Is(err, reportErr) {
				t.Fatalf("EnableSSH() error = %v, want the original error", err)
			}
		})
	}
}

func TestCanceledOperationReportIsDelivered(t *testing.T) {
	t.Parallel()

	canceled := &device.Error{Code: device.CodeCanceled, Message: "operation canceled"}
	application := &fakeApplication{
		started:      make(chan struct{}),
		doctorReport: app.DoctorReport{SchemaVersion: 1, Operation: "doctor", Status: app.StatusFailed, Error: canceled},
	}
	service := guiapi.New(application, &eventRecorder{}, nil)
	type outcome struct {
		report app.DoctorReport
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		report, err := service.Doctor(t.Context(), guiapi.OperationRequest{OperationID: "cancel-report", Target: device.DefaultTarget()})
		done <- outcome{report: report, err: err}
	}()
	<-application.started
	if !service.Cancel("cancel-report") {
		t.Fatal("Cancel() = false")
	}
	result := <-done
	if result.err != nil {
		t.Fatalf("Doctor() error = %v, want the canceled report delivered", result.err)
	}
	if result.report.Error == nil || result.report.Error.Code != device.CodeCanceled {
		t.Fatalf("report = %+v", result.report)
	}
}

func TestPreDispatchFailureLeavesNoHistoryAndFreesOperationID(t *testing.T) {
	t.Parallel()

	application := &fakeApplication{}
	service := guiapi.New(application, &eventRecorder{}, nil)
	_, err := service.Doctor(t.Context(), guiapi.OperationRequest{
		OperationID:   "op-reused",
		Target:        device.DefaultTarget(),
		WithAdminAuth: true,
		AdminPassword: "bad\x00password",
	})
	var deviceErr *device.Error
	if !errors.As(err, &deviceErr) || deviceErr.Code != device.CodeInvalidPassword {
		t.Fatalf("Doctor() error = %v, want INVALID_PASSWORD", err)
	}
	if history := service.ListReports(); len(history) != 0 {
		t.Fatalf("history = %+v, want no entry for an operation that never reached the application", history)
	}
	if service.Cancel("op-reused") {
		t.Fatal("Cancel() = true for an operation that already finished")
	}
	if _, err := service.Doctor(t.Context(), guiapi.OperationRequest{OperationID: "op-reused", Target: device.DefaultTarget()}); err != nil {
		t.Fatalf("Doctor() with the freed operation ID error = %v", err)
	}
	if service.Cancel("op-reused") {
		t.Fatal("Cancel() = true after the operation completed")
	}
}
