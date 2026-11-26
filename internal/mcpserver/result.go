package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/secret"
)

// toolResult is the JSON envelope of every tool result.
type toolResult struct {
	OperationID   string        `json:"operation_id,omitempty"`
	Tool          string        `json:"tool"`
	Done          bool          `json:"done"`
	Status        string        `json:"status"`
	PlanID        string        `json:"plan_id,omitempty"`
	WritesEnabled *bool         `json:"writes_enabled,omitempty"`
	Error         *device.Error `json:"error,omitempty"`
	Progress      []app.Event   `json:"progress,omitempty"`
	Report        any           `json:"report,omitempty"`
}

type reply struct {
	// called is the tool that was called; result.Tool names the operation's tool for operation_wait.
	called    string
	host      string
	result    toolResult
	failure   bool
	errorCode string
}

// respond writes the call's audit record and renders its result.
func (s *Server) respond(started time.Time, r reply) *mcp.CallToolResult {
	code := r.errorCode
	if code == "" && r.result.Error != nil {
		code = r.result.Error.Code
	}
	s.audit.Record(AuditRecord{
		Time: s.now().UTC(), Kind: AuditCall, Tool: r.called, OperationID: r.result.OperationID, Host: r.host,
		Status: r.result.Status, ErrorCode: code, DurationMS: s.now().Sub(started).Milliseconds(), Done: r.result.Done,
	})
	return render(r.result, r.failure)
}

func (s *Server) reject(started time.Time, tool string, err *device.Error) *mcp.CallToolResult {
	return s.respond(started, reply{
		called: tool, failure: true,
		result: toolResult{Tool: tool, Done: true, Status: app.StatusFailed, Error: err},
	})
}

// render encodes a result. Reports, errors and progress are already redacted (see Operations.finish and
// Operations.Emit); everything else in the envelope is produced by the server.
func render(result toolResult, failure bool) *mcp.CallToolResult {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	text := ""
	if err := encoder.Encode(result); err != nil {
		failure = true
		text = `{"tool":"` + result.Tool + `","done":true,"status":"failed","error":{"code":"INTERNAL_ERROR","message":"cannot encode the result"}}`
	} else {
		text = strings.TrimSuffix(buffer.String(), "\n")
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}, IsError: failure}
}

// runOperation starts an operation and waits for it as long as the call allows.
func (s *Server) runOperation(ctx context.Context, request *mcp.CallToolRequest, started time.Time, tool, host string,
	wait time.Duration, redactor *secret.Redactor, run func(context.Context, string) outcome,
) *mcp.CallToolResult {
	host = device.CanonicalHost(host)
	op, err := s.ops.start(s.base, tool, host, redactor, run)
	if err != nil {
		return s.respond(started, reply{
			called: tool, host: host, failure: true,
			result: toolResult{Tool: tool, Done: true, Status: app.StatusFailed, Error: err},
		})
	}
	return s.awaitAndRespond(ctx, request, started, tool, op, wait)
}

// runWrite is runOperation for router writes: the write switch is checked, and the operation started, under
// the same read lock that SetAllowWrites takes for writing, so turning writes off cannot interleave.
func (s *Server) runWrite(ctx context.Context, request *mcp.CallToolRequest, started time.Time, tool, host string,
	wait time.Duration, redactor *secret.Redactor, run func(context.Context, string) outcome,
) *mcp.CallToolResult {
	s.writeMu.RLock()
	if !s.allowWrites {
		s.writeMu.RUnlock()
		return s.reject(started, tool, &device.Error{
			Code:    device.CodeAgentWritesDisabled,
			Message: "router writes are not enabled for agents; the user can start routerctl mcp with --allow-writes",
		})
	}
	op, err := s.ops.start(s.base, tool, host, redactor, run)
	s.writeMu.RUnlock()
	if err != nil {
		return s.respond(started, reply{
			called: tool, host: host, failure: true,
			result: toolResult{Tool: tool, Done: true, Status: app.StatusFailed, Error: err},
		})
	}
	return s.awaitAndRespond(ctx, request, started, tool, op, wait)
}

func (s *Server) awaitAndRespond(ctx context.Context, request *mcp.CallToolRequest, started time.Time, called string,
	op *operation, wait time.Duration,
) *mcp.CallToolResult {
	s.await(ctx, request, op, wait)
	done, result, events := s.ops.snapshot(op)
	view := toolResult{OperationID: op.id, Tool: op.tool, Done: done}
	r := reply{called: called, host: op.host}
	if done {
		view.Status, view.Report, view.Error, view.PlanID = result.status, result.report, result.err, result.planID
		r.failure, r.errorCode = isFailure(result.status, result.err), result.errorCode()
	} else {
		view.Status, view.Progress = statusRunning, events
	}
	if op.tool == toolGuidePlan {
		enabled := s.WritesEnabled()
		view.WritesEnabled = &enabled
	}
	r.result = view
	return s.respond(started, r)
}

// await returns when the operation ends, the call's context ends, the server stops, or the wait is over.
// While it waits it forwards the operation's (already redacted) progress when the request asked for it,
// including events that arrived right before the end.
func (s *Server) await(ctx context.Context, request *mcp.CallToolRequest, op *operation, wait time.Duration) {
	if wait <= 0 {
		return
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	var token any
	if request != nil && request.Params != nil && request.Session != nil {
		token = request.Params.GetProgressToken()
	}
	var events <-chan app.Event
	sent := 0
	notify := func(event app.Event) {
		sent++
		_ = request.Session.NotifyProgress(ctx, &mcp.ProgressNotificationParams{
			ProgressToken: token, Progress: float64(sent), Message: progressMessage(event),
		})
	}
	if token != nil {
		history, channel, unsubscribe := s.ops.subscribe(op)
		defer unsubscribe()
		for _, event := range history {
			notify(event)
		}
		events = channel
	}
	for {
		select {
		case <-op.done:
			// finish closes the listener channels before done, so this drain ends.
			if events != nil {
				for event := range events {
					notify(event)
				}
			}
			return
		case <-ctx.Done():
			return
		case <-s.base.Done():
			return
		case <-timer.C:
			return
		case event, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			notify(event)
		}
	}
}

func progressMessage(event app.Event) string {
	parts := make([]string, 0, 4)
	for _, part := range []string{event.Operation, event.Step, event.Stage, event.Status} {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, " ")
}
