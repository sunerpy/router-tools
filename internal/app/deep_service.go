package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"

	"github.com/sunerpy/router-tools/internal/backup"
	"github.com/sunerpy/router-tools/internal/device"
	reportpkg "github.com/sunerpy/router-tools/internal/report"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
	"github.com/sunerpy/router-tools/internal/secret"
	"github.com/sunerpy/router-tools/internal/state"
)

// DeepRequest contains every mandatory high-risk gate.
type DeepRequest struct {
	Target        device.Target
	ProfileID     string
	RootPassword  secret.Password
	HostKeyPolicy hostkey.Policy
	AllowTelnet   bool
	AckDeep       string
	BackupDir     string
	RunDir        string
}

// DeepReport points to the journal needed for manual interruption analysis.
type DeepReport struct {
	SchemaVersion int                `json:"schema_version"`
	Operation     string             `json:"operation"`
	Status        string             `json:"status"`
	Host          string             `json:"host"`
	Device        device.Observation `json:"device"`
	Profile       device.Profile     `json:"profile"`
	Result        DeepResult         `json:"result"`
	JournalPath   string             `json:"journal_path,omitempty"`
	Error         *device.Error      `json:"error,omitempty"`
}

// DeepPersist enforces local safety gates before any adapter write.
func (s *Service) DeepPersist(ctx context.Context, request DeepRequest, events EventSink) (DeepReport, error) {
	report := DeepReport{SchemaVersion: 1, Operation: "deep-persist", Status: "running", Host: request.Target.Host}
	emit := operationEmitter("deep-persist", events)
	fail := func(err error) (DeepReport, error) {
		normalized := normalizeError(err)
		redacted := *normalized
		redacted.Message = secret.NewRedactor(request.RootPassword.Expose()).Text(redacted.Message)
		if report.Status == "running" || report.Status == "" {
			report.Status = StatusFailed
		}
		report.Error = &redacted
		emit("failed", report.Status)
		return report, &redacted
	}
	emit("validate", "running")
	if err := request.Target.Validate(); err != nil {
		return fail(err)
	}
	if err := request.RootPassword.Validate(); err != nil {
		return fail(err)
	}
	if !request.AllowTelnet {
		return fail(&device.Error{Code: device.CodeDeepAckRequired, Message: "deep persistence requires explicit Telnet permission"})
	}
	if err := s.checkProfileID(request.ProfileID); err != nil {
		return fail(err)
	}
	if err := s.checkDeepAcknowledgement(request.ProfileID, request.AckDeep); err != nil {
		return fail(err)
	}
	if request.BackupDir == "" {
		return fail(&device.Error{Code: device.CodeBackupMissing, Message: "a verified backup directory is required"})
	}
	if err := backup.Verify(request.BackupDir); err != nil {
		return fail(err)
	}
	manifest, err := backup.LoadManifest(request.BackupDir)
	if err != nil {
		return fail(err)
	}
	release, err := s.locker.Acquire(request.Target)
	if err != nil {
		return fail(err)
	}
	defer func() { _ = release() }()
	adapter, probe, err := s.resolveOperationTarget(ctx, request.Target, request.ProfileID)
	if err != nil {
		return fail(err)
	}
	report.Device = probe.Observation
	report.Profile = adapter.Profile()
	if !report.Profile.Supports(device.CapabilityDeepPersist) {
		return fail(&device.Error{Code: device.CodeUnsupportedCapability, Message: "the selected profile does not support deep persistence"})
	}
	if report.Profile.Deep == nil || request.AckDeep != report.Profile.Deep.Acknowledgement {
		return fail(&device.Error{Code: device.CodeDeepAckRequired, Message: "deep persistence requires the exact acknowledgement of the resolved profile"})
	}
	persister, ok := adapter.Operations.(DeepPersister)
	if !ok {
		return fail(&device.Error{Code: device.CodeUnsupportedCapability, Message: "deep persistence is unavailable"})
	}
	// With no run directory the router's managed journal directory is used, so an earlier journal is
	// always found; a temporary directory remains only for services built without one (tests).
	runDir := request.RunDir
	switch {
	case runDir != "":
		err = os.MkdirAll(runDir, 0o700)
	case s.journalRoot != "":
		runDir = s.managedJournalDir(request.Target, report.Profile)
		err = os.MkdirAll(runDir, 0o700)
	default:
		runDir, err = os.MkdirTemp("", "routerctl-deep-")
	}
	if err != nil {
		return fail(err)
	}
	journalPath := filepath.Join(runDir, deepSafeHost(device.CanonicalHost(request.Target.Host))+"-deep.json")
	report.JournalPath = journalPath
	if _, err := os.Stat(journalPath); err == nil {
		return fail(&device.Error{Code: device.CodeDeepJournalExists, Message: "a deep journal already exists for this host; do not restart from step one"})
	} else if !os.IsNotExist(err) {
		return fail(err)
	}
	guard, err := s.beginWrite(request.Target, "deep-persist", state.WriteEvidence{JournalPath: journalPath})
	if err != nil {
		return fail(err)
	}
	var result DeepResult
	finish := func(final DeepReport, finalErr error) (DeepReport, error) {
		writePossible := result.WriteStarted || final.Status == StatusPartial || final.Status == StatusUnknown
		evidence := state.WriteEvidence{WriteStarted: result.WriteStarted, AcceptedSteps: result.AcceptedWrites, JournalPath: journalPath}
		return settleReport(guard, &final, final.Status, writePossible, evidence, finalErr, func(e *device.Error) { final.Error = e })
	}
	journal, err := reportpkg.CreateJournal(journalPath, request.Target.Host, request.BackupDir)
	if err != nil {
		return finish(fail(err))
	}
	operationObservation := probe.Observation
	operationObservation.Hardware = backupIdentity(probe.Observation, report.Profile).Hardware
	emit("deep", "running")
	result, err = persister.DeepPersist(ctx, DeepParams{
		Target:        request.Target,
		RootPassword:  request.RootPassword,
		HostKeyPolicy: request.HostKeyPolicy,
		BackupDir:     request.BackupDir,
		Manifest:      manifest,
		Observation:   operationObservation,
		ProfileID:     report.Profile.ID,
		Journal:       journal,
	})
	report.Result = result
	report.Status = result.Status
	if err != nil {
		var deviceErr *device.Error
		if errors.As(err, &deviceErr) && deviceErr.Code == device.CodeRemoteResultUnknown {
			report.Status = StatusUnknown
			_ = journal.Stage("UNKNOWN")
		} else {
			if report.Status == "" || report.Status == "running" {
				report.Status = StatusFailed
			}
			_ = journal.Stage("FAILED")
		}
		return finish(fail(err))
	}
	if len(result.Transitions) != report.Profile.Deep.RebootTransitions || !result.FinalSSHVerified || !result.SoftVerified {
		report.Status = StatusPartial
		_ = journal.Stage("FAILED")
		return finish(fail(&device.Error{Code: device.CodeRootVerifyFailed, Message: "deep-persist final evidence is incomplete"}))
	}
	report.Status = StatusSucceeded
	emit("completed", StatusSucceeded)
	return finish(report, nil)
}

// checkDeepAcknowledgement runs before any network access. An explicit profile must declare deep
// persistence and the phrase must be its own; with auto the phrase must belong to some registered deep
// profile, and the resolved profile's own phrase is checked again before any journal or adapter call.
func (s *Service) checkDeepAcknowledgement(profileID, acknowledgement string) error {
	if profileID != "" && profileID != "auto" {
		adapter, ok := s.registry.Lookup(profileID)
		if !ok {
			return &device.Error{Code: device.CodeUnsupportedProfile, Message: "the requested profile is not registered; list profiles with routerctl profiles"}
		}
		profile := adapter.Profile()
		if !profile.Supports(device.CapabilityDeepPersist) || profile.Deep == nil {
			return &device.Error{Code: device.CodeUnsupportedCapability, Message: "the selected profile does not support deep persistence"}
		}
		if acknowledgement != profile.Deep.Acknowledgement {
			return &device.Error{Code: device.CodeDeepAckRequired, Message: "deep persistence requires the exact acknowledgement of the selected profile"}
		}
		return nil
	}
	for _, entry := range s.registry.Catalog() {
		for _, profile := range entry.Profiles {
			if profile.Deep != nil && acknowledgement != "" && acknowledgement == profile.Deep.Acknowledgement {
				return nil
			}
		}
	}
	return &device.Error{Code: device.CodeDeepAckRequired, Message: "deep persistence requires the exact acknowledgement of the device profile"}
}

var deepHostUnsafe = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

func deepSafeHost(host string) string {
	return deepHostUnsafe.ReplaceAllString(host, "_")
}
