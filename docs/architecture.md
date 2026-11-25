# Architecture and compatibility contract

## Dependency direction

```text
cmd/routerctl and future frontends
    -> internal/app
        -> internal/device contracts
            <- internal/devices/<vendor>/<model>/<firmware>
                -> reusable protocol, transport, backup, and workflow packages
```

Only the composition root wires concrete adapters. `internal/app` and `internal/cli` do not import an RD08 package. A future GUI must call the same typed application services in process; it must not scrape human CLI output or expose a localhost control service.

## Exact profiles and capabilities

A profile identifies one code adapter and one explicitly verified firmware, currently `xiaomi-rd08-1.1.96`. Resolution yields exactly one adapter or fails with `UNSUPPORTED_DEVICE`, `UNSUPPORTED_FIRMWARE`, `UNSUPPORTED_PROFILE`, or `AMBIGUOUS_DEVICE`. Explicit profile selection never falls back to another adapter.

Capabilities describe implemented behavior rather than intended support. Small interfaces (`AdminVerifier`, `RootVerifier`, `SSHEnabler`, `SoftPersistence`, `BackupCreator`, `DeepPersister`, and related online verifiers) avoid a mandatory all-device interface. The current RD08 adapter advertises doctor, Web auth, verify, enable, soft persistence, backup, and deep persistence after their respective tests.

Unknown firmware is never treated as compatible merely because the hardware string matches. Hardware identity takes priority over display-name fallback.

## Secret and presentation boundaries

`secret.Password` validates exact UTF-8 input without trimming or normalization. `Expose()` is used only at protocol boundaries. `secret.Redactor` covers raw/Base64/URL representations and structured `stok/token/key/nonce` fields. Application failures are copied and redacted before report serialization; CLI stdout/stderr receives a second redaction pass.

The Web Session keeps `stok` private and prevents JSON/formatted disclosure. Login responses and remote command output are not copied into errors by default. Reports never contain admin/root passwords, password hashes, Base64 password payloads, Web keys, or tokens.

## Transport contracts

`transport/shell` preserves `Started`, exit code, and separate stdout/stderr. A nonzero exit returns `RemoteCommandError`; a timeout after submission is not safe retry evidence. POSIX dynamic fragments use one quote function.

SSH requires an explicit host-key callback and supports pinned SHA-256 plus TOFU known_hosts. Unknown keys require acceptance; changed keys stop without Telnet fallback. Independent sessions drain both output streams concurrently. Binary backup uses a separate bounded streaming API rather than the ordinary 4 MiB command buffer.

Telnet incrementally decodes IAC negotiation/subnegotiation and executes marker-framed commands. EOF, timeout, invalid markers, or stream desynchronization closes the connection. Telnet is never accepted for backup or final SSH success.

## Operation state

All changing/backup operations acquire a per-target non-blocking lock before network probing. Production uses flock; the embedded default uses the same non-blocking semantics in process.

- Enable: Web auth → existing SSH check → fixed commands with attempted-MAC tracking → cleanup → final SSH verification.
- Soft: conflict/read-only checks → temporary file syntax/hash → atomic install → UCI commit → complete read-back. Removal is limited to the managed path.
- Reboot: canonical old `boot_id` → one submitted reboot → all three ports down → root reconnect → different `boot_id`. No blind reboot retry.
- Backup: identity/profile → `/proc/mtd` parse → free-space check → bounded independent SSH streams → exact length/hash → atomic local files → offline verification.
- Deep: local backup → online identity/layout/boot → tool/crash checks → journaled crash write → three independently verified reboot transitions and flag checks → soft reinstall → independent final SSH.

A remote write timeout is `unknown`. Accepted writes followed by failure are at least `partial`. Cleanup errors are secondary and never overwrite the primary error.

## Local integrity and journals

Backup verification rejects absolute paths, traversal, symlink escape, non-regular files, trailing JSON, missing/extra sums, layout differences, length changes, and hash changes. `SHA256SUMS` covers every partition, `mtd.txt`, and `manifest.json`. During creation, the formal manifest is committed last; a sums failure cannot leave a new `complete=true` manifest.

Deep journals use stable stage names and verified old/new boot transitions. Files are 0600, directories 0700, and updates are fsynced and atomically renamed. Existing journals are never overwritten. There is no `resume` command.

CLI run reports are unique, redacted 0600 JSON documents under `run-dir`; unspecified directories are created under the OS temporary directory. Backup writes its own report and manifest in the new backup directory.

## Error and frontend model

Typed requests/results/events are shared across frontends. Stable string error codes retain device/transport meaning; CLI maps them to exit codes 0–12 and 130. Human text is not a machine API. JSON stdout contains one report object, with progress on stderr.

Wails discards the return value of a bound method that returns an error. Desktop bindings therefore resolve with the report for failed, partial, and unknown results. A canceled run carries code `CANCELED` and keeps the status its use case assigns. Enable and deep persistence report partial or unknown once a write may have happened, and the UI keeps those statuses with their non-retry warning; the other use cases report failed for every error, so their cancellations show as canceled even after a write. A binding rejects when the call fails before the application runs (for example an invalid password or a duplicate operation ID), or when an error comes with a report that claims success; deep persistence does this if its final journal write fails, and the report then appears only in operation history.

## Adapter acceptance requirements

A new adapter must provide exact/unknown/ambiguous compatibility fixtures, read-only probe proof, capability-specific loopback/fake tests, secret-free reports, and real-device evidence before marking a profile verified. Similar hardware is not proof that write commands or layouts are reusable.
