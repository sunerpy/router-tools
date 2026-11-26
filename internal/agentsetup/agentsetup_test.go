package agentsetup_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"
	"unicode"

	"github.com/sunerpy/router-tools/internal/agentsetup"
)

const (
	posixCommand   = "/opt/rt/routerctl"
	windowsCommand = `C:\Program Files\Router Tools\routerctl.exe`
	endpoint       = "http://127.0.0.1:18731/mcp"
)

func build(t *testing.T, agent, url, goos, command string) agentsetup.Setup {
	t.Helper()
	setup, err := agentsetup.Build(agentsetup.Request{Agent: agent, Command: command, URL: url, OS: goos})
	if err != nil {
		t.Fatalf("Build(%s, %q, %s): %v", agent, url, goos, err)
	}
	if setup.Format == "json" {
		var parsed any
		if err := json.Unmarshal([]byte(setup.Snippet), &parsed); err != nil {
			t.Fatalf("%s %s snippet is not JSON: %v\n%s", agent, setup.Transport, err, setup.Snippet)
		}
	}
	return setup
}

func mustContain(t *testing.T, label, text string, parts ...string) {
	t.Helper()
	for _, part := range parts {
		if !strings.Contains(text, part) {
			t.Fatalf("%s lacks %q:\n%s", label, part, text)
		}
	}
}

// Each snippet follows the host's own documented format (see docs/agents.md for the sources).
func TestStdioSetups(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		file, format, command string
		parts                 []string
	}{
		agentsetup.ClaudeCode: {
			"~/.claude.json", "json", "claude mcp add --transport stdio --scope user router-tools -- /opt/rt/routerctl mcp",
			[]string{`"mcpServers"`, `"router-tools"`, `"type": "stdio"`, `"command": "/opt/rt/routerctl"`, `"mcp"`},
		},
		agentsetup.Codex: {
			"~/.codex/config.toml", "toml", "codex mcp add router-tools -- /opt/rt/routerctl mcp",
			[]string{"[mcp_servers.router-tools]\n", "command = \"/opt/rt/routerctl\"\n", "args = [\"mcp\"]\n"},
		},
		// The mcp.<name> form, which opencode 1.x and 2.x both read; 1.x rejects 2.x's mcp.servers.
		agentsetup.OpenCode: {
			"~/.config/opencode/opencode.json", "json", "",
			[]string{`"$schema": "https://opencode.ai/config.json"`, "\"mcp\": {\n    \"router-tools\": {", `"type": "local"`, `"/opt/rt/routerctl"`},
		},
		agentsetup.Pi: {
			"~/.pi/agent/mcp.json", "json", "pi mcp add router-tools --exposure direct -- /opt/rt/routerctl mcp",
			[]string{`"mcpServers"`, `"command": "/opt/rt/routerctl"`, `"exposure": "direct"`},
		},
		agentsetup.DSH: {
			"~/.dsh/cordis.patch.yml", "yaml", "",
			[]string{"- insert:\n", "name: '@deepseek-ai/dsh-mcp-client'", "serverName: router-tools", "transport: stdio", "command: '/opt/rt/routerctl'", "args: [mcp]"},
		},
		agentsetup.WorkBuddy: {
			"~/.workbuddy/mcp.json", "json", "",
			[]string{`"mcpServers"`, `"command": "/opt/rt/routerctl"`, `"mcp"`},
		},
		agentsetup.Generic: {
			"", "json", "",
			[]string{`"mcpServers"`, `"command": "/opt/rt/routerctl"`},
		},
	}
	for agent, want := range cases {
		setup := build(t, agent, "", "linux", posixCommand)
		if setup.Agent != agent || setup.Transport != "stdio" || setup.Format != want.format || setup.Command != want.command {
			t.Fatalf("%s: %+v", agent, setup)
		}
		if want.file != "" && setup.File != want.file {
			t.Fatalf("%s: file = %q, want %q", agent, setup.File, want.file)
		}
		mustContain(t, agent, setup.Snippet, want.parts...)
	}
	for _, url := range []string{"", endpoint} {
		if setup := build(t, agentsetup.OpenCode, url, "linux", posixCommand); strings.Contains(setup.Snippet, `"servers"`) {
			t.Fatalf("opencode %s uses the 2.x-only mcp.servers form:\n%s", setup.Transport, setup.Snippet)
		}
	}
}

// Writes stay the user's decision: no snippet or command turns them on.
func TestSetupsNeverEnableWrites(t *testing.T) {
	t.Parallel()
	for _, agent := range agentsetup.Agents() {
		for _, url := range []string{"", endpoint} {
			if url != "" && !agent.HTTP {
				continue
			}
			for _, goos := range []string{"linux", "windows"} {
				setup := build(t, agent.ID, url, goos, posixCommand)
				if strings.Contains(setup.Snippet+setup.Command, "allow-writes") {
					t.Fatalf("%s %s %s enables writes: %+v", agent.ID, setup.Transport, goos, setup)
				}
			}
		}
	}
}

func TestHTTPSetups(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		command string
		parts   []string
	}{
		agentsetup.ClaudeCode: {
			"claude mcp add-json router-tools '",
			[]string{`"type": "http"`, `"url": "` + endpoint + `"`, `"Authorization": "Bearer ${ROUTERCTL_MCP_TOKEN}"`},
		},
		agentsetup.Codex: {
			"codex mcp add router-tools --url " + endpoint + " --bearer-token-env-var ROUTERCTL_MCP_TOKEN",
			[]string{"[mcp_servers.router-tools]\n", "url = \"" + endpoint + "\"\n", "bearer_token_env_var = \"ROUTERCTL_MCP_TOKEN\"\n"},
		},
		agentsetup.OpenCode: {
			"",
			[]string{"\"mcp\": {\n    \"router-tools\": {", `"type": "remote"`, `"oauth": false`, `"Authorization": "Bearer {env:ROUTERCTL_MCP_TOKEN}"`},
		},
		agentsetup.Pi: {
			"pi mcp add router-tools --url " + endpoint + " --bearer-token-env-var ROUTERCTL_MCP_TOKEN --exposure direct",
			[]string{`"type": "http"`, `"Authorization": "Bearer ${ROUTERCTL_MCP_TOKEN}"`, `"exposure": "direct"`},
		},
		agentsetup.DSH: {
			"",
			[]string{"transport: streamable-http", "url: '" + endpoint + "'", "Authorization: !!js '`Bearer ${process.env.ROUTERCTL_MCP_TOKEN}`'"},
		},
		agentsetup.Generic: {
			"",
			[]string{`"url": "` + endpoint + `"`, `"Authorization": "Bearer <token>"`},
		},
	}
	for agent, want := range cases {
		setup := build(t, agent, endpoint, "linux", posixCommand)
		if setup.Transport != "http" || !strings.HasPrefix(setup.Command, want.command) || (want.command == "" && setup.Command != "") {
			t.Fatalf("%s: %+v", agent, setup)
		}
		mustContain(t, agent, setup.Snippet, want.parts...)
		if strings.Contains(setup.Snippet, posixCommand) {
			t.Fatalf("%s: an HTTP setup names the routerctl path", agent)
		}
	}
	// The Claude Code command carries the same server entry as the snippet.
	claude := build(t, agentsetup.ClaudeCode, endpoint, "linux", posixCommand)
	mustContain(t, "claude command", claude.Command, `{"type":"http","url":"`+endpoint+`","headers":{"Authorization":"Bearer ${ROUTERCTL_MCP_TOKEN}"}}`, "--scope user")
}

func TestQuotingPerPlatform(t *testing.T) {
	t.Parallel()
	codex := build(t, agentsetup.Codex, "", "windows", windowsCommand)
	mustContain(t, "codex toml", codex.Snippet, `command = "C:\\Program Files\\Router Tools\\routerctl.exe"`)
	if codex.Command != `codex mcp add router-tools -- "C:\Program Files\Router Tools\routerctl.exe" mcp` {
		t.Fatalf("windows command = %s", codex.Command)
	}
	claude := build(t, agentsetup.ClaudeCode, "", "windows", windowsCommand)
	mustContain(t, "claude json", claude.Snippet, `"command": "C:\\Program Files\\Router Tools\\routerctl.exe"`)
	if httpSetup := build(t, agentsetup.ClaudeCode, endpoint, "windows", windowsCommand); httpSetup.Command != "" {
		t.Fatalf("a JSON-in-shell command is offered on Windows: %s", httpSetup.Command)
	}
	dsh := build(t, agentsetup.DSH, "", "windows", `C:\it's\routerctl.exe`)
	mustContain(t, "dsh yaml", dsh.Snippet, `command: 'C:\it''s\routerctl.exe'`)
	spaced := build(t, agentsetup.Pi, "", "linux", "/opt/my tools/it's/routerctl")
	if spaced.Command != `pi mcp add router-tools --exposure direct -- '/opt/my tools/it'\''s/routerctl' mcp` {
		t.Fatalf("posix quoting = %s", spaced.Command)
	}
	// The XDG_CONFIG_HOME entry depends on the environment; TestCodexSetsXDGConfigHome covers it.
}

func TestBuildRefuses(t *testing.T) {
	t.Parallel()
	for name, request := range map[string]agentsetup.Request{
		"unknown agent":   {Agent: "cursorx", Command: posixCommand},
		"workbuddy http":  {Agent: agentsetup.WorkBuddy, URL: endpoint},
		"ftp url":         {Agent: agentsetup.Codex, URL: "ftp://127.0.0.1/mcp"},
		"no host":         {Agent: agentsetup.Codex, URL: "http:///mcp"},
		"stdio no binary": {Agent: agentsetup.Codex},
	} {
		if _, err := agentsetup.Build(request); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
	if setup := build(t, agentsetup.Codex, "http://192.168.1.5:18731", "linux", posixCommand); !strings.Contains(setup.Snippet, `url = "http://192.168.1.5:18731/mcp"`) {
		t.Fatalf("a URL without a path does not get /mcp: %s", setup.Snippet)
	}
}

func TestAgentsAndSkillTargets(t *testing.T) {
	t.Parallel()
	ids := []string{}
	for _, agent := range agentsetup.Agents() {
		ids = append(ids, agent.ID)
		if agent.Name == "" || (agent.ID != agentsetup.Generic && !strings.HasPrefix(agent.Docs, "https://")) {
			t.Fatalf("incomplete agent %+v", agent)
		}
	}
	if strings.Join(ids, ",") != "claude-code,codex,opencode,pi,dsh,workbuddy,generic" {
		t.Fatalf("agents = %v", ids)
	}
	home := filepath.Join("home", "u")
	for agent, want := range map[string]string{
		agentsetup.ClaudeCode: filepath.Join(home, ".claude", "skills"),
		agentsetup.Codex:      filepath.Join(home, ".agents", "skills"),
		agentsetup.OpenCode:   filepath.Join(home, ".agents", "skills"),
		agentsetup.Pi:         filepath.Join(home, ".agents", "skills"),
		agentsetup.DSH:        filepath.Join(home, ".agents", "skills"),
	} {
		got, err := agentsetup.SkillRoot(agent, home, noEnv)
		if err != nil || got != want {
			t.Fatalf("SkillRoot(%s) = %q, %v; want %q", agent, got, err, want)
		}
	}
	for _, agent := range []string{agentsetup.WorkBuddy, agentsetup.Generic, "nope"} {
		if _, err := agentsetup.SkillRoot(agent, home, noEnv); err == nil {
			t.Fatalf("SkillRoot(%s) has a target", agent)
		}
	}
}

func noEnv(string) string { return "" }

func envOf(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

// Hosts move their configuration and skills when their documented variables are set; the printed file and
// the install target follow them.
func TestEnvironmentOverrides(t *testing.T) {
	t.Parallel()
	posix := envOf(map[string]string{
		"CLAUDE_CONFIG_DIR": "/work/claude", "CODEX_HOME": "/work/codex/", "XDG_CONFIG_HOME": "/work/xdg",
		"PI_CODING_AGENT_DIR": "/work/pi", "DSH_HOME": "/work/dsh", "DSH_AGENTS_HOME": "/work/agents",
	})
	for agent, want := range map[string]string{
		agentsetup.ClaudeCode: "/work/claude/.claude.json",
		agentsetup.Codex:      "/work/codex/config.toml",
		agentsetup.OpenCode:   "/work/xdg/opencode/opencode.json",
		agentsetup.Pi:         "/work/pi/mcp.json",
		agentsetup.DSH:        "/work/dsh/cordis.patch.yml",
		agentsetup.WorkBuddy:  "~/.workbuddy/mcp.json",
	} {
		for _, url := range []string{"", endpoint} {
			if url != "" && agent == agentsetup.WorkBuddy {
				continue
			}
			setup, err := agentsetup.Build(agentsetup.Request{Agent: agent, Command: posixCommand, URL: url, OS: "linux", Getenv: posix})
			if err != nil || setup.File != want {
				t.Fatalf("%s %q: file = %q (%v), want %q", agent, url, setup.File, err, want)
			}
		}
	}
	for _, url := range []string{"", endpoint} {
		setup, err := agentsetup.Build(agentsetup.Request{Agent: agentsetup.OpenCode, Command: posixCommand, URL: url, OS: "linux", Getenv: envOf(map[string]string{"OPENCODE_CONFIG_DIR": "/work/oc", "XDG_CONFIG_HOME": "/work/xdg"})})
		if err != nil || setup.File != "/work/oc/opencode.json" || !strings.Contains(strings.Join(setup.Notes, " "), "OPENCODE_CONFIG_DIR") {
			t.Fatalf("OPENCODE_CONFIG_DIR %q: %+v (%v)", url, setup, err)
		}
	}
	windows, err := agentsetup.Build(agentsetup.Request{Agent: agentsetup.ClaudeCode, Command: windowsCommand, OS: "windows", Getenv: envOf(map[string]string{"CLAUDE_CONFIG_DIR": `D:\claude`})})
	if err != nil || windows.File != `D:\claude\.claude.json` {
		t.Fatalf("windows file = %q (%v)", windows.File, err)
	}
	if notes := strings.Join(windows.Notes, "\n"); !strings.Contains(notes, "CLAUDE_CONFIG_DIR") {
		t.Fatalf("the notes do not say why the file moved: %s", notes)
	}
	if setup := build(t, agentsetup.ClaudeCode, "", "linux", posixCommand); setup.File != "~/.claude.json" {
		t.Fatalf("default file = %q", setup.File)
	}

	home, claude, agents := t.TempDir(), t.TempDir(), t.TempDir()
	lookup := envOf(map[string]string{"CLAUDE_CONFIG_DIR": claude, "DSH_AGENTS_HOME": agents})
	for agent, want := range map[string]string{
		agentsetup.ClaudeCode: filepath.Join(claude, "skills"),
		agentsetup.DSH:        filepath.Join(agents, "skills"),
		agentsetup.Codex:      filepath.Join(home, ".agents", "skills"),
	} {
		if got, err := agentsetup.SkillRoot(agent, home, lookup); err != nil || got != want {
			t.Fatalf("SkillRoot(%s) = %q, %v; want %q", agent, got, err, want)
		}
	}
	if _, err := agentsetup.SkillRoot(agentsetup.ClaudeCode, home, envOf(map[string]string{"CLAUDE_CONFIG_DIR": "relative/claude"})); err == nil || !strings.Contains(err.Error(), "--dir") {
		t.Fatalf("a relative CLAUDE_CONFIG_DIR was used: %v", err)
	}
}

// Codex passes only a few variables to stdio servers. Where routerctl's configuration directory follows
// XDG_CONFIG_HOME, the entry sets it, in the form codex mcp add --env writes.
func TestCodexSetsXDGConfigHome(t *testing.T) {
	t.Parallel()
	xdg := envOf(map[string]string{"XDG_CONFIG_HOME": "/home/u/.xdg"})
	linux, err := agentsetup.Build(agentsetup.Request{Agent: agentsetup.Codex, Command: posixCommand, OS: "linux", Getenv: xdg})
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, "codex snippet", linux.Snippet, "args = [\"mcp\"]\n\n[mcp_servers.router-tools.env]\nXDG_CONFIG_HOME = \"/home/u/.xdg\"\n")
	if linux.Command != "codex mcp add router-tools --env XDG_CONFIG_HOME=/home/u/.xdg -- /opt/rt/routerctl mcp" {
		t.Fatalf("command = %s", linux.Command)
	}
	mustContain(t, "codex notes", strings.Join(linux.Notes, "\n"), "XDG_CONFIG_HOME", "recovery records")
	for _, request := range []agentsetup.Request{
		{Agent: agentsetup.Codex, Command: posixCommand, OS: "linux", Getenv: envOf(map[string]string{"HOME": "/home/u", "XDG_CONFIG_HOME": "/home/u/.config/"})},
		{Agent: agentsetup.Codex, Command: posixCommand, OS: "darwin", Getenv: xdg},
		{Agent: agentsetup.Codex, Command: posixCommand, OS: "linux", Getenv: noEnv},
		{Agent: agentsetup.Codex, Command: windowsCommand, OS: "windows", Getenv: xdg},
		{Agent: agentsetup.Codex, URL: endpoint, OS: "linux", Getenv: xdg},
	} {
		setup, err := agentsetup.Build(request)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(setup.Snippet+setup.Command+strings.Join(setup.Notes, " "), "XDG_CONFIG_HOME") {
			t.Fatalf("%s %q: XDG_CONFIG_HOME where routerctl does not read it: %+v", request.OS, request.URL, setup)
		}
	}
}

// PowerShell expands $ and backticks inside double quotes and cmd.exe expands %; no single quoting suits both,
// so such a path gets no command.
func TestWindowsCommandsWithExpandingCharacters(t *testing.T) {
	t.Parallel()
	for _, command := range []string{`C:\Users\a$b\routerctl.exe`, "C:\\Users\\a`b\\routerctl.exe", `C:\Users\100%\routerctl.exe`} {
		for _, agent := range []string{agentsetup.ClaudeCode, agentsetup.Codex, agentsetup.Pi} {
			setup := build(t, agent, "", "windows", command)
			if setup.Command != "" || !strings.Contains(strings.Join(setup.Notes, "\n"), "PowerShell") {
				t.Fatalf("%s %s: command = %q, notes = %v", agent, command, setup.Command, setup.Notes)
			}
		}
	}
	if setup := build(t, agentsetup.Codex, "", "windows", windowsCommand); setup.Command == "" {
		t.Fatal("a path with spaces lost its command")
	}
	if setup := build(t, agentsetup.Codex, "http://127.0.0.1:18731/mcp?x=%41", "windows", windowsCommand); setup.Command != "" {
		t.Fatalf("a URL with %% kept its command: %s", setup.Command)
	}
}

func skillFS() fstest.MapFS {
	return fstest.MapFS{
		"SKILL.md":            {Data: []byte("---\nname: router-tools\n---\nbody\n")},
		"references/tools.md": {Data: []byte("tools\n")},
	}
}

func TestInstallSkill(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "skills")
	files := []string{"SKILL.md", "references/tools.md"}

	result, err := agentsetup.InstallSkill(skillFS(), files, root, "router-tools", false)
	if err != nil || result.Action != "installed" || result.Path != filepath.Join(root, "router-tools") {
		t.Fatalf("first install = %+v, %v", result, err)
	}
	data, err := os.ReadFile(filepath.Join(root, "router-tools", "references", "tools.md"))
	if err != nil || string(data) != "tools\n" {
		t.Fatalf("installed file = %q, %v", data, err)
	}
	if again, err := agentsetup.InstallSkill(skillFS(), files, root, "router-tools", false); err != nil || again.Action != "unchanged" {
		t.Fatalf("second install = %+v, %v", again, err)
	}

	edited := filepath.Join(root, "router-tools", "SKILL.md")
	if err := os.WriteFile(edited, []byte("edited by the user\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	extra := filepath.Join(root, "router-tools", "notes.md")
	if err := os.WriteFile(extra, []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := agentsetup.InstallSkill(skillFS(), files, root, "router-tools", false); err == nil || !strings.Contains(err.Error(), "--force") || !errors.Is(err, agentsetup.ErrRefused) || !errors.Is(err, agentsetup.ErrDiffers) {
		t.Fatalf("a changed skill was overwritten without force: %v", err)
	}
	if data, _ := os.ReadFile(edited); string(data) != "edited by the user\n" {
		t.Fatal("the refused install changed the user's file")
	}
	forced, err := agentsetup.InstallSkill(skillFS(), files, root, "router-tools", true)
	if err != nil || forced.Action != "updated" {
		t.Fatalf("forced install = %+v, %v", forced, err)
	}
	if data, _ := os.ReadFile(edited); !strings.HasPrefix(string(data), "---\nname: router-tools") {
		t.Fatal("force did not restore SKILL.md")
	}
	if data, _ := os.ReadFile(extra); string(data) != "mine\n" {
		t.Fatal("force removed a file the skill does not ship")
	}
}

func TestInstallSkillRefusesSymlinksAndFiles(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	root := t.TempDir()
	elsewhere := t.TempDir()
	if err := os.Symlink(elsewhere, filepath.Join(root, "router-tools")); err != nil {
		t.Fatal(err)
	}
	if _, err := agentsetup.InstallSkill(skillFS(), []string{"SKILL.md"}, root, "router-tools", true); !errors.Is(err, agentsetup.ErrRefused) || errors.Is(err, agentsetup.ErrDiffers) {
		t.Fatalf("installed through a symbolic link: %v", err)
	}
	fileRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(fileRoot, "router-tools"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := agentsetup.InstallSkill(skillFS(), []string{"SKILL.md"}, fileRoot, "router-tools", true); !errors.Is(err, agentsetup.ErrRefused) {
		t.Fatalf("installed over a regular file: %v", err)
	}
	if entries, _ := os.ReadDir(elsewhere); len(entries) != 0 {
		t.Fatal("wrote through the symbolic link")
	}

	// A symbolic link inside an installed skill is refused too, even with force.
	nested := t.TempDir()
	if _, err := agentsetup.InstallSkill(skillFS(), []string{"SKILL.md", "references/tools.md"}, nested, "router-tools", false); err != nil {
		t.Fatal(err)
	}
	references := filepath.Join(nested, "router-tools", "references")
	if err := os.RemoveAll(references); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, references); err != nil {
		t.Fatal(err)
	}
	if _, err := agentsetup.InstallSkill(skillFS(), []string{"SKILL.md", "references/tools.md"}, nested, "router-tools", true); !errors.Is(err, agentsetup.ErrRefused) {
		t.Fatalf("force wrote through a symbolic link inside the skill: %v", err)
	}
	// A file where the skills directory should be is an ordinary file-system error, not a refusal.
	blocked := filepath.Join(t.TempDir(), "skills")
	if err := os.WriteFile(blocked, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := agentsetup.InstallSkill(skillFS(), []string{"SKILL.md"}, filepath.Join(blocked, "inner"), "router-tools", false); err == nil || errors.Is(err, agentsetup.ErrRefused) {
		t.Fatalf("file-system error = %v", err)
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Fatal("wrote through the symbolic link inside the skill")
	}
}

// The desktop shows notes in Simplified Chinese: every note of every setup has a Chinese text, and the
// English and Chinese setups carry the same notes otherwise.
func TestChineseNotes(t *testing.T) {
	t.Parallel()
	hasHan := func(text string) bool {
		for _, r := range text {
			if unicode.Is(unicode.Han, r) {
				return true
			}
		}
		return false
	}
	env := envOf(map[string]string{"CLAUDE_CONFIG_DIR": "/work/claude", "XDG_CONFIG_HOME": "/home/u/.xdg", "HOME": "/home/u"})
	for _, agent := range agentsetup.Agents() {
		for _, url := range []string{"", endpoint} {
			if url != "" && !agent.HTTP {
				continue
			}
			for _, goos := range []string{"linux", "windows"} {
				command := posixCommand
				if goos == "windows" {
					command = `C:\Users\a$b\routerctl.exe`
				}
				english, err := agentsetup.Build(agentsetup.Request{Agent: agent.ID, Command: command, URL: url, OS: goos, Getenv: env})
				if err != nil {
					t.Fatal(err)
				}
				chinese, err := agentsetup.Build(agentsetup.Request{Agent: agent.ID, Command: command, URL: url, OS: goos, Getenv: env, Lang: "zh"})
				if err != nil {
					t.Fatal(err)
				}
				if len(chinese.Notes) != len(english.Notes) || len(chinese.Notes) == 0 || chinese.Snippet != english.Snippet || chinese.Command != english.Command {
					t.Fatalf("%s %q %s: english %q, chinese %q", agent.ID, url, goos, english.Notes, chinese.Notes)
				}
				for _, note := range chinese.Notes {
					if !hasHan(note) {
						t.Fatalf("%s %q %s: an English note in the Chinese setup: %q", agent.ID, url, goos, note)
					}
				}
			}
		}
	}
	desktop := build(t, agentsetup.Codex, endpoint, "linux", posixCommand)
	mustContain(t, "english token note", strings.Join(desktop.Notes, "\n"), "routerctl mcp token")
}
