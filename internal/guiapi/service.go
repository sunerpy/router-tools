// Package guiapi adapts typed application use cases to frontend bindings: the desktop app and the MCP server.
package guiapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/credentials"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
	"github.com/sunerpy/router-tools/internal/secret"
)

// Application is the complete frontend-independent router service.
type Application interface {
	Doctor(context.Context, app.DoctorRequest, app.EventSink) (app.DoctorReport, error)
	Verify(context.Context, app.VerifyRequest, app.EventSink) (app.VerifyReport, error)
	EnableSSH(context.Context, app.EnableRequest, app.EventSink) (app.EnableReport, error)
	PersistSoft(context.Context, app.SoftRequest, app.EventSink) (app.SoftReport, error)
	PersistRemove(context.Context, app.SoftRequest, app.EventSink) (app.SoftReport, error)
	RebootVerify(context.Context, app.SoftRequest, app.EventSink) (app.RebootReport, error)
	BackupCreate(context.Context, app.BackupCreateRequest, app.EventSink) (app.BackupCreateReport, error)
	BackupVerify(context.Context, string, app.EventSink) (app.BackupVerifyReport, error)
	BackupVerifyOnline(context.Context, app.BackupOnlineVerifyRequest, app.EventSink) (app.BackupVerifyReport, error)
	DeepPersist(context.Context, app.DeepRequest, app.EventSink) (app.DeepReport, error)
	GuidePlan(context.Context, app.GuideRequest, app.EventSink) (app.GuidePlanReport, error)
	GuideApply(context.Context, app.GuideApplyRequest, app.EventSink) (app.GuideApplyReport, error)
	AcknowledgeRecovery(context.Context, device.Target) (app.RecoveryReport, error)
	DeepStatus(context.Context, app.DeepStatusRequest, app.EventSink) (app.DeepStatusReport, error)
	ListCredentials() (app.CredentialReport, error)
	ForgetCredentials(credentials.Filter) (app.CredentialReport, error)
	ForgetAllCredentials() (app.CredentialReport, error)
	Catalog() []device.CatalogEntry
}

// EventEmitter forwards operation-scoped progress to the frontend.
type EventEmitter interface {
	Emit(operationID string, event app.Event)
}

// Confirmer handles unknown public host keys without hiding fingerprints.
type Confirmer interface {
	Confirm(operationID string, prompt hostkey.Prompt) bool
}

// HostKeyRequest contains only public host-key preferences.
type HostKeyRequest struct {
	KnownHostsPath string
	PinSHA256      string
	AcceptNew      bool
}

// OperationRequest is the desktop DTO. Password strings are converted immediately.
type OperationRequest struct {
	OperationID    string
	Target         device.Target
	ProfileID      string
	AdminPassword  string
	RootPassword   string
	Transport      string
	AllowTelnet    bool
	Confirmed      bool
	WithAdminAuth  bool
	WithRootVerify bool
	OutputDir      string
	BackupDir      string
	RunDir         string
	AckDeep        string
	ToolVersion    string
	HostKey        HostKeyRequest
	// UseSavedPassword fills passwords left empty from the vault; RememberPassword saves typed ones after
	// the router accepted them. Deep persistence ignores both and always needs the typed root password.
	UseSavedPassword bool
	RememberPassword bool
}

func (r OperationRequest) credentials() app.CredentialUse {
	return app.CredentialUse{UseSaved: r.UseSavedPassword, Remember: r.RememberPassword}
}

// GuideRequest is the desktop DTO of a guided plan or run. Password strings are converted immediately;
// Confirmation, PlanDigest and Confirmed are used only to apply a plan.
type GuideRequest struct {
	OperationID       string
	Target            device.Target
	ProfileID         string
	Goals             []string
	AdminPassword     string
	RootPassword      string
	ResetRootPassword bool
	Confirmation      *app.GuideConfirmation
	PlanDigest        string
	Confirmed         bool
	ToolVersion       string
	HostKey           HostKeyRequest
	UseSavedPassword  bool
	RememberPassword  bool
}

func (r GuideRequest) credentials() app.CredentialUse {
	return app.CredentialUse{UseSaved: r.UseSavedPassword, Remember: r.RememberPassword}
}

// CredentialFilter selects saved passwords to forget; empty fields match every entry.
type CredentialFilter struct {
	Kind  string
	Scope string
	Host  string
	Port  int
}

// HistoryEntry contains a report, never the originating secrets.
type HistoryEntry struct {
	OperationID string    `json:"operation_id"`
	Operation   string    `json:"operation"`
	Report      any       `json:"report"`
	Time        time.Time `json:"time"`
}

// Service owns cancellation and operation-scoped event/history state.
//
// Bound methods resolve with the application report for failed, partial, and
// unknown results, because Wails rejects the promise and discards the value
// whenever a bound method returns a non-nil error. A binding error means either
// that the call failed before the application produced a report, or that the
// application returned an error with a report that does not carry a failure
// status; settle keeps that report in history only.
type Service struct {
	application Application
	emitter     EventEmitter
	confirmer   Confirmer
	mu          sync.Mutex
	operations  map[string]context.CancelFunc
	history     []HistoryEntry
}

// New constructs a desktop service.
func New(application Application, emitter EventEmitter, confirmer Confirmer) *Service {
	return &Service{application: application, emitter: emitter, confirmer: confirmer, operations: make(map[string]context.CancelFunc)}
}

// OperationRequest helpers.
func (r OperationRequest) adminPassword() (secret.Password, error) {
	return makePassword(r.AdminPassword)
}

func (r OperationRequest) rootPassword() (secret.Password, error) {
	return makePassword(r.RootPassword)
}

func makePassword(raw string) (secret.Password, error) {
	if raw == "" {
		return secret.Password{}, nil
	}
	return secret.NewPassword(raw)
}

// Doctor runs read-only discovery.
func (s *Service) Doctor(ctx context.Context, request OperationRequest) (app.DoctorReport, error) {
	operationCtx, finish, err := s.start(ctx, request.OperationID)
	if err != nil {
		return app.DoctorReport{}, err
	}
	defer finish()
	admin, err := request.adminPassword()
	if err != nil {
		return app.DoctorReport{}, err
	}
	root, err := request.rootPassword()
	if err != nil {
		return app.DoctorReport{}, err
	}
	report, err := s.application.Doctor(operationCtx, app.DoctorRequest{
		Target: request.Target, ProfileID: request.ProfileID,
		WithAdminAuth: request.WithAdminAuth, AdminPassword: admin,
		WithRootVerify: request.WithRootVerify, RootPassword: root,
		Transport: request.Transport, AllowTelnet: request.AllowTelnet,
		HostKeyPolicy: s.hostPolicy(request), Credentials: request.credentials(),
	}, s.events(request.OperationID))
	return report, s.settle(request.OperationID, "doctor", report, report.Status, err)
}

// Verify verifies root shell authentication.
func (s *Service) Verify(ctx context.Context, request OperationRequest) (app.VerifyReport, error) {
	operationCtx, finish, err := s.start(ctx, request.OperationID)
	if err != nil {
		return app.VerifyReport{}, err
	}
	defer finish()
	root, err := request.rootPassword()
	if err != nil {
		return app.VerifyReport{}, err
	}
	report, err := s.application.Verify(operationCtx, app.VerifyRequest{
		Target: request.Target, ProfileID: request.ProfileID, RootPassword: root,
		Transport: request.Transport, AllowTelnet: request.AllowTelnet, HostKeyPolicy: s.hostPolicy(request),
		Credentials: request.credentials(),
	}, s.events(request.OperationID))
	return report, s.settle(request.OperationID, "verify", report, report.Status, err)
}

// EnableSSH executes the ordinary-confirmed enable flow.
func (s *Service) EnableSSH(ctx context.Context, request OperationRequest) (app.EnableReport, error) {
	operationCtx, finish, err := s.start(ctx, request.OperationID)
	if err != nil {
		return app.EnableReport{}, err
	}
	defer finish()
	admin, err := request.adminPassword()
	if err != nil {
		return app.EnableReport{}, err
	}
	root, err := request.rootPassword()
	if err != nil {
		return app.EnableReport{}, err
	}
	report, err := s.application.EnableSSH(operationCtx, app.EnableRequest{
		Target: request.Target, ProfileID: request.ProfileID,
		AdminPassword: admin, RootPassword: root,
		HostKeyPolicy: s.hostPolicy(request), Confirmed: request.Confirmed, Credentials: request.credentials(),
	}, s.events(request.OperationID))
	return report, s.settle(request.OperationID, "enable", report, report.Status, err)
}

// PersistSoft installs soft persistence.
func (s *Service) PersistSoft(ctx context.Context, request OperationRequest) (app.SoftReport, error) {
	return s.soft(ctx, request, false)
}

// PersistRemove removes managed soft persistence.
func (s *Service) PersistRemove(ctx context.Context, request OperationRequest) (app.SoftReport, error) {
	return s.soft(ctx, request, true)
}

func (s *Service) soft(ctx context.Context, request OperationRequest, remove bool) (app.SoftReport, error) {
	operationCtx, finish, err := s.start(ctx, request.OperationID)
	if err != nil {
		return app.SoftReport{}, err
	}
	defer finish()
	operation := "persist soft"
	if remove {
		operation = "persist remove"
	}
	root, err := request.rootPassword()
	if err != nil {
		return app.SoftReport{}, err
	}
	applicationRequest := app.SoftRequest{
		Target: request.Target, ProfileID: request.ProfileID, RootPassword: root,
		HostKeyPolicy: s.hostPolicy(request), AllowTelnet: request.AllowTelnet, Confirmed: request.Confirmed,
		Credentials: request.credentials(),
	}
	var report app.SoftReport
	if remove {
		report, err = s.application.PersistRemove(operationCtx, applicationRequest, s.events(request.OperationID))
	} else {
		report, err = s.application.PersistSoft(operationCtx, applicationRequest, s.events(request.OperationID))
	}
	return report, s.settle(request.OperationID, operation, report, report.Status, err)
}

// RebootVerify verifies soft persistence across one reboot.
func (s *Service) RebootVerify(ctx context.Context, request OperationRequest) (app.RebootReport, error) {
	operationCtx, finish, err := s.start(ctx, request.OperationID)
	if err != nil {
		return app.RebootReport{}, err
	}
	defer finish()
	root, err := request.rootPassword()
	if err != nil {
		return app.RebootReport{}, err
	}
	report, err := s.application.RebootVerify(operationCtx, app.SoftRequest{
		Target: request.Target, ProfileID: request.ProfileID, RootPassword: root,
		HostKeyPolicy: s.hostPolicy(request), AllowTelnet: request.AllowTelnet, Confirmed: request.Confirmed,
		Credentials: request.credentials(),
	}, s.events(request.OperationID))
	return report, s.settle(request.OperationID, "reboot-verify", report, report.Status, err)
}

// BackupCreate creates a new MTD backup directory.
func (s *Service) BackupCreate(ctx context.Context, request OperationRequest) (app.BackupCreateReport, error) {
	operationCtx, finish, err := s.start(ctx, request.OperationID)
	if err != nil {
		return app.BackupCreateReport{}, err
	}
	defer finish()
	root, err := request.rootPassword()
	if err != nil {
		return app.BackupCreateReport{}, err
	}
	report, err := s.application.BackupCreate(operationCtx, app.BackupCreateRequest{
		Target: request.Target, ProfileID: request.ProfileID, RootPassword: root,
		HostKeyPolicy: s.hostPolicy(request), OutputDir: request.OutputDir, ToolVersion: request.ToolVersion,
		Credentials: request.credentials(),
	}, s.events(request.OperationID))
	return report, s.settle(request.OperationID, "backup create", report, report.Status, err)
}

// BackupVerify performs local backup verification.
func (s *Service) BackupVerify(ctx context.Context, request OperationRequest) (app.BackupVerifyReport, error) {
	operationCtx, finish, err := s.start(ctx, request.OperationID)
	if err != nil {
		return app.BackupVerifyReport{}, err
	}
	defer finish()
	report, err := s.application.BackupVerify(operationCtx, request.BackupDir, s.events(request.OperationID))
	return report, s.settle(request.OperationID, "backup verify", report, report.Status, err)
}

// BackupVerifyOnline adds online identity/layout/boot comparison.
func (s *Service) BackupVerifyOnline(ctx context.Context, request OperationRequest) (app.BackupVerifyReport, error) {
	operationCtx, finish, err := s.start(ctx, request.OperationID)
	if err != nil {
		return app.BackupVerifyReport{}, err
	}
	defer finish()
	root, err := request.rootPassword()
	if err != nil {
		return app.BackupVerifyReport{}, err
	}
	report, err := s.application.BackupVerifyOnline(operationCtx, app.BackupOnlineVerifyRequest{
		BackupCreateRequest: app.BackupCreateRequest{
			Target: request.Target, ProfileID: request.ProfileID,
			RootPassword: root, HostKeyPolicy: s.hostPolicy(request), Credentials: request.credentials(),
		},
		BackupDir: request.BackupDir,
	}, s.events(request.OperationID))
	return report, s.settle(request.OperationID, "backup verify online", report, report.Status, err)
}

// DeepPersist keeps every dedicated high-risk input intact.
func (s *Service) DeepPersist(ctx context.Context, request OperationRequest) (app.DeepReport, error) {
	operationCtx, finish, err := s.start(ctx, request.OperationID)
	if err != nil {
		return app.DeepReport{}, err
	}
	defer finish()
	root, err := request.rootPassword()
	if err != nil {
		return app.DeepReport{}, err
	}
	report, err := s.application.DeepPersist(operationCtx, app.DeepRequest{
		Target: request.Target, ProfileID: request.ProfileID, RootPassword: root,
		HostKeyPolicy: s.hostPolicy(request), AllowTelnet: request.AllowTelnet,
		AckDeep: request.AckDeep, BackupDir: request.BackupDir, RunDir: request.RunDir,
	}, s.events(request.OperationID))
	return report, s.settle(request.OperationID, "deep-persist", report, report.Status, err)
}

// GuidePlan detects the router and returns the guided plan. A plan changes nothing, so it is not recorded
// in the run history; a blocked or failed plan still resolves with its report.
func (s *Service) GuidePlan(ctx context.Context, request GuideRequest) (app.GuidePlanReport, error) {
	operationCtx, finish, err := s.start(ctx, request.OperationID)
	if err != nil {
		return app.GuidePlanReport{}, err
	}
	defer finish()
	root, err := makePassword(request.RootPassword)
	if err != nil {
		return app.GuidePlanReport{}, err
	}
	report, err := s.application.GuidePlan(operationCtx, app.GuideRequest{
		Target: request.Target, ProfileID: request.ProfileID, Goals: request.Goals, RootPassword: root,
		ResetRootPassword: request.ResetRootPassword, HostKeyPolicy: s.policy(request.OperationID, request.HostKey),
		Credentials: request.credentials(),
	}, s.events(request.OperationID))
	return report, resolution(report.Status, err)
}

// GuideApply runs a confirmed guided plan; the run enters the history as one entry.
func (s *Service) GuideApply(ctx context.Context, request GuideRequest) (app.GuideApplyReport, error) {
	if request.Confirmation == nil {
		return app.GuideApplyReport{}, fmt.Errorf("a guided run requires the confirmation returned by its plan")
	}
	operationCtx, finish, err := s.start(ctx, request.OperationID)
	if err != nil {
		return app.GuideApplyReport{}, err
	}
	defer finish()
	admin, err := makePassword(request.AdminPassword)
	if err != nil {
		return app.GuideApplyReport{}, err
	}
	root, err := makePassword(request.RootPassword)
	if err != nil {
		return app.GuideApplyReport{}, err
	}
	report, err := s.application.GuideApply(operationCtx, app.GuideApplyRequest{
		Confirmation: *request.Confirmation, PlanDigest: request.PlanDigest, Confirmed: request.Confirmed,
		AdminPassword: admin, RootPassword: root, HostKeyPolicy: s.policy(request.OperationID, request.HostKey),
		ToolVersion: request.ToolVersion, Credentials: request.credentials(),
	}, s.events(request.OperationID))
	return report, s.settle(request.OperationID, "guide apply", report, report.Status, err)
}

// AcknowledgeRecovery records that the operator checked the router after an uncertain change.
func (s *Service) AcknowledgeRecovery(ctx context.Context, request OperationRequest) (app.RecoveryReport, error) {
	operationCtx, finish, err := s.start(ctx, request.OperationID)
	if err != nil {
		return app.RecoveryReport{}, err
	}
	defer finish()
	report, err := s.application.AcknowledgeRecovery(operationCtx, request.Target)
	return report, s.settle(request.OperationID, "recovery ack", report, report.Status, err)
}

// DeepStatus reads the deep-persistence page's state without changing the router. It keeps no history.
func (s *Service) DeepStatus(ctx context.Context, request OperationRequest) (app.DeepStatusReport, error) {
	operationCtx, finish, err := s.start(ctx, request.OperationID)
	if err != nil {
		return app.DeepStatusReport{}, err
	}
	defer finish()
	root, err := request.rootPassword()
	if err != nil {
		return app.DeepStatusReport{}, err
	}
	report, err := s.application.DeepStatus(operationCtx, app.DeepStatusRequest{
		Target: request.Target, ProfileID: request.ProfileID, RootPassword: root,
		HostKeyPolicy: s.hostPolicy(request), Credentials: request.credentials(),
	}, s.events(request.OperationID))
	return report, resolution(report.Status, err)
}

// CredentialStatus lists the saved passwords' metadata; no binding ever returns a password.
func (s *Service) CredentialStatus() (app.CredentialReport, error) {
	report, err := s.application.ListCredentials()
	return report, resolution(report.Status, err)
}

// ForgetCredential forgets the saved passwords matching the filter.
func (s *Service) ForgetCredential(filter CredentialFilter) (app.CredentialReport, error) {
	report, err := s.application.ForgetCredentials(credentials.Filter{
		Kind: credentials.Kind(filter.Kind), Scope: filter.Scope, Host: filter.Host, Port: filter.Port,
	})
	return report, resolution(report.Status, err)
}

// ForgetAllCredentials forgets every saved password and deletes the vault key.
func (s *Service) ForgetAllCredentials() (app.CredentialReport, error) {
	report, err := s.application.ForgetAllCredentials()
	return report, resolution(report.Status, err)
}

// Catalog lists the registered device models and exact firmware profiles. It needs no operation ID, no
// network and records no history.
func (s *Service) Catalog() []device.CatalogEntry {
	return s.application.Catalog()
}

// Cancel requests cancellation of an active operation.
func (s *Service) Cancel(operationID string) bool {
	s.mu.Lock()
	cancel, ok := s.operations[operationID]
	s.mu.Unlock()
	if ok {
		cancel()
	}
	return ok
}

// ListReports returns a copy of operation history.
func (s *Service) ListReports() []HistoryEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]HistoryEntry, len(s.history))
	copy(result, s.history)
	return result
}

func (s *Service) start(parent context.Context, operationID string) (context.Context, func(), error) {
	if operationID == "" {
		return nil, nil, fmt.Errorf("operation ID is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.operations[operationID]; exists {
		return nil, nil, fmt.Errorf("operation ID %q is already active", operationID)
	}
	ctx, cancel := context.WithCancel(parent)
	s.operations[operationID] = cancel
	finish := func() {
		s.mu.Lock()
		delete(s.operations, operationID)
		s.mu.Unlock()
		cancel()
	}
	return ctx, finish, nil
}

// settle records a report returned by the application and chooses the binding
// error. Failed, partial, and unknown reports carry their status and redacted
// error themselves, so they resolve with a nil error. Any other status keeps the
// error, so the frontend never receives a report that claims success or is still
// running alongside an error.
func (s *Service) settle(operationID, operation string, report any, status string, err error) error {
	s.record(operationID, operation, report)
	return resolution(status, err)
}

// resolution is the binding error for a report with the given status: nil for failed, partial and unknown
// reports, which carry their own redacted error, and the error otherwise.
func resolution(status string, err error) error {
	if err == nil {
		return nil
	}
	switch status {
	case app.StatusFailed, app.StatusPartial, app.StatusUnknown:
		return nil
	default:
		return err
	}
}

func (s *Service) record(operationID, operation string, report any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = append([]HistoryEntry{{OperationID: operationID, Operation: operation, Report: report, Time: time.Now().UTC()}}, s.history...)
	if len(s.history) > 50 {
		s.history = s.history[:50]
	}
}

func (s *Service) events(operationID string) app.EventSink {
	return app.EventSinkFunc(func(event app.Event) {
		if s.emitter != nil {
			s.emitter.Emit(operationID, event)
		}
	})
}

func (s *Service) hostPolicy(request OperationRequest) hostkey.Policy {
	return s.policy(request.OperationID, request.HostKey)
}

func (s *Service) policy(operationID string, request HostKeyRequest) hostkey.Policy {
	path := request.KnownHostsPath
	if path == "" {
		if directory, err := os.UserConfigDir(); err == nil {
			path = filepath.Join(directory, "routerctl", "known_hosts")
		}
	}
	policy := hostkey.Policy{KnownHostsPath: path, PinSHA256: request.PinSHA256, AcceptNew: request.AcceptNew}
	if s.confirmer != nil {
		policy.Confirm = func(prompt hostkey.Prompt) bool {
			return s.confirmer.Confirm(operationID, prompt)
		}
	}
	return policy
}

// NewOperationID returns a random frontend-safe operation ID.
func NewOperationID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}
