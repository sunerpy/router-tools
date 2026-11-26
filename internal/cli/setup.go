package cli

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/internal/secret"
)

// GuideService plans and applies the guided flow.
type GuideService interface {
	GuidePlan(context.Context, app.GuideRequest, app.EventSink) (app.GuidePlanReport, error)
	GuideApply(context.Context, app.GuideApplyRequest, app.EventSink) (app.GuideApplyReport, error)
}

// RecoveryService records that a router was checked after an uncertain change.
type RecoveryService interface {
	AcknowledgeRecovery(context.Context, device.Target) (app.RecoveryReport, error)
}

// effectText explains each effect code in the confirmation and the plan.
var effectText = map[string]string{
	app.EffectTemporaryMACRules:  "adds temporary MAC filter rules through the Web API and removes them again",
	app.EffectNVRAMCommit:        "commits the SSH and Telnet enable flags to NVRAM; they survive reboots",
	app.EffectDropbearStart:      "switches the SSH server to its debug channel and starts it",
	app.EffectRootPasswordChange: "sets the root password to the new one you enter",
	app.EffectLocalBackup:        "reads every flash partition over SSH into a new local backup directory",
	app.EffectScriptWrite:        "writes the managed boot script on the router",
	app.EffectUCICommit:          "commits a managed firewall include that runs the script at boot",
	app.EffectReboot:             "reboots the router; the network is down for several minutes",
}

func addGuideCommands(root *cobra.Command, options *operationOptions) {
	setup := &cobra.Command{
		Use:   "setup",
		Short: "Detect the router, plan only the missing steps, and apply them after one confirmation",
		Args:  cobra.NoArgs,
		RunE:  options.runSetup,
	}
	setup.Flags().StringSlice("goal", []string{app.GoalSSH, app.GoalBackup, app.GoalSoft}, "goals: ssh, backup, soft, reboot (ssh is always included; reboot requires soft)")
	setup.Flags().Bool("reset-root-password", false, "when SSH is open but the current root password is unknown, let the enable flow set a new one")
	setup.Flags().Bool("plan-only", false, "detect and print the plan without changing anything")
	root.AddCommand(setup)

	recovery := &cobra.Command{Use: "recovery", Short: "Handle the record of an uncertain device change"}
	recovery.AddCommand(&cobra.Command{
		Use:   "ack",
		Short: "Record that the router was checked after an uncertain change, so changes can continue",
		Args:  cobra.NoArgs,
		RunE:  options.runRecoveryAck,
	})
	root.AddCommand(recovery)
}

func (o *operationOptions) runSetup(cmd *cobra.Command, _ []string) error {
	service, ok := o.service.(GuideService)
	if !ok {
		return &ExitError{Code: 12, Err: fmt.Errorf("guided setup is not configured")}
	}
	goals, _ := cmd.Flags().GetStringSlice("goal")
	reset, _ := cmd.Flags().GetBool("reset-root-password")
	planOnly, _ := cmd.Flags().GetBool("plan-only")
	stderr := cmd.ErrOrStderr()

	var admin, root secret.Password
	read := func(prompt, nonInteractiveMessage string, fromStdin bool) (secret.Password, error) {
		password, err := readPassword(o.stdin, stderr, prompt, nonInteractiveMessage, fromStdin, *o.nonInteractive)
		if err != nil {
			return secret.Password{}, &ExitError{Code: exitCode(err), Err: err}
		}
		o.redactor.Add(password.Expose())
		return password, nil
	}
	var err error
	if *o.adminPasswordStdin {
		if admin, err = read("Admin password", "", true); err != nil {
			return err
		}
	}
	if *o.rootPasswordStdin {
		if root, err = read("Root password", "", true); err != nil {
			return err
		}
	}

	request := app.GuideRequest{
		Target: *o.target, ProfileID: *o.profileID, Goals: goals, RootPassword: root,
		ResetRootPassword: reset, HostKeyPolicy: o.hostPolicy(cmd),
	}
	plan, err := service.GuidePlan(cmd.Context(), request, o.events(cmd))
	if err == nil && !planOnly && enableNeedsInput(plan) && root.Expose() == "" {
		if *o.nonInteractive {
			return o.finish(cmd, plan, &device.Error{
				Code:    device.CodeInvalidPassword,
				Message: "SSH is open: pass the current root password with --root-password-stdin, or use --reset-root-password to set a new one",
			})
		}
		if root, err = read("Current root password", "", false); err != nil {
			return err
		}
		request.RootPassword = root
		plan, err = service.GuidePlan(cmd.Context(), request, o.events(cmd))
	}
	if err != nil || planOnly {
		return o.finish(cmd, plan, err)
	}
	if enableNeedsInput(plan) {
		return o.finish(cmd, plan, &device.Error{
			Code:    device.CodeSSHAuthFailed,
			Message: "SSH rejected the root password; enter the current one, or use --reset-root-password to set a new one",
		})
	}
	if !plan.Ready || plan.Confirmation == nil {
		return o.finish(cmd, plan, &device.Error{Code: device.CodeConfirmationRequired, Message: "the plan cannot be applied as detected"})
	}
	if !plan.Changes {
		return o.finish(cmd, plan, nil)
	}

	if enableRuns(plan) {
		if admin.Expose() == "" {
			if admin, err = read("Admin password", "non-interactive setup requires --admin-password-stdin", false); err != nil {
				return err
			}
		}
		if !*o.rootPasswordStdin {
			if root, err = o.readNewRootPassword(read); err != nil {
				return err
			}
		}
	} else if root.Expose() == "" {
		if root, err = read("Root password", "non-interactive setup requires --root-password-stdin", false); err != nil {
			return err
		}
	}

	renderConfirmation(stderr, plan)
	confirmed, err := o.confirm(cmd)
	if err != nil {
		return err
	}
	report, err := service.GuideApply(cmd.Context(), app.GuideApplyRequest{
		Confirmation: *plan.Confirmation, PlanDigest: plan.PlanDigest, Confirmed: confirmed,
		AdminPassword: admin, RootPassword: root, HostKeyPolicy: o.hostPolicy(cmd), ToolVersion: o.info.Version,
	}, o.events(cmd))
	return o.finish(cmd, report, err)
}

// readNewRootPassword asks for the password the enable flow sets, twice.
func (o *operationOptions) readNewRootPassword(read func(string, string, bool) (secret.Password, error)) (secret.Password, error) {
	const hint = "non-interactive setup requires --root-password-stdin"
	first, err := read("New root password", hint, false)
	if err != nil {
		return secret.Password{}, err
	}
	second, err := read("Repeat the new root password", hint, false)
	if err != nil {
		return secret.Password{}, err
	}
	if subtle.ConstantTimeCompare([]byte(first.Expose()), []byte(second.Expose())) != 1 {
		mismatch := &device.Error{Code: device.CodeInvalidArgument, Message: "the two new root passwords differ"}
		return secret.Password{}, &ExitError{Code: exitCode(mismatch), Err: mismatch}
	}
	return first, nil
}

func (o *operationOptions) runRecoveryAck(cmd *cobra.Command, _ []string) error {
	service, ok := o.service.(RecoveryService)
	if !ok {
		return &ExitError{Code: 12, Err: fmt.Errorf("recovery acknowledgement is not configured")}
	}
	if !*o.yes {
		notConfirmed := &device.Error{Code: device.CodeConfirmationRequired, Message: "recovery acknowledgement requires --yes"}
		if *o.nonInteractive {
			return &ExitError{Code: 9, Err: notConfirmed}
		}
		_, _ = fmt.Fprint(cmd.ErrOrStderr(), "Confirm that you checked the router as RECOVERY.md describes. Type yes to record it: ")
		answer, err := readSecretLine(o.stdin)
		if err != nil || !strings.EqualFold(strings.TrimSpace(answer), "yes") {
			notConfirmed.Message = "the acknowledgement was not confirmed"
			return &ExitError{Code: 9, Err: notConfirmed}
		}
	}
	report, err := service.AcknowledgeRecovery(cmd.Context(), *o.target)
	if *o.jsonOutput {
		if encodeErr := json.NewEncoder(cmd.OutOrStdout()).Encode(report); encodeErr != nil {
			return &ExitError{Code: 12, Err: encodeErr}
		}
	} else if renderErr := renderRecovery(cmd.OutOrStdout(), report); renderErr != nil {
		return &ExitError{Code: 12, Err: renderErr}
	}
	if err != nil {
		return &ExitError{Code: exitCode(err), Err: err}
	}
	return nil
}

func enableNeedsInput(plan app.GuidePlanReport) bool {
	for _, step := range plan.Steps {
		if step.ID == app.StepEnableSSH {
			return step.Action == app.ActionNeedsInput
		}
	}
	return false
}

func enableRuns(plan app.GuidePlanReport) bool {
	for _, step := range plan.Steps {
		if step.ID == app.StepEnableSSH {
			return step.Action == app.ActionRun
		}
	}
	return false
}

func renderConfirmation(writer io.Writer, plan app.GuidePlanReport) {
	name := plan.Device.DisplayName
	if name == "" {
		name = plan.Device.Hardware
	}
	_, _ = fmt.Fprintf(writer, "Planned changes on %s (%s, profile %s):\n", plan.Host, name, plan.Profile.ID)
	for index, step := range plan.Steps {
		label := step.ID
		switch step.Action {
		case app.ActionSkip:
			continue
		case app.ActionDecideAfter:
			label += " (only if still needed once SSH works)"
		}
		_, _ = fmt.Fprintf(writer, "  %d. %s\n", index+1, label)
		for _, effect := range step.Effects {
			_, _ = fmt.Fprintf(writer, "       - %s: %s\n", effect, effectText[effect])
		}
	}
	_, _ = fmt.Fprintln(writer, "The run stops at the first step that does not succeed and is never retried automatically.")
}

func renderGuidePlan(writer io.Writer, plan app.GuidePlanReport) error {
	var b strings.Builder
	fmt.Fprintf(&b, "Status: %s\n", plan.Status)
	if plan.Profile.ID != "" {
		fmt.Fprintf(&b, "Device: %s (hardware %s), firmware %s\nProfile: %s\n",
			plan.Device.DisplayName, plan.Device.Hardware, plan.Device.ROMVersion, plan.Profile.ID)
	}
	soft := string(plan.State.Soft)
	if soft == "" {
		soft = "unknown"
	}
	backupState := "none"
	if plan.State.Backup != nil {
		backupState = plan.State.Backup.Path
	}
	fmt.Fprintf(&b, "State: SSH port %s, root SSH %s, soft persistence %s, managed backup %s\n",
		valueOr(string(plan.State.SSH), "unknown"), plan.State.RootSSH, soft, backupState)
	if plan.State.Recovery != nil {
		fmt.Fprintf(&b, "Last recorded change: %s ended %s\n", plan.State.Recovery.Operation, plan.State.Recovery.Status)
	}
	b.WriteString("Steps:\n")
	for _, step := range plan.Steps {
		fmt.Fprintf(&b, "  %-14s %-13s %s\n", step.ID, step.Action, step.Reason)
		if step.Maintenance != "" {
			fmt.Fprintf(&b, "      maintenance: %s\n", step.Maintenance)
		}
		for _, effect := range step.Effects {
			fmt.Fprintf(&b, "      effect %s: %s\n", effect, effectText[effect])
		}
	}
	if len(plan.RequiredInputs) > 0 {
		fmt.Fprintf(&b, "Required inputs: %s\n", strings.Join(plan.RequiredInputs, ", "))
	}
	for _, candidate := range plan.Candidates {
		fmt.Fprintf(&b, "Supported firmware of this model: %s (profile %s)\n", candidate.Profile.Firmware, candidate.Profile.ID)
	}
	if plan.Ready && !plan.Changes && plan.Error == nil {
		b.WriteString("Nothing to do: every selected goal is already in place.\n")
	}
	if _, err := io.WriteString(writer, b.String()); err != nil {
		return err
	}
	if err := renderOperationError(writer, plan.Error); err != nil {
		return err
	}
	if plan.Error != nil && plan.Error.Code == device.CodeRecoveryReviewRequired {
		_, err := fmt.Fprintln(writer, "Check the router as RECOVERY.md describes, then run: routerctl recovery ack")
		return err
	}
	return nil
}

func renderGuideApply(writer io.Writer, report app.GuideApplyReport) error {
	var b strings.Builder
	fmt.Fprintf(&b, "Status: %s\nSteps:\n", report.Status)
	for _, step := range report.Steps {
		line := fmt.Sprintf("  %-14s %-10s", step.ID, step.Status)
		if step.Error != nil {
			line += fmt.Sprintf(" %s: %s", step.Error.Code, step.Error.Message)
		}
		b.WriteString(strings.TrimRight(line, " ") + "\n")
		if step.Backup != nil && step.Backup.OutputDir != "" && step.Status == app.StatusSucceeded {
			fmt.Fprintf(&b, "      backup directory: %s\n", step.Backup.OutputDir)
		}
	}
	if report.Status == app.StatusPartial || report.Status == app.StatusUnknown {
		b.WriteString("Inspect the step reports before running setup again; nothing is retried automatically.\n")
	}
	if _, err := io.WriteString(writer, b.String()); err != nil {
		return err
	}
	return renderOperationError(writer, report.Error)
}

func renderRecovery(writer io.Writer, report app.RecoveryReport) error {
	var b strings.Builder
	fmt.Fprintf(&b, "Status: %s\nAcknowledged: %t\n", report.Status, report.Acknowledged)
	if report.Last != nil {
		fmt.Fprintf(&b, "Last recorded change: %s ended %s\n", report.Last.Operation, report.Last.Status)
	} else if report.Error == nil {
		b.WriteString("No change is recorded for this router.\n")
	}
	if _, err := io.WriteString(writer, b.String()); err != nil {
		return err
	}
	return renderOperationError(writer, report.Error)
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
