package mcpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sort"
	"sync"
	"time"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/secret"
)

const (
	maxRunningOperations  = 4
	maxFinishedOperations = 50
	maxOperationEvents    = 20
	operationPrefix       = "agent-"
	statusRunning         = "running"
)

// Operations keeps the operations that agent tools started. Each runs in a context owned by the server, so a
// client that stops waiting (timeout, cancellation, dropped connection) never cancels it.
//
// Operations implements guiapi.EventEmitter: the composer passes it to guiapi.New so that progress of these
// operations reaches the tool calls waiting for them. Events for IDs it did not start are ignored.
type Operations struct {
	mu       sync.Mutex
	byID     map[string]*operation
	finished []string
	running  int
	closed   bool
	now      func() time.Time
	onFinish func(*operation, outcome)
}

// OperationSummary describes one agent operation without its report.
type OperationSummary struct {
	OperationID string     `json:"operation_id"`
	Tool        string     `json:"tool"`
	Host        string     `json:"host,omitempty"`
	Status      string     `json:"status"`
	Done        bool       `json:"done"`
	Started     time.Time  `json:"started"`
	Ended       *time.Time `json:"ended,omitempty"`
	ErrorCode   string     `json:"error_code,omitempty"`
}

type operation struct {
	id      string
	tool    string
	host    string
	started time.Time
	cancel  context.CancelFunc
	done    chan struct{}

	// Guarded by Operations.mu; result and ended are final once done is closed.
	//
	// redactor knows the passwords typed for this operation. Events are redacted when they arrive and the
	// result when the operation ends; then the redactor is dropped, so a finished operation holds no
	// password.
	redactor  *secret.Redactor
	ended     time.Time
	result    outcome
	events    []app.Event
	listeners map[chan app.Event]struct{}
}

// outcome is what a finished operation produced.
type outcome struct {
	status string
	report any
	// err is set when the frontend call itself failed, or returned an error together with its report.
	err *device.Error
	// reportCode is the error code inside the report, for audit records and summaries.
	reportCode string
	planID     string
}

func (o outcome) errorCode() string {
	if o.err != nil {
		return o.err.Code
	}
	return o.reportCode
}

// NewOperations returns an empty operation registry.
func NewOperations() *Operations {
	return &Operations{byID: make(map[string]*operation), now: time.Now}
}

func newOperationID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return operationPrefix + hex.EncodeToString(value), nil
}

// start registers an operation and runs it in a goroutine with a context derived from base.
func (o *Operations) start(base context.Context, tool, host string, redactor *secret.Redactor,
	run func(context.Context, string) outcome,
) (*operation, *device.Error) {
	id, err := newOperationID()
	if err != nil {
		return nil, &device.Error{Code: device.CodeInternalError, Message: "cannot create an operation ID"}
	}
	ctx, cancel := context.WithCancel(base)
	o.mu.Lock()
	if o.closed {
		o.mu.Unlock()
		cancel()
		return nil, &device.Error{Code: device.CodeCanceled, Message: "the MCP server is shutting down"}
	}
	if o.running >= maxRunningOperations {
		o.mu.Unlock()
		cancel()
		return nil, &device.Error{Code: device.CodeOperationLocked, Message: "too many agent operations are running; wait for one to finish"}
	}
	op := &operation{
		id: id, tool: tool, host: host, started: o.now(), redactor: redactor, cancel: cancel,
		done: make(chan struct{}), listeners: make(map[chan app.Event]struct{}),
	}
	o.byID[id] = op
	o.running++
	o.mu.Unlock()
	go func() {
		o.finish(op, runSafely(ctx, id, run))
	}()
	return op, nil
}

func runSafely(ctx context.Context, id string, run func(context.Context, string) outcome) (result outcome) {
	defer func() {
		// The panic value is not reported: it could contain request data.
		if recover() != nil {
			result = outcome{status: app.StatusFailed, err: &device.Error{Code: device.CodeInternalError, Message: "the operation stopped unexpectedly"}}
		}
	}()
	return run(ctx, id)
}

// finish records the redacted result, releases the redactor, runs the finish hook (the audit record) and
// only then marks the operation done, so a process that exits right after Shutdown has the record.
func (o *Operations) finish(op *operation, result outcome) {
	o.mu.Lock()
	result = redactOutcome(op.redactor, result)
	op.redactor = nil
	op.result = result
	op.ended = o.now()
	o.running--
	for listener := range op.listeners {
		close(listener)
	}
	op.listeners = nil
	o.finished = append(o.finished, op.id)
	for len(o.finished) > maxFinishedOperations {
		delete(o.byID, o.finished[0])
		o.finished = o.finished[1:]
	}
	hook := o.onFinish
	op.cancel()
	o.mu.Unlock()
	if hook != nil {
		hook(op, result)
	}
	close(op.done)
}

// Emit records progress for an operation this registry started; other IDs are ignored.
func (o *Operations) Emit(operationID string, event app.Event) {
	o.mu.Lock()
	defer o.mu.Unlock()
	op, ok := o.byID[operationID]
	if !ok || op.listeners == nil {
		return
	}
	event = redactEvent(op.redactor, event)
	op.events = append(op.events, event)
	if len(op.events) > maxOperationEvents {
		op.events = op.events[len(op.events)-maxOperationEvents:]
	}
	for listener := range op.listeners {
		select {
		case listener <- event:
		default:
		}
	}
}

// Has reports whether the ID belongs to an operation this registry started and still keeps.
func (o *Operations) Has(operationID string) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	_, ok := o.byID[operationID]
	return ok
}

func (o *Operations) get(operationID string) (*operation, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	op, ok := o.byID[operationID]
	return op, ok
}

// subscribe returns the events recorded so far and a channel of the following ones, taken atomically so
// that no event is lost or delivered twice. The channel is closed when the operation ends.
func (o *Operations) subscribe(op *operation) ([]app.Event, <-chan app.Event, func()) {
	channel := make(chan app.Event, 16)
	o.mu.Lock()
	defer o.mu.Unlock()
	history := append([]app.Event(nil), op.events...)
	if op.listeners == nil {
		close(channel)
		return history, channel, func() {}
	}
	op.listeners[channel] = struct{}{}
	return history, channel, func() {
		o.mu.Lock()
		defer o.mu.Unlock()
		if op.listeners == nil {
			return
		}
		if _, ok := op.listeners[channel]; ok {
			delete(op.listeners, channel)
			close(channel)
		}
	}
}

func (o *Operations) snapshot(op *operation) (bool, outcome, []app.Event) {
	o.mu.Lock()
	defer o.mu.Unlock()
	select {
	case <-op.done:
		return true, op.result, nil
	default:
		return false, outcome{}, append([]app.Event(nil), op.events...)
	}
}

// List returns every kept operation, newest first.
func (o *Operations) List() []OperationSummary {
	o.mu.Lock()
	defer o.mu.Unlock()
	summaries := make([]OperationSummary, 0, len(o.byID))
	for _, op := range o.byID {
		summary := OperationSummary{OperationID: op.id, Tool: op.tool, Host: op.host, Status: statusRunning, Started: op.started}
		select {
		case <-op.done:
			ended := op.ended
			summary.Done, summary.Status, summary.Ended, summary.ErrorCode = true, op.result.status, &ended, op.result.errorCode()
		default:
		}
		summaries = append(summaries, summary)
	}
	sort.Slice(summaries, func(i, j int) bool { return summaries[i].Started.After(summaries[j].Started) })
	return summaries
}

// cancelRunning cancels every running operation and returns them.
func (o *Operations) cancelRunning() []*operation {
	o.mu.Lock()
	defer o.mu.Unlock()
	var running []*operation
	for _, op := range o.byID {
		select {
		case <-op.done:
		default:
			op.cancel()
			running = append(running, op)
		}
	}
	return running
}

// close refuses new operations from now on.
func (o *Operations) close() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.closed = true
}
