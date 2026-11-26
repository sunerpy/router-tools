// Package agentsetup describes how each supported agent host connects to the Router Tools MCP server and
// where it reads skills. The formats follow each host's own documentation (docs/agents.md cites them).
// It only produces text and installs the embedded skill; it never edits another program's configuration.
package agentsetup

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

// Agent IDs.
const (
	ClaudeCode = "claude-code"
	Codex      = "codex"
	OpenCode   = "opencode"
	Pi         = "pi"
	DSH        = "dsh"
	WorkBuddy  = "workbuddy"
	Generic    = "generic"
)

const (
	serverName = "router-tools"
	tokenEnv   = "ROUTERCTL_MCP_TOKEN"
)

// Agent is one host Router Tools documents.
type Agent struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// HTTP reports whether a Streamable HTTP setup is offered; WorkBuddy documents stdio only.
	HTTP bool `json:"http"`
	// SkillDir is the user skills directory relative to the home directory; empty when none is verified.
	SkillDir string `json:"skill_dir,omitempty"`
	// Docs is the host's MCP documentation.
	Docs string `json:"docs,omitempty"`
}

var agents = []Agent{
	{ID: ClaudeCode, Name: "Claude Code", HTTP: true, SkillDir: filepath.Join(".claude", "skills"), Docs: "https://code.claude.com/docs/en/mcp"},
	{ID: Codex, Name: "Codex CLI", HTTP: true, SkillDir: filepath.Join(".agents", "skills"), Docs: "https://learn.chatgpt.com/docs/extend/mcp"},
	{ID: OpenCode, Name: "opencode", HTTP: true, SkillDir: filepath.Join(".agents", "skills"), Docs: "https://opencode.ai/docs/mcp-servers"},
	{ID: Pi, Name: "pi", HTTP: true, SkillDir: filepath.Join(".agents", "skills"), Docs: "https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/mcp.md"},
	{ID: DSH, Name: "DeepSeek Harness", HTTP: true, SkillDir: filepath.Join(".agents", "skills"), Docs: "https://github.com/deepseek-ai/deepseek-harness/blob/master/packages/mcp/mcp-client/README.md"},
	{ID: WorkBuddy, Name: "WorkBuddy", Docs: "https://www.workbuddy.cn/docs/workbuddy/From-Beginner-to-Expert-Guide/Function-Description/MCP-Guide"},
	{ID: Generic, Name: "Other MCP clients", HTTP: true},
}

// Agents lists the documented hosts in a stable order.
func Agents() []Agent {
	return append([]Agent(nil), agents...)
}

// Lookup finds an agent by ID.
func Lookup(id string) (Agent, bool) {
	for _, agent := range agents {
		if agent.ID == id {
			return agent, true
		}
	}
	return Agent{}, false
}

// IDs returns the agent IDs, for help texts.
func IDs() string {
	ids := make([]string, 0, len(agents))
	for _, agent := range agents {
		ids = append(ids, agent.ID)
	}
	return strings.Join(ids, ", ")
}

// Request selects one setup.
type Request struct {
	Agent string
	// Command is the absolute path of routerctl, used for stdio.
	Command string
	// URL is the Streamable HTTP endpoint; empty selects stdio. A URL without a path gets /mcp.
	URL string
	// OS is the GOOS of the computer that runs the host: it selects path separators, shell quoting, and whether
	// routerctl's configuration directory follows XDG_CONFIG_HOME.
	OS string
	// Getenv reads the environment the host starts with, for the documented variables that move a host's
	// configuration (CLAUDE_CONFIG_DIR, CODEX_HOME, OPENCODE_CONFIG_DIR, XDG_CONFIG_HOME, PI_CODING_AGENT_DIR,
	// DSH_HOME) and for HOME. Nil reads nothing.
	Getenv func(string) string
}

// Setup is what a user adds to one host.
type Setup struct {
	Agent     string `json:"agent"`
	Transport string `json:"transport"`
	// File is where the snippet goes; ~ is the home directory. A documented override variable replaces the
	// default directory with its value.
	File    string `json:"file,omitempty"`
	Format  string `json:"format"`
	Snippet string `json:"snippet"`
	// Command adds the same configuration from a shell, when the host has such a command.
	Command string   `json:"command,omitempty"`
	Notes   []string `json:"notes,omitempty"`
}

// Build returns the setup for one host and transport.
func Build(request Request) (Setup, error) {
	agent, ok := Lookup(request.Agent)
	if !ok {
		return Setup{}, fmt.Errorf("unknown agent %q; use one of %s", request.Agent, IDs())
	}
	env := newPlatform(request)
	var setup Setup
	if request.URL == "" {
		if request.Command == "" {
			return Setup{}, errors.New("the routerctl path is required for a stdio setup")
		}
		setup = stdioSetup(agent.ID, request.Command, env)
		setup.Transport = "stdio"
	} else {
		if !agent.HTTP {
			return Setup{}, fmt.Errorf("%s documents only stdio MCP servers; use routerctl mcp config --agent %s without --http", agent.Name, agent.ID)
		}
		endpoint, err := normalizeURL(request.URL)
		if err != nil {
			return Setup{}, err
		}
		setup = httpSetup(agent.ID, endpoint, env)
		setup.Transport = "http"
		setup.Notes = append(setup.Notes, "Every request carries Authorization: Bearer <token>; create a token with routerctl mcp token.")
	}
	setup.Agent = agent.ID
	for _, override := range configOverrides[agent.ID] {
		if dir := env.getenv(override.variable); dir != "" {
			setup.File = env.join(dir, override.file...)
			setup.Notes = append(setup.Notes, override.variable+" is set, so "+agent.Name+" reads "+setup.File+".")
			break
		}
	}
	if env.quote.unsafe && setup.Command != "" {
		setup.Command = ""
		setup.Notes = append(setup.Notes, "No command is printed: the path or URL contains $, ` or %, which PowerShell or cmd.exe would change. Add the snippet to the file instead.")
	}
	if env.windows {
		setup.Notes = append(setup.Notes, "~ means %USERPROFILE%.")
	}
	return setup, nil
}

// configOverride is a variable that moves a host's user configuration directory, with the configuration file
// below that directory.
type configOverride struct {
	variable string
	file     []string
}

// configOverrides lists each host's override variables; the first one set wins. opencode 2.x takes
// OPENCODE_CONFIG_DIR as its global configuration directory and 1.x loads it after the global one, so it comes
// before XDG_CONFIG_HOME, which both versions read for the default directory.
var configOverrides = map[string][]configOverride{
	ClaudeCode: {{"CLAUDE_CONFIG_DIR", []string{".claude.json"}}},
	Codex:      {{"CODEX_HOME", []string{"config.toml"}}},
	OpenCode:   {{"OPENCODE_CONFIG_DIR", []string{"opencode.json"}}, {"XDG_CONFIG_HOME", []string{"opencode", "opencode.json"}}},
	Pi:         {{"PI_CODING_AGENT_DIR", []string{"mcp.json"}}},
	DSH:        {{"DSH_HOME", []string{"cordis.patch.yml"}}},
}

// platform is the computer that runs the host.
type platform struct {
	windows bool
	// xdg reports whether routerctl's configuration directory (os.UserConfigDir) follows XDG_CONFIG_HOME there.
	xdg    bool
	getenv func(string) string
	quote  *quoter
}

func newPlatform(request Request) platform {
	getenv := request.Getenv
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	windows := request.OS == "windows"
	xdg := true
	switch request.OS {
	case "windows", "darwin", "ios", "plan9":
		xdg = false
	}
	return platform{windows: windows, xdg: xdg, getenv: getenv, quote: &quoter{windows: windows}}
}

// defaultConfigHome reports whether an XDG_CONFIG_HOME value names the directory used without it, $HOME/.config.
func (p platform) defaultConfigHome(xdg string) bool {
	home := strings.TrimRight(p.getenv("HOME"), "/")
	return home != "" && strings.TrimRight(xdg, "/") == home+"/.config"
}

// join appends path elements to dir with the platform's separator.
func (p platform) join(dir string, elements ...string) string {
	separator, trim := "/", "/"
	if p.windows {
		separator, trim = `\`, `\/`
	}
	if trimmed := strings.TrimRight(dir, trim); trimmed != "" {
		dir = trimmed
	}
	return dir + separator + strings.Join(elements, separator)
}

func normalizeURL(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", errors.New("the MCP URL must look like http://127.0.0.1:18731/mcp")
	}
	if parsed.Path == "" || parsed.Path == "/" {
		parsed.Path = "/mcp"
	}
	return parsed.String(), nil
}

// openCodeNotes explains the mcp.<name> form: opencode 2.x still reads it, while opencode 1.x rejects the
// mcp.servers form that 2.x documents and that its own opencode mcp add writes.
var openCodeNotes = []string{
	"opencode 1.x and 2.x both read this form; 1.x rejects the mcp.servers form that 2.x documents.",
	"Leave timeout unset: a numeric timeout also limits tool calls.",
}

const writeApprovalNote = "For a prompt before every router write, add permissions.ask rules for mcp__router-tools__guide_apply, mcp__router-tools__soft_remove and mcp__router-tools__operation_cancel."

func stdioSetup(agent, command string, env platform) Setup {
	quoted := env.quote.quote(command)
	switch agent {
	case ClaudeCode:
		return Setup{
			File: "~/.claude.json", Format: "json",
			Snippet: pretty(object{{"mcpServers", object{{serverName, object{{"type", "stdio"}, {"command", command}, {"args", []string{"mcp"}}}}}}}),
			Command: "claude mcp add --transport stdio --scope user " + serverName + " -- " + quoted + " mcp",
			Notes:   []string{"Run the command, or add the entry under mcpServers in that file (all projects) or in .mcp.json at a project root.", writeApprovalNote},
		}
	case Codex:
		setup := Setup{
			File: "~/.codex/config.toml", Format: "toml",
			Snippet: "[mcp_servers." + serverName + "]\ncommand = " + tomlString(command) + "\nargs = [\"mcp\"]\n",
			Command: "codex mcp add " + serverName + " -- " + quoted + " mcp",
		}
		// Codex passes only a few variables to stdio servers (not XDG_CONFIG_HOME), so the entry sets it the way
		// codex mcp add --env writes it.
		if xdg := env.getenv("XDG_CONFIG_HOME"); env.xdg && strings.HasPrefix(xdg, "/") && !env.defaultConfigHome(xdg) {
			setup.Snippet += "\n[mcp_servers." + serverName + ".env]\nXDG_CONFIG_HOME = " + tomlString(xdg) + "\n"
			setup.Command = "codex mcp add " + serverName + " --env " + env.quote.quote("XDG_CONFIG_HOME="+xdg) + " -- " + quoted + " mcp"
			setup.Notes = append(setup.Notes, "XDG_CONFIG_HOME is set and Codex does not pass it to stdio servers, so the entry sets it. Without it routerctl would keep its locks, recovery records, known hosts and saved passwords under ~/.config/routerctl, apart from the routerctl you run yourself.")
		}
		return setup
	case OpenCode:
		return Setup{
			File: "~/.config/opencode/opencode.json", Format: "json",
			Snippet: pretty(object{{"$schema", "https://opencode.ai/config.json"}, {"mcp", object{{serverName, object{{"type", "local"}, {"command", []string{command, "mcp"}}}}}}}),
			Notes:   append([]string(nil), openCodeNotes...),
		}
	case Pi:
		return Setup{
			File: "~/.pi/agent/mcp.json", Format: "json",
			Snippet: pretty(object{{"mcpServers", object{{serverName, object{{"command", command}, {"args", []string{"mcp"}}, {"exposure", "direct"}}}}}}),
			Command: "pi mcp add " + serverName + " --exposure direct -- " + quoted + " mcp",
			Notes:   []string{`"exposure": "direct" lets the model see the tools; pi's default codemode hides them.`},
		}
	case DSH:
		return Setup{
			File: "~/.dsh/cordis.patch.yml", Format: "yaml",
			Snippet: dshEntry("transport: stdio\n        command: " + yamlString(command) + "\n        args: [mcp]\n"),
			Notes:   []string{"Merge the insert into the existing file, or save it separately and start dsh with --patch <file>."},
		}
	case WorkBuddy:
		return Setup{
			File: "~/.workbuddy/mcp.json", Format: "json",
			Snippet: pretty(object{{"mcpServers", object{{serverName, object{{"command", command}, {"args", []string{"mcp"}}}}}}}),
			Notes:   []string{"Add it in 专家·技能·连接器 → 连接器 → 自定义连接器, or edit the file. WorkBuddy documents only stdio MCP servers."},
		}
	default:
		return Setup{
			Format:  "json",
			Snippet: pretty(object{{"mcpServers", object{{serverName, object{{"command", command}, {"args", []string{"mcp"}}}}}}}),
			Notes:   []string{"Most MCP clients accept this mcpServers entry; others ask for the command routerctl and the argument mcp."},
		}
	}
}

func httpSetup(agent, endpoint string, env platform) Setup {
	windows := env.windows
	switch agent {
	case ClaudeCode:
		entry := object{{"type", "http"}, {"url", endpoint}, {"headers", object{{"Authorization", "Bearer ${" + tokenEnv + "}"}}}}
		setup := Setup{
			File: "~/.claude.json", Format: "json",
			Snippet: pretty(object{{"mcpServers", object{{serverName, entry}}}}),
			Notes:   []string{"Set " + tokenEnv + " in the environment that starts Claude Code; Claude Code expands ${" + tokenEnv + "} in the header.", writeApprovalNote},
		}
		if !windows {
			setup.Command = "claude mcp add-json " + serverName + " " + env.quote.quote(compact(entry)) + " --scope user"
		}
		return setup
	case Codex:
		return Setup{
			File: "~/.codex/config.toml", Format: "toml",
			Snippet: "[mcp_servers." + serverName + "]\nurl = " + tomlString(endpoint) + "\nbearer_token_env_var = \"" + tokenEnv + "\"\n",
			Command: "codex mcp add " + serverName + " --url " + env.quote.quote(endpoint) + " --bearer-token-env-var " + tokenEnv,
			Notes:   []string{"Codex sends the value of " + tokenEnv + " as the bearer token."},
		}
	case OpenCode:
		return Setup{
			File: "~/.config/opencode/opencode.json", Format: "json",
			Snippet: pretty(object{{"$schema", "https://opencode.ai/config.json"}, {"mcp", object{{serverName, object{
				{"type", "remote"}, {"url", endpoint}, {"oauth", false}, {"headers", object{{"Authorization", "Bearer {env:" + tokenEnv + "}"}}},
			}}}}}),
			Notes: append([]string{"Edit the file: opencode mcp add cannot turn OAuth off."}, openCodeNotes...),
		}
	case Pi:
		return Setup{
			File: "~/.pi/agent/mcp.json", Format: "json",
			Snippet: pretty(object{{"mcpServers", object{{serverName, object{
				{"type", "http"}, {"url", endpoint}, {"headers", object{{"Authorization", "Bearer ${" + tokenEnv + "}"}}}, {"exposure", "direct"},
			}}}}}),
			Command: "pi mcp add " + serverName + " --url " + env.quote.quote(endpoint) + " --bearer-token-env-var " + tokenEnv + " --exposure direct",
			Notes:   []string{`"exposure": "direct" lets the model see the tools; pi's default codemode hides them.`},
		}
	case DSH:
		return Setup{
			File: "~/.dsh/cordis.patch.yml", Format: "yaml",
			Snippet: dshEntry("transport: streamable-http\n        url: " + yamlString(endpoint) + "\n        headers:\n          Authorization: !!js '`Bearer ${process.env." + tokenEnv + "}`'\n"),
			Notes:   []string{"The !!js expression reads " + tokenEnv + " when dsh loads its configuration.", "Merge the insert into the existing file, or save it separately and start dsh with --patch <file>."},
		}
	default:
		return Setup{
			Format: "json",
			Snippet: pretty(object{{"mcpServers", object{{serverName, object{
				{"type", "http"}, {"url", endpoint}, {"headers", object{{"Authorization", "Bearer <token>"}}},
			}}}}}),
			Notes: []string{"Replace <token> with the access token. Some clients call this type streamable-http."},
		}
	}
}

func dshEntry(config string) string {
	return "- insert:\n    - id: mcp-router-tools\n      name: '@deepseek-ai/dsh-mcp-client'\n      config:\n        serverName: " + serverName + "\n        " + config
}

// object is a JSON object that keeps its member order.
type object []member

type member struct {
	key   string
	value any
}

func (o object) MarshalJSON() ([]byte, error) {
	var buffer bytes.Buffer
	buffer.WriteByte('{')
	for index, item := range o {
		if index > 0 {
			buffer.WriteByte(',')
		}
		key, err := encode(item.key)
		if err != nil {
			return nil, err
		}
		value, err := encode(item.value)
		if err != nil {
			return nil, err
		}
		buffer.Write(key)
		buffer.WriteByte(':')
		buffer.Write(value)
	}
	buffer.WriteByte('}')
	return buffer.Bytes(), nil
}

func encode(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buffer.Bytes(), []byte("\n")), nil
}

func compact(value any) string {
	data, err := encode(value)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func pretty(value any) string {
	var buffer bytes.Buffer
	if err := json.Indent(&buffer, []byte(compact(value)), "", "  "); err != nil {
		panic(err)
	}
	return buffer.String() + "\n"
}

func tomlString(value string) string {
	var builder strings.Builder
	builder.WriteByte('"')
	for _, r := range value {
		switch {
		case r == '\\':
			builder.WriteString(`\\`)
		case r == '"':
			builder.WriteString(`\"`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&builder, `\u%04X`, r)
		default:
			builder.WriteRune(r)
		}
	}
	builder.WriteByte('"')
	return builder.String()
}

func yamlString(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

var plainArgument = regexp.MustCompile(`^[A-Za-z0-9_./:@%+=,-]+$`)

// quoter quotes command arguments for a POSIX shell, or with double quotes for Windows, where PowerShell and
// cmd.exe both keep them literal unless they contain $, ` (PowerShell) or % (cmd.exe); such a value marks the
// command unsafe.
type quoter struct {
	windows bool
	unsafe  bool
}

func (q *quoter) quote(value string) string {
	if q.windows && strings.ContainsAny(value, "$`%") {
		q.unsafe = true
	}
	if plainArgument.MatchString(value) {
		return value
	}
	if q.windows {
		return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
