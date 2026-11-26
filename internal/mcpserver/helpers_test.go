package mcpserver_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/guiapi"
	"github.com/sunerpy/router-tools/internal/mcpserver"
)

// fakeBackend records every request and returns configurable reports. When gate is set, use cases block
// until it is closed or their context ends; a canceled use case returns a failed CANCELED report, like the
// application does before any write.
type fakeBackend struct {
	mu       sync.Mutex
	requests []fakeRequest
	gate     chan struct{}
	started  chan string

	doctor    func(guiapi.OperationRequest) (app.DoctorReport, error)
	planReady bool
	apply     func(guiapi.GuideRequest) (app.GuideApplyReport, error)
	remove    func(guiapi.OperationRequest) (app.SoftReport, error)
	emit      func(operationID string)
}

type fakeRequest struct {
	Method    string
	Operation guiapi.OperationRequest
	Guide     guiapi.GuideRequest
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{planReady: true, started: make(chan string, 64)}
}

func (f *fakeBackend) record(method string, operation guiapi.OperationRequest, guide guiapi.GuideRequest) {
	f.mu.Lock()
	f.requests = append(f.requests, fakeRequest{Method: method, Operation: operation, Guide: guide})
	f.mu.Unlock()
	id := operation.OperationID
	if id == "" {
		id = guide.OperationID
	}
	select {
	case f.started <- id:
	default:
	}
}

func (f *fakeBackend) calls() []fakeRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]fakeRequest(nil), f.requests...)
}

func (f *fakeBackend) block(ctx context.Context, operationID string) error {
	if f.emit != nil {
		f.emit(operationID)
	}
	if f.gate == nil {
		return nil
	}
	select {
	case <-f.gate:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func canceledError() *device.Error {
	return &device.Error{Code: device.CodeCanceled, Message: "operation canceled"}
}

func (f *fakeBackend) Doctor(ctx context.Context, request guiapi.OperationRequest) (app.DoctorReport, error) {
	f.record("Doctor", request, guiapi.GuideRequest{})
	if err := f.block(ctx, request.OperationID); err != nil {
		return app.DoctorReport{Operation: "doctor", Status: app.StatusFailed, Host: request.Target.Host, Error: canceledError()}, nil
	}
	if f.doctor != nil {
		return f.doctor(request)
	}
	return app.DoctorReport{Operation: "doctor", Status: app.StatusSucceeded, Host: request.Target.Host}, nil
}

func (f *fakeBackend) GuidePlan(ctx context.Context, request guiapi.GuideRequest) (app.GuidePlanReport, error) {
	f.record("GuidePlan", guiapi.OperationRequest{}, request)
	if err := f.block(ctx, request.OperationID); err != nil {
		return app.GuidePlanReport{Operation: "guide plan", Status: app.StatusFailed, Error: canceledError()}, nil
	}
	report := app.GuidePlanReport{Operation: "guide plan", Status: app.StatusSucceeded, Host: request.Target.Host, Goals: request.Goals}
	if f.planReady {
		report.Ready = true
		report.Changes = true
		report.Confirmation = &app.GuideConfirmation{SchemaVersion: 1, Target: request.Target, Goals: request.Goals}
		report.PlanDigest = "digest-" + request.Target.Host
	}
	return report, nil
}

func (f *fakeBackend) GuideApply(ctx context.Context, request guiapi.GuideRequest) (app.GuideApplyReport, error) {
	f.record("GuideApply", guiapi.OperationRequest{}, request)
	if err := f.block(ctx, request.OperationID); err != nil {
		return app.GuideApplyReport{Operation: "guide apply", Status: app.StatusPartial, Error: canceledError()}, nil
	}
	if f.apply != nil {
		return f.apply(request)
	}
	return app.GuideApplyReport{Operation: "guide apply", Status: app.StatusSucceeded, PlanDigest: request.PlanDigest}, nil
}

func (f *fakeBackend) PersistRemove(ctx context.Context, request guiapi.OperationRequest) (app.SoftReport, error) {
	f.record("PersistRemove", request, guiapi.GuideRequest{})
	if err := f.block(ctx, request.OperationID); err != nil {
		return app.SoftReport{Operation: "persist remove", Status: app.StatusFailed, Error: canceledError()}, nil
	}
	if f.remove != nil {
		return f.remove(request)
	}
	return app.SoftReport{Operation: "persist remove", Status: app.StatusSucceeded, Host: request.Target.Host}, nil
}

func (f *fakeBackend) BackupCreate(ctx context.Context, request guiapi.OperationRequest) (app.BackupCreateReport, error) {
	f.record("BackupCreate", request, guiapi.GuideRequest{})
	if err := f.block(ctx, request.OperationID); err != nil {
		return app.BackupCreateReport{Operation: "backup create", Status: app.StatusFailed, Error: canceledError()}, nil
	}
	return app.BackupCreateReport{Operation: "backup create", Status: app.StatusSucceeded, Host: request.Target.Host, OutputDir: "/managed/backup"}, nil
}

func (f *fakeBackend) DeepStatus(ctx context.Context, request guiapi.OperationRequest) (app.DeepStatusReport, error) {
	f.record("DeepStatus", request, guiapi.GuideRequest{})
	if err := f.block(ctx, request.OperationID); err != nil {
		return app.DeepStatusReport{Operation: "deep status", Status: app.StatusFailed, Error: canceledError()}, nil
	}
	return app.DeepStatusReport{Operation: "deep status", Status: app.StatusSucceeded, Host: request.Target.Host}, nil
}

func (f *fakeBackend) CredentialStatus() (app.CredentialReport, error) {
	f.record("CredentialStatus", guiapi.OperationRequest{}, guiapi.GuideRequest{})
	return app.CredentialReport{Operation: "credentials list", Status: app.StatusSucceeded}, nil
}

func (f *fakeBackend) Catalog() []device.CatalogEntry {
	f.record("Catalog", guiapi.OperationRequest{}, guiapi.GuideRequest{})
	return []device.CatalogEntry{{ModelID: "xiaomi-rd08", Vendor: "Xiaomi", Model: "RD08"}}
}

// memoryAudit collects audit records.
type memoryAudit struct {
	mu      sync.Mutex
	records []mcpserver.AuditRecord
}

func (m *memoryAudit) Record(record mcpserver.AuditRecord) {
	m.mu.Lock()
	m.records = append(m.records, record)
	m.mu.Unlock()
}

func (m *memoryAudit) all() []mcpserver.AuditRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]mcpserver.AuditRecord(nil), m.records...)
}

type harness struct {
	t       *testing.T
	backend *fakeBackend
	server  *mcpserver.Server
	ops     *mcpserver.Operations
	audit   *memoryAudit
	session *mcp.ClientSession
	changed chan struct{}
	notes   chan *mcp.ProgressNotificationParams
}

func newHarness(t *testing.T, backend *fakeBackend, allowWrites bool) *harness {
	t.Helper()
	ops := mcpserver.NewOperations()
	audit := &memoryAudit{}
	server := mcpserver.New(backend, ops, mcpserver.Options{Version: "v-test", AllowWrites: allowWrites, Audit: audit})
	h := &harness{t: t, backend: backend, server: server, ops: ops, audit: audit, changed: make(chan struct{}, 16), notes: make(chan *mcp.ProgressNotificationParams, 64)}
	h.session = h.connect()
	t.Cleanup(func() {
		_ = h.session.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	})
	return h
}

// connect opens one more client session to the same server.
func (h *harness) connect() *mcp.ClientSession {
	h.t.Helper()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := h.server.MCP().Connect(ctx, serverTransport, nil); err != nil {
		h.t.Fatalf("server connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0"}, &mcp.ClientOptions{
		ToolListChangedHandler: func(context.Context, *mcp.ToolListChangedRequest) {
			select {
			case h.changed <- struct{}{}:
			default:
			}
		},
		ProgressNotificationHandler: func(_ context.Context, request *mcp.ProgressNotificationClientRequest) {
			select {
			case h.notes <- request.Params:
			default:
			}
		},
	})
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		h.t.Fatalf("client connect: %v", err)
	}
	return session
}

func (h *harness) toolNames() []string {
	h.t.Helper()
	result, err := h.session.ListTools(context.Background(), nil)
	if err != nil {
		h.t.Fatalf("list tools: %v", err)
	}
	names := make([]string, 0, len(result.Tools))
	for _, tool := range result.Tools {
		names = append(names, tool.Name)
	}
	return names
}

// toolResult is the JSON envelope every tool returns.
type toolResult struct {
	OperationID   string          `json:"operation_id"`
	Tool          string          `json:"tool"`
	Done          bool            `json:"done"`
	Status        string          `json:"status"`
	Report        json.RawMessage `json:"report"`
	Error         *device.Error   `json:"error"`
	Progress      []app.Event     `json:"progress"`
	PlanID        string          `json:"plan_id"`
	WritesEnabled *bool           `json:"writes_enabled"`
	IsError       bool            `json:"-"`
	Text          string          `json:"-"`
}

func (h *harness) call(name string, arguments map[string]any) toolResult {
	h.t.Helper()
	return h.callWith(context.Background(), h.session, name, arguments, nil)
}

func (h *harness) callWith(ctx context.Context, session *mcp.ClientSession, name string, arguments map[string]any, progressToken any) toolResult {
	h.t.Helper()
	params := &mcp.CallToolParams{Name: name, Arguments: arguments}
	if progressToken != nil {
		params.SetProgressToken(progressToken)
	}
	result, err := session.CallTool(ctx, params)
	if err != nil {
		h.t.Fatalf("call %s: %v", name, err)
	}
	return decodeResult(h.t, result)
}

func decodeResult(t *testing.T, result *mcp.CallToolResult) toolResult {
	t.Helper()
	if len(result.Content) != 1 {
		t.Fatalf("expected one content block, got %d", len(result.Content))
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected text content, got %T", result.Content[0])
	}
	var decoded toolResult
	if err := json.Unmarshal([]byte(text.Text), &decoded); err != nil {
		t.Fatalf("decode result %q: %v", text.Text, err)
	}
	decoded.IsError = result.IsError
	decoded.Text = text.Text
	return decoded
}

func target(host string) map[string]any {
	return map[string]any{"host": host}
}

func waitFor(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func hasPrefixAll(t *testing.T, id string) {
	t.Helper()
	if !strings.HasPrefix(id, "agent-") || len(id) != len("agent-")+32 {
		t.Fatalf("operation ID %q is not agent- plus 32 hex", id)
	}
}
