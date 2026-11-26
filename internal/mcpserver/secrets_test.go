package mcpserver_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/guiapi"
)

const (
	adminSecret = "Adm1n-S3cret-Value"
	rootSecret  = "R00t-S3cret-Value"
)

// Passwords given in tool arguments never come back in any result, progress notification or audit record,
// even when a use case's message would contain them or the arguments are malformed.
func TestPasswordsNeverLeaveTheServer(t *testing.T) {
	t.Parallel()
	backend := newFakeBackend()
	h := newHarness(t, backend, true)
	backend.emit = func(operationID string) {
		// A careless stage text containing a typed password must be redacted in progress notifications.
		h.ops.Emit(operationID, app.Event{Operation: "doctor", Stage: "probe " + rootSecret, Status: "running"})
	}
	backend.doctor = func(request guiapi.OperationRequest) (app.DoctorReport, error) {
		// A careless message containing the typed passwords must still be redacted on the way out.
		return app.DoctorReport{Status: app.StatusFailed, Error: &device.Error{
			Code: device.CodeWebAuthFailed, Message: "login with " + request.AdminPassword + " and " + request.RootPassword + " failed",
		}}, nil
	}
	backend.apply = func(request guiapi.GuideRequest) (app.GuideApplyReport, error) {
		return app.GuideApplyReport{Status: app.StatusFailed}, &device.Error{Code: device.CodeInternalError, Message: "boom " + request.RootPassword}
	}

	var texts []string
	collect := func(result toolResult) { texts = append(texts, result.Text) }
	collect(h.callWith(context.Background(), h.session, "preflight", map[string]any{
		"target": target("192.168.31.1"), "check_admin_login": true, "check_root_login": true,
		"admin_password": adminSecret, "root_password": rootSecret,
	}, "progress-1"))
	for _, tool := range []string{"guide_plan", "deep_status", "backup_create"} {
		collect(h.call(tool, map[string]any{"target": target("192.168.31.1"), "root_password": rootSecret}))
	}
	plan := h.call("guide_plan", map[string]any{"target": target("192.168.31.1")})
	var report app.GuidePlanReport
	if err := json.Unmarshal(plan.Report, &report); err != nil {
		t.Fatal(err)
	}
	collect(h.call("guide_apply", map[string]any{
		"plan_id": plan.PlanID, "plan_digest": report.PlanDigest, "admin_password": adminSecret, "root_password": rootSecret,
	}))
	collect(h.call("soft_remove", map[string]any{"target": target("192.168.31.1"), "confirm": true, "root_password": rootSecret}))
	collect(h.call("operation_list", nil))

	// Malformed arguments: an unknown field named after the secret, a secret in a field of the wrong type,
	// and a secret as a goal value.
	collect(h.call("deep_status", map[string]any{"target": target("192.168.31.1"), rootSecret: true}))
	collect(h.call("deep_status", map[string]any{"target": map[string]any{"host": "192.168.31.1", "web_port": rootSecret}}))
	collect(h.call("guide_plan", map[string]any{"target": target("192.168.31.1"), "goals": []string{rootSecret}}))
	collect(h.call("guide_apply", map[string]any{"plan_id": rootSecret, "plan_digest": adminSecret}))

	for _, text := range texts {
		assertNoSecret(t, "result", text)
	}
	for {
		select {
		case note := <-h.notes:
			assertNoSecret(t, "progress", note.Message)
			continue
		default:
		}
		break
	}
	for _, record := range h.audit.all() {
		data, _ := json.Marshal(record)
		assertNoSecret(t, "audit", string(data))
	}
}

// The advertised schemas leave password fields unconstrained (validation messages could quote a value) and
// mark them write-only.
func TestPasswordSchemasAreWriteOnlyAndUnconstrained(t *testing.T) {
	t.Parallel()
	h := newHarness(t, newFakeBackend(), true)
	result, err := h.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, tool := range result.Tools {
		schema := toolSchema(t, tool)
		properties, _ := schema["properties"].(map[string]any)
		for _, name := range []string{"admin_password", "root_password"} {
			property, ok := properties[name].(map[string]any)
			if !ok {
				continue
			}
			seen++
			if property["writeOnly"] != true {
				t.Fatalf("%s.%s is not writeOnly: %v", tool.Name, name, property)
			}
			for _, constraint := range []string{"minLength", "maxLength", "pattern", "enum", "format"} {
				if _, ok := property[constraint]; ok {
					t.Fatalf("%s.%s carries %s", tool.Name, name, constraint)
				}
			}
		}
		if schema["additionalProperties"] != false {
			t.Fatalf("%s allows additional properties: %v", tool.Name, schema["additionalProperties"])
		}
	}
	if seen < 7 {
		t.Fatalf("only %d password properties found", seen)
	}
}

func toolSchema(t *testing.T, tool *mcp.Tool) map[string]any {
	t.Helper()
	data, err := json.Marshal(tool.InputSchema)
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	return schema
}

func assertNoSecret(t *testing.T, where, text string) {
	t.Helper()
	for _, secret := range []string{adminSecret, rootSecret} {
		if strings.Contains(text, secret) {
			t.Fatalf("%s leaks a password: %s", where, text)
		}
	}
}
