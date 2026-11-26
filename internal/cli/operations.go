package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/buildinfo"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
	"github.com/sunerpy/router-tools/internal/secret"
)

type EnableService interface {
	EnableSSH(context.Context, app.EnableRequest, app.EventSink) (app.EnableReport, error)
}
type SoftService interface {
	PersistSoft(context.Context, app.SoftRequest, app.EventSink) (app.SoftReport, error)
	PersistRemove(context.Context, app.SoftRequest, app.EventSink) (app.SoftReport, error)
	RebootVerify(context.Context, app.SoftRequest, app.EventSink) (app.RebootReport, error)
}
type BackupService interface {
	BackupCreate(context.Context, app.BackupCreateRequest, app.EventSink) (app.BackupCreateReport, error)
	BackupVerify(context.Context, string, app.EventSink) (app.BackupVerifyReport, error)
}
type DeepService interface {
	DeepPersist(context.Context, app.DeepRequest, app.EventSink) (app.DeepReport, error)
}
type OnlineBackupService interface {
	BackupVerifyOnline(context.Context, app.BackupOnlineVerifyRequest, app.EventSink) (app.BackupVerifyReport, error)
}

type operationOptions struct {
	target             *device.Target
	profileID          *string
	jsonOutput         *bool
	stdin              io.Reader
	service            DoctorService
	info               buildinfo.Info
	redactor           *secret.Redactor
	nonInteractive     *bool
	adminPasswordStdin *bool
	rootPasswordStdin  *bool
	allowTelnet        *bool
	acceptNewHostKey   *bool
	hostKeySHA256      *string
	knownHostsPath     *string
	yes                *bool
	ackDeep            *string
	runDir             *string
}

func addOperationCommands(root *cobra.Command, options *operationOptions) {
	root.AddCommand(&cobra.Command{
		Use: "enable", Short: "Temporarily enable and verify SSH", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, ok := options.service.(EnableService)
			if !ok {
				return &ExitError{Code: 12, Err: fmt.Errorf("enable service is not configured")}
			}
			admin, err := readAdminPassword(options.stdin, cmd.ErrOrStderr(), *options.adminPasswordStdin, *options.nonInteractive)
			if err != nil {
				return &ExitError{Code: exitCode(err), Err: err}
			}
			options.redactor.Add(admin.Expose())
			rootPassword, err := options.rootPassword(cmd)
			if err != nil {
				return err
			}
			confirmed, err := options.confirm(cmd)
			if err != nil {
				return err
			}
			report, err := service.EnableSSH(cmd.Context(), app.EnableRequest{
				Target: *options.target, ProfileID: *options.profileID,
				AdminPassword: admin, RootPassword: rootPassword,
				HostKeyPolicy: options.hostPolicy(cmd), Confirmed: confirmed,
			}, options.events(cmd))
			return options.finish(cmd, report, err)
		},
	})

	persist := &cobra.Command{Use: "persist", Short: "Manage soft SSH persistence"}
	persist.AddCommand(&cobra.Command{
		Use: "soft", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return options.runSoft(cmd, false)
		},
	})
	persist.AddCommand(&cobra.Command{
		Use: "remove", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return options.runSoft(cmd, true)
		},
	})
	root.AddCommand(persist)

	root.AddCommand(&cobra.Command{
		Use: "reboot-verify", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, ok := options.service.(SoftService)
			if !ok {
				return &ExitError{Code: 12, Err: fmt.Errorf("reboot service is not configured")}
			}
			rootPassword, err := options.rootPassword(cmd)
			if err != nil {
				return err
			}
			confirmed, err := options.confirm(cmd)
			if err != nil {
				return err
			}
			report, err := service.RebootVerify(cmd.Context(), app.SoftRequest{
				Target: *options.target, ProfileID: *options.profileID, RootPassword: rootPassword,
				HostKeyPolicy: options.hostPolicy(cmd), AllowTelnet: *options.allowTelnet, Confirmed: confirmed,
			}, options.events(cmd))
			return options.finish(cmd, report, err)
		},
	})

	backupCommand := &cobra.Command{Use: "backup", Short: "Create and verify MTD backups"}
	backupCreate := &cobra.Command{
		Use: "create", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, ok := options.service.(BackupService)
			if !ok {
				return &ExitError{Code: 12, Err: fmt.Errorf("backup service is not configured")}
			}
			rootPassword, err := options.rootPassword(cmd)
			if err != nil {
				return err
			}
			output, _ := cmd.Flags().GetString("output")
			report, err := service.BackupCreate(cmd.Context(), app.BackupCreateRequest{
				Target: *options.target, ProfileID: *options.profileID, RootPassword: rootPassword,
				HostKeyPolicy: options.hostPolicy(cmd), OutputDir: output, ToolVersion: options.info.Version,
			}, options.events(cmd))
			return options.finish(cmd, report, err)
		},
	}
	backupCreate.Flags().String("output", "", "new backup output directory")
	backupCommand.AddCommand(backupCreate)
	backupVerify := &cobra.Command{
		Use: "verify", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, ok := options.service.(BackupService)
			if !ok {
				return &ExitError{Code: 12, Err: fmt.Errorf("backup service is not configured")}
			}
			directory, _ := cmd.Flags().GetString("dir")
			online, _ := cmd.Flags().GetBool("online")
			if online {
				onlineService, ok := options.service.(OnlineBackupService)
				if !ok {
					return &ExitError{Code: 12, Err: fmt.Errorf("online backup service is not configured")}
				}
				rootPassword, err := options.rootPassword(cmd)
				if err != nil {
					return err
				}
				report, err := onlineService.BackupVerifyOnline(cmd.Context(), app.BackupOnlineVerifyRequest{
					BackupCreateRequest: app.BackupCreateRequest{
						Target: *options.target, ProfileID: *options.profileID,
						RootPassword: rootPassword, HostKeyPolicy: options.hostPolicy(cmd),
					},
					BackupDir: directory,
				}, options.events(cmd))
				return options.finish(cmd, report, err)
			}
			report, err := service.BackupVerify(cmd.Context(), directory, options.events(cmd))
			return options.finish(cmd, report, err)
		},
	}
	backupVerify.Flags().String("dir", "", "existing backup directory")
	backupVerify.Flags().Bool("online", false, "also compare current device identity, MTD layout, and boot_id")
	backupCommand.AddCommand(backupVerify)
	root.AddCommand(backupCommand)

	deepCommand := &cobra.Command{
		Use: "deep-persist", Short: "HIGH RISK: deep persistence with boot-flag writes and the profile's reboot sequence (three reboots on RD08 1.1.96)", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, ok := options.service.(DeepService)
			if !ok {
				return &ExitError{Code: 12, Err: fmt.Errorf("deep-persist service is not configured")}
			}
			rootPassword, err := options.rootPassword(cmd)
			if err != nil {
				return err
			}
			ack := *options.ackDeep
			if ack == "" && !*options.nonInteractive {
				policy, err := deepPolicy(options.service, *options.profileID)
				if err != nil {
					return err
				}
				phrase, reboots := "the deep-persist acknowledgement phrase", "several times"
				if policy != nil {
					phrase, reboots = policy.Acknowledgement, fmt.Sprintf("%d times", policy.RebootTransitions)
				}
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "WARNING: this writes/erases MTD and bdata and reboots %s. Interruption may brick the router.\n", reboots)
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Type %s to continue: ", phrase)
				ack, err = readSecretLine(options.stdin)
				if err != nil {
					return &ExitError{Code: exitCode(err), Err: err}
				}
			}
			backupDir, _ := cmd.Flags().GetString("backup-dir")
			report, err := service.DeepPersist(cmd.Context(), app.DeepRequest{
				Target: *options.target, ProfileID: *options.profileID, RootPassword: rootPassword,
				HostKeyPolicy: options.hostPolicy(cmd), AllowTelnet: *options.allowTelnet,
				AckDeep: ack, BackupDir: backupDir, RunDir: *options.runDir,
			}, options.events(cmd))
			return options.finish(cmd, report, err)
		},
	}
	deepCommand.Flags().String("backup-dir", "", "verified backup directory")
	root.AddCommand(deepCommand)
}

func (o *operationOptions) runSoft(cmd *cobra.Command, remove bool) error {
	service, ok := o.service.(SoftService)
	if !ok {
		return &ExitError{Code: 12, Err: fmt.Errorf("soft service is not configured")}
	}
	rootPassword, err := o.rootPassword(cmd)
	if err != nil {
		return err
	}
	confirmed, err := o.confirm(cmd)
	if err != nil {
		return err
	}
	request := app.SoftRequest{
		Target: *o.target, ProfileID: *o.profileID, RootPassword: rootPassword,
		HostKeyPolicy: o.hostPolicy(cmd), AllowTelnet: *o.allowTelnet, Confirmed: confirmed,
	}
	if remove {
		report, err := service.PersistRemove(cmd.Context(), request, o.events(cmd))
		return o.finish(cmd, report, err)
	}
	report, err := service.PersistSoft(cmd.Context(), request, o.events(cmd))
	return o.finish(cmd, report, err)
}

func (o *operationOptions) rootPassword(cmd *cobra.Command) (secret.Password, error) {
	password, err := readRootPassword(o.stdin, cmd.ErrOrStderr(), *o.rootPasswordStdin, *o.nonInteractive)
	if err != nil {
		return secret.Password{}, &ExitError{Code: exitCode(err), Err: err}
	}
	o.redactor.Add(password.Expose())
	return password, nil
}

func (o *operationOptions) confirm(cmd *cobra.Command) (bool, error) {
	if *o.yes {
		return true, nil
	}
	if *o.nonInteractive {
		return false, &ExitError{Code: 9, Err: &device.Error{Code: device.CodeConfirmationRequired, Message: "non-interactive device changes require --yes"}}
	}
	_, _ = fmt.Fprint(cmd.ErrOrStderr(), "This operation changes router state and is not fully reversible. Type yes to continue: ")
	answer, err := readSecretLine(o.stdin)
	if err != nil || !strings.EqualFold(strings.TrimSpace(answer), "yes") {
		return false, &ExitError{Code: 9, Err: &device.Error{Code: device.CodeConfirmationRequired, Message: "operation was not confirmed"}}
	}
	return true, nil
}

func (o *operationOptions) hostPolicy(cmd *cobra.Command) hostkey.Policy {
	if *o.knownHostsPath == "" {
		*o.knownHostsPath = defaultKnownHostsPath()
	}
	policy := hostkey.Policy{KnownHostsPath: *o.knownHostsPath, PinSHA256: *o.hostKeySHA256, AcceptNew: *o.acceptNewHostKey}
	if !*o.nonInteractive {
		policy.Confirm = func(prompt hostkey.Prompt) bool {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Unknown SSH host %s\nAlgorithm: %s\nFingerprint: %s\nAccept this key? [yes/no]: ", prompt.Address, prompt.Algorithm, prompt.Fingerprint)
			answer, err := readSecretLine(o.stdin)
			return err == nil && strings.EqualFold(strings.TrimSpace(answer), "yes")
		}
	}
	return policy
}

func (o *operationOptions) events(cmd *cobra.Command) app.EventSink {
	return app.EventSinkFunc(func(event app.Event) {
		message := fmt.Sprintf("%s: %s (%s)\n", event.Operation, event.Stage, event.Status)
		_, _ = fmt.Fprint(cmd.ErrOrStderr(), o.redactor.Text(message))
	})
}

func (o *operationOptions) finish(cmd *cobra.Command, report any, operationErr error) error {
	var output bytes.Buffer
	if *o.jsonOutput {
		if err := json.NewEncoder(&output).Encode(report); err != nil {
			return &ExitError{Code: 12, Err: err}
		}
	} else if err := renderOperationHuman(&output, report); err != nil {
		return &ExitError{Code: 12, Err: err}
	}
	if _, err := fmt.Fprint(cmd.OutOrStdout(), o.redactor.Text(output.String())); err != nil {
		return &ExitError{Code: 12, Err: err}
	}
	if err := o.persistReport(report); err != nil {
		return &ExitError{Code: 12, Err: err}
	}
	if operationErr != nil {
		return &ExitError{Code: exitCode(operationErr), Err: operationErr}
	}
	if status := operationStatus(report); status == app.StatusPartial || status == app.StatusUnknown {
		return &ExitError{Code: 10, Err: fmt.Errorf("operation finished with status %s; inspect the report before retrying", status)}
	}
	return nil
}

func (o *operationOptions) persistReport(report any) error {
	return persistJSONReport(o.runDir, report, o.redactor)
}

func persistJSONReport(runDir *string, report any, redactor *secret.Redactor) error {
	if *runDir == "" {
		directory, err := os.MkdirTemp("", "routerctl-run-")
		if err != nil {
			return err
		}
		*runDir = directory
	}
	if err := os.MkdirAll(*runDir, 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(report)
	if err != nil {
		return err
	}
	data = []byte(redactor.Text(string(data)))
	temporary, err := os.CreateTemp(*runDir, "report-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	finalPath := strings.TrimSuffix(temporaryPath, ".tmp") + ".json"
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, finalPath)
}

func operationStatus(report any) string {
	switch value := report.(type) {
	case app.EnableReport:
		return value.Status
	case app.SoftReport:
		return value.Status
	case app.RebootReport:
		return value.Status
	case app.BackupCreateReport:
		return value.Status
	case app.BackupVerifyReport:
		return value.Status
	case app.DeepReport:
		return value.Status
	case app.GuideApplyReport:
		return value.Status
	case app.GuidePlanReport:
		return value.Status
	default:
		return ""
	}
}

func renderOperationHuman(writer io.Writer, report any) error {
	switch value := report.(type) {
	case app.EnableReport:
		_, err := fmt.Fprintf(writer, "Status: %s\nOutcome: %s\nSSH verified: %t\nAccepted steps: %d\nAttempted MACs: %v\n", value.Status, value.Outcome, value.SSHVerified, value.AcceptedSteps, value.AttemptedMACs)
		if err != nil {
			return err
		}
		for _, cleanup := range value.Cleanup {
			if !cleanup.Success {
				if _, err := fmt.Fprintf(writer, "Cleanup failed: %s (%s)\n", cleanup.MAC, cleanup.Error); err != nil {
					return err
				}
			}
		}
		return renderOperationError(writer, value.Error)
	case app.SoftReport:
		_, err := fmt.Fprintf(writer, "Status: %s\nOutcome: %s\nVerified: %t\nReboot verified: %t\n", value.Status, value.Result.Outcome, value.Result.Verified, value.Result.RebootVerified)
		if err != nil {
			return err
		}
		return renderOperationError(writer, value.Error)
	case app.RebootReport:
		_, err := fmt.Fprintf(writer, "Status: %s\nOld boot ID: %s\nNew boot ID: %s\nSSH verified: %t\nSoft verified: %t\n", value.Status, value.Result.OldBootID, value.Result.NewBootID, value.Result.SSHVerified, value.Result.SoftVerified)
		if err != nil {
			return err
		}
		return renderOperationError(writer, value.Error)
	case app.BackupCreateReport:
		partitions := 0
		if value.Manifest != nil {
			partitions = len(value.Manifest.Partitions)
		}
		_, err := fmt.Fprintf(writer, "Status: %s\nBackup directory: %s\nPartitions: %d\n", value.Status, value.OutputDir, partitions)
		if err != nil {
			return err
		}
		return renderOperationError(writer, value.Error)
	case app.BackupVerifyReport:
		_, err := fmt.Fprintf(writer, "Status: %s\nBackup directory: %s\nValid: %t\n", value.Status, value.Directory, value.Valid)
		if err != nil {
			return err
		}
		return renderOperationError(writer, value.Error)
	case app.DeepReport:
		_, err := fmt.Fprintf(writer, "Status: %s\nJournal: %s\nVerified boot transitions: %d\nFinal SSH verified: %t\n", value.Status, value.JournalPath, len(value.Result.Transitions), value.Result.FinalSSHVerified)
		if err != nil {
			return err
		}
		if value.Status != app.StatusSucceeded {
			if _, err := fmt.Fprintln(writer, "Do not restart deep-persist from step one; inspect the journal and device manually."); err != nil {
				return err
			}
		}
		return renderOperationError(writer, value.Error)
	case app.GuidePlanReport:
		return renderGuidePlan(writer, value)
	case app.GuideApplyReport:
		return renderGuideApply(writer, value)
	default:
		return fmt.Errorf("unknown operation report type %T", report)
	}
}

func renderOperationError(writer io.Writer, operationErr *device.Error) error {
	if operationErr == nil {
		return nil
	}
	_, err := fmt.Fprintf(writer, "Error: %s: %s\n", operationErr.Code, operationErr.Message)
	return err
}

// deepPolicy finds the deep policy the prompt describes: the explicit profile's own policy, or with auto
// the single policy all registered deep profiles share. The application still verifies the phrase against
// the resolved profile; this only decides what the warning and prompt show.
func deepPolicy(service any, profileID string) (*device.DeepPolicy, error) {
	catalogService, ok := service.(CatalogService)
	if !ok {
		return nil, nil
	}
	explicit := profileID != "" && profileID != "auto"
	policies := make(map[device.DeepPolicy]struct{})
	for _, entry := range catalogService.Catalog() {
		for _, p := range entry.Profiles {
			if p.Deep == nil {
				continue
			}
			if explicit {
				if p.ID == profileID {
					policy := *p.Deep
					return &policy, nil
				}
				continue
			}
			policies[*p.Deep] = struct{}{}
		}
	}
	if explicit || len(policies) == 0 {
		return nil, nil
	}
	if len(policies) > 1 {
		return nil, &ExitError{Code: 2, Err: &device.Error{
			Code:    device.CodeInvalidArgument,
			Message: "registered profiles use different deep acknowledgements or reboot sequences; choose one with --profile or pass --ack-deep",
		}}
	}
	for policy := range policies {
		return &policy, nil
	}
	return nil, nil
}
