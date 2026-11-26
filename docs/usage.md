# CLI usage

## Global inputs

Common target flags:

```text
--host 192.168.31.1
--web-port 80
--ssh-port 22
--telnet-port 23
--profile auto
--config <non-secret.json>
--json
--non-interactive
--run-dir <private-report-directory>
```

`--profile` takes `auto` (the single exact match) or an exact profile ID listed by `routerctl profiles`; an unregistered ID fails before any network access, and an explicit profile never falls back:

```text
$ routerctl profiles
Xiaomi BE6500 Pro (RD08)
  firmware 1.1.96     --profile xiaomi-rd08-1.1.96
Only the listed firmware versions are accepted; auto picks the single exact match.
```

`routerctl --json profiles` prints one JSON object (`schema_version`, `operation`, `models`). When `doctor` recognizes the model but not its firmware, it prints the supported firmware for that model.

Configuration precedence is explicit CLI flag > JSON file > built-in default. The default file is the OS user configuration directory under `routerctl/config.json`. Unknown fields, secret-looking fields, invalid ports, and unsupported schema versions are rejected.

SSH flags:

```text
--known-hosts <path>
--host-key-sha256 SHA256:<base64-without-padding>
--accept-new-host-key
--allow-telnet
```

`--allow-telnet` permits plaintext Telnet only where the selected operation supports it. Backup and final SSH evidence never use Telnet.

## Secret input

- `--admin-password-stdin`: read the Web admin password.
- `--root-password-stdin`: read the root password.
- With both flags, line 1 is admin and line 2 is root.
- Only one LF or CRLF is removed. Other leading/trailing spaces and Unicode are preserved.
- Empty passwords, control characters, invalid UTF-8, and values over 1024 UTF-8 bytes are rejected before network access.

Interactive mode uses hidden TTY input. Non-TTY interactive input warns before visible line input. `--non-interactive` never prompts and fails when a required secret or confirmation is absent.

Example with an approved secret provider (placeholder command, not a runtime dependency):

```bash
secret-provider read router-admin | \
  routerctl --non-interactive --admin-password-stdin \
  doctor --with-admin-auth
```

Do not put passwords in command-line arguments, exported environment variables, JSON configuration, or shell history.

## Saved passwords

```bash
routerctl --remember-password verify          # save the root password once SSH root verification accepted it
routerctl --use-saved-password setup          # fill passwords that are not typed from the vault
routerctl credentials list                    # kind, model, host, port, saved and rejected times; never values
routerctl credentials forget --host 192.168.31.1
routerctl credentials forget --all            # delete every saved password and the vault key
```

- `--remember-password` saves a typed password once the step that used it was accepted by the router: the Web admin password after a successful Web login (`doctor --with-admin-auth`, or a successful `enable`), the root password after password-authenticated SSH root verification (`doctor --with-root-verify`, `verify`, `setup` detection, and successful `enable`, `persist`, `reboot-verify` and `backup` runs without `--allow-telnet`). A step that itself ends partial, unknown or failed saves nothing and never overwrites an entry; a later step failing in the same run does not undo an earlier save.
- `--use-saved-password` fills only passwords that are not typed or piped. Interactive runs prompt when no usable entry exists for the host and port, and always prompt for the root password with `--allow-telnet`; non-interactive runs fail with `CREDENTIAL_NOT_FOUND` (exit 2) before any device write. `setup` asks only for what its plan still needs; without a usable saved password, a non-interactive `setup` fails with `INVALID_PASSWORD` (also exit 2).
- A saved root password is never used when Telnet is allowed, and it disables `--accept-new-host-key`. The new root password of `enable` and of the guided enable step is always typed. `deep-persist` refuses both flags.
- A saved password the router refuses is not sent again or retried and is marked rejected: a single command fails, `setup` detection asks for the current password instead. The entry is not filled again until a typed password is verified and saved with `--remember-password`.
- `credentials forget` needs at least one of `--kind`, `--scope`, `--host`, `--port`, or `--all`. A vault that cannot be read or decrypted reports `CREDENTIAL_STORE_FAILED` (exit 11).

## Read-only discovery

```bash
routerctl doctor
routerctl doctor --with-admin-auth
routerctl doctor --with-root-verify
routerctl doctor --with-admin-auth --with-root-verify
routerctl verify --transport ssh
routerctl verify --transport auto --allow-telnet
```

`verify --transport telnet --allow-telnet` proves only Telnet root access. It does not set `ssh_verified=true`.

## Guided setup

```bash
routerctl setup --plan-only                    # detect and show the plan; changes nothing
routerctl setup                                # detect, ask only for what is missing, confirm once, apply
routerctl setup --goal ssh,backup,soft,reboot  # also prove soft persistence across one reboot
routerctl setup --reset-root-password          # SSH is open but the current root password is unknown
```

`setup` detects the router read-only, then plans the steps `enable_ssh`, `backup`, `soft_install` and `reboot_verify` for the selected goals (default `ssh,backup,soft`; `ssh` is always included and `reboot` requires `soft`). A step whose goal is already in place is skipped: SSH root already works, a verified managed backup exists, or the soft persistence is installed and verified. It prompts only for what the plan needs:

- SSH closed: the Web admin password and the new root password, twice.
- SSH open: the current root password. When it is rejected, `setup` does not reset it; pass `--reset-root-password` to let the enable flow set a new one, and that change is listed in the confirmation.

The confirmation lists every planned write and its effect. `--yes` replaces the typed `yes`. The run detects again first. If that detection fails, it stops with the detection's own error (for example `UNSUPPORTED_FIRMWARE`, exit 5, after a firmware change). If the device identity changed or a step would do more or something else than you confirmed, it stops with `CONFIRMATION_REQUIRED` (exit 9). Both happen before any write; run `setup` again then. It stops at the first step that does not succeed and exits with that step's code. Nothing is retried. Soft persistence that drifted or conflicts is reported with the separate maintenance it needs (`persist remove`, or a manual check) and is never removed by `setup`. Deep persistence is never part of `setup`.

Non-interactive use reads the passwords with `--admin-password-stdin` (first line) and `--root-password-stdin` (next line) and needs `--yes`. With nothing to do, `setup` prints `Nothing to do` and exits 0 without asking for a confirmation.

## Recovery acknowledgement

```bash
routerctl recovery ack
```

After a change ended `partial` or `unknown`, failed after a write may have been submitted, or stopped without a final record, every further change of that router fails with `RECOVERY_REVIEW_REQUIRED` (exit 9). Check the router as `RECOVERY.md` describes, then acknowledge; `--yes` replaces the typed confirmation. The record is kept and blocks again after the next uncertain change.

## Managed directories

| Purpose                         | Location                                                       |
| ------------------------------- | -------------------------------------------------------------- |
| Recovery records                | `<user config dir>/routerctl/state/`                           |
| Target locks                    | `<user config dir>/routerctl/locks/`                           |
| Deep journals without a run dir | `<user config dir>/routerctl/deep/<model>/<host>-<key>/`       |
| Backups without `--output`      | `<home>/RouterTools/backups/<model>/<host>-<key>/<UTC stamp>/` |
| Saved passwords                 | `<user config dir>/routerctl/credentials/`                     |

`<host>` is the canonical host (IP literals normalized, host names in lower case) and `<key>` the first 12 hex characters of the router's port-independent recovery key, so changing a management port or the spelling of the host never selects another lock, record or directory. A journal in a custom `--run-dir`, or written by a desktop build from before the 向导 page (which used the directory you chose), is found only in that directory; check it before another deep attempt. The current desktop deep flow always uses the managed directory.

## Temporary SSH enable

```bash
routerctl --yes enable
```

`--yes` confirms the ordinary state change after the command displays its risk boundary. The adapter first checks whether the supplied root password already works; if so, it returns `already_verified` without MAC writes. Otherwise it executes the three fixed commands, cleans every attempted MAC, and verifies SSH root up to three times.

A cleanup failure after root verification is `partial` (exit 10), not success. An add-request timeout is `unknown` and is not retried.

## Soft persistence

```bash
routerctl --yes persist soft
routerctl --yes persist remove
routerctl --yes reboot-verify
```

Installation verifies file permissions, syntax, SHA-256, and all four UCI values. It does not claim boot verification. `reboot-verify` requires a changed `boot_id`, final SSH root, and a second complete soft-configuration verification.

## Backup

```bash
routerctl backup create                       # new directory under the managed backup root
routerctl backup create --output /new/nonexistent/backup-directory
routerctl backup verify --dir /new/nonexistent/backup-directory
routerctl backup verify --dir /new/nonexistent/backup-directory --online
```

An explicit output directory must not already exist; without `--output` a new time-stamped directory is created under the managed backup root. Creation requires SSH root and streams each MTD node with exact size and SHA-256 verification. Local verify checks manifest schema, paths, regular files, exact sizes, hashes, MTD layout, and complete sorted `SHA256SUMS` coverage.

`--online` additionally compares device identity, full layout, and unchanged `boot_id` through read-only SSH commands.

## Deep persistence — independently authorized use only

```bash
routerctl \
  --allow-telnet \
  --ack-deep RD08-DEEP \
  --backup-dir /verified/current-boot/backup \
  --run-dir /private/deep-run \
  deep-persist
```

Read `RISKS.md` and `RECOVERY.md` first. The acknowledgement is the resolved profile's own phrase (`RD08-DEEP` for `xiaomi-rd08-1.1.96`); the interactive prompt shows it, and when registered profiles use different phrases it asks for `--profile` or `--ack-deep`. `--yes` has no effect on this dedicated acknowledgement. An existing `<host>-deep.json` in the run directory blocks the operation. Without `--run-dir` the journal lives in the router's managed deep directory, so a retry with other ports still meets it. Do not rename an old journal merely to bypass the safety gate.

## Agent access (MCP)

`routerctl mcp` serves the Model Context Protocol so an AI agent can run detection, backups and guided setup. Read the "Agent access" section of `RISKS.md` first; the binding rules are in §23 of `docs/rd08-protocol-spec.md`.

```bash
# stdio: the agent host starts this command itself
routerctl mcp
routerctl mcp --allow-writes          # also offer guide_apply and soft_remove

# Streamable HTTP at http://127.0.0.1:18731/mcp with a bearer token
export ROUTERCTL_MCP_TOKEN="$(routerctl mcp token)"
routerctl mcp --http 127.0.0.1:18731
routerctl mcp --http 0.0.0.0:18731 --token-file ~/.config/routerctl/mcp.token   # LAN: plaintext warning
```

- Tools that never change the router: `list_profiles`, `preflight`, `guide_plan`, `deep_status`, `backup_create` (reads the router, writes the managed backup directory), `saved_passwords`, `operation_wait`, `operation_list`. `operation_cancel` stops an operation of this server, which can leave a write partial. With `--allow-writes`: `guide_apply` (runs a plan from `guide_plan` once) and `soft_remove`. Deep persistence, `recovery ack`, forgetting passwords and Telnet are never offered.
- Tool arguments may carry the admin and root passwords; `use_saved_password` uses the vault instead. Results never contain passwords. `host_key_sha256` cannot be combined with `use_saved_password`, because a pin replaces the known_hosts check. Argument names match exactly (case-sensitive, no duplicates, no unknown names).
- A call waits at most `wait_seconds` (default 20, maximum 50) and then returns `done: false` with an `operation_id`; continue with `operation_wait`. The operation keeps running when the agent stops waiting; only `operation_cancel` or stopping the server cancels it.
- With `--http`, every request needs `Authorization: Bearer <token>`. The token comes from `--token-file` (a regular file of at most 4 KiB, refused when group- or world-readable on macOS and Linux) or `ROUTERCTL_MCP_TOKEN`; after trimming it must have at least 32 printable ASCII characters without spaces. The host part of the address must be an IP address or `localhost`; port `0` picks a free port. The bound URL is printed on stderr.
- stdout carries only MCP messages; warnings and one JSON audit line per tool call and per finished operation go to stderr.
- Exit codes: `0` after a clean stop (SIGINT, SIGTERM, or the end of input over stdio), `2` for invalid flags, address or token, `10` when an operation was still running at exit (stderr names it), `12` when listening or serving fails. A stop cancels running operations and ends waiting calls at once.

### Agent hosts and the skill

```bash
routerctl mcp config --agent claude-code                       # stdio entry for the host, plus its add command
routerctl mcp config --agent pi --http http://127.0.0.1:18731/mcp
routerctl skill install --agent codex                          # ~/.agents/skills/router-tools
routerctl skill install --dir ~/skills-export                   # any directory; the skill goes to <dir>/router-tools
routerctl skill show                                           # print SKILL.md
```

- `mcp config` prints the snippet, the file it belongs in and, where the host has one, its own add command. It writes nothing and names this `routerctl` by its absolute path. Agent IDs: `claude-code`, `codex`, `opencode`, `pi`, `dsh`, `workbuddy`, `generic`; `--http` is refused for `workbuddy`, which documents only stdio servers.
- `skill install --agent` knows the directories of `claude-code` (`~/.claude/skills`, or `$CLAUDE_CONFIG_DIR/skills`) and of `codex`, `opencode`, `pi` and `dsh` (`~/.agents/skills`; for dsh `$DSH_AGENTS_HOME/skills` when set); `workbuddy` and `generic` need `--dir`. An identical installed copy is left alone; a changed one is replaced only with `--force`, which rewrites the skill's own files and keeps others. Exit 9 refuses a symbolic link or non-directory at `<dir>/router-tools` or at a directory the install creates inside it, and a symbolic link or directory where it writes a file; `<dir>` itself may be a symbolic link. Other special files (a named pipe, for example) count as changed and are replaced with `--force`. Other file-system errors exit 12.
- `mcp config` prints the moved file when a host's documented variable is set (`CLAUDE_CONFIG_DIR`, `CODEX_HOME`, `OPENCODE_CONFIG_DIR` or `XDG_CONFIG_HOME` for opencode, `PI_CODING_AGENT_DIR`, `DSH_HOME`). For Codex on Linux with `XDG_CONFIG_HOME` set to a directory other than `$HOME/.config`, the entry also sets that variable, because Codex does not pass it to stdio servers and routerctl would otherwise use other locks, recovery records, known hosts and saved passwords.
- Per-host details, approvals, timeouts and troubleshooting: [agents.md](agents.md).

## Output and exit codes

Human reports go to stdout; progress and warnings go to stderr. `--json` emits one JSON report object on stdout, including failed operations where a report is available. Run reports are unique 0600 JSON files under `run-dir`; deep operations additionally write the stable-stage journal.

| Exit | Meaning                                                                                     |
| ---: | ------------------------------------------------------------------------------------------- |
|    0 | All required evidence passed or the exact target state already existed                      |
|    2 | CLI/configuration/password validation, or no usable saved password                          |
|    3 | Network/transport/protocol connection failure                                               |
|    4 | Web/SSH/Telnet authentication failure                                                       |
|    5 | Device/profile/capability/host-key compatibility failure                                    |
|    6 | Device API explicitly rejected a request                                                    |
|    7 | Remote command or tool failure                                                              |
|    8 | Final state/root/reboot verification failure                                                |
|    9 | Safety precondition, confirmation, lock, deep journal, recovery gate, or agent write switch |
|   10 | Partial completion, cleanup failure, or unknown remote result                               |
|   11 | Local backup integrity/disk verification, recovery record or password vault failure         |
|   12 | Unexpected internal error                                                                   |
|  130 | Cancellation                                                                                |

Nonzero statuses require reading the report. In particular, `partial` and `unknown` are not safe automatic-retry signals. `routerctl mcp` uses 10 and 12 as described in [Agent access (MCP)](#agent-access-mcp).
