package app

import (
	"context"

	"github.com/sunerpy/router-tools/internal/backup"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
	"github.com/sunerpy/router-tools/internal/secret"
)

// SoftPersistence implements adapter-specific soft install/remove transactions.
type SoftPersistence interface {
	InstallSoft(context.Context, device.Target, secret.Password, hostkey.Policy, bool) (SoftResult, error)
	RemoveSoft(context.Context, device.Target, secret.Password, hostkey.Policy, bool) (SoftResult, error)
}

// RebootVerifier proves soft persistence across a reboot.
type RebootVerifier interface {
	RebootVerify(context.Context, device.Target, secret.Password, hostkey.Policy, bool) (RebootResult, error)
}

// BackupCreator downloads all partitions through adapter-approved SSH.
type BackupCreator interface {
	CreateBackup(context.Context, device.Target, secret.Password, hostkey.Policy, backup.CreateOptions) (backup.Manifest, error)
}

// SoftRequest is shared by soft install/remove and reboot verification.
type SoftRequest struct {
	Target        device.Target
	ProfileID     string
	RootPassword  secret.Password
	HostKeyPolicy hostkey.Policy
	AllowTelnet   bool
	Confirmed     bool
}

// SoftReport wraps installation evidence without claiming reboot verification.
type SoftReport struct {
	SchemaVersion int                `json:"schema_version"`
	Operation     string             `json:"operation"`
	Status        string             `json:"status"`
	Host          string             `json:"host"`
	Device        device.Observation `json:"device"`
	Profile       device.Profile     `json:"profile"`
	Result        SoftResult         `json:"result"`
	Error         *device.Error      `json:"error,omitempty"`
}

// RebootReport wraps final boot, SSH, and soft verification evidence.
type RebootReport struct {
	SchemaVersion int                `json:"schema_version"`
	Operation     string             `json:"operation"`
	Status        string             `json:"status"`
	Host          string             `json:"host"`
	Device        device.Observation `json:"device"`
	Profile       device.Profile     `json:"profile"`
	Result        RebootResult       `json:"result"`
	Error         *device.Error      `json:"error,omitempty"`
}

// PersistSoft installs and verifies the current boot's soft configuration.
func (s *Service) PersistSoft(ctx context.Context, request SoftRequest, events EventSink) (SoftReport, error) {
	return s.softOperation(ctx, request, events, false)
}

// PersistRemove removes only the managed soft configuration.
func (s *Service) PersistRemove(ctx context.Context, request SoftRequest, events EventSink) (SoftReport, error) {
	return s.softOperation(ctx, request, events, true)
}

func (s *Service) softOperation(ctx context.Context, request SoftRequest, events EventSink, remove bool) (SoftReport, error) {
	operation := "persist soft"
	if remove {
		operation = "persist remove"
	}
	report := SoftReport{SchemaVersion: 1, Operation: operation, Status: "running", Host: request.Target.Host}
	emit := operationEmitter(operation, events)
	fail := func(err error) (SoftReport, error) {
		normalized := normalizeError(err)
		redacted := *normalized
		redacted.Message = secret.NewRedactor(request.RootPassword.Expose()).Text(redacted.Message)
		report.Status = StatusFailed
		report.Error = &redacted
		emit("failed", StatusFailed)
		return report, &redacted
	}
	emit("validate", "running")
	if err := validateSoftRequest(request); err != nil {
		return fail(err)
	}
	if err := s.checkProfileID(request.ProfileID); err != nil {
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
	if !report.Profile.Supports(device.CapabilitySoftPersist) {
		return fail(&device.Error{Code: device.CodeUnsupportedCapability, Message: "the selected profile does not support soft persistence"})
	}
	provider, ok := adapter.Operations.(SoftPersistence)
	if !ok {
		return fail(&device.Error{Code: device.CodeUnsupportedCapability, Message: "soft persistence is unavailable"})
	}
	emit("soft", "running")
	var result SoftResult
	if remove {
		result, err = provider.RemoveSoft(ctx, request.Target, request.RootPassword, request.HostKeyPolicy, request.AllowTelnet)
	} else {
		result, err = provider.InstallSoft(ctx, request.Target, request.RootPassword, request.HostKeyPolicy, request.AllowTelnet)
	}
	report.Result = result
	if err != nil {
		return fail(err)
	}
	if !result.Verified {
		return fail(&device.Error{Code: device.CodeRemoteCommandFailed, Message: "soft operation did not pass read-back verification"})
	}
	report.Status = StatusSucceeded
	emit("completed", StatusSucceeded)
	return report, nil
}

// RebootVerify verifies a new boot_id, final SSH root, and soft configuration.
func (s *Service) RebootVerify(ctx context.Context, request SoftRequest, events EventSink) (RebootReport, error) {
	report := RebootReport{SchemaVersion: 1, Operation: "reboot-verify", Status: "running", Host: request.Target.Host}
	emit := operationEmitter("reboot-verify", events)
	fail := func(err error) (RebootReport, error) {
		normalized := normalizeError(err)
		redacted := *normalized
		redacted.Message = secret.NewRedactor(request.RootPassword.Expose()).Text(redacted.Message)
		report.Status = StatusFailed
		report.Error = &redacted
		emit("failed", StatusFailed)
		return report, &redacted
	}
	if err := validateSoftRequest(request); err != nil {
		return fail(err)
	}
	if err := s.checkProfileID(request.ProfileID); err != nil {
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
	if !report.Profile.Supports(device.CapabilitySoftPersist) {
		return fail(&device.Error{Code: device.CodeUnsupportedCapability, Message: "the selected profile does not support reboot verification"})
	}
	verifier, ok := adapter.Operations.(RebootVerifier)
	if !ok {
		return fail(&device.Error{Code: device.CodeUnsupportedCapability, Message: "reboot verification is unavailable"})
	}
	emit("reboot", "running")
	result, err := verifier.RebootVerify(ctx, request.Target, request.RootPassword, request.HostKeyPolicy, request.AllowTelnet)
	report.Result = result
	if err != nil {
		return fail(err)
	}
	if !result.SSHVerified || !result.SoftVerified || result.NewBootID == result.OldBootID {
		return fail(&device.Error{Code: device.CodeRebootTimeout, Message: "reboot verification evidence is incomplete"})
	}
	report.Status = StatusSucceeded
	emit("completed", StatusSucceeded)
	return report, nil
}

// BackupCreateRequest controls one new backup directory.
type BackupCreateRequest struct {
	Target        device.Target
	ProfileID     string
	RootPassword  secret.Password
	HostKeyPolicy hostkey.Policy
	OutputDir     string
	ToolVersion   string
}

// BackupCreateReport includes the completed manifest only on success.
type BackupCreateReport struct {
	SchemaVersion int                `json:"schema_version"`
	Operation     string             `json:"operation"`
	Status        string             `json:"status"`
	Host          string             `json:"host"`
	Device        device.Observation `json:"device"`
	Profile       device.Profile     `json:"profile"`
	OutputDir     string             `json:"output_dir"`
	Manifest      *backup.Manifest   `json:"manifest,omitempty"`
	Error         *device.Error      `json:"error,omitempty"`
}

// BackupCreate resolves identity before reading any partition.
func (s *Service) BackupCreate(ctx context.Context, request BackupCreateRequest, events EventSink) (BackupCreateReport, error) {
	report := BackupCreateReport{SchemaVersion: 1, Operation: "backup create", Status: "running", Host: request.Target.Host, OutputDir: request.OutputDir}
	emit := operationEmitter("backup create", events)
	fail := func(err error) (BackupCreateReport, error) {
		normalized := normalizeError(err)
		redacted := *normalized
		redacted.Message = secret.NewRedactor(request.RootPassword.Expose()).Text(redacted.Message)
		report.Status = StatusFailed
		report.Error = &redacted
		emit("failed", StatusFailed)
		return report, &redacted
	}
	if err := request.Target.Validate(); err != nil {
		return fail(err)
	}
	if err := request.RootPassword.Validate(); err != nil {
		return fail(err)
	}
	if request.OutputDir == "" {
		return fail(&device.Error{Code: device.CodeInvalidArgument, Message: "backup output directory is required"})
	}
	if err := s.checkProfileID(request.ProfileID); err != nil {
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
	if !report.Profile.Supports(device.CapabilityBackup) {
		return fail(&device.Error{Code: device.CodeUnsupportedCapability, Message: "the selected profile does not support backup"})
	}
	creator, ok := adapter.Operations.(BackupCreator)
	if !ok {
		return fail(&device.Error{Code: device.CodeUnsupportedCapability, Message: "backup is unavailable"})
	}
	emit("backup", "running")
	hardware := probe.Observation.Hardware
	if hardware == "" {
		hardware = report.Profile.Hardware
	}
	manifest, err := creator.CreateBackup(ctx, request.Target, request.RootPassword, request.HostKeyPolicy, backup.CreateOptions{
		OutputDir:   request.OutputDir,
		ToolVersion: request.ToolVersion,
		Device: backup.DeviceIdentity{
			DeviceID:   probe.Observation.DeviceID,
			Hardware:   hardware,
			ROMVersion: probe.Observation.ROMVersion,
			Profile:    report.Profile.ID,
		},
	})
	if err != nil {
		return fail(err)
	}
	if !manifest.Complete {
		return fail(&device.Error{Code: device.CodeBackupIncomplete, Message: "adapter returned an incomplete backup manifest"})
	}
	if err := backup.Verify(request.OutputDir); err != nil {
		return fail(err)
	}
	report.Manifest = &manifest
	report.Status = StatusSucceeded
	emit("completed", StatusSucceeded)
	return report, nil
}

// BackupVerifyReport is produced entirely offline.
type BackupVerifyReport struct {
	SchemaVersion int           `json:"schema_version"`
	Operation     string        `json:"operation"`
	Status        string        `json:"status"`
	Directory     string        `json:"directory"`
	Valid         bool          `json:"valid"`
	Error         *device.Error `json:"error,omitempty"`
}

// BackupVerify performs no network access.
func (s *Service) BackupVerify(_ context.Context, directory string, events EventSink) (BackupVerifyReport, error) {
	report := BackupVerifyReport{SchemaVersion: 1, Operation: "backup verify", Status: "running", Directory: directory}
	emit := operationEmitter("backup verify", events)
	if directory == "" {
		err := &device.Error{Code: device.CodeBackupMissing, Message: "backup directory is required"}
		report.Status = StatusFailed
		report.Error = err
		return report, err
	}
	if err := backup.Verify(directory); err != nil {
		normalized := normalizeError(err)
		report.Status = StatusFailed
		report.Error = normalized
		emit("failed", StatusFailed)
		return report, normalized
	}
	report.Status = StatusSucceeded
	report.Valid = true
	emit("completed", StatusSucceeded)
	return report, nil
}

func validateSoftRequest(request SoftRequest) error {
	if err := request.Target.Validate(); err != nil {
		return err
	}
	if err := request.RootPassword.Validate(); err != nil {
		return err
	}
	if !request.Confirmed {
		return &device.Error{Code: device.CodeConfirmationRequired, Message: "this operation requires explicit confirmation"}
	}
	return nil
}

func (s *Service) resolveOperationTarget(ctx context.Context, target device.Target, profileID string) (device.Adapter, device.ProbeResult, error) {
	probe, err := s.discovery.Probe(ctx, target)
	if err != nil {
		return device.Adapter{}, device.ProbeResult{}, err
	}
	adapter, err := s.resolveProfile(profileID, probe.Observation)
	return adapter, probe, err
}
