package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/device"
)

func TestPlansExpireAndEvictTheOldest(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	store := newPlans(func() time.Time { return now })
	confirmation := app.GuideConfirmation{Target: device.Target{Host: "192.168.31.1"}}

	expiring, err := store.put(confirmation, "d1")
	if err != nil {
		t.Fatal(err)
	}
	if store.host(expiring) != "192.168.31.1" {
		t.Fatalf("host = %q", store.host(expiring))
	}
	now = now.Add(planTTL + time.Second)
	if _, _, takeErr := store.take(expiring, "d1"); takeErr == nil || takeErr.Code != device.CodeConfirmationRequired {
		t.Fatalf("an expired plan was accepted: %v", takeErr)
	}

	first, _ := store.put(confirmation, "first")
	for i := 0; i < maxPlans; i++ {
		if _, err := store.put(confirmation, "later"); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, takeErr := store.take(first, "first"); takeErr == nil {
		t.Fatal("the oldest plan survived eviction")
	}
	if len(store.entries) != maxPlans || len(store.order) != maxPlans {
		t.Fatalf("store holds %d entries, %d ordered", len(store.entries), len(store.order))
	}
}

// The write switch is checked again when a write tool runs, so a call that raced with turning writes off
// is refused without reaching the backend.
func TestWriteHandlersRecheckTheSwitch(t *testing.T) {
	t.Parallel()
	ops := NewOperations()
	server := New(nil, ops, Options{AllowWrites: false})
	called := false
	result := server.runWrite(context.Background(), nil, time.Now(), toolSoftRemove, "192.168.31.1", time.Second, newRedactor(),
		func(context.Context, string) outcome {
			called = true
			return outcome{status: app.StatusSucceeded}
		})
	if called {
		t.Fatal("a write ran while writes were disabled")
	}
	if !result.IsError || !resultHasCode(t, result, device.CodeAgentWritesDisabled) {
		t.Fatalf("result = %+v", result)
	}
	if len(ops.List()) != 0 {
		t.Fatal("a refused write created an operation")
	}
}

func TestOperationEventsAreBoundedAndPanicsAreContained(t *testing.T) {
	t.Parallel()
	ops := NewOperations()
	release := make(chan struct{})
	op, err := ops.start(context.Background(), toolDeepStatus, "h", newRedactor(), func(ctx context.Context, id string) outcome {
		for i := 0; i < maxOperationEvents+5; i++ {
			ops.Emit(id, app.Event{Stage: "s"})
		}
		<-release
		panic("boom with request data")
	})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		_, _, events := ops.snapshot(op)
		if len(events) == maxOperationEvents {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("events = %d", len(events))
		}
		time.Sleep(time.Millisecond)
	}
	close(release)
	<-op.done
	done, result, _ := ops.snapshot(op)
	if !done || result.status != app.StatusFailed || result.err == nil || result.err.Code != device.CodeInternalError {
		t.Fatalf("panicking operation = %+v", result)
	}
	if result.err.Message != "the operation stopped unexpectedly" {
		t.Fatalf("the panic value leaked into the message: %q", result.err.Message)
	}
}

func TestFinishedOperationsAreBounded(t *testing.T) {
	t.Parallel()
	ops := NewOperations()
	var first string
	for i := 0; i < maxFinishedOperations+3; i++ {
		op, err := ops.start(context.Background(), toolDeepStatus, "h", newRedactor(), func(context.Context, string) outcome {
			return outcome{status: app.StatusSucceeded}
		})
		if err != nil {
			t.Fatal(err)
		}
		<-op.done
		if i == 0 {
			first = op.id
		}
	}
	if ops.Has(first) {
		t.Fatal("the oldest finished operation was kept beyond the limit")
	}
	if got := len(ops.List()); got != maxFinishedOperations {
		t.Fatalf("kept %d operations, want %d", got, maxFinishedOperations)
	}
}

func resultHasCode(t *testing.T, result *mcp.CallToolResult, code string) bool {
	t.Helper()
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content %T", result.Content[0])
	}
	var decoded toolResult
	if err := json.Unmarshal([]byte(text.Text), &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded.Error != nil && decoded.Error.Code == code
}

// The finish hook (the finish audit record) runs before waiters see the operation as done, so a process
// that exits right after Shutdown still has the record.
func TestFinishHookRunsBeforeDone(t *testing.T) {
	t.Parallel()
	ops := NewOperations()
	hooked := make(chan struct{})
	ops.onFinish = func(*operation, outcome) {
		time.Sleep(50 * time.Millisecond)
		close(hooked)
	}
	op, err := ops.start(context.Background(), toolDeepStatus, "h", newRedactor(), func(context.Context, string) outcome {
		return outcome{status: app.StatusSucceeded}
	})
	if err != nil {
		t.Fatal(err)
	}
	<-op.done
	select {
	case <-hooked:
	default:
		t.Fatal("done was closed before the finish hook ran")
	}
}

func TestNoOperationStartsAfterStop(t *testing.T) {
	t.Parallel()
	ops := NewOperations()
	ops.cancelRunning()
	ops.close()
	if _, err := ops.start(context.Background(), toolDeepStatus, "h", newRedactor(), func(context.Context, string) outcome {
		return outcome{status: app.StatusSucceeded}
	}); err == nil {
		t.Fatal("an operation started after the registry was closed")
	}
}

// A finished operation keeps only its redacted result: the typed passwords (and the redactor holding them)
// are released when it ends.
func TestFinishedOperationsReleaseTheirPasswords(t *testing.T) {
	t.Parallel()
	ops := NewOperations()
	op, err := ops.start(context.Background(), toolPreflight, "h", newRedactor("pw-123456"), func(context.Context, string) outcome {
		return outcome{status: app.StatusFailed, report: map[string]string{"message": "login with pw-123456 failed"}, err: &device.Error{Code: device.CodeWebAuthFailed, Message: "pw-123456 refused"}}
	})
	if err != nil {
		t.Fatal(err)
	}
	<-op.done
	if op.redactor != nil {
		t.Fatal("a finished operation still holds its redactor")
	}
	_, result, _ := ops.snapshot(op)
	data, _ := json.Marshal(result.report)
	if strings.Contains(string(data), "pw-123456") || strings.Contains(result.err.Message, "pw-123456") {
		t.Fatalf("finished result keeps the password: %s / %s", data, result.err.Message)
	}
}
