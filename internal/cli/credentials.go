package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sunerpy/router-tools/internal/app"
	"github.com/sunerpy/router-tools/internal/credentials"
	"github.com/sunerpy/router-tools/internal/device"
)

// CredentialService lists and forgets saved passwords; it never returns a value.
type CredentialService interface {
	ListCredentials() (app.CredentialReport, error)
	ForgetCredentials(credentials.Filter) (app.CredentialReport, error)
	ForgetAllCredentials() (app.CredentialReport, error)
}

func addCredentialCommands(root *cobra.Command, options *operationOptions) {
	command := &cobra.Command{Use: "credentials", Short: "List or forget passwords saved in the encrypted vault"}
	command.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List saved passwords (metadata only, never the values)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, ok := options.service.(CredentialService)
			if !ok {
				return &ExitError{Code: 12, Err: fmt.Errorf("saved passwords are not configured")}
			}
			report, err := service.ListCredentials()
			return options.finishCredentials(cmd, report, err)
		},
	})
	forget := &cobra.Command{
		Use:   "forget",
		Short: "Forget the saved passwords matching every given filter, or all of them",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, ok := options.service.(CredentialService)
			if !ok {
				return &ExitError{Code: 12, Err: fmt.Errorf("saved passwords are not configured")}
			}
			all, _ := cmd.Flags().GetBool("all")
			kind, _ := cmd.Flags().GetString("kind")
			scope, _ := cmd.Flags().GetString("scope")
			host, _ := cmd.Flags().GetString("host")
			port, _ := cmd.Flags().GetInt("port")
			filter := credentials.Filter{Kind: credentials.Kind(kind), Scope: scope, Host: host, Port: port}
			invalid := func(message string) error {
				err := &device.Error{Code: device.CodeInvalidArgument, Message: message}
				return &ExitError{Code: exitCode(err), Err: err}
			}
			switch {
			case all && !filter.Empty():
				return invalid("--all cannot be combined with a filter")
			case all:
				report, err := service.ForgetAllCredentials()
				return options.finishCredentials(cmd, report, err)
			case filter.Empty():
				return invalid("choose what to forget with --kind, --scope, --host or --port, or use --all")
			}
			report, err := service.ForgetCredentials(filter)
			return options.finishCredentials(cmd, report, err)
		},
	}
	forget.Flags().Bool("all", false, "forget every saved password and delete the vault key")
	forget.Flags().String("kind", "", "web-admin or root")
	forget.Flags().String("scope", "", "device model ID, for example xiaomi-rd08")
	// --host is a global flag with a default; the forget filter has its own, empty by default.
	forget.Flags().String("host", "", "router address the password was saved for")
	forget.Flags().Int("port", 0, "Web port of an admin password, SSH port of a root password")
	command.AddCommand(forget)
	root.AddCommand(command)
}

func (o *operationOptions) finishCredentials(cmd *cobra.Command, report app.CredentialReport, operationErr error) error {
	if *o.jsonOutput {
		if err := json.NewEncoder(cmd.OutOrStdout()).Encode(report); err != nil {
			return &ExitError{Code: 12, Err: err}
		}
	} else if err := renderCredentials(cmd.OutOrStdout(), report); err != nil {
		return &ExitError{Code: 12, Err: err}
	}
	if operationErr != nil {
		return &ExitError{Code: exitCode(operationErr), Err: operationErr}
	}
	return nil
}

func renderCredentials(writer io.Writer, report app.CredentialReport) error {
	var b strings.Builder
	fmt.Fprintf(&b, "Status: %s\n", report.Status)
	if len(report.Entries) == 0 && report.Error == nil {
		if report.Operation == "credentials list" {
			b.WriteString("No password is saved.\n")
		} else {
			b.WriteString("Nothing was forgotten.\n")
		}
	}
	for _, entry := range report.Entries {
		state := "saved " + entry.SavedAt.Format("2006-01-02 15:04 MST")
		if entry.Rejected() {
			state += ", rejected by the router " + entry.RejectedAt.Format("2006-01-02 15:04 MST")
		}
		fmt.Fprintf(&b, "  %-9s %-14s %s:%d  %s\n", entry.Kind, entry.Scope, entry.Host, entry.Port, state)
	}
	if _, err := io.WriteString(writer, b.String()); err != nil {
		return err
	}
	return renderOperationError(writer, report.Error)
}

// renderCredentialNotes prints what happened to saved or remembered passwords; it never prints a value.
func renderCredentialNotes(writer io.Writer, notes []app.CredentialNote) error {
	for _, note := range notes {
		var line string
		switch note.Action {
		case app.CredentialUsedSaved:
			line = "used"
		case app.CredentialSaved:
			line = "saved"
		case app.CredentialSaveFailed:
			line = "could not be saved"
		case app.CredentialRejected:
			line = "rejected by the router; it is not used again until a typed password is verified and saved"
		case app.CredentialUnavailable:
			line = "unavailable; the vault could not be read"
		default:
			line = note.Action
		}
		if note.Error != nil {
			line += fmt.Sprintf(": %s: %s", note.Error.Code, note.Error.Message)
		}
		if _, err := fmt.Fprintf(writer, "Saved password (%s): %s\n", note.Kind, line); err != nil {
			return err
		}
	}
	return nil
}
