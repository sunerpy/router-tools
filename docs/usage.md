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
routerctl backup create --output /new/nonexistent/backup-directory
routerctl backup verify --dir /new/nonexistent/backup-directory
routerctl backup verify --dir /new/nonexistent/backup-directory --online
```

The output directory must not already exist. Creation requires SSH root and streams each MTD node with exact size and SHA-256 verification. Local verify checks manifest schema, paths, regular files, exact sizes, hashes, MTD layout, and complete sorted `SHA256SUMS` coverage.

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

Read `RISKS.md` and `RECOVERY.md` first. The acknowledgement is the resolved profile's own phrase (`RD08-DEEP` for `xiaomi-rd08-1.1.96`); the interactive prompt shows it, and when registered profiles use different phrases it asks for `--profile` or `--ack-deep`. `--yes` has no effect on this dedicated acknowledgement. An existing `<host>-deep.json` in the run directory blocks the operation. Do not rename an old journal merely to bypass the safety gate.

## Output and exit codes

Human reports go to stdout; progress and warnings go to stderr. `--json` emits one JSON report object on stdout, including failed operations where a report is available. Run reports are unique 0600 JSON files under `run-dir`; deep operations additionally write the stable-stage journal.

| Exit | Meaning                                                                |
| ---: | ---------------------------------------------------------------------- |
|    0 | All required evidence passed or the exact target state already existed |
|    2 | CLI/configuration/password validation                                  |
|    3 | Network/transport/protocol connection failure                          |
|    4 | Web/SSH/Telnet authentication failure                                  |
|    5 | Device/profile/capability/host-key compatibility failure               |
|    6 | Device API explicitly rejected a request                               |
|    7 | Remote command or tool failure                                         |
|    8 | Final state/root/reboot verification failure                           |
|    9 | Safety precondition, confirmation, lock, or deep journal gate          |
|   10 | Partial completion, cleanup failure, or unknown remote result          |
|   11 | Local backup integrity/disk verification failure                       |
|   12 | Unexpected internal error                                              |
|  130 | Cancellation                                                           |

Nonzero statuses require reading the report. In particular, `partial` and `unknown` are not safe automatic-retry signals.
