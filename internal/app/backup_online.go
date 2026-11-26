package app

import (
	"context"

	"github.com/sunerpy/router-tools/internal/backup"
	"github.com/sunerpy/router-tools/internal/credentials"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
	"github.com/sunerpy/router-tools/internal/secret"
)

// OnlineBackupVerifier performs read-only identity/layout/boot checks.
type OnlineBackupVerifier interface {
	VerifyBackupOnline(context.Context, device.Target, secret.Password, hostkey.Policy, backup.Manifest, device.Observation, string) error
}

// BackupOnlineVerifyRequest combines local integrity and online read-only checks.
type BackupOnlineVerifyRequest struct {
	BackupCreateRequest
	BackupDir string
}

// BackupVerifyOnline first verifies local files, then compares current device evidence.
func (s *Service) BackupVerifyOnline(ctx context.Context, request BackupOnlineVerifyRequest, events EventSink) (BackupVerifyReport, error) {
	report := BackupVerifyReport{SchemaVersion: 1, Operation: "backup verify --online", Status: "running", Directory: request.BackupDir}
	redactor := secret.NewRedactor(request.RootPassword.Expose())
	var creds *credentialSession
	fail := func(err error) (BackupVerifyReport, error) {
		normalized := normalizeError(err)
		redacted := *normalized
		redacted.Message = redactor.Text(redacted.Message)
		report.Status = StatusFailed
		report.Error = &redacted
		report.Credentials = creds.report()
		return report, &redacted
	}
	if err := backup.Verify(request.BackupDir); err != nil {
		return fail(err)
	}
	manifest, err := backup.LoadManifest(request.BackupDir)
	if err != nil {
		return fail(err)
	}
	if err := request.Target.Validate(); err != nil {
		return fail(err)
	}
	if err := checkTypedPassword(request.RootPassword, request.Credentials); err != nil {
		return fail(err)
	}
	if err := s.checkProfileID(request.ProfileID); err != nil {
		return fail(err)
	}
	adapter, probe, err := s.resolveOperationTarget(ctx, request.Target, request.ProfileID)
	if err != nil {
		return fail(err)
	}
	if !adapter.Profile().Supports(device.CapabilityBackup) {
		return fail(&device.Error{Code: device.CodeUnsupportedCapability, Message: "the selected profile does not support online backup verification"})
	}
	verifier, ok := adapter.Operations.(OnlineBackupVerifier)
	if !ok {
		return fail(&device.Error{Code: device.CodeUnsupportedCapability, Message: "online backup verification is unavailable"})
	}
	creds = s.credentialSession(request.Credentials, request.Target, adapter.Profile(), redactor)
	root, err := creds.root(request.RootPassword, false)
	if err != nil {
		return fail(err)
	}
	observation := probe.Observation
	observation.Hardware = backupIdentity(probe.Observation, adapter.Profile()).Hardware
	if err := verifier.VerifyBackupOnline(ctx, request.Target, root, creds.hostKeys(request.HostKeyPolicy), manifest, observation, adapter.Profile().ID); err != nil {
		creds.observe(err)
		return fail(err)
	}
	creds.remember(credentials.KindRoot)
	report.Credentials = creds.report()
	report.Valid = true
	report.Status = StatusSucceeded
	return report, nil
}
