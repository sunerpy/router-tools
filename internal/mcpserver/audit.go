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
