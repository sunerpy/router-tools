package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
	"github.com/sunerpy/router-tools/internal/secret"
	"github.com/sunerpy/router-tools/internal/state"
)

// Reasons a deep-persistence run cannot start yet, reported by DeepStatus.
const (
	DeepBlockedRecovery       = "recovery_review_required"
	DeepBlockedJournal        = "journal_exists"
	DeepBlockedAlreadySet     = "flags_already_set"
	DeepBlockedMixedFlags     = "flags_mixed"
	DeepBlockedCrashPartition = "crash_partition_unavailable"
	DeepBlockedNoBackup       = "no_verified_backup"
)

// DeepStatusRequest asks for the read-only state the deep-persistence page shows.
type DeepStatusRequest struct {
	Target    device.Target
	ProfileID string
	// RootPassword is optional; without it the device flags are not inspected.
	RootPassword  secret.Password
	HostKeyPolicy hostkey.Policy
	// Credentials may fill the root password for this read-only SSH check; deep persistence itself never
	// uses a saved password.
	Credentials CredentialUse
}

// DeepStatusReport is the resolved deep policy, the managed journal and backups, and the device state.
type DeepStatusReport struct {
	SchemaVersion int                   `json:"schema_version"`
	Operation     string                `json:"operation"`
	Status        string                `json:"status"`
	Host          string                `json:"host"`
	Device        device.Observation    `json:"device"`
	Profile       device.Profile        `json:"profile"`
	Candidates    []device.ProfileMatch `json:"candidates,omitempty"`
	JournalPath   string                `json:"journal_path,omitempty"`
	JournalExists bool                  `json:"journal_exists"`
	Backups       []ManagedBackup       `json:"backups"`
	SSH           device.PortState      `json:"ssh,omitempty"`
	RootSSH       string                `json:"root_ssh"`
	Inspection    *DeepInspection       `json:"inspection,omitempty"`
	Recovery      *state.Outcome        `json:"recovery,omitempty"`
	// Blocked lists the problems this read found. A recovery record that blocks, an existing journal and
	// an unavailable crash partition make DeepPersist refuse before any write; boot flags that are already
	// or partly set and a missing managed backup are warnings that DeepPersist does not check. Ready means
	// nothing was found and the device state was inspected over a verified SSH root shell; the backup's
	// boot match, the lock and the SSH root shell are checked again when the run starts.
	Blocked     []string         `json:"blocked"`
	Ready       bool             `json:"ready"`
	Credentials []CredentialNote `json:"credentials,omitempty"`
	Error       *device.Error    `json:"error,omitempty"`
}

// DeepStatus reads everything the deep-persistence page needs without changing the router.
func (s *Service) DeepStatus(ctx context.Context, request DeepStatusRequest, events EventSink) (DeepStatusReport, error) {
	report := DeepStatusReport{
		SchemaVersion: 1, Operation: "deep status", Status: "running", Host: request.Target.Host,
		Backups: []ManagedBackup{}, RootSSH: RootSSHNotChecked, Blocked: []string{},
	}
	emit := operationEmitter("deep status", events)
	redactor := secret.NewRedactor(request.RootPassword.Expose())
	var creds *credentialSession
	fail := func(err error) (DeepStatusReport, error) {
		normalized := normalizeError(err)
		redacted := *normalized
		redacted.Message = redactor.Text(redacted.Message)
		report.Status, report.Error = StatusFailed, &redacted
		report.Credentials = creds.report()
		emit("failed", StatusFailed)
		return report, &redacted
	}
	emit("validate", "running")
	if err := request.Target.Validate(); err != nil {
		return fail(err)
	}
	typedRoot := request.RootPassword.Expose() != ""
	if typedRoot {
		if err := request.RootPassword.Validate(); err != nil {
			return fail(err)
		}
	}
	if err := s.checkProfileID(request.ProfileID); err != nil {
		return fail(err)
	}
	last, err := s.lastOutcome(request.Target)
	if err != nil {
		return fail(err)
	}
	report.Recovery = last
	if last != nil && last.Blocks() {
		report.Blocked = append(report.Blocked, DeepBlockedRecovery)
	}

	emit("probe", "running")
	probe, err := s.discovery.Probe(ctx, request.Target)
	if err != nil {
		return fail(err)
	}
	report.Device = probe.Observation
	report.SSH = probe.SSH
	adapter, err := s.resolveProfile(request.ProfileID, probe.Observation)
	if err != nil {
		report.Candidates = s.registry.Candidates(probe.Observation)
		return fail(err)
	}
	profile := adapter.Profile()
	report.Profile = profile
	if !profile.Supports(device.CapabilityDeepPersist) || profile.Deep == nil {
		return fail(&device.Error{Code: device.CodeUnsupportedCapability, Message: "the resolved profile does not support deep persistence"})
	}

	if s.journalRoot != "" {
		report.JournalPath = filepath.Join(s.managedJournalDir(request.Target, profile), deepSafeHost(device.CanonicalHost(request.Target.Host))+"-deep.json")
		if _, err := os.Stat(report.JournalPath); err == nil {
			report.JournalExists = true
			report.Blocked = append(report.Blocked, DeepBlockedJournal)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fail(err)
		}
	}
	report.Backups = s.managedBackups(request.Target, probe.Observation, profile, 20)
	if len(report.Backups) == 0 {
		report.Blocked = append(report.Blocked, DeepBlockedNoBackup)
	}

	creds = s.credentialSession(CredentialUse{UseSaved: request.Credentials.UseSaved, Redactor: request.Credentials.Redactor}, request.Target, profile, redactor)
	root := request.RootPassword
	hasRoot := typedRoot
	if hasRoot {
		root, _ = creds.root(request.RootPassword, false)
	} else if request.Credentials.UseSaved {
		saved, err := creds.root(secret.Password{}, false)
		switch {
		case err == nil:
			root, hasRoot = saved, true
		case hasCode(err, device.CodeCredentialNotFound):
		default:
			creds.notes = append(creds.notes, CredentialNote{Kind: "root", Action: CredentialUnavailable, Error: normalizeError(err)})
		}
	}
	if hasRoot && probe.SSH == device.PortOpen {
		verifier, ok := adapter.Operations.(RootVerifier)
		inspector, inspectable := adapter.Operations.(DeepInspector)
		if !ok || !inspectable {
			return fail(&device.Error{Code: device.CodeUnsupportedCapability, Message: "deep-persistence inspection is unavailable"})
		}
		policy := creds.hostKeys(request.HostKeyPolicy)
		emit("root_verify", "running")
		result, err := verifier.VerifyRoot(ctx, request.Target, root, policy, "ssh", false)
		creds.observe(err)
		switch {
		case err == nil && result.RootVerified && result.Transport == "ssh":
			report.RootSSH = RootSSHVerified
		case hasCode(err, device.CodeSSHAuthFailed):
			report.RootSSH = RootSSHAuthFailed
		case err == nil:
			return fail(&device.Error{Code: device.CodeRootVerifyFailed, Message: "root verification did not succeed"})
		default:
			return fail(err)
		}
		if report.RootSSH == RootSSHVerified {
			emit("inspect", "running")
			inspection, err := inspector.InspectDeep(ctx, request.Target, root, policy)
			if err != nil {
				return fail(err)
			}
			report.Inspection = &inspection
			switch inspection.Flags {
			case DeepFlagsAllSet:
				report.Blocked = append(report.Blocked, DeepBlockedAlreadySet)
			case DeepFlagsMixed:
				report.Blocked = append(report.Blocked, DeepBlockedMixedFlags)
			}
			if !inspection.CrashPartition || !inspection.CrashReadable {
				report.Blocked = append(report.Blocked, DeepBlockedCrashPartition)
			}
		}
	}
	report.Ready = len(report.Blocked) == 0 && report.Inspection != nil
	report.Credentials = creds.report()
	report.Status = StatusSucceeded
	emit("completed", StatusSucceeded)
	return report, nil
}
