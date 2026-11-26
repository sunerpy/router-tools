// Package cli renders application use cases without owning device behavior.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/buildinfo"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/safety/hostkey"
	"github.com/sunerpy/router-tools/internal/secret"
)

// DoctorService is the application boundary required by the CLI.
type DoctorService interface {
	Doctor(context.Context, app.DoctorRequest, app.EventSink) (app.DoctorReport, error)
}

// CatalogService lists registered device models and their exact firmware profiles without network access.
type CatalogService interface {
	Catalog() []device.CatalogEntry
}

// VerifyService is the application boundary for root shell verification.
type VerifyService interface {
	Verify(context.Context, app.VerifyRequest, app.EventSink) (app.VerifyReport, error)
}

// Execute runs the CLI without interactive input. It is kept for embedders and tests.
func Execute(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
	service DoctorService,
	info buildinfo.Info,
) int {
	return ExecuteWithInput(ctx, args, strings.NewReader(""), stdout, stderr, service, info)
}

// ExecuteWithInput runs the CLI with an explicit secret input source.
func ExecuteWithInput(
	ctx context.Context,
	args []string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
	service DoctorService,
	info buildinfo.Info,
) int {
	cfg, err := loadCLIConfig(args)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "Error: %v\n", err)
		return 2
	}
	target := device.Target{Host: cfg.Host, WebPort: cfg.WebPort, SSHPort: cfg.SSHPort, TelnetPort: cfg.TelnetPort}
	profileID := cfg.Profile
	runDir := cfg.RunDir
	jsonOutput := false
	withAdminAuth := false
	withRootVerify := false
	adminPasswordStdin := false
	rootPasswordStdin := false
	nonInteractive := false
	allowTelnet := false
	acceptNewHostKey := false
	hostKeySHA256 := ""
	knownHostsPath := cfg.KnownHostsFile
	yes := false
	ackDeep := ""
	redactor := secret.NewRedactor()

	root := &cobra.Command{
		Use:           "routerctl",
		Short:         "Read-only and safety-gated router management",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(args)
	flags := root.PersistentFlags()
	flags.StringVar(&target.Host, "host", target.Host, "router IP address or hostname")
	flags.IntVar(&target.WebPort, "web-port", target.WebPort, "router Web HTTP port")
	flags.IntVar(&target.SSHPort, "ssh-port", target.SSHPort, "router SSH port")
	flags.IntVar(&target.TelnetPort, "telnet-port", target.TelnetPort, "router Telnet port")
	flags.StringVar(&profileID, "profile", profileID, "exact device profile ID from 'routerctl profiles', or auto")
	flags.BoolVar(&jsonOutput, "json", false, "write one JSON result object to stdout")
	flags.BoolVar(&nonInteractive, "non-interactive", false, "disable interactive prompts")
	flags.BoolVar(&adminPasswordStdin, "admin-password-stdin", false, "read the Web admin password from stdin")
	flags.BoolVar(&rootPasswordStdin, "root-password-stdin", false, "read the root password from stdin")
	flags.BoolVar(&allowTelnet, "allow-telnet", false, "allow the plaintext Telnet fallback")
	flags.BoolVar(&acceptNewHostKey, "accept-new-host-key", false, "accept and store a previously unknown SSH host key")
	flags.StringVar(&hostKeySHA256, "host-key-sha256", "", "pin the SSH host key SHA256 fingerprint")
	flags.StringVar(&knownHostsPath, "known-hosts", knownHostsPath, "SSH known_hosts file path")
	flags.String("config", "", "JSON config file (loaded before other flags)")
	flags.StringVar(&runDir, "run-dir", runDir, "operation report and journal directory")
	flags.BoolVar(&yes, "yes", false, "confirm ordinary device changes")
	flags.StringVar(&ackDeep, "ack-deep", "", "deep-persist acknowledgement phrase")

	root.AddCommand(&cobra.Command{
		Use:   "profiles",
		Short: "List supported device models and their exact firmware profiles",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			catalogService, ok := service.(CatalogService)
			if !ok {
				return &ExitError{Code: 12, Err: fmt.Errorf("the device catalog is not configured")}
			}
			return renderProfiles(cmd.OutOrStdout(), catalogService.Catalog(), jsonOutput)
		},
	})

	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print build information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if jsonOutput {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(info)
			}
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "routerctl %s\ncommit: %s\nbuild time: %s\n", info.Version, info.CommitID, info.BuildTime)
			return err
		},
	})

	doctorCommand := &cobra.Command{
		Use:   "doctor",
		Short: "Run a read-only device preflight",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if service == nil {
				return &ExitError{Code: 12, Err: fmt.Errorf("doctor service is not configured")}
			}
			var adminPassword secret.Password
			if withAdminAuth {
				var err error
				adminPassword, err = readAdminPassword(stdin, cmd.ErrOrStderr(), adminPasswordStdin, nonInteractive)
				if err != nil {
					return &ExitError{Code: exitCode(err), Err: err}
				}
				redactor.Add(adminPassword.Expose())
			}
			var rootPassword secret.Password
			if withRootVerify {
				var err error
				rootPassword, err = readRootPassword(stdin, cmd.ErrOrStderr(), rootPasswordStdin, nonInteractive)
				if err != nil {
					return &ExitError{Code: exitCode(err), Err: err}
				}
				redactor.Add(rootPassword.Expose())
			}
			events := app.EventSinkFunc(func(event app.Event) {
				message := fmt.Sprintf("%s: %s (%s)\n", event.Operation, event.Stage, event.Status)
				_, _ = fmt.Fprint(cmd.ErrOrStderr(), redactor.Text(message))
			})
			var policy hostkey.Policy
			if withRootVerify {
				if knownHostsPath == "" {
					knownHostsPath = defaultKnownHostsPath()
				}
				policy = hostkey.Policy{KnownHostsPath: knownHostsPath, PinSHA256: hostKeySHA256, AcceptNew: acceptNewHostKey}
				if !nonInteractive {
					policy.Confirm = func(prompt hostkey.Prompt) bool {
						_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Unknown SSH host %s\nAlgorithm: %s\nFingerprint: %s\nAccept? [yes/no]: ", prompt.Address, prompt.Algorithm, prompt.Fingerprint)
						answer, err := readSecretLine(stdin)
						return err == nil && strings.EqualFold(strings.TrimSpace(answer), "yes")
					}
				}
			}
			transport, _ := cmd.Flags().GetString("transport")
			report, err := service.Doctor(cmd.Context(), app.DoctorRequest{
				Target: target, ProfileID: profileID,
				WithAdminAuth: withAdminAuth, AdminPassword: adminPassword,
				WithRootVerify: withRootVerify, RootPassword: rootPassword,
				Transport: transport, AllowTelnet: allowTelnet, HostKeyPolicy: policy,
			}, events)
			var output bytes.Buffer
			if renderErr := renderDoctor(&output, report, jsonOutput); renderErr != nil {
				return &ExitError{Code: 12, Err: renderErr}
			}
			if _, renderErr := fmt.Fprint(cmd.OutOrStdout(), redactor.Text(output.String())); renderErr != nil {
				return &ExitError{Code: 12, Err: renderErr}
			}
			if reportErr := persistJSONReport(&runDir, report, redactor); reportErr != nil {
				return &ExitError{Code: 12, Err: reportErr}
			}
			if err != nil {
				return &ExitError{Code: exitCode(err), Err: err}
			}
			return nil
		},
	}
	doctorCommand.Flags().BoolVar(&withAdminAuth, "with-admin-auth", false, "verify Web admin authentication")
	doctorCommand.Flags().BoolVar(&withRootVerify, "with-root-verify", false, "verify root shell authentication")
	doctorCommand.Flags().String("transport", "ssh", "root shell transport: ssh, telnet, or auto")
	root.AddCommand(doctorCommand)

	verifyCommand := &cobra.Command{
		Use:   "verify",
		Short: "Verify root shell authentication",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			verifier, ok := service.(VerifyService)
			if !ok {
				return &ExitError{Code: 12, Err: fmt.Errorf("verify service is not configured")}
			}
			rootPassword, err := readRootPassword(stdin, cmd.ErrOrStderr(), rootPasswordStdin, nonInteractive)
			if err != nil {
				return &ExitError{Code: exitCode(err), Err: err}
			}
			redactor.Add(rootPassword.Expose())
			if allowTelnet {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "Warning: Telnet transmits credentials and commands in plaintext on the local network")
			}
			if knownHostsPath == "" {
				knownHostsPath = defaultKnownHostsPath()
			}
			policy := hostkey.Policy{
				KnownHostsPath: knownHostsPath,
				PinSHA256:      hostKeySHA256,
				AcceptNew:      acceptNewHostKey,
			}
			if !nonInteractive {
				policy.Confirm = func(prompt hostkey.Prompt) bool {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Unknown SSH host %s\nAlgorithm: %s\nFingerprint: %s\nAccept this key? [yes/no]: ", prompt.Address, prompt.Algorithm, prompt.Fingerprint)
					answer, err := readSecretLine(stdin)
					return err == nil && strings.EqualFold(strings.TrimSpace(answer), "yes")
				}
			}
			events := app.EventSinkFunc(func(event app.Event) {
				message := fmt.Sprintf("%s: %s (%s)\n", event.Operation, event.Stage, event.Status)
				_, _ = fmt.Fprint(cmd.ErrOrStderr(), redactor.Text(message))
			})
			report, err := verifier.Verify(cmd.Context(), app.VerifyRequest{
				Target:        target,
				ProfileID:     profileID,
				RootPassword:  rootPassword,
				Transport:     cmd.Flags().Lookup("transport").Value.String(),
				AllowTelnet:   allowTelnet,
				HostKeyPolicy: policy,
			}, events)
			var output bytes.Buffer
			if renderErr := renderVerify(&output, report, jsonOutput); renderErr != nil {
				return &ExitError{Code: 12, Err: renderErr}
			}
			if _, renderErr := fmt.Fprint(cmd.OutOrStdout(), redactor.Text(output.String())); renderErr != nil {
				return &ExitError{Code: 12, Err: renderErr}
			}
			if reportErr := persistJSONReport(&runDir, report, redactor); reportErr != nil {
				return &ExitError{Code: 12, Err: reportErr}
			}
			if err != nil {
				return &ExitError{Code: exitCode(err), Err: err}
			}
			return nil
		},
	}
	verifyCommand.Flags().String("transport", "ssh", "shell transport: ssh, telnet, or auto")
	root.AddCommand(verifyCommand)
	options := &operationOptions{
		target: &target, profileID: &profileID, jsonOutput: &jsonOutput,
		stdin: stdin, service: service, info: info, redactor: redactor,
		nonInteractive: &nonInteractive, adminPasswordStdin: &adminPasswordStdin,
		rootPasswordStdin: &rootPasswordStdin, allowTelnet: &allowTelnet,
		acceptNewHostKey: &acceptNewHostKey, hostKeySHA256: &hostKeySHA256,
		knownHostsPath: &knownHostsPath, yes: &yes, ackDeep: &ackDeep, runDir: &runDir,
	}
	addOperationCommands(root, options)
	addGuideCommands(root, options)

	if err := root.ExecuteContext(ctx); err != nil {
		_, _ = fmt.Fprint(stderr, redactor.Text(fmt.Sprintf("Error: %v\n", err)))
		var exitErr *ExitError
		if errors.As(err, &exitErr) {
			return exitErr.Code
		}
		return 2
	}
	return 0
}

// ExitError carries the stable process status across the CLI boundary.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string { return e.Err.Error() }
func (e *ExitError) Unwrap() error { return e.Err }

func renderDoctor(writer io.Writer, report app.DoctorReport, jsonOutput bool) error {
	if jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetEscapeHTML(false)
		return encoder.Encode(report)
	}
	if _, err := fmt.Fprintf(writer, "Status: %s\n", report.Status); err != nil {
		return err
	}
	if report.Device.Hardware != "" {
		if _, err := fmt.Fprintf(writer, "Hardware: %s\n", report.Device.Hardware); err != nil {
			return err
		}
	}
	if report.Device.DisplayName != "" {
		if _, err := fmt.Fprintf(writer, "Model: %s\n", report.Device.DisplayName); err != nil {
			return err
		}
	}
	if report.Device.ROMVersion != "" {
		if _, err := fmt.Fprintf(writer, "Firmware: %s\n", report.Device.ROMVersion); err != nil {
			return err
		}
	}
	if report.Profile.ID != "" {
		if _, err := fmt.Fprintf(writer, "Profile: %s\nCompatible: %t\n", report.Profile.ID, report.Compatible); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(
		writer,
		"SSH port: %s (not authentication proof)\nTelnet port: %s (not authentication proof)\nAdmin auth: %s\nRoot verify: %s\n",
		report.SSH,
		report.Telnet,
		report.AdminAuth,
		report.RootVerify,
	); err != nil {
		return err
	}
	if err := renderCandidates(writer, report.Candidates); err != nil {
		return err
	}
	if report.Error != nil {
		_, err := fmt.Fprintf(writer, "Error: %s: %s\n", report.Error.Code, report.Error.Message)
		return err
	}
	return nil
}

// renderCandidates lists the registered firmware of a recognized model after a compatibility failure.
func renderCandidates(writer io.Writer, candidates []device.ProfileMatch) error {
	var models []string
	firmware := make(map[string][]string)
	for _, candidate := range candidates {
		p := candidate.Profile
		model := fmt.Sprintf("%s %s (%s)", p.Vendor, p.Model, p.Hardware)
		if _, known := firmware[model]; !known {
			models = append(models, model)
		}
		firmware[model] = append(firmware[model], fmt.Sprintf("%s (profile %s)", p.Firmware, p.ID))
	}
	for _, model := range models {
		if _, err := fmt.Fprintf(writer, "Supported firmware for %s: %s\n", model, strings.Join(firmware[model], ", ")); err != nil {
			return err
		}
	}
	return nil
}

func renderProfiles(writer io.Writer, catalog []device.CatalogEntry, jsonOutput bool) error {
	if jsonOutput {
		if catalog == nil {
			catalog = []device.CatalogEntry{}
		}
		return json.NewEncoder(writer).Encode(struct {
			SchemaVersion int                   `json:"schema_version"`
			Operation     string                `json:"operation"`
			Models        []device.CatalogEntry `json:"models"`
		}{SchemaVersion: 1, Operation: "profiles", Models: catalog})
	}
	for _, entry := range catalog {
		if _, err := fmt.Fprintf(writer, "%s %s (%s)\n", entry.Vendor, entry.Model, entry.Hardware); err != nil {
			return err
		}
		for _, p := range entry.Profiles {
			if _, err := fmt.Fprintf(writer, "  firmware %-10s --profile %s\n", p.Firmware, p.ID); err != nil {
				return err
			}
		}
	}
	_, err := fmt.Fprintln(writer, "Only the listed firmware versions are accepted; auto picks the single exact match.")
	return err
}

func renderVerify(writer io.Writer, report app.VerifyReport, jsonOutput bool) error {
	if jsonOutput {
		return json.NewEncoder(writer).Encode(report)
	}
	if _, err := fmt.Fprintf(writer, "Status: %s\nTransport: %s\nRoot verified: %t\n", report.Status, report.Transport, report.RootVerified); err != nil {
		return err
	}
	if report.Transport == "telnet" {
		if _, err := fmt.Fprintln(writer, "Telnet root: verified (not SSH success evidence)"); err != nil {
			return err
		}
	}
	if report.BootID != "" {
		if _, err := fmt.Fprintf(writer, "Boot ID: %s\n", report.BootID); err != nil {
			return err
		}
	}
	if report.Error != nil {
		_, err := fmt.Fprintf(writer, "Error: %s: %s\n", report.Error.Code, report.Error.Message)
		return err
	}
	return nil
}

func defaultKnownHostsPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(".", "routerctl", "known_hosts")
	}
	return filepath.Join(configDir, "routerctl", "known_hosts")
}

func exitCode(err error) int {
	if errors.Is(err, context.Canceled) {
		return 130
	}
	var deviceErr *device.Error
	if !errors.As(err, &deviceErr) {
		return 12
	}
	switch deviceErr.Code {
	case device.CodeCanceled:
		return 130
	case device.CodeInvalidArgument, device.CodeInvalidPassword:
		return 2
	case device.CodeWebUnreachable, device.CodeWebProtocolError, device.CodeSSHUnreachable, device.CodeTelnetUnreachable:
		return 3
	case device.CodeWebAuthFailed, device.CodeSSHAuthFailed, device.CodeTelnetAuthFailed:
		return 4
	case device.CodeHostKeyMismatch, device.CodeSSHHostKeyRejected:
		return 5
	case device.CodeRemoteAPIRejected:
		return 6
	case device.CodeRemoteCommandFailed:
		return 7
	case device.CodeRootVerifyFailed, device.CodeRebootNotObserved, device.CodeBootIDUnchanged, device.CodeRebootTimeout:
		return 8
	case device.CodeConfirmationRequired,
		device.CodeDeepAckRequired,
		device.CodeDeepJournalExists,
		device.CodeOperationLocked,
		device.CodeSoftConfigConflict,
		device.CodeBackupMissing,
		device.CodeRecoveryReviewRequired:
		return 9
	case device.CodeRemoteResultUnknown, device.CodeMACCleanupFailed:
		return 10
	case device.CodeBackupIncomplete, device.CodeBackupHashMismatch, device.CodeBackupLayoutMismatch, device.CodeStateStoreFailed:
		return 11
	case device.CodeUnsupportedDevice,
		device.CodeUnsupportedFirmware,
		device.CodeUnsupportedProfile,
		device.CodeUnsupportedCapability,
		device.CodeAmbiguousDevice:
		return 5
	default:
		return 12
	}
}
