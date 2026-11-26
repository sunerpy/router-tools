package app

import (
	"context"
	"fmt"

	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/state"
)

// OutcomeStore persists the write-ahead record of the last device change per router recovery key.
type OutcomeStore interface {
	Load(key string) (state.Outcome, bool, error)
	Save(key string, outcome state.Outcome) error
}

// writeGuard is the write-ahead record of one device change, opened under the target lock.
type writeGuard struct {
	service *Service
	key     string
	outcome state.Outcome
}

// beginWrite runs after the lock, probe, resolution and capability checks and before any adapter call. It
// refuses while the router's last change is uncertain and unacknowledged, and durably records
// in_progress with device_write_possible=true; if that record cannot be stored nothing is dispatched.
func (s *Service) beginWrite(target device.Target, operation string, evidence state.WriteEvidence) (*writeGuard, error) {
	if s.outcomes == nil {
		return &writeGuard{}, nil
	}
	key := device.RecoveryKey(target)
	last, ok, err := s.outcomes.Load(key)
	if err != nil {
		return nil, stateStoreError(err)
	}
	if ok && last.Blocks() {
		return nil, &device.Error{
			Code: device.CodeRecoveryReviewRequired,
			Message: fmt.Sprintf("the last %s on this router ended %s; check the router as RECOVERY.md describes and acknowledge before another change",
				last.Operation, last.Status),
		}
	}
	outcome := state.Outcome{
		Operation: operation, Host: target.Host, Status: state.StatusInProgress,
		DeviceWritePossible: true, Evidence: evidence, StartedAt: s.clock(),
	}
	if err := s.outcomes.Save(key, outcome); err != nil {
		return nil, stateStoreError(err)
	}
	return &writeGuard{service: s, key: key, outcome: outcome}, nil
}

// settle stores the final status and the adapter's evidence before the lock is released. writePossible may
// be false only when the adapter reported that no device write was submitted. A failed save leaves the
// in_progress record, which keeps blocking.
func (g *writeGuard) settle(status string, writePossible bool, evidence state.WriteEvidence, err error) error {
	if g == nil || g.service == nil {
		return nil
	}
	g.outcome.Status = status
	g.outcome.DeviceWritePossible = writePossible
	g.outcome.Evidence = evidence
	g.outcome.ErrorCode = ""
	if err != nil {
		g.outcome.ErrorCode = normalizeError(err).Code
	}
	g.outcome.FinishedAt = g.service.clock()
	if saveErr := g.service.outcomes.Save(g.key, g.outcome); saveErr != nil {
		return stateStoreError(saveErr)
	}
	return nil
}

func stateStoreError(err error) error {
	return device.WrapError(device.CodeStateStoreFailed, "the recovery record could not be read or stored", err)
}

// RecoveryReport is the result of acknowledging a router's last uncertain change.
type RecoveryReport struct {
	SchemaVersion int            `json:"schema_version"`
	Operation     string         `json:"operation"`
	Status        string         `json:"status"`
	Host          string         `json:"host"`
	Acknowledged  bool           `json:"acknowledged"`
	Last          *state.Outcome `json:"last,omitempty"`
	Error         *device.Error  `json:"error,omitempty"`
}

// AcknowledgeRecovery records that the operator checked the router after its last change. It never
// deletes the record; a later uncertain change blocks again.
func (s *Service) AcknowledgeRecovery(ctx context.Context, target device.Target) (RecoveryReport, error) {
	report := RecoveryReport{SchemaVersion: 1, Operation: "recovery ack", Status: "running", Host: target.Host}
	fail := func(err error) (RecoveryReport, error) {
		normalized := normalizeError(err)
		report.Status = StatusFailed
		report.Error = normalized
		return report, normalized
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if err := target.Validate(); err != nil {
		return fail(err)
	}
	if s.outcomes == nil {
		report.Status = StatusSucceeded
		return report, nil
	}
	release, err := s.locker.Acquire(target)
	if err != nil {
		return fail(err)
	}
	defer func() { _ = release() }()
	key := device.RecoveryKey(target)
	last, ok, err := s.outcomes.Load(key)
	if err != nil {
		return fail(stateStoreError(err))
	}
	if ok {
		last.AcknowledgedAt = s.clock()
		if err := s.outcomes.Save(key, last); err != nil {
			return fail(stateStoreError(err))
		}
		report.Acknowledged = true
		report.Last = &last
	}
	report.Status = StatusSucceeded
	return report, nil
}

// lastOutcome returns the router's recorded last change, if any.
func (s *Service) lastOutcome(target device.Target) (*state.Outcome, error) {
	if s.outcomes == nil {
		return nil, nil
	}
	last, ok, err := s.outcomes.Load(device.RecoveryKey(target))
	if err != nil {
		return nil, stateStoreError(err)
	}
	if !ok {
		return nil, nil
	}
	return &last, nil
}

// settleReport stores the guard's final status. When storing fails the primary error stays primary; a run
// that otherwise succeeded returns STATE_STORE_FAILED, keeping its device status, so the caller knows the
// router stays blocked until acknowledged.
func settleReport[R any](guard *writeGuard, report *R, status string, writePossible bool, evidence state.WriteEvidence, err error, setError func(*device.Error)) (R, error) {
	if settleErr := guard.settle(status, writePossible, evidence, err); settleErr != nil && err == nil {
		stored := normalizeError(settleErr)
		setError(stored)
		return *report, stored
	}
	return *report, err
}
