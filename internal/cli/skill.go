package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/sunerpy/router-tools/internal/agentsetup"
	"github.com/sunerpy/router-tools/internal/device"
	"github.com/sunerpy/router-tools/skills"
)

func addSkillCommands(root *cobra.Command, options *operationOptions) {
	command := &cobra.Command{Use: "skill", Short: "Show or install the router-tools agent skill"}
	command.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Print the skill's SKILL.md",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := fs.ReadFile(skills.RouterTools(), "SKILL.md")
			if err != nil {
				return &ExitError{Code: 12, Err: err}
			}
			_, err = cmd.OutOrStdout().Write(data)
			return err
		},
	})

	var agent, dir string
	var force bool
	install := &cobra.Command{
		Use:   "install",
		Short: "Install the skill for an agent host or into a skills directory",
		Long: "Install the router-tools skill into the agent's user skills directory (--agent) or into any skills directory (--dir).\n" +
			"An identical copy is left alone; a changed copy is replaced only with --force, which rewrites the skill's own files and keeps others.\n" +
			"Agents with a documented directory: claude-code (~/.claude/skills, or $CLAUDE_CONFIG_DIR/skills), codex, opencode, pi and dsh\n" +
			"(~/.agents/skills; for dsh $DSH_AGENTS_HOME/skills when set).",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runSkillInstall(cmd.OutOrStdout(), agent, dir, force, *options.jsonOutput)
		},
	}
	install.Flags().StringVar(&agent, "agent", "", "agent host: "+agentsetup.IDs())
	install.Flags().StringVar(&dir, "dir", "", "skills directory to install into (the skill goes to <dir>/router-tools)")
	install.Flags().BoolVar(&force, "force", false, "replace the skill's files when an installed copy differs")
	command.AddCommand(install)
	root.AddCommand(command)
}

func invalidInput(message string) error {
	return &ExitError{Code: 2, Err: &device.Error{Code: device.CodeInvalidArgument, Message: message}}
}

func runSkillInstall(stdout io.Writer, agent, dir string, force, jsonOutput bool) error {
	if (agent == "") == (dir == "") {
		return invalidInput("pass exactly one of --agent or --dir")
	}
	root := dir
	if agent != "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return &ExitError{Code: 12, Err: fmt.Errorf("locate the home directory: %w", err)}
		}
		root, err = agentsetup.SkillRoot(agent, home, os.Getenv)
		if err != nil {
			return invalidInput(err.Error())
		}
	}
	files, err := skills.Files()
	if err != nil {
		return &ExitError{Code: 12, Err: err}
	}
	result, err := agentsetup.InstallSkill(skills.RouterTools(), files, root, skills.Name, force)
	switch {
	case errors.Is(err, agentsetup.ErrRefused):
		return &ExitError{Code: 9, Err: err}
	case err != nil:
		return &ExitError{Code: 12, Err: err}
	}
	if jsonOutput {
		return json.NewEncoder(stdout).Encode(result)
	}
	_, err = fmt.Fprintf(stdout, "Skill %s: %s\n", result.Action, result.Path)
	return err
}

func addMCPConfigCommand(parent *cobra.Command, options *operationOptions) {
	var agent, endpoint string
	command := &cobra.Command{
		Use:   "config",
		Short: "Print how to add this MCP server to an agent host",
		Long: "Print the configuration snippet, the file it belongs in, and the host's own add command when it has one.\n" +
			"Without --http the snippet starts this routerctl over stdio; with --http it connects to a running routerctl mcp --http.\n" +
			"Nothing is written: copy the snippet or run the command yourself.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if agent == "" {
				return invalidInput("--agent is required: " + agentsetup.IDs())
			}
			executable, err := os.Executable()
			if err != nil {
				return &ExitError{Code: 12, Err: fmt.Errorf("locate routerctl: %w", err)}
			}
			setup, err := agentsetup.Build(agentsetup.Request{Agent: agent, Command: executable, URL: endpoint, OS: runtime.GOOS, Getenv: os.Getenv})
			if err != nil {
				return invalidInput(err.Error())
			}
			return renderSetup(cmd.OutOrStdout(), setup, *options.jsonOutput)
		},
	}
	command.Flags().StringVar(&agent, "agent", "", "agent host: "+agentsetup.IDs())
	command.Flags().StringVar(&endpoint, "http", "", "URL of a running routerctl mcp --http, for example http://127.0.0.1:18731/mcp")
	parent.AddCommand(command)
}

func renderSetup(w io.Writer, setup agentsetup.Setup, jsonOutput bool) error {
	if jsonOutput {
		encoder := json.NewEncoder(w)
		encoder.SetEscapeHTML(false)
		return encoder.Encode(setup)
	}
	agent, _ := agentsetup.Lookup(setup.Agent)
	text := fmt.Sprintf("Agent: %s (%s), %s\n", agent.Name, setup.Agent, setup.Transport)
	if setup.File != "" {
		text += "File: " + setup.File + "\n"
	}
	text += "\n" + setup.Snippet
	if setup.Command != "" {
		text += "\nOr run:\n  " + setup.Command + "\n"
	}
	if len(setup.Notes) > 0 {
		text += "\nNotes:\n"
		for _, note := range setup.Notes {
			text += "- " + note + "\n"
		}
	}
	if agent.Docs != "" {
		text += "\nDocumentation: " + agent.Docs + "\n"
	}
	_, err := io.WriteString(w, text)
	return err
}
