<div align="center">

# router-tools

### Manage routers by exact model and firmware: a read-only check first, one confirmation, then only the missing steps, with a safety gate before every write

[![CI](https://github.com/sunerpy/router-tools/actions/workflows/ci.yml/badge.svg)](https://github.com/sunerpy/router-tools/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/sunerpy/router-tools)](https://github.com/sunerpy/router-tools/releases)
[![License](https://img.shields.io/badge/license-MIT-green)](../../LICENSE)

[Install](#install) · [Quick start](#quick-start) · [Usage](#usage) · [Using with an AI agent](#using-with-an-ai-agent) · [Safety boundaries](#safety-boundaries) · [Development](#development)

[简体中文](../../README.md) · [**English**](./README.en.md)

</div>

---

`routerctl` is a Go CLI; the same core drives the Wails desktop app (Router Tools). The first adapter targets **Xiaomi BE6500 Pro (RD08) firmware 1.1.96**.

> **Verification status:** protocol, state-machine, loopback SSH/Telnet, filesystem, and CLI tests run offline; this repository's tests have never contacted a real router, and no feature has passed real-device acceptance yet (see [docs/compatibility.md](../compatibility.md)). Deep persistence in particular is implemented but has **never run on a real device**.

## Features

- **Exact models and firmware only**: writes are open only to registered model + firmware pairs; unknown devices and unknown firmware are refused; an unknown SSH host key is accepted only after you confirm it, and a changed host key is always refused. `routerctl profiles` lists the supported devices.
- **Guided flow**: `routerctl setup` and the desktop 向导 page first run a read-only check, plan only the missing steps (enable SSH, back up every partition, soft persistence, optional reboot verification), list every write in one confirmation, and stop at the first step that does not succeed.
- **Record before writing**: one operation lock per router; each change is recorded before its first write. After a partial, unknown, or interrupted change, or a failure after a write may have been submitted, every further change is refused until you check the router and run `routerctl recovery ack`.
- **Backup and verification**: streaming MTD backups with local integrity checks; `backup verify --online` also compares device identity and the full partition layout, and confirms the router has not rebooted since the backup (unchanged `boot_id`).
- **Desktop app**: guided flow, maintenance, deep persistence (with a read-only status check), agent access, run reports, and settings, running the same use cases as the CLI; a light (白瓷) and a dark (石墨) theme.
- **AI agent access**: `routerctl mcp` serves MCP over stdio or over Streamable HTTP with a token, and the desktop's Agent 接入 page turns it on; router writes need their own switch, and deep persistence and the recovery acknowledgement always stay with a person.
- **Optional saved passwords**: AES-256-GCM encrypted, saved only after the router accepted them, never sent over Telnet.

## Install

### Install script

The install scripts verify the SHA-256 of what they download; `TOOL_VERSION` pins a release and `TOOL_INSTALL_DIR` sets the install directory (default `~/.local/bin`):

```bash
curl -fsSL https://raw.githubusercontent.com/sunerpy/router-tools/main/scripts/install.sh | sh
```

```powershell
irm https://raw.githubusercontent.com/sunerpy/router-tools/main/scripts/install.ps1 | iex
```

### Prebuilt files

Download from [GitHub Releases](https://github.com/sunerpy/router-tools/releases):

| File                                                | Contents                                                                                                                   |
| --------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------- |
| `routerctl_<version>_<os>_<arch>.tar.gz` / `.zip`   | CLI for Linux, macOS, and Windows on amd64 and arm64, with `LICENSE`, `README.md`, `CHANGELOG.md`, and third-party notices |
| `router-tools_<version>_windows_amd64_setup.exe`    | Desktop installer for Windows (NSIS; installs the WebView2 runtime when missing)                                           |
| `router-tools_<version>_windows_amd64_portable.zip` | Desktop app for Windows without installer (`Router Tools.exe`; needs the WebView2 runtime)                                 |
| `router-tools_<version>_darwin_universal.zip`       | Desktop app for macOS on Apple silicon and Intel (`Router Tools.app`)                                                      |
| `checksums.txt`                                     | SHA-256 of every file above                                                                                                |

Verify a download and its build provenance attestation:

```bash
sha256sum --check --ignore-missing checksums.txt
gh attestation verify router-tools_<version>_darwin_universal.zip --repo sunerpy/router-tools
```

The builds are not code-signed or notarized. Windows SmartScreen may block the app on first launch. On macOS, unzip the archive, move `Router Tools.app` to `/Applications`, and remove the quarantine attribute, otherwise Gatekeeper blocks the app:

```bash
xattr -dr com.apple.quarantine "/Applications/Router Tools.app"
```

### go install

Needs Go 1.27 or newer. A binary installed this way reports `dev` in `routerctl version`:

```bash
go install github.com/sunerpy/router-tools/cmd/routerctl@latest
```

### Build from source

See [Development](#development).

## Quick start

```bash
routerctl profiles                              # supported models and firmware
routerctl doctor --host 192.168.31.1            # read-only preflight: identify model and firmware
routerctl setup --host 192.168.31.1 --plan-only # read-only detection and plan; no changes
routerctl setup --host 192.168.31.1             # ask only for the passwords the plan needs, confirm once, apply
```

With SSH closed, `setup` asks for the Web admin password and the new root password (twice); with SSH open, for the current root password. The confirmation lists every write and its effect.

## Usage

```text
routerctl
├── version · profiles
├── doctor [--with-admin-auth] [--with-root-verify]
├── setup [--goal ssh,backup,soft[,reboot]] [--reset-root-password] [--plan-only]
├── verify [--transport ssh|telnet|auto]
├── enable · reboot-verify
├── persist soft | remove
├── backup create [--output <new-directory>] | verify --dir <directory> [--online]
├── deep-persist --backup-dir <verified-directory>
├── recovery ack
├── credentials list | forget
├── mcp [--http <ip:port>] [--allow-writes] [--token-file <file>] · mcp token · mcp config --agent <id>
└── skill show | install --agent <id> | --dir <directory> [--force]
```

Run `routerctl <command> --help` for flags. Secret input, configuration, managed directories, and exit codes are in [docs/usage.md](../usage.md).

Desktop pages: 向导 (guide) detects the router and runs the guided flow; 维护 (maintenance) removes soft persistence, verifies backups and the root login, and runs the raw preflight; 深度固化 (deep persistence) is the high-risk flow with its read-only status check; Agent 接入 (agent access) turns on the MCP service; 运行报告 (run reports) lists this session's operations and reports; 设置 (settings) manages the default connection and device, saved passwords, host keys, and theme. Runtime requirements and packaging are in [docs/gui.md](../gui.md).

## Using with an AI agent

Install `routerctl` as described in [Install](#install), then let an agent drive it over MCP:

<details>
<summary>Agent access quick reference</summary>

- `routerctl mcp`: the stdio MCP server, started by the agent host itself; only `--allow-writes` adds the router-writing `guide_apply` and `soft_remove`.
- `routerctl mcp --http 127.0.0.1:18731`: Streamable HTTP; every request needs `Authorization: Bearer <token>` (the token comes from `ROUTERCTL_MCP_TOKEN` or `--token-file`; `routerctl mcp token` creates one). The desktop's Agent 接入 page offers the same service.
- `routerctl mcp config --agent <id>`: prints the configuration snippet for Claude Code, Codex CLI, opencode, pi, DeepSeek Harness (dsh), WorkBuddy, or another MCP client.
- `routerctl skill install --agent <id>`: installs the embedded skill that teaches the agent the flow.
- Flow: `guide_plan` runs a read-only check and returns a plan; after the user agrees, `guide_apply` runs it only once; follow it with `operation_wait`; stop on `partial` or `unknown` and let the user check the router.
- Contract: stdout carries only MCP messages; warnings and JSON audit lines go to stderr, one per tool call and one per finished operation; results never contain passwords. Deep persistence, the recovery acknowledgement, forgetting passwords, and Telnet are never offered to an agent.

Per-host setup, approvals, and timeouts are in [docs/agents.md](../agents.md); the risks in [RISKS.md](../../RISKS.md#agent-access-mcp).

</details>

## Safety boundaries

- The CLI never takes passwords as arguments or environment variables: use a hidden prompt or pipe exactly the required lines on stdin. Saving passwords is opt-in; read the threat model in [RISKS.md](../../RISKS.md#saved-passwords).
- Over MCP, an agent can pass passwords as tool arguments; they stay in the agent host's conversation history and go to its model provider. Prefer letting the agent use saved passwords (`use_saved_password`); see [RISKS.md](../../RISKS.md#agent-access-mcp).
- An open port is not authentication: SSH success requires password authentication and `id -u == 0`.
- An unknown SSH host key is refused unless you pin its fingerprint with `--host-key-sha256`, pass `--accept-new-host-key`, or confirm it in the desktop app; a changed host key is always refused, and a mismatch never falls back to Telnet.
- `--yes` confirms ordinary changes only and has no effect on deep persistence. Deep persistence requires `--allow-telnet`, a verified backup of the current boot, and the profile's acknowledgement phrase (`RD08-DEEP` for RD08 1.1.96); a deep write timeout is recorded as `unknown` and never retried, and an existing deep-persistence journal blocks running `deep-persist` again.
- There is no automatic restore, automatic rescue, generic remote shell, or scripted profile.

Read [RISKS.md](../../RISKS.md) and [RECOVERY.md](../../RECOVERY.md) before any device-changing operation.

## Development

Needs Go 1.27.1 (`make test` also needs CGO and a C compiler), Node.js, and pnpm. `make fmt-check` also needs goimports, gofumpt, and oxfmt, and `make lint` needs golangci-lint v2; both print the install command when a tool is missing. The desktop app also needs Wails CLI 2.17.1, and on Linux the GTK3 and WebKit2GTK 4.1 development packages.

`cmd/routergui` embeds the frontend build, so build the frontend once in a fresh checkout, otherwise `make lint` and `make test` fail with `pattern all:frontend/dist: no matching files found`:

```bash
(cd cmd/routergui/frontend && pnpm install && pnpm build)
make fmt          # goimports, gofumpt, oxfmt
make fmt-check
make lint
make test         # race detector and coverage
make build-all    # Windows, macOS, Linux on amd64 and arm64
make gui-test     # Vitest, vue-tsc, Vite build, and Go binding tests
make gui-build    # Linux desktop binary
```

Tests never contact a real router: HTTP uses `httptest`, SSH and Telnet use loopback servers, and protocol state machines run against in-memory fake shells; CI never runs MTD writes.

- Architecture: [docs/architecture.md](../architecture.md); compatibility records: [docs/compatibility.md](../compatibility.md)
- RD08 protocol and safety specification (binding on the implementation; Chinese): [docs/rd08-protocol-spec.md](../rd08-protocol-spec.md)
- Design system: [DESIGN.md](../../DESIGN.md); TDD log: [docs/tdd-log.md](../tdd-log.md)
- Contributor rules: [AGENTS.md](../../AGENTS.md)

## License

MIT; see [LICENSE](../../LICENSE) and [THIRD_PARTY_NOTICES.md](../../THIRD_PARTY_NOTICES.md).
