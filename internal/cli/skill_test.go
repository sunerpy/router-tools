package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sunerpy/router-tools/internal/agentsetup"
)

func TestSkillShowPrintsTheEmbeddedSkill(t *testing.T) {
	t.Parallel()
	code, stdout, stderr := executeWithInput(t, mcpApplication{}, "", "skill", "show")
	if code != 0 || !strings.HasPrefix(stdout, "---\nname: router-tools\n") {
		t.Fatalf("code = %d, stdout = %.80q, stderr = %q", code, stdout, stderr)
	}
}

// skill install writes to the agent's documented directory, leaves an identical copy alone, refuses to
// overwrite a changed one without --force, and needs --dir for hosts without a documented directory.
func TestSkillInstall(t *testing.T) {
	home := isolateAgentEnvironment(t)

	code, stdout, stderr := executeWithInput(t, mcpApplication{}, "", "skill", "install", "--agent", agentsetup.ClaudeCode)
	installed := filepath.Join(home, ".claude", "skills", "router-tools", "SKILL.md")
	if code != 0 || !strings.Contains(stdout, "installed") || !strings.Contains(stdout, filepath.Dir(installed)) {
		t.Fatalf("install: code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
	if _, err := os.Stat(installed); err != nil {
		t.Fatal(err)
	}
	if code, stdout, _ = executeWithInput(t, mcpApplication{}, "", "skill", "install", "--agent", agentsetup.ClaudeCode); code != 0 || !strings.Contains(stdout, "unchanged") {
		t.Fatalf("second install: code = %d, stdout = %q", code, stdout)
	}

	if err := os.WriteFile(installed, []byte("local edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr = executeWithInput(t, mcpApplication{}, "", "skill", "install", "--agent", agentsetup.ClaudeCode); code != 9 || !strings.Contains(stderr, "--force") {
		t.Fatalf("changed skill without --force: code = %d, stderr = %q", code, stderr)
	}
	if code, stdout, _ = executeWithInput(t, mcpApplication{}, "", "--json", "skill", "install", "--agent", agentsetup.ClaudeCode, "--force"); code != 0 {
		t.Fatalf("--force: code = %d", code)
	}
	var result agentsetup.InstallResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil || result.Action != "updated" {
		t.Fatalf("--json result = %q (%v)", stdout, err)
	}

	if code, _, _ = executeWithInput(t, mcpApplication{}, "", "skill", "install", "--agent", agentsetup.Codex); code != 0 {
		t.Fatalf("codex install: code = %d", code)
	}
	if _, err := os.Stat(filepath.Join(home, ".agents", "skills", "router-tools", "SKILL.md")); err != nil {
		t.Fatal(err)
	}

	for _, agent := range []string{agentsetup.WorkBuddy, agentsetup.Generic} {
		if code, _, stderr = executeWithInput(t, mcpApplication{}, "", "skill", "install", "--agent", agent); code != 2 || !strings.Contains(stderr, "--dir") {
			t.Fatalf("%s: code = %d, stderr = %q", agent, code, stderr)
		}
	}
	dir := t.TempDir()
	if code, _, _ = executeWithInput(t, mcpApplication{}, "", "skill", "install", "--dir", dir); code != 0 {
		t.Fatalf("--dir: code = %d", code)
	}
	if _, err := os.Stat(filepath.Join(dir, "router-tools", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"skill", "install"}, {"skill", "install", "--agent", agentsetup.Codex, "--dir", dir}, {"skill", "install", "--agent", "nope"}} {
		if code, _, _ = executeWithInput(t, mcpApplication{}, "", args...); code != 2 {
			t.Fatalf("%v: code = %d", args, code)
		}
	}

	// A file where the skills directory should be is a file-system error (12), not a safety refusal (9).
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr = executeWithInput(t, mcpApplication{}, "", "skill", "install", "--dir", filepath.Join(blocked, "skills")); code != 12 {
		t.Fatalf("file-system error: code = %d, stderr = %q", code, stderr)
	}

	// CLAUDE_CONFIG_DIR moves Claude Code's skills, and mcp config names the file it moves .claude.json to.
	claudeDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)
	if code, stdout, stderr = executeWithInput(t, mcpApplication{}, "", "skill", "install", "--agent", agentsetup.ClaudeCode); code != 0 || !strings.Contains(stdout, filepath.Join(claudeDir, "skills", "router-tools")) {
		t.Fatalf("CLAUDE_CONFIG_DIR install: code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
	if code, stdout, _ = executeWithInput(t, mcpApplication{}, "", "mcp", "config", "--agent", agentsetup.ClaudeCode); code != 0 || !strings.Contains(stdout, "File: "+filepath.Join(claudeDir, ".claude.json")) {
		t.Fatalf("CLAUDE_CONFIG_DIR config: code = %d, stdout = %q", code, stdout)
	}
}

func TestMCPConfigPrintsTheHostSetup(t *testing.T) {
	isolateAgentEnvironment(t)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := executeWithInput(t, mcpApplication{}, "", "mcp", "config", "--agent", agentsetup.Codex)
	if code != 0 || !strings.Contains(stdout, "~/.codex/config.toml") || !strings.Contains(stdout, "[mcp_servers.router-tools]") || !strings.Contains(stdout, "codex mcp add router-tools") {
		t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, filepath.Base(executable)) {
		t.Fatalf("the snippet does not name this routerctl (%s): %q", executable, stdout)
	}
	code, stdout, _ = executeWithInput(t, mcpApplication{}, "", "--json", "mcp", "config", "--agent", agentsetup.Pi, "--http", "http://127.0.0.1:18731/mcp")
	var setup agentsetup.Setup
	if err := json.Unmarshal([]byte(stdout), &setup); code != 0 || err != nil || setup.Transport != "http" || !strings.Contains(setup.Snippet, "ROUTERCTL_MCP_TOKEN") {
		t.Fatalf("--json: code = %d, stdout = %q (%v)", code, stdout, err)
	}
	for _, args := range [][]string{
		{"mcp", "config"},
		{"mcp", "config", "--agent", "nope"},
		{"mcp", "config", "--agent", agentsetup.WorkBuddy, "--http", "http://127.0.0.1:18731/mcp"},
		{"mcp", "config", "--agent", agentsetup.Codex, "--http", "ftp://x/mcp"},
	} {
		if code, _, stderr = executeWithInput(t, mcpApplication{}, "", args...); code != 2 {
			t.Fatalf("%v: code = %d, stderr = %q", args, code, stderr)
		}
	}
}

// isolateAgentEnvironment points the home directory at a temporary one and clears the variables that move a host's
// configuration or skills, so a developer's own settings never receive a test's files.
func isolateAgentEnvironment(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	for _, name := range []string{"CLAUDE_CONFIG_DIR", "CODEX_HOME", "XDG_CONFIG_HOME", "OPENCODE_CONFIG_DIR", "PI_CODING_AGENT_DIR", "DSH_HOME", "DSH_AGENTS_HOME"} {
		t.Setenv(name, "")
	}
	return home
}
