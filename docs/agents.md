# Agent hosts

An AI agent uses Router Tools through two pieces that ship in `routerctl`:

- the MCP server `routerctl mcp` (stdio or Streamable HTTP), which offers the tools listed in
  [usage.md](usage.md#agent-access-mcp);
- the `router-tools` skill, which tells the agent how to use them: plan first, get the user's approval, run
  the plan once, follow long operations, stop on `partial` or `unknown`.

Read the "Agent access" section of [RISKS.md](../RISKS.md) first. The binding rules are §23 of
[rd08-protocol-spec.md](rd08-protocol-spec.md).

## Desktop app

The desktop app serves the same tools over Streamable HTTP, so an agent that speaks HTTP needs no `routerctl`
binary: open 「Agent 接入」, turn on 允许 Agent 连接 and apply. The page shows the access token once (set it as
`ROUTERCTL_MCP_TOKEN` where the agent starts), the endpoint, each HTTP host's snippet for that endpoint, and
installs the skill. WorkBuddy documents only stdio servers, so the page gives no snippet for it: install
`routerctl` and run `routerctl mcp config --agent workbuddy`. Writes stay off until you turn on 允许 Agent
写入路由器 there. The settings persist across restarts; the service runs only while the app runs, and quitting
cancels agent operations. An unknown SSH host key in an agent's run opens the app's own confirmation dialog, so
you see the fingerprint before accepting it, unless the agent passed `accept_new_host_key` after asking you in
chat.

## Quick start

```bash
routerctl skill install --agent codex        # the skill, into ~/.agents/skills/router-tools
routerctl mcp config --agent codex           # stdio: prints the snippet, its file and the host's add command
routerctl mcp config --agent codex --http http://127.0.0.1:18731/mcp   # Streamable HTTP
```

- `routerctl mcp config` writes nothing. It prints the snippet with the absolute path of the `routerctl` that
  ran it; run it again after moving `routerctl` or installing it somewhere else. `--json` prints
  `agent`, `transport`, `file`, `format`, `snippet`, `command` and `notes`.
- Router writes stay off until the user turns them on. Over stdio, add `--allow-writes` after `mcp` in the
  host's server entry (for example `args = ["mcp", "--allow-writes"]`). Over HTTP, start the server with
  `routerctl mcp --http <address> --allow-writes`. Without it the agent can detect, plan, back up and check,
  and `guide_apply` and `soft_remove` are not offered.
- Over HTTP, create a token with `routerctl mcp token`, start the server with `ROUTERCTL_MCP_TOKEN` set (or
  `--token-file`), and set the same variable in the environment that starts the agent host. Every snippet
  except the generic one reads the token from `ROUTERCTL_MCP_TOKEN`; none contains the token itself.
- `routerctl skill install` leaves an identical copy alone and refuses to replace a changed copy without
  `--force`. `--force` rewrites only the files the skill ships. `routerctl skill show` prints `SKILL.md`.

## Hosts

| `--agent`     | Host                   | stdio | HTTP | Server configuration (user scope)  | Skill directory                |
| ------------- | ---------------------- | ----- | ---- | ---------------------------------- | ------------------------------ |
| `claude-code` | Claude Code            | yes   | yes  | `~/.claude.json`                   | `~/.claude/skills`             |
| `codex`       | Codex CLI              | yes   | yes  | `~/.codex/config.toml`             | `~/.agents/skills`             |
| `opencode`    | opencode 1.x and 2.x   | yes   | yes  | `~/.config/opencode/opencode.json` | `~/.agents/skills`             |
| `pi`          | pi                     | yes   | yes  | `~/.pi/agent/mcp.json`             | `~/.agents/skills`             |
| `dsh`         | DeepSeek Harness (dsh) | yes   | yes  | `~/.dsh/cordis.patch.yml`          | `~/.agents/skills`             |
| `workbuddy`   | WorkBuddy              | yes   | no   | `~/.workbuddy/mcp.json`            | none documented; use `--dir`   |
| `generic`     | other MCP clients      | yes   | yes  | depends on the client              | depends on the client; `--dir` |

Each host section ends with what was run against it on 2026-11-26, in an isolated home directory.
"Connected" means the host's own list command started `routerctl mcp` (or reached it over HTTP) and
initialized a session. None of it covers router operations, which only the fake-server tests and
[compatibility.md](compatibility.md) cover.

### Directories that move

Some hosts move their configuration or skills when a variable is set. When these are set in the shell that
runs `routerctl`, `routerctl mcp config` prints the moved file and `routerctl skill install --agent` installs
into the moved skills directory; set them the same way for the host.

| Variable              | Host        | Moves                                                                                                    |
| --------------------- | ----------- | -------------------------------------------------------------------------------------------------------- |
| `CLAUDE_CONFIG_DIR`   | Claude Code | `~/.claude.json` and `~/.claude/skills` into that directory                                              |
| `CODEX_HOME`          | Codex CLI   | `~/.codex/config.toml`                                                                                   |
| `OPENCODE_CONFIG_DIR` | opencode    | `~/.config/opencode/opencode.json`, as `$OPENCODE_CONFIG_DIR/opencode.json`; wins over `XDG_CONFIG_HOME` |
| `XDG_CONFIG_HOME`     | opencode    | `~/.config/opencode/opencode.json` (opencode's source reads it on every platform)                        |
| `PI_CODING_AGENT_DIR` | pi          | `~/.pi/agent/mcp.json`                                                                                   |
| `DSH_HOME`            | dsh         | `~/.dsh/cordis.patch.yml`                                                                                |
| `DSH_AGENTS_HOME`     | dsh         | `~/.agents/skills`, as `$DSH_AGENTS_HOME/skills`                                                         |

## The first SSH enable

When `guide_plan` plans `enable_ssh`, the run logs in over SSH as root, and this computer may not know the
router's SSH host key yet (it never does for a router whose SSH was off and never used from here).
`routerctl mcp` has no prompt to show the key to you. The skill and the server's instructions tell the agent to
ask you whether to trust it and then to pass `accept_new_host_key: true` to `guide_apply`. Without it an
unknown key stops the run: before any write when SSH was already on, or, when SSH was off, after turning it
on, which ends `partial` and blocks the router until a person runs `routerctl --host <address> recovery ack`.
A key this computer already knows needs no flag, and a changed key (`HOST_KEY_MISMATCH`) is refused either
way. The CLI and the desktop app show the fingerprint and ask instead.

## Timeouts

Every tool call returns within `wait_seconds` (default 20 s, at most 50 s). Work that takes longer, such as a
backup or a guided run with a reboot, continues on the server, and `operation_wait` collects it. The hosts'
per-call limits are 60 s or more by default (Claude Code over HTTP about 60 s to the first response byte,
Codex 60 s in its documentation, pi 60 s reset by progress notifications, dsh 60 s, opencode 2.x 12 hours),
so no timeout needs raising. Do not set one below 60 s.

## Approvals and replays

- The write switch records the user's consent; it is not a sandbox. An agent that also has a shell can run
  `routerctl` itself or edit its host's configuration. Limiting that is the host's permission system's job.
- Tools carry MCP annotations by class (spec §23.2). Tools that touch only local state are
  `readOnlyHint: true`; tools that contact the router are `openWorldHint: true`; router writes and
  `operation_cancel` are `destructiveHint: true` and never read-only or idempotent. Claude Code replays only
  read-only or idempotent calls after a dropped connection, so it never replays a write.
- Codex re-sends a call once when its HTTP session has expired, whatever the annotations. A replayed
  `guide_apply` gets `CONFIRMATION_REQUIRED`, because a plan works once; a replayed `soft_remove` finds
  nothing installed and writes nothing.
- Whether a host asks before a call depends on the host, as the sections below say. pi has no built-in
  per-call approval, and dsh documents none for MCP tools.

## Claude Code

```bash
routerctl mcp config --agent claude-code                  # claude mcp add --transport stdio --scope user ...
routerctl mcp config --agent claude-code --http http://127.0.0.1:18731/mcp   # claude mcp add-json ...
```

- The HTTP entry sends `Authorization: Bearer ${ROUTERCTL_MCP_TOKEN}`; Claude Code expands the variable.
  When it is missing, `claude mcp list` warns `Missing environment variables: ROUTERCTL_MCP_TOKEN`.
- On Windows the HTTP command is not printed, because the JSON argument needs shell-specific quoting; add the
  entry to `%USERPROFILE%\.claude.json` instead.
- Whether a call asks first depends on the permission mode. For a prompt before every router write, add
  `permissions.ask` rules for `mcp__router-tools__guide_apply`, `mcp__router-tools__soft_remove` and
  `mcp__router-tools__operation_cancel`; an `ask` rule is never approved automatically.
- When a session ends, Claude Code stops a stdio server within about half a second. A write cut off that way
  leaves the router blocked until `routerctl --host <address> recovery ack`; run long writes over HTTP, or do not end the
  session while one runs.
- Claude Code passes its whole environment to stdio servers (checked with 2.1.332).
- Run with Claude Code 2.1.332: `claude mcp add` and `claude mcp add-json` with the printed commands, then
  `claude mcp list` connected (`✔ Connected`) over stdio and HTTP.

## Codex CLI

```bash
routerctl mcp config --agent codex    # [mcp_servers.router-tools] for ~/.codex/config.toml, or codex mcp add
```

- Codex passes only a few environment variables to stdio servers, and `XDG_CONFIG_HOME` is not among them. On
  Linux and other systems where routerctl's configuration directory follows that variable (not macOS or
  Windows), `routerctl mcp config` therefore adds it to the entry when it names a directory other than
  `~/.config`, in the form
  `codex mcp add --env` writes (`[mcp_servers.router-tools.env]`). Without it the agent's `routerctl` keeps
  its own locks, recovery records, known hosts and saved passwords under `~/.config/routerctl`: a recovery
  record left by your own runs would not block the agent, the agent's would not block you, and both could
  change the router at the same time. Run `mcp config` again after changing the variable.
- The HTTP entry uses `bearer_token_env_var = "ROUTERCTL_MCP_TOKEN"`.
- Approvals follow `default_tools_approval_mode` and per-tool `approval_mode`. In the default `auto` mode
  Codex calls `list_profiles`, `saved_passwords`, `operation_wait` and `operation_list` without asking and,
  under interactive approval policies, asks before every tool that contacts the router or cancels an
  operation. With `approval_policy = "never"` it approves those calls when it has full disk access and
  refuses them otherwise; `codex exec` cancels them.
- Run with Codex CLI 0.171.0: `codex mcp add` with the printed commands wrote exactly the snippet (stdio, stdio
  with `XDG_CONFIG_HOME`, and HTTP), and `codex mcp list` listed it. Codex has no command that starts a server
  outside a session, so the connection itself was not exercised.

## opencode

```bash
routerctl mcp config --agent opencode    # entry for ~/.config/opencode/opencode.json
```

- The snippet places the server directly under `mcp`. opencode 1.x requires that form; opencode 2.x documents
  `mcp.servers` but still reads the 1.x form ("The V1 config format remains supported", Migrate from V1).
  opencode 1.x rejects `mcp.servers` (`Configuration is invalid ... mcp.servers`), and the
  `opencode mcp add --global` of 2.x writes that form, so no add command is printed. An existing
  `opencode.jsonc` in the same directory is read as well.
- The HTTP entry sets `"oauth": false` and sends `Bearer {env:ROUTERCTL_MCP_TOKEN}`.
- Leave `timeout` unset: a numeric timeout also limits tool calls.
- opencode runs MCP tools without asking by default. For a prompt before writes in opencode 2.x, add
  `{ "action": "router-tools_guide_apply", "resource": "*", "effect": "ask" }` (and the same for
  `router-tools_soft_remove` and `router-tools_operation_cancel`) to `permissions`.
- opencode passes its whole environment to stdio servers (checked with 1.18.18).
- Run with opencode 1.18.18: `opencode mcp list` connected (`✓ router-tools connected`) over stdio and over
  HTTP with the token, and `401` without it; with `OPENCODE_CONFIG_DIR` set it loaded the server from
  `$OPENCODE_CONFIG_DIR/opencode.json`. opencode 2.4.2 read the file (`opencode debug config`), but in
  the same headless test its `opencode mcp list` listed no servers at all, including an entry written by its
  own `opencode mcp add`; the 2.x connection is therefore not verified here.

## pi

```bash
routerctl mcp config --agent pi    # entry for ~/.pi/agent/mcp.json, or pi mcp add
```

- The entry sets `"exposure": "direct"`. pi's default code mode hides MCP tools from the model.
- The HTTP command passes `--bearer-token-env-var ROUTERCTL_MCP_TOKEN`.
- pi passes its whole environment to stdio servers (checked with 1.6.0).
- Run with pi 1.6.0: `pi mcp add` with the printed commands, then `pi mcp list --json` reported both
  servers `connected` with their tools.

## DeepSeek Harness (dsh)

```bash
routerctl mcp config --agent dsh    # an insert for ~/.dsh/cordis.patch.yml
```

- The snippet is one `@deepseek-ai/dsh-mcp-client` insert. Merge it into the existing patch file, or save it
  separately and start dsh with `--patch <file>`.
- The HTTP entry reads the token with the YAML expression
  ``!!js '`Bearer ${process.env.ROUTERCTL_MCP_TOKEN}`'``.
- `toolCallTimeoutMs` defaults to 60000, which covers the longest call.
- dsh removes variables whose names contain `KEY`, `PASSWORD`, `SECRET` or `TOKEN`, and every `DSH_*` variable,
  from a stdio server's environment (source); routerctl needs none of them over stdio.
- Not run locally.

## WorkBuddy

```bash
routerctl mcp config --agent workbuddy    # stdio entry for ~/.workbuddy/mcp.json
routerctl skill install --dir ~/router-tools-skill
```

- Add the server in 专家·技能·连接器 → 连接器 → 自定义连接器, or edit `~/.workbuddy/mcp.json`.
- stdio only: WorkBuddy's MCP guide shows only stdio servers. Its CodeBuddy engine documents `"type": "http"`
  entries, but WorkBuddy documents neither header handling in its own `mcp.json` nor whether the desktop
  process sees variables exported in a shell.
- WorkBuddy documents no skill directory, only an upload in 专家·技能·连接器 → 技能 → 添加技能 → 上传技能.
  Install the skill into a folder with `--dir` and upload `<folder>/router-tools` from there; the accepted
  package format is not documented.
- Not run locally.

## Other MCP clients

```bash
routerctl mcp config --agent generic
routerctl mcp config --agent generic --http http://192.168.31.10:18731/mcp
```

- stdio: the command is the absolute path of `routerctl`, the argument `mcp` (and `--allow-writes` for
  writes). The client must keep stdin open for the session; closing it stops the server.
- Streamable HTTP: the URL ends in `/mcp`; every request carries `Authorization: Bearer <token>`. Some
  clients call this transport `streamable-http`.
- The client should allow at least 60 s per call and must not replay write tools on its own.
- Install the skill with `routerctl skill install --dir <the client's skills directory>`, or paste the output
  of `routerctl skill show` where the client takes instructions.

## Windows

- `~` means `%USERPROFILE%`: `%USERPROFILE%\.claude.json`, `%USERPROFILE%\.codex\config.toml`,
  `%USERPROFILE%\.config\opencode\opencode.json`, `%USERPROFILE%\.pi\agent\mcp.json`,
  `%USERPROFILE%\.dsh\cordis.patch.yml`. WorkBuddy documents only `~/.workbuddy/mcp.json`.
- Snippets escape backslashes. Printed commands put paths and URLs in double quotes, which PowerShell and
  cmd.exe both keep literal, except for `$` and a backtick (PowerShell) and `%` (cmd.exe): when a path or URL
  contains one of those, no command is printed; add the snippet to the file instead.
- A server on Windows that listens on `127.0.0.1` is not reachable from WSL with its default NAT networking.
  Start it on an address WSL can reach and connect to that address, or switch WSL to mirrored networking.

## Troubleshooting

| Symptom                                                      | Cause and fix                                                                                                                                                        |
| ------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| HTTP `401`                                                   | The host did not send the token or sent another one. Set `ROUTERCTL_MCP_TOKEN` where the host starts; a desktop app may not see variables exported in a shell.       |
| No `guide_apply` or `soft_remove`, `AGENT_WRITES_DISABLED`   | Writes are off. Add `--allow-writes` as described in the quick start.                                                                                                |
| pi lists the server but the model sees no tools              | Add `"exposure": "direct"`.                                                                                                                                          |
| opencode: `Configuration is invalid ... mcp.servers`         | opencode 1.x. Use the snippet from `routerctl mcp config --agent opencode`.                                                                                          |
| Under Codex, saved passwords or recovery records are missing | Linux with `XDG_CONFIG_HOME` set: Codex does not pass it. Run `routerctl mcp config --agent codex` with the variable set and use that entry.                         |
| `RECOVERY_REVIEW_REQUIRED`                                   | A write did not finish cleanly (often a session ended during it). A person checks the router and runs `routerctl --host <address> recovery ack`.                     |
| An agent's first SSH enable ends `partial`, SSH is on        | The agent did not pass `accept_new_host_key: true`, so root could not be verified. Check the router, run `routerctl --host <address> recovery ack`, then plan again. |
| `OPERATION_LOCKED`                                           | Another operation uses this router, or four operations run. Wait, then try once more.                                                                                |

## Sources

Read on 2026-11-26.

- Claude Code: <https://code.claude.com/docs/en/mcp>, <https://code.claude.com/docs/en/permissions>,
  <https://code.claude.com/docs/en/skills>
- Codex CLI: <https://learn.chatgpt.com/docs/extend/mcp>, <https://learn.chatgpt.com/docs/config-file/config-reference>,
  <https://learn.chatgpt.com/docs/build-skills>, <https://github.com/openai/codex> (tag `rust-v0.172.0`)
- opencode: <https://opencode.ai/docs/mcp-servers>, <https://opencode.ai/docs/migrate-v1>,
  <https://opencode.ai/v1/docs/mcp-servers>, <https://opencode.ai/docs/permissions>,
  <https://opencode.ai/v1/docs/skills>
- pi: <https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/mcp.md>,
  <https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/skills.md>
- DeepSeek Harness: <https://github.com/deepseek-ai/deepseek-harness/blob/master/packages/mcp/mcp-client/README.md>,
  <https://github.com/deepseek-ai/deepseek-harness/blob/master/packages/skill/skill-filesystem/README.md>
- WorkBuddy: <https://www.workbuddy.cn/docs/workbuddy/From-Beginner-to-Expert-Guide/Function-Description/MCP-Guide>,
  <https://www.workbuddy.cn/docs/workbuddy/From-Beginner-to-Expert-Guide/Function-Description/Connector>
- WSL networking: <https://learn.microsoft.com/windows/wsl/networking>
