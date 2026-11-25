# router-tools

[![CI](https://github.com/sunerpy/router-tools/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/sunerpy/router-tools/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/sunerpy/router-tools?sort=semver)](https://github.com/sunerpy/router-tools/releases)

`routerctl` is a Go CLI for exact-profile, safety-gated router management. The first adapter targets **Xiaomi BE6500 Pro / RD08 firmware 1.1.96**. The same application core also drives a Wails desktop GUI.

> **Verification status:** protocol, state-machine, loopback SSH/Telnet, filesystem, and CLI tests run offline. No real router has been used in this repository's test runs. In particular, deep persistence is implemented but is **not real-device verified**.

## Implemented capabilities

- Exact model/firmware registry, capability interfaces, and fail-closed compatibility checks.
- Xiaomi login-page parsing, SHA-1/SHA-256 login, cookie sessions, opaque `stok` sessions, and secret redaction.
- SSH password authentication, TOFU/pinned host keys, bounded concurrent stdout/stderr reads, and binary streaming.
- Telnet negotiation and marker-framed commands for explicitly permitted fallback paths.
- `doctor` with optional Web-admin and root verification.
- Fixed three-step SSH enable with attempted-MAC tracking, cleanup, and final SSH root evidence.
- Soft persistence install/read-back verification/idempotency/removal.
- Reboot state machine using disconnect observation and changed canonical `boot_id`.
- Streaming MTD backup, local integrity verification, and optional online identity/layout/boot comparison.
- High-risk deep persistence with dedicated acknowledgement, verified backup, online layout check, operation lock, three-reboot journal, and final independent SSH verification.
- Strict non-secret JSON configuration, per-target flock, private run reports, human and single-object JSON output.

## Command tree

```text
routerctl
├── version
├── doctor [--with-admin-auth] [--with-root-verify]
├── verify [--transport ssh|telnet|auto]
├── enable
├── persist
│   ├── soft
│   └── remove
├── reboot-verify
├── backup
│   ├── create --output <new-directory>
│   └── verify --dir <directory> [--online]
└── deep-persist --backup-dir <verified-directory>
```

Run `routerctl <command> --help` for command flags. See [docs/usage.md](docs/usage.md) for secret input and examples.

## Download and install

Releases are published on [GitHub Releases](https://github.com/sunerpy/router-tools/releases) once the release pull request is merged. Every release carries:

| Asset                                               | Contents                                                                                                                        |
| --------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------- |
| `routerctl_<version>_<os>_<arch>.tar.gz` / `.zip`   | CLI for Linux, macOS, and Windows on amd64 and arm64, with `LICENSE`, `README.md`, `CHANGELOG.md`, and `THIRD_PARTY_NOTICES.md` |
| `router-tools_<version>_windows_amd64_setup.exe`    | Desktop GUI installer for Windows (NSIS; installs the WebView2 runtime when missing)                                            |
| `router-tools_<version>_windows_amd64_portable.zip` | Desktop GUI without installer (`Router Tools.exe`; needs the WebView2 runtime)                                                  |
| `router-tools_<version>_darwin_universal.zip`       | Desktop GUI for macOS on Apple silicon and Intel (`Router Tools.app`)                                                           |
| `checksums.txt`                                     | SHA-256 of every asset above                                                                                                    |

Install the CLI with the checksum-verifying installers (`TOOL_VERSION` pins a release, `TOOL_INSTALL_DIR` overrides `~/.local/bin`):

```bash
curl -fsSL https://raw.githubusercontent.com/sunerpy/router-tools/main/scripts/install.sh | sh
```

```powershell
irm https://raw.githubusercontent.com/sunerpy/router-tools/main/scripts/install.ps1 | iex
```

Verify a downloaded asset against the checksum file and the build provenance attestation:

```bash
sha256sum --check --ignore-missing checksums.txt
gh attestation verify router-tools_<version>_darwin_universal.zip --repo sunerpy/router-tools
```

The builds are not code-signed or notarized. Windows SmartScreen may warn on first start. On macOS, unzip the archive, move `Router Tools.app` to `/Applications`, and remove the quarantine attribute, otherwise Gatekeeper blocks the app:

```bash
xattr -dr com.apple.quarantine "/Applications/Router Tools.app"
```

## Build and test

Requirements: Go 1.27.0+; the toolchain is pinned to Go 1.27.1.

```bash
go test ./...
make test          # race detector and coverage; requires CGO and a C compiler
make lint
make build
make build-all     # Windows, macOS, Linux on amd64/arm64
```

```bash
./dist/routerctl version
./dist/routerctl doctor --host 192.168.31.1
./dist/routerctl --json doctor --with-admin-auth
./dist/routerctl backup verify --dir /path/to/backup
```

Normal tests never contact a real router. HTTP uses `httptest`; SSH and Telnet use loopback fixture servers; dangerous operations use in-memory shells. CI must not run MTD writes.

## Desktop GUI

The Wails application implements the same operations with typed bindings, operation-scoped events, cancellation, host-key dialogs, private reports, and deep-persist confirmation flows. `DESIGN.md` locks the design system: 白瓷 (light, default) and 石墨 (dark) themes with bundled fonts; `/primitives` is the development showcase.

```bash
make gui-test
make gui-build
make gui-build-windows
make gui-installer-windows
```

Local artifacts are under `cmd/routergui/build/bin/`. `scripts/package-gui.sh windows|darwin` builds the release assets listed above (the darwin package needs macOS); CI runs both on every pull request. These are unsigned builds. See [docs/gui.md](docs/gui.md) for runtime requirements, packaging status, and 375/768/1280 browser QA evidence.

## Safety boundaries

- Passwords are never accepted as literal CLI arguments or environment variables. Use a hidden prompt or pipe exactly the required lines from an approved secret provider.
- An open TCP port is not authentication evidence. SSH success requires password authentication plus exact `id -u == 0`.
- Unknown or changed SSH host keys fail closed. A host-key mismatch never falls back to Telnet.
- Device changes require exact profile compatibility and a per-target non-blocking operation lock.
- `--yes` confirms ordinary changes only. Deep persistence additionally requires `--allow-telnet`, a verified current-boot backup, and the resolved profile's exact acknowledgement (`RD08-DEEP` for RD08 1.1.96). `routerctl profiles` lists the supported models and firmware.
- A deep write timeout is `unknown`, not a retryable failure. Existing deep journals block blind restart.
- There is no automatic restore, automatic rescue, generic remote-shell command, or arbitrary script profile mechanism.

Read [RISKS.md](RISKS.md) and [RECOVERY.md](RECOVERY.md) before any device-changing operation.

## Architecture

```text
CLI / Wails GUI
      │
      ▼
internal/app                  use cases, reports, safety gates, operation locks
      │
      ▼
internal/device               exact profiles, capabilities, registry, errors
      │
      ▼
internal/devices/...          model/firmware adapters
      │
      ├── Xiaomi Web auth/MAC channel
      ├── RD08 soft/backup/deep state machines
      └── reusable workflow primitives
      │
      ▼
internal/transport            SSH / Telnet / shell contracts
```

New adapters are compiled in and registered at the composition root. The application and CLI do not import concrete RD08 packages. Dynamic Go plugins and data-file-defined remote commands are intentionally excluded.

See [docs/architecture.md](docs/architecture.md), [docs/compatibility.md](docs/compatibility.md), and the self-contained [RD08 protocol specification](docs/rd08-protocol-spec.md). The architecture document governs multi-adapter differences from the original single-device layout; protocol and safety requirements remain binding.

## TDD

Behavior was implemented in red-green-refactor slices: fixed login vectors, protocol parsers, loopback transports, command state machines, filesystem integrity, application gates, and CLI contracts. [docs/tdd-log.md](docs/tdd-log.md) records the slices and final gates.

## License

MIT; see [LICENSE](LICENSE) and [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
