package mcpserver_test

import (
	"context"
	"encoding/json"
	"io"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/guiapi"
	"github.com/sunerpy/router-tools/internal/mcpserver"
)

var readTools = []string{
	"backup_create", "deep_status", "guide_plan", "list_profiles", "operation_cancel",
	"operation_list", "operation_wait", "preflight", "saved_passwords",
}

func TestToolListOffersWritesOnlyWhenEnabled(t *testing.T) {
	t.Parallel()
	h := newHarness(t, newFakeBackend(), false)
	names := h.toolNames()
	sort.Strings(names)
	if strings.Join(names, ",") != strings.Join(readTools, ",") {
		t.Fatalf("tools without writes = %v, want %v", names, readTools)
	}

	h.server.SetAllowWrites(true)
	select {
	case <-h.changed:
	case <-time.After(5 * time.Second):
		t.Fatal("no tools/list_changed after enabling writes")
	}
	names = h.toolNames()
	if !contains(names, "guide_apply") || !contains(names, "soft_remove") || len(names) != len(readTools)+2 {
		t.Fatalf("tools with writes = %v", names)
	}
	if !h.server.WritesEnabled() {
		t.Fatal("WritesEnabled() = false after SetAllowWrites(true)")
	}

	h.server.SetAllowWrites(false)
	select {
	case <-h.changed:
	case <-time.After(5 * time.Second):
		t.Fatal("no tools/list_changed after disabling writes")
	}
	if names = h.toolNames(); contains(names, "guide_apply") || contains(names, "soft_remove") {
		t.Fatalf("write tools still listed after disabling: %v", names)
	}
}

// Every tool has exactly one annotation class: R has no side effect, L leaves the router untouched but may
// change the local vault or files, W writes the router or stops a write.
func TestToolAnnotationClasses(t *testing.T) {
	t.Parallel()
	h := newHarness(t, newFakeBackend(), true)
	classes := map[string]string{
		"list_profiles": "R", "saved_passwords": "R", "operation_wait": "R", "operation_list": "R",
		"preflight": "L", "guide_plan": "L", "deep_status": "L", "backup_create": "L",
		"operation_cancel": "W", "guide_apply": "W", "soft_remove": "W",
	}
	result, err := h.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Tools) != len(classes) {
		t.Fatalf("listed %d tools, classified %d", len(result.Tools), len(classes))
	}
	for _, tool := range result.Tools {
		class, ok := classes[tool.Name]
		if !ok {
			t.Fatalf("unclassified tool %q", tool.Name)
		}
		a := tool.Annotations
		if a == nil || a.DestructiveHint == nil || a.OpenWorldHint == nil {
			t.Fatalf("%s: annotations must set every hint explicitly: %+v", tool.Name, a)
		}
		switch class {
		case "R":
			if !a.ReadOnlyHint || *a.OpenWorldHint {
				t.Fatalf("%s: want read-only closed-world, got %+v", tool.Name, a)
			}
		case "L":
			if a.ReadOnlyHint || *a.DestructiveHint || a.IdempotentHint || !*a.OpenWorldHint {
				t.Fatalf("%s: want local-change class, got %+v", tool.Name, a)
			}
		case "W":
			if a.ReadOnlyHint || !*a.DestructiveHint || a.IdempotentHint || !*a.OpenWorldHint {
				t.Fatalf("%s: want write class, got %+v", tool.Name, a)
			}
		}
		if tool.Description == "" {
			t.Fatalf("%s has no description", tool.Name)
		}
		if strings.Contains(tool.Name, "deep_persist") || strings.Contains(tool.Name, "recovery") {
			t.Fatalf("human-only capability exposed: %s", tool.Name)
		}
	}
}

// The MCP layer forces SSH, no Telnet, default paths and the server's own operation IDs whatever the agent
// sends, and passes the agent's credential and host-key choices through.
func TestToolsMapToBackendWithForcedFields(t *testing.T) {
	t.Parallel()
	backend := newFakeBackend()
	h := newHarness(t, backend, true)

	preflight := h.call("preflight", map[string]any{
		"target": map[string]any{"host": "192.168.31.1", "web_port": 8080}, "check_admin_login": true, "check_root_login": true,
		"admin_password": "admin-pw-1", "root_password": "root-pw-1", "remember_password": true,
		"host_key_sha256": "SHA256:pin", "accept_new_host_key": true,
	})
	if !preflight.Done || preflight.Status != app.StatusSucceeded || preflight.IsError {
		t.Fatalf("preflight result = %+v", preflight)
	}
	hasPrefixAll(t, preflight.OperationID)
	backup := h.call("backup_create", map[string]any{"target": target("192.168.31.1"), "root_password": "root-pw-1"})
	deep := h.call("deep_status", map[string]any{"target": target("192.168.31.1"), "use_saved_password": true})
	remove := h.call("soft_remove", map[string]any{"target": target("192.168.31.1"), "confirm": true})
	for _, result := range []toolResult{backup, deep, remove} {
		if !result.Done || result.IsError {
			t.Fatalf("%s result = %+v", result.Tool, result)
		}
	}

	calls := backend.calls()
	byMethod := map[string]guiapi.OperationRequest{}
	for _, call := range calls {
		byMethod[call.Method] = call.Operation
	}
	doctor := byMethod["Doctor"]
	if doctor.Target != (device.Target{Host: "192.168.31.1", WebPort: 8080, SSHPort: 22, TelnetPort: 23}) || doctor.ProfileID != "auto" {
		t.Fatalf("doctor target/profile = %+v %q", doctor.Target, doctor.ProfileID)
	}
	if !doctor.WithAdminAuth || !doctor.WithRootVerify || doctor.AdminPassword != "admin-pw-1" || doctor.RootPassword != "root-pw-1" {
		t.Fatalf("doctor checks/passwords not passed: %+v", doctor)
	}
	if doctor.UseSavedPassword || !doctor.RememberPassword || doctor.HostKey.PinSHA256 != "SHA256:pin" || !doctor.HostKey.AcceptNew {
		t.Fatalf("doctor credential/host-key choices not passed: %+v", doctor)
	}
	if !byMethod["DeepStatus"].UseSavedPassword {
		t.Fatal("use_saved_password not passed")
	}
	for method, request := range byMethod {
		if method == "Catalog" || method == "CredentialStatus" || method == "" {
			continue
		}
		if request.Transport != "ssh" || request.AllowTelnet || request.HostKey.KnownHostsPath != "" ||
			request.OutputDir != "" || request.RunDir != "" || request.BackupDir != "" || request.AckDeep != "" {
			t.Fatalf("%s: forced fields not applied: %+v", method, request)
		}
		hasPrefixAll(t, request.OperationID)
		if request.ToolVersion != "v-test" {
			t.Fatalf("%s: tool version = %q", method, request.ToolVersion)
		}
	}
	if !byMethod["PersistRemove"].Confirmed {
		t.Fatal("soft_remove must pass Confirmed=true")
	}
}

func TestSoftRemoveRequiresConfirm(t *testing.T) {
	t.Parallel()
	backend := newFakeBackend()
	h := newHarness(t, backend, true)
	result := h.call("soft_remove", map[string]any{"target": target("192.168.31.1")})
	if !result.IsError || result.Error == nil || result.Error.Code != device.CodeConfirmationRequired {
		t.Fatalf("soft_remove without confirm = %+v", result)
	}
	for _, call := range backend.calls() {
		if call.Method == "PersistRemove" {
			t.Fatal("soft_remove without confirm reached the backend")
		}
	}
}

func TestInvalidArgumentsAreRejectedBeforeTheBackend(t *testing.T) {
	t.Parallel()
	backend := newFakeBackend()
	h := newHarness(t, backend, true)
	cases := map[string]map[string]any{
		"unknown field":   {"target": target("192.168.31.1"), "telnet": true},
		"missing host":    {"target": map[string]any{}},
		"bad host":        {"target": target("http://192.168.31.1")},
		"bad port":        {"target": map[string]any{"host": "192.168.31.1", "ssh_port": 70000}},
		"wait too long":   {"target": target("192.168.31.1"), "wait_seconds": 51},
		"negative wait":   {"target": target("192.168.31.1"), "wait_seconds": -1},
		"wrong type":      {"target": target("192.168.31.1"), "root_password": 12345},
		"run dir smuggle": {"target": target("192.168.31.1"), "run_dir": "/tmp/x"},
	}
	for name, arguments := range cases {
		result := h.call("deep_status", arguments)
		if !result.IsError || result.Error == nil || result.Error.Code != device.CodeInvalidArgument {
			t.Fatalf("%s: result = %+v", name, result)
		}
	}
	for _, call := range backend.calls() {
		if call.Method == "DeepStatus" {
			t.Fatal("an invalid request reached the backend")
		}
	}
}

func TestGuidePlanStoresAOneShotPlan(t *testing.T) {
	t.Parallel()
	backend := newFakeBackend()
	h := newHarness(t, backend, true)

	plan := h.call("guide_plan", map[string]any{"target": target("192.168.31.1")})
	if !plan.Done || plan.PlanID == "" || plan.WritesEnabled == nil || !*plan.WritesEnabled {
		t.Fatalf("plan result = %+v", plan)
	}
	var report app.GuidePlanReport
	if err := json.Unmarshal(plan.Report, &report); err != nil {
		t.Fatal(err)
	}
	if strings.Join(report.Goals, ",") != "ssh,backup,soft" {
		t.Fatalf("default goals = %v", report.Goals)
	}

	wrong := h.call("guide_apply", map[string]any{"plan_id": plan.PlanID, "plan_digest": "not-it"})
	if !wrong.IsError || wrong.Error == nil || wrong.Error.Code != device.CodeConfirmationRequired {
		t.Fatalf("apply with a wrong digest = %+v", wrong)
	}

	// A second session (an HTTP reconnect, another client) can apply the plan once.
	other := h.connect()
	defer other.Close()
	applied := h.callWith(context.Background(), other, "guide_apply", map[string]any{
		"plan_id": plan.PlanID, "plan_digest": report.PlanDigest, "admin_password": "admin-pw-1", "root_password": "new-root-pw-1",
	}, nil)
	if !applied.Done || applied.Status != app.StatusSucceeded || applied.IsError {
		t.Fatalf("apply = %+v", applied)
	}
	var applyRequest guiapi.GuideRequest
	for _, call := range backend.calls() {
		if call.Method == "GuideApply" {
			applyRequest = call.Guide
		}
	}
	if applyRequest.Confirmation == nil || applyRequest.PlanDigest != report.PlanDigest || !applyRequest.Confirmed ||
		applyRequest.AdminPassword != "admin-pw-1" || applyRequest.RootPassword != "new-root-pw-1" || applyRequest.ToolVersion != "v-test" {
		t.Fatalf("apply request = %+v", applyRequest)
	}

	again := h.call("guide_apply", map[string]any{"plan_id": plan.PlanID, "plan_digest": report.PlanDigest})
	if !again.IsError || again.Error == nil || again.Error.Code != device.CodeConfirmationRequired {
		t.Fatalf("a replayed apply must be refused, got %+v", again)
	}
	applies := 0
	for _, call := range backend.calls() {
		if call.Method == "GuideApply" {
			applies++
		}
	}
	if applies != 1 {
		t.Fatalf("backend GuideApply calls = %d, want 1", applies)
	}
}

func TestGuidePlanWithoutWritesSaysSo(t *testing.T) {
	t.Parallel()
	h := newHarness(t, newFakeBackend(), false)
	plan := h.call("guide_plan", map[string]any{"target": target("192.168.31.1"), "goals": []string{"ssh", "backup", "soft", "reboot"}})
	if plan.WritesEnabled == nil || *plan.WritesEnabled {
		t.Fatalf("writes_enabled = %v, want false", plan.WritesEnabled)
	}
	if plan.PlanID == "" {
		t.Fatal("a ready plan still gets a plan_id while writes are off")
	}
}

func TestGuidePlanNotReadyHasNoPlanID(t *testing.T) {
	t.Parallel()
	backend := newFakeBackend()
	backend.planReady = false
	h := newHarness(t, backend, true)
	plan := h.call("guide_plan", map[string]any{"target": target("192.168.31.1")})
	if plan.PlanID != "" {
		t.Fatalf("plan_id %q for a plan that is not ready", plan.PlanID)
	}
}

func TestLongOperationContinuesThroughOperationWait(t *testing.T) {
	t.Parallel()
	backend := newFakeBackend()
	backend.gate = make(chan struct{})
	h := newHarness(t, backend, true)

	first := h.call("backup_create", map[string]any{"target": target("192.168.31.1"), "wait_seconds": 0})
	if first.Done || first.Status != "running" || first.IsError || first.OperationID == "" {
		t.Fatalf("first return = %+v", first)
	}
	pending := h.call("operation_wait", map[string]any{"operation_id": first.OperationID, "wait_seconds": 0})
	if pending.Done || pending.OperationID != first.OperationID {
		t.Fatalf("operation_wait before completion = %+v", pending)
	}
	close(backend.gate)
	final := h.call("operation_wait", map[string]any{"operation_id": first.OperationID, "wait_seconds": 5})
	if !final.Done || final.Status != app.StatusSucceeded || final.Tool != "backup_create" {
		t.Fatalf("final = %+v", final)
	}
	var report app.BackupCreateReport
	if err := json.Unmarshal(final.Report, &report); err != nil || report.OutputDir != "/managed/backup" {
		t.Fatalf("report = %+v (%v)", report, err)
	}
	list := h.call("operation_list", nil)
	if !strings.Contains(string(list.Report), first.OperationID) {
		t.Fatalf("operation_list does not show %s: %s", first.OperationID, list.Report)
	}
}

// A host timeout cancels the request; the operation must keep running because a write may be under way.
func TestRequestCancellationOnlyEndsTheWait(t *testing.T) {
	t.Parallel()
	backend := newFakeBackend()
	backend.gate = make(chan struct{})
	h := newHarness(t, backend, true)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := h.session.CallTool(ctx, &mcp.CallToolParams{Name: "deep_status", Arguments: map[string]any{"target": target("192.168.31.1"), "wait_seconds": 50}})
		done <- err
	}()
	var operationID string
	select {
	case operationID = <-backend.started:
	case <-time.After(5 * time.Second):
		t.Fatal("the operation did not start")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the canceled call did not return")
	}
	status := h.call("operation_wait", map[string]any{"operation_id": operationID, "wait_seconds": 0})
	if status.Done {
		t.Fatalf("request cancellation stopped the operation: %+v", status)
	}
	close(backend.gate)
	final := h.call("operation_wait", map[string]any{"operation_id": operationID, "wait_seconds": 5})
	if !final.Done || final.Status != app.StatusSucceeded {
		t.Fatalf("final = %+v", final)
	}
}

func TestOperationCancelStopsOnlyOwnOperations(t *testing.T) {
	t.Parallel()
	backend := newFakeBackend()
	backend.gate = make(chan struct{})
	h := newHarness(t, backend, true)

	started := h.call("preflight", map[string]any{"target": target("192.168.31.1"), "wait_seconds": 0})
	canceled := h.call("operation_cancel", map[string]any{"operation_id": started.OperationID})
	if canceled.IsError || !strings.Contains(string(canceled.Report), `"cancel_requested": true`) {
		t.Fatalf("cancel = %+v", canceled)
	}
	final := h.call("operation_wait", map[string]any{"operation_id": started.OperationID, "wait_seconds": 5})
	if !final.Done || final.Status != app.StatusFailed || !final.IsError {
		t.Fatalf("canceled operation = %+v", final)
	}

	unknown := h.call("operation_cancel", map[string]any{"operation_id": "0123456789abcdef0123456789abcdef"})
	if !unknown.IsError || unknown.Error == nil || unknown.Error.Code != device.CodeInvalidArgument {
		t.Fatalf("cancel of a foreign ID = %+v", unknown)
	}
	waitUnknown := h.call("operation_wait", map[string]any{"operation_id": "agent-ffffffffffffffffffffffffffffffff"})
	if !waitUnknown.IsError || waitUnknown.Error == nil || waitUnknown.Error.Code != device.CodeInvalidArgument {
		t.Fatalf("wait on an unknown ID = %+v", waitUnknown)
	}
	if h.ops.Has("0123456789abcdef0123456789abcdef") {
		t.Fatal("Has() reports an ID this server never started")
	}
}

func TestAtMostFourOperationsRun(t *testing.T) {
	t.Parallel()
	backend := newFakeBackend()
	backend.gate = make(chan struct{})
	h := newHarness(t, backend, true)
	for i := 0; i < 4; i++ {
		if result := h.call("deep_status", map[string]any{"target": target("192.168.31.1"), "wait_seconds": 0}); result.Done {
			t.Fatalf("operation %d finished early: %+v", i, result)
		}
	}
	fifth := h.call("deep_status", map[string]any{"target": target("192.168.31.1"), "wait_seconds": 0})
	if !fifth.IsError || fifth.Error == nil || fifth.Error.Code != device.CodeOperationLocked {
		t.Fatalf("fifth operation = %+v", fifth)
	}
	close(backend.gate)
}

func TestShutdownCancelsRunningOperations(t *testing.T) {
	t.Parallel()
	backend := newFakeBackend()
	backend.gate = make(chan struct{})
	ops := mcpserver.NewOperations()
	server := mcpserver.New(backend, ops, mcpserver.Options{Version: "v-test"})
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	if _, err := server.MCP().Connect(context.Background(), serverTransport, nil); err != nil {
		t.Fatal(err)
	}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "c", Version: "v"}, nil).Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "preflight", Arguments: map[string]any{"target": target("192.168.31.1"), "wait_seconds": 0}})
	if err != nil {
		t.Fatal(err)
	}
	started := decodeResult(t, result)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if still := server.Shutdown(ctx); len(still) != 0 {
		t.Fatalf("operations still running after shutdown: %v", still)
	}
	summaries := ops.List()
	if len(summaries) != 1 || summaries[0].OperationID != started.OperationID || !summaries[0].Done || summaries[0].Status != app.StatusFailed {
		t.Fatalf("after shutdown: %+v", summaries)
	}
	after, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_profiles"})
	if err != nil {
		t.Fatal(err)
	}
	if decoded := decodeResult(t, after); !decoded.IsError {
		t.Fatalf("a call after shutdown must be refused: %+v", decoded)
	}
}

func TestShutdownReportsOperationsThatDoNotStop(t *testing.T) {
	t.Parallel()
	backend := newFakeBackend()
	stuck := make(chan struct{})
	defer close(stuck)
	backend.doctor = func(guiapi.OperationRequest) (app.DoctorReport, error) {
		<-stuck
		return app.DoctorReport{Status: app.StatusSucceeded}, nil
	}
	h := newHarness(t, backend, false)
	started := h.call("preflight", map[string]any{"target": target("192.168.31.1"), "wait_seconds": 0})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	still := h.server.Shutdown(ctx)
	if len(still) != 1 || still[0] != started.OperationID {
		t.Fatalf("Shutdown returned %v, want [%s]", still, started.OperationID)
	}
}

func TestErrorResults(t *testing.T) {
	t.Parallel()
	backend := newFakeBackend()
	h := newHarness(t, backend, true)
	for _, status := range []string{app.StatusFailed, app.StatusPartial, app.StatusUnknown} {
		backend.remove = func(guiapi.OperationRequest) (app.SoftReport, error) {
			return app.SoftReport{Status: status, Error: &device.Error{Code: device.CodeRemoteCommandFailed, Message: "x"}}, nil
		}
		if result := h.call("soft_remove", map[string]any{"target": target("192.168.31.1"), "confirm": true}); !result.IsError || result.Status != status {
			t.Fatalf("%s report: %+v", status, result)
		}
	}
	// A change that succeeded but whose outcome record failed carries both the report and the error.
	backend.remove = func(guiapi.OperationRequest) (app.SoftReport, error) {
		return app.SoftReport{Status: app.StatusSucceeded}, &device.Error{Code: device.CodeStateStoreFailed, Message: "record"}
	}
	result := h.call("soft_remove", map[string]any{"target": target("192.168.31.1"), "confirm": true})
	if !result.IsError || result.Status != app.StatusSucceeded || result.Error == nil || result.Error.Code != device.CodeStateStoreFailed {
		t.Fatalf("success with error = %+v", result)
	}
	// A binding error without a report is a failure.
	backend.remove = func(guiapi.OperationRequest) (app.SoftReport, error) {
		return app.SoftReport{}, &device.Error{Code: device.CodeInvalidPassword, Message: "password must not be empty"}
	}
	result = h.call("soft_remove", map[string]any{"target": target("192.168.31.1"), "confirm": true})
	if !result.IsError || result.Status != app.StatusFailed || result.Error == nil || result.Error.Code != device.CodeInvalidPassword {
		t.Fatalf("binding error = %+v", result)
	}
}

func TestAuditRecordsEveryCallAndEveryFinish(t *testing.T) {
	t.Parallel()
	backend := newFakeBackend()
	backend.gate = make(chan struct{})
	h := newHarness(t, backend, true)
	h.call("list_profiles", nil)
	h.call("saved_passwords", nil)
	started := h.call("guide_plan", map[string]any{"target": target("192.168.31.1"), "wait_seconds": 0})
	h.call("operation_list", nil)
	close(backend.gate)
	h.call("operation_wait", map[string]any{"operation_id": started.OperationID, "wait_seconds": 5})

	waitFor(t, "finish record", func() bool {
		for _, record := range h.audit.all() {
			if record.Kind == "finish" && record.OperationID == started.OperationID {
				return true
			}
		}
		return false
	})
	var calls []string
	var running, finished bool
	for _, record := range h.audit.all() {
		switch record.Kind {
		case "call":
			calls = append(calls, record.Tool)
			if record.Tool == "guide_plan" && !record.Done && record.Status == "running" && record.OperationID == started.OperationID && record.Host == "192.168.31.1" {
				running = true
			}
		case "finish":
			if record.Tool == "guide_plan" && record.Done && record.Status == app.StatusSucceeded {
				finished = true
			}
		default:
			t.Fatalf("unknown audit kind %q", record.Kind)
		}
	}
	if strings.Join(calls, ",") != "list_profiles,saved_passwords,guide_plan,operation_list,operation_wait" {
		t.Fatalf("call records = %v", calls)
	}
	if !running || !finished {
		t.Fatalf("missing running call (%v) or finish (%v) records: %+v", running, finished, h.audit.all())
	}
}

func TestJSONAuditWritesOneLinePerRecord(t *testing.T) {
	t.Parallel()
	var out strings.Builder
	audit := mcpserver.NewJSONAudit(&out)
	audit.Record(mcpserver.AuditRecord{Kind: "call", Tool: "list_profiles", Done: true, Status: "succeeded"})
	audit.Record(mcpserver.AuditRecord{Kind: "finish", Tool: "preflight", OperationID: "agent-1", Done: true})
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %q", out.String())
	}
	for _, line := range lines {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("not JSON: %q", line)
		}
		if record["audit"] != "mcp" {
			t.Fatalf("audit line lacks its marker: %q", line)
		}
	}
}

func TestProgressIsForwardedWhileWaiting(t *testing.T) {
	t.Parallel()
	backend := newFakeBackend()
	h := newHarness(t, backend, true)
	backend.emit = func(operationID string) {
		h.ops.Emit(operationID, app.Event{Operation: "doctor", Stage: "probe", Status: "running"})
	}
	result := h.callWith(context.Background(), h.session, "preflight", map[string]any{"target": target("192.168.31.1")}, "token-1")
	if !result.Done {
		t.Fatalf("result = %+v", result)
	}
	select {
	case note := <-h.notes:
		if note.Message != "doctor probe running" || note.ProgressToken != "token-1" {
			t.Fatalf("progress = %+v", note)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no progress notification")
	}
	// Events of an operation this server did not start are ignored.
	h.ops.Emit("someone-else", app.Event{Operation: "doctor", Stage: "probe", Status: "running"})
}

func TestReadOnlyCatalogAndCredentials(t *testing.T) {
	t.Parallel()
	h := newHarness(t, newFakeBackend(), false)
	profiles := h.call("list_profiles", nil)
	if !profiles.Done || profiles.IsError || !strings.Contains(string(profiles.Report), "xiaomi-rd08") || profiles.OperationID != "" {
		t.Fatalf("list_profiles = %+v", profiles)
	}
	saved := h.call("saved_passwords", nil)
	if !saved.Done || saved.IsError {
		t.Fatalf("saved_passwords = %+v", saved)
	}
}

// An agent-supplied fingerprint pin replaces the known_hosts check, so it must never be combined with a
// saved password: a pinned impostor would receive the vault's root password.
func TestSavedPasswordsAreNeverSentWithAnAgentPin(t *testing.T) {
	t.Parallel()
	backend := newFakeBackend()
	h := newHarness(t, backend, true)
	pinned := map[string]any{"use_saved_password": true, "host_key_sha256": "SHA256:impostor"}
	for _, tool := range []string{"preflight", "guide_plan", "deep_status", "backup_create", "soft_remove"} {
		arguments := map[string]any{"target": target("192.168.31.1"), "confirm": true}
		for key, value := range pinned {
			arguments[key] = value
		}
		result := h.call(tool, arguments)
		if !result.IsError || result.Error == nil || result.Error.Code != device.CodeInvalidArgument {
			t.Fatalf("%s with a saved password and an agent pin = %+v", tool, result)
		}
	}
	plan := h.call("guide_plan", map[string]any{"target": target("192.168.31.1")})
	var report app.GuidePlanReport
	if err := json.Unmarshal(plan.Report, &report); err != nil {
		t.Fatal(err)
	}
	apply := h.call("guide_apply", map[string]any{"plan_id": plan.PlanID, "plan_digest": report.PlanDigest, "use_saved_password": true, "host_key_sha256": "SHA256:impostor"})
	if !apply.IsError || apply.Error == nil || apply.Error.Code != device.CodeInvalidArgument {
		t.Fatalf("guide_apply with a saved password and an agent pin = %+v", apply)
	}
	for _, call := range backend.calls() {
		if call.Method != "GuidePlan" && call.Method != "" {
			t.Fatalf("%s reached the backend", call.Method)
		}
	}
	// A pin with a given password, and a saved password without a pin, are both fine.
	if result := h.call("preflight", map[string]any{"target": target("192.168.31.1"), "root_password": "pw-1", "host_key_sha256": "SHA256:x"}); result.IsError {
		t.Fatalf("pin with a given password refused: %+v", result)
	}
	if result := h.call("preflight", map[string]any{"target": target("192.168.31.1"), "use_saved_password": true}); result.IsError {
		t.Fatalf("saved password without a pin refused: %+v", result)
	}
}

// Argument names match exactly: no case-insensitive or Unicode-folded matches and no duplicates, as the
// advertised schema says.
func TestArgumentNamesMatchExactly(t *testing.T) {
	t.Parallel()
	backend := newFakeBackend()
	h := newHarness(t, backend, true)
	for name, raw := range map[string]string{
		"upper case":       `{"target":{"host":"192.168.31.1"},"CONFIRM":true}`,
		"mixed case":       `{"target":{"host":"192.168.31.1"},"confirm":true,"Use_Saved_Password":true}`,
		"unicode fold":     `{"target":{"host":"192.168.31.1"},"confirm":true,"accept_new_hoſt_key":true}`,
		"duplicate":        `{"target":{"host":"192.168.31.1"},"confirm":false,"confirm":true}`,
		"nested variant":   `{"target":{"Host":"192.168.31.1"},"confirm":true}`,
		"nested duplicate": `{"target":{"host":"192.168.31.1","host":"10.0.0.1"},"confirm":true}`,
	} {
		result, err := h.session.CallTool(context.Background(), &mcp.CallToolParams{Name: "soft_remove", Arguments: json.RawMessage(raw)})
		if err != nil {
			continue // refused before the tool, which is as good
		}
		decoded := decodeResult(t, result)
		if !decoded.IsError || decoded.Error == nil || decoded.Error.Code != device.CodeInvalidArgument {
			t.Fatalf("%s: %+v", name, decoded)
		}
	}
	for _, call := range backend.calls() {
		if call.Method == "PersistRemove" {
			t.Fatal("an inexact argument name reached the backend")
		}
	}
}

func TestUnknownArgumentNamesAreNotEchoed(t *testing.T) {
	t.Parallel()
	h := newHarness(t, newFakeBackend(), true)
	result := h.call("deep_status", map[string]any{"target": target("192.168.31.1"), "hunter_two_pw": true})
	if !result.IsError || strings.Contains(result.Text, "hunter_two_pw") {
		t.Fatalf("unknown argument echoed: %s", result.Text)
	}
}

// Redaction works on string values, so a weak password such as "1" or "true" cannot corrupt the JSON.
func TestWeakPasswordsDoNotCorruptResults(t *testing.T) {
	t.Parallel()
	backend := newFakeBackend()
	h := newHarness(t, backend, true)
	for _, password := range []string{"1", "true", "status", "a", `"`, `\`} {
		result := h.call("backup_create", map[string]any{"target": target("192.168.31.1"), "root_password": password})
		if !result.Done || result.Status != app.StatusSucceeded || result.Tool != "backup_create" {
			t.Fatalf("password %q: %s", password, result.Text)
		}
		var report map[string]any
		if err := json.Unmarshal(result.Report, &report); err != nil || report["schema_version"] != float64(0) || report["status"] == nil {
			t.Fatalf("password %q: report %s (%v)", password, result.Report, err)
		}
	}
}

// Stopping the server ends every waiting call at once, so stdio and HTTP shut down within the host's grace
// period even while an agent waits for a long operation.
func TestStopEndsWaitingCallsImmediately(t *testing.T) {
	t.Parallel()
	backend := newFakeBackend()
	backend.gate = make(chan struct{})
	ops := mcpserver.NewOperations()
	server := mcpserver.New(backend, ops, mcpserver.Options{Version: "v-test"})
	serverIn, clientOut := io.Pipe()
	clientIn, serverOut := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- server.ServeIO(ctx, serverIn, serverOut) }()
	client := mcp.NewClient(&mcp.Implementation{Name: "c", Version: "v"}, nil)
	session, err := client.Connect(context.Background(), &mcp.IOTransport{Reader: clientIn, Writer: clientOut}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		// The pipes stand in for stdio: closing them ends the client side too.
		_ = serverOut.Close()
		_ = clientOut.Close()
		_ = session.Close()
	}()
	go func() {
		_, _ = session.CallTool(context.Background(), &mcp.CallToolParams{Name: "deep_status", Arguments: map[string]any{"target": target("192.168.31.1"), "wait_seconds": 50}})
	}()
	select {
	case <-backend.started:
	case <-time.After(5 * time.Second):
		t.Fatal("the operation did not start")
	}
	time.Sleep(50 * time.Millisecond) // let the call reach its wait
	begin := time.Now()
	cancel()
	select {
	case <-served:
	case <-time.After(5 * time.Second):
		t.Fatal("ServeIO kept running while a call waited")
	}
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if still := server.Shutdown(stopCtx); len(still) != 0 {
		t.Fatalf("still running: %v", still)
	}
	if elapsed := time.Since(begin); elapsed > 5*time.Second {
		t.Fatalf("stopping took %s", elapsed)
	}
}

// Progress emitted right before an operation ends still reaches the waiting call.
func TestProgressRightBeforeTheEndIsDelivered(t *testing.T) {
	t.Parallel()
	for round := 0; round < 20; round++ {
		backend := newFakeBackend()
		backend.gate = make(chan struct{})
		h := newHarness(t, backend, true)
		backend.doctor = func(request guiapi.OperationRequest) (app.DoctorReport, error) {
			for i := 0; i < 10; i++ {
				h.ops.Emit(request.OperationID, app.Event{Operation: "doctor", Stage: "s" + strconv.Itoa(i), Status: "running"})
			}
			return app.DoctorReport{Status: app.StatusSucceeded}, nil
		}
		done := make(chan toolResult, 1)
		go func() {
			done <- h.callWith(context.Background(), h.session, "preflight", map[string]any{"target": target("192.168.31.1")}, "tok")
		}()
		<-backend.started
		time.Sleep(20 * time.Millisecond) // the call subscribes while the backend waits
		close(backend.gate)
		if result := <-done; !result.Done {
			t.Fatalf("result = %+v", result)
		}
		waitFor(t, "ten progress notifications", func() bool { return len(h.notes) == 10 })
	}
}

// A call to a tool that is not offered (here a write tool while writes are off) never reaches a handler; it
// is audited anyway, with the tool name only when it looks like one.
func TestRejectedToolCallsAreAudited(t *testing.T) {
	t.Parallel()
	h := newHarness(t, newFakeBackend(), false)
	if _, err := h.session.CallTool(context.Background(), &mcp.CallToolParams{Name: "guide_apply", Arguments: map[string]any{}}); err == nil {
		t.Fatal("guide_apply answered while writes are off")
	}
	if _, err := h.session.CallTool(context.Background(), &mcp.CallToolParams{Name: rootSecret}); err == nil {
		t.Fatal("an unknown tool answered")
	}
	waitFor(t, "audit of the rejected calls", func() bool {
		found := 0
		for _, record := range h.audit.all() {
			if record.Kind == "call" && record.Status == app.StatusFailed && record.ErrorCode == "UNKNOWN_TOOL" {
				found++
				if strings.Contains(record.Tool, rootSecret) {
					t.Fatalf("audit leaks the tool name: %+v", record)
				}
			}
		}
		return found == 2
	})
}
