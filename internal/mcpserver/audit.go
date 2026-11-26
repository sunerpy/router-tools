package mcpserver

import (
	"encoding/json"
	"io"
	"sync"
	"time"
)

// Audit record kinds.
const (
	AuditCall   = "call"
	AuditFinish = "finish"
)

// AuditRecord is one secret-free audit entry: a "call" record when a tool call returns (Done=false when the
// wait ended first) and a "finish" record when an operation ends. It never carries arguments, passwords,
// tokens or report bodies.
type AuditRecord struct {
	Audit       string    `json:"audit"`
	Time        time.Time `json:"time"`
	Kind        string    `json:"kind"`
	Tool        string    `json:"tool"`
	OperationID string    `json:"operation_id,omitempty"`
	Host        string    `json:"host,omitempty"`
	Status      string    `json:"status,omitempty"`
	ErrorCode   string    `json:"error_code,omitempty"`
	DurationMS  int64     `json:"duration_ms"`
	Done        bool      `json:"done"`
}

// AuditSink receives audit records. Implementations must be safe for concurrent use.
type AuditSink interface {
	Record(AuditRecord)
}

type jsonAudit struct {
	mu      sync.Mutex
	encoder *json.Encoder
}

// NewJSONAudit writes each record as one JSON line marked "audit":"mcp".
func NewJSONAudit(w io.Writer) AuditSink {
	return &jsonAudit{encoder: json.NewEncoder(w)}
}

func (a *jsonAudit) Record(record AuditRecord) {
	record.Audit = "mcp"
	a.mu.Lock()
	defer a.mu.Unlock()
	_ = a.encoder.Encode(record)
}

type discardAudit struct{}

func (discardAudit) Record(AuditRecord) {}

// AuditFunc adapts a function to an AuditSink.
type AuditFunc func(AuditRecord)

// Record calls the function.
func (f AuditFunc) Record(record AuditRecord) { f(record) }

type multiAudit []AuditSink

// MultiAudit sends every record to each sink in order.
func MultiAudit(sinks ...AuditSink) AuditSink { return multiAudit(sinks) }

func (m multiAudit) Record(record AuditRecord) {
	for _, sink := range m {
		sink.Record(record)
	}
}

// AuditRing keeps the newest records in memory, for the desktop's activity list.
type AuditRing struct {
	mu       sync.Mutex
	records  []AuditRecord
	capacity int
}

// NewAuditRing keeps at most capacity records.
func NewAuditRing(capacity int) *AuditRing {
	return &AuditRing{capacity: capacity}
}

// Record keeps the record, dropping the oldest beyond the capacity.
func (r *AuditRing) Record(record AuditRecord) {
	record.Audit = "mcp"
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, record)
	if len(r.records) > r.capacity {
		r.records = append([]AuditRecord(nil), r.records[len(r.records)-r.capacity:]...)
	}
}

// Records returns a copy, newest first.
func (r *AuditRing) Records() []AuditRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	copied := make([]AuditRecord, len(r.records))
	for i, record := range r.records {
		copied[len(r.records)-1-i] = record
	}
	return copied
}
