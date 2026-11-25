# Compatibility and verification evidence

## Supported profile

| Field              | Value                                                       |
| ------------------ | ----------------------------------------------------------- |
| Profile            | `xiaomi-rd08-1.1.96`                                        |
| Device             | Xiaomi BE6500 Pro                                           |
| Hardware           | `RD08` (display-name fallback: BE6500 Pro)                  |
| Firmware           | Exact `1.1.96`, after trimming surrounding whitespace only  |
| Management         | HTTP 80, SSH 22, Telnet 23                                  |
| Real-device status | **Not verified on hardware in this repository's test runs** |

No other model or firmware is accepted automatically. There is no `--force` compatibility bypass.

## Offline evidence

- Fixed SHA-1/SHA-256 login vectors and deterministic nonce.
- Login-page/init-info/API parser and redirect/limit/cookie/form tests.
- SSH loopback tests for authentication, host keys, large stderr, nonzero exits, multiple commands, timeout, and binary streams.
- Telnet loopback tests for negotiation, login, markers, nonzero exits, and timeout; incremental decoder tests cover IAC splitting/subnegotiation/escaped IAC.
- Enable fixed commands, Base64 password handling, attempted-MAC cleanup, add failure, cleanup failure, unknown write, and already-verified state.
- Exact soft script bytes/Base64/SHA-256, transaction ordering, conflicts, idempotency, removal, and hash-read failures.
- MTD parsing, binary backup, short-read retry, disk-space precheck, local manifest/sums verification, tampering, traversal, and symlinks.
- Reboot disconnect/new-`boot_id` requirements.
- Deep success and release-blocking failures: missing crash, invalid/unchanged `boot_id`, first/second reboot failure, bad flags, erase failure, unknown writes, final SSH failure, and existing journal.
- CLI secret lines, JSON output, configuration precedence, flags, exit codes, and report permissions.

These tests establish implementation behavior against the written contract, not physical-device compatibility.

## Ordered real-device QA — requires separate authorization

Record tool version/commit, redacted device identifier, firmware, date, tester, and outcome for every step. Do not reorder steps to make progress after a failure.

1. `doctor`: confirm identity and no configuration change.
2. `enable`: verify root login and complete MAC cleanup.
3. `backup create` + `backup verify`: verify every partition and local hash before any persistence test.
4. `persist soft`: verify file and UCI read-back.
5. `reboot-verify`: verify real boot-time restoration, changed `boot_id`, and final SSH root.
6. `persist remove`: verify that only managed auto-start state was removed.
7. Reinstall soft persistence and create a fresh current-boot backup; verify it locally and online.
8. Only after independent high-risk authorization, run `deep-persist` on a recoverable test device and retain the complete journal.
9. After deep success, manually log in with SSH and check all four `bdata` values again.

Firmware-upgrade and factory-reset persistence are separate destructive tests. They are not implied by a successful three-reboot sequence and require their own authorization.

## Current release labels

| Area              | Offline implementation tests       | Real-device acceptance                          |
| ----------------- | ---------------------------------- | ----------------------------------------------- |
| Doctor/Web auth   | Passed                             | Pending                                         |
| SSH/Telnet verify | Loopback passed                    | Pending                                         |
| Enable            | Fake/HTTP contract passed          | Pending                                         |
| Soft/reboot       | Fake state-machine passed          | Pending                                         |
| Backup            | Filesystem/loopback passed         | Pending                                         |
| Deep persistence  | Release-blocking fake tests passed | **Not run; independent authorization required** |
| Desktop GUI       | Go/Vitest/build and browser QA     | Windows UI and real-device operations pending   |
