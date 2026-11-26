package mcpserver_test

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sunerpy/router-tools/internal/mcpserver"
)

// The desktop cancels an agent operation by ID; finished and unknown IDs report false.
func TestOperationsCancel(t *testing.T) {
	t.Parallel()
	backend := newFakeBackend()
	backend.gate = make(chan struct{})
	h := newHarness(t, backend, false)
	started := h.call("backup_create", map[string]any{"target": target("192.168.31.1"), "wait_seconds": 0})
	if !h.ops.Cancel(started.OperationID) {
		t.Fatal("Cancel refused a running operation")
	}
	final := h.call("operation_wait", map[string]any{"operation_id": started.OperationID, "wait_seconds": 5})
	if !final.Done || final.Status != "failed" || !strings.Contains(string(final.Report), `"CANCELED"`) {
		t.Fatalf("final = %+v", final)
	}
	if h.ops.Cancel(started.OperationID) || h.ops.Cancel("agent-unknown") {
		t.Fatal("Cancel accepted a finished or unknown operation")
	}
}

// OnChange runs when an operation starts and when it ends, so the desktop can refresh its list.
func TestOperationsNotifyChanges(t *testing.T) {
	t.Parallel()
	backend := newFakeBackend()
	backend.gate = make(chan struct{})
	h := newHarness(t, backend, false)
	var changes atomic.Int32
	h.ops.OnChange(func() { changes.Add(1) })
	started := h.call("backup_create", map[string]any{"target": target("192.168.31.1"), "wait_seconds": 0})
	if changes.Load() != 1 {
		t.Fatalf("changes after start = %d", changes.Load())
	}
	close(backend.gate)
	h.call("operation_wait", map[string]any{"operation_id": started.OperationID, "wait_seconds": 5})
	waitFor(t, "the finish notification", func() bool { return changes.Load() == 2 })
}

func TestAuditRingKeepsTheNewest(t *testing.T) {
	t.Parallel()
	ring := mcpserver.NewAuditRing(3)
	var seen atomic.Int32
	sink := mcpserver.MultiAudit(ring, mcpserver.AuditFunc(func(mcpserver.AuditRecord) { seen.Add(1) }))
	for i := range 5 {
		sink.Record(mcpserver.AuditRecord{Kind: mcpserver.AuditCall, Tool: "preflight", DurationMS: int64(i), Time: time.Unix(int64(i), 0)})
	}
	records := ring.Records()
	if len(records) != 3 || records[0].DurationMS != 4 || records[2].DurationMS != 2 || seen.Load() != 5 {
		t.Fatalf("records = %+v, seen = %d", records, seen.Load())
	}
	records[0].Tool = "changed"
	if ring.Records()[0].Tool != "preflight" {
		t.Fatal("Records returned the ring's own storage")
	}
}
