# Architecture and compatibility contract

## Dependency direction

```text
cmd/routerctl and future frontends
    -> internal/app
        -> internal/device contracts
            <- internal/devices/<vendor>/<model>/<firmware>
                -> reusable protocol, transport, backup, and workflow packages
```

Only the composition root `internal/wiring`, used by both mains, wires concrete adapters through `internal/devices/builtin` (`Adapters()` and `Discovery()`), together with the flock locker, the outcome store and the managed directories. `internal/app` and `internal/cli` do not import an adapter or the builtin list. A future GUI must call the same typed application services in process; it must not scrape human CLI output or expose a localhost control service.

## Exact profiles and capabilities

A profile identifies one code adapter and one exact firmware, currently `xiaomi-rd08-1.1.96` (model `xiaomi-rd08`). Profile IDs are always `<model-id>-<firmware>`; the binding RD08 specification's shorter `rd08-1.1.96` refers to the same profile. A `device.Adapter` registers the profile's driver together with the `Operations` that serve only it. Resolution yields exactly one adapter or fails with `UNSUPPORTED_DEVICE`, `UNSUPPORTED_FIRMWARE`, `UNSUPPORTED_PROFILE`, or `AMBIGUOUS_DEVICE`. Explicit profile selection never falls back to another adapter, and an unregistered explicit ID fails before any lock or network access. Every capability call goes to the resolved adapter's `Operations`; the discovery prober only identifies devices, so registering a second firmware can never route its writes through another firmware's code.

`Registry.Catalog()` groups profiles by model in registration order; `Registry.Candidates()` lists the registered firmware of a recognized model after a compatibility failure. Candidates explain failures (`routerctl doctor`, the Doctor report) and are never used to pick an adapter.

Capabilities describe implemented behavior rather than intended support. A profile that declares `deep_persist` carries a `DeepPolicy` (typed acknowledgement phrase and exact reboot count); the application checks the phrase before any network access and again against the resolved profile before creating a journal or calling the adapter. Small interfaces (`AdminVerifier`, `RootVerifier`, `SSHEnabler`, `SoftPersistence`, `BackupCreator`, `DeepPersister`, and related online verifiers) avoid a mandatory all-device interface. The current RD08 adapter advertises doctor, Web auth, verify, enable, soft persistence, backup, and deep persistence after their respective tests.

Unknown firmware is never treated as compatible merely because the hardware string matches. Hardware identity takes priority over display-name fallback.

## Secret and presentation boundaries

`secret.Password` validates exact UTF-8 input without trimming or normalization. `Expose()` is used only at protocol boundaries. `secret.Redactor` covers raw/Base64/URL representations and structured `stok/token/key/nonce` fields. Application failures are copied and redacted before report serialization; CLI stdout/stderr receives a second redaction pass.

The Web Session keeps `stok` private and prevents JSON/formatted disclosure. Login responses and remote command output are not copied into errors by default. Reports never contain admin/root passwords, password hashes, Base64 password payloads, Web keys, or tokens.

## Transport contracts

`transport/shell` preserves `Started`, exit code, and separate stdout/stderr. A nonzero exit returns `RemoteCommandError`; a timeout after submission is not safe retry evidence. POSIX dynamic fragments use one quote function.

SSH requires an explicit host-key callback and supports pinned SHA-256 plus TOFU known_hosts. Unknown keys require acceptance; changed keys stop without Telnet fallback. Independent sessions drain both output streams concurrently. Binary backup uses a separate bounded streaming API rather than the ordinary 4 MiB command buffer.

Telnet incrementally decodes IAC negotiation/subnegotiation and executes marker-framed commands. EOF, timeout, invalid markers, or stream desynchronization closes the connection. Telnet is never accepted for backup or final SSH success.

## Operation state

All changing/backup operations acquire a per-router non-blocking lock before network probing. Production uses flock; the embedded default uses the same non-blocking semantics in process. Both lock by `device.RecoveryKey`, the SHA-256 of a versioned prefix plus the canonical host, so neither the management ports nor the spelling of the host select a different lock, outcome record or managed directory.

Every device write is guarded by a write-ahead outcome record (`internal/state`, metadata only). After the lock, resolution and capability checks the use case stores `in_progress` before the first adapter call and its final status before the lock is released. A record that is in progress, partial, unknown, or failed after a possible write blocks every further write of that router with `RECOVERY_REVIEW_REQUIRED` until `AcknowledgeRecovery` records an operator check. Only adapter stage evidence (set before the first write, never cleared) can mark a failure as write-free.

- Enable: Web auth → existing SSH check → fixed commands with attempted-MAC tracking → cleanup → final SSH verification.
- Soft: conflict/read-only checks → temporary file syntax/hash → atomic install → UCI commit → complete read-back. Removal is limited to the managed path.
- Reboot: canonical old `boot_id` → one submitted reboot → all three ports down → root reconnect → different `boot_id`. No blind reboot retry.
- Backup: identity/profile → `/proc/mtd` parse → free-space check → bounded independent SSH streams → exact length/hash → atomic local files → offline verification.
- Deep: local backup → online identity/layout/boot → tool/crash checks → journaled crash write → three independently verified reboot transitions and flag checks → soft reinstall → independent final SSH.

A remote write timeout is `unknown`. Accepted writes followed by failure are at least `partial`. Cleanup errors are secondary and never overwrite the primary error.

## Guided flow

`GuidePlan` detects read-only and plans `enable_ssh -> backup -> soft_install -> reboot_verify` for the selected goals; a step whose goal is already in place is skipped. Root evidence comes from password-authenticated SSH only, soft state from the adapter's `SoftInspector`, and backup state from the local managed backup directory. A ready plan carries a secret-free confirmation (target, device identity, profile, goals, reset choice, each step's action and effects) and the SHA-256 of its JSON. `GuideApply` accepts that confirmation only with a matching digest, detects again with the confirmed profile, and allows each step only to keep its action or do less with unchanged effects. It then runs each step through the step's own use case with the confirmed identity, which the use case checks again under the lock, and stops at the first step that does not succeed. The guided flow never uses Telnet and never includes deep persistence or soft-persistence removal.

## Local integrity and journals

Backup verification rejects absolute paths, traversal, symlink escape, non-regular files, trailing JSON, missing/extra sums, layout differences, length changes, and hash changes. `SHA256SUMS` covers every partition, `mtd.txt`, and `manifest.json`. During creation, the formal manifest is committed last; a sums failure cannot leave a new `complete=true` manifest.

Deep journals use stable stage names and verified old/new boot transitions. Files are 0600, directories 0700, and updates are fsynced and atomically renamed. Existing journals are never overwritten. There is no `resume` command.

CLI run reports are unique, redacted 0600 JSON documents under `run-dir`; unspecified directories are created under the OS temporary directory. Backup writes its own report and manifest in the new backup directory.

## Error and frontend model

Typed requests/results/events are shared across frontends. Stable string error codes retain device/transport meaning; CLI maps them to exit codes 0–12 and 130. Human text is not a machine API. JSON stdout contains one report object, with progress on stderr.

Wails discards the return value of a bound method that returns an error. Desktop bindings therefore resolve with the report for failed, partial, and unknown results. A canceled run carries code `CANCELED` and keeps the status its use case assigns. Enable and deep persistence report partial or unknown once a write may have happened, and the UI keeps those statuses with their non-retry warning; the other use cases report failed for every error, so their cancellations show as canceled even after a write. A binding rejects when the call fails before the application runs (for example an invalid password or a duplicate operation ID), or when an error comes with a report that claims success; deep persistence does this if its final journal write fails, and the report then appears only in operation history. The same happens when a guarded change succeeds but its final outcome record cannot be stored (`STATE_STORE_FAILED`); the router then stays blocked until the change is acknowledged.

## Adapter acceptance requirements

A new adapter must provide exact/unknown/ambiguous compatibility fixtures, read-only probe proof, capability-specific loopback/fake tests, secret-free reports, and real-device evidence before marking a profile verified. Similar hardware is not proof that write commands or layouts are reusable.
