# Architecture and compatibility contract

## Dependency direction

```text
cmd/routerctl and future frontends
    -> internal/app
        -> internal/device contracts
            <- internal/devices/<vendor>/<model>/<firmware>
                -> reusable protocol, transport, backup, and workflow packages
```

Only the composition root `internal/wiring`, used by both mains, wires concrete adapters through `internal/devices/builtin` (`Adapters()` and `Discovery()`), together with the flock locker, the outcome store and the managed directories. `internal/app` and `internal/cli` do not import an adapter or the builtin list. Frontends (CLI, desktop, MCP server) call the same typed application services in process and never scrape human CLI output. The MCP server (`internal/mcpserver`, below) is the only control service; it runs only when the user starts it and its HTTP transport always requires a token.

## Exact profiles and capabilities

A profile identifies one code adapter and one exact firmware, currently `xiaomi-rd08-1.1.96` (model `xiaomi-rd08`). Profile IDs are always `<model-id>-<firmware>`; the binding RD08 specification's shorter `rd08-1.1.96` refers to the same profile. A `device.Adapter` registers the profile's driver together with the `Operations` that serve only it. Resolution yields exactly one adapter or fails with `UNSUPPORTED_DEVICE`, `UNSUPPORTED_FIRMWARE`, `UNSUPPORTED_PROFILE`, or `AMBIGUOUS_DEVICE`. Explicit profile selection never falls back to another adapter, and an unregistered explicit ID fails before any lock or network access. Every capability call goes to the resolved adapter's `Operations`; the discovery prober only identifies devices, so registering a second firmware can never route its writes through another firmware's code.

`Registry.Catalog()` groups profiles by model in registration order; `Registry.Candidates()` lists the registered firmware of a recognized model after a compatibility failure. Candidates explain failures (`routerctl doctor`, the Doctor report) and are never used to pick an adapter.

Capabilities describe implemented behavior rather than intended support. A profile that declares `deep_persist` carries a `DeepPolicy` (typed acknowledgement phrase and exact reboot count); the application checks the phrase before any network access and again against the resolved profile before creating a journal or calling the adapter. Small interfaces (`AdminVerifier`, `RootVerifier`, `SSHEnabler`, `SoftPersistence`, `BackupCreator`, `DeepPersister`, and related online verifiers) avoid a mandatory all-device interface. The current RD08 adapter advertises doctor, Web auth, verify, enable, soft persistence, backup, and deep persistence after their respective tests.

Unknown firmware is never treated as compatible merely because the hardware string matches. Hardware identity takes priority over display-name fallback.

## Secret and presentation boundaries

`secret.Password` validates exact UTF-8 input without trimming or normalization. `Expose()` is used only at protocol boundaries. `secret.Redactor` covers raw/Base64/URL representations and structured `stok/token/key/nonce` fields. Application failures are copied and redacted before report serialization; CLI stdout/stderr receives a second redaction pass.

Saved passwords are the one persisted secret: `internal/credentials` keeps them AES-256-GCM encrypted per entry (the entry's kind, model, canonical host and port are additional data), with the vault key DPAPI-protected on Windows and a 0600 file elsewhere. The application fills a missing password only after profile resolution and only when the request asks for it, refuses saved root passwords on Telnet-capable runs, never auto-accepts host keys for them, marks a refused value rejected, and saves typed values only after the device accepted them. Frontends get metadata and notes, never values.

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

## Agent access (MCP)

`internal/mcpserver` is a frontend over `internal/guiapi.Service`, the same facade the desktop uses, so every use-case gate applies unchanged. `routerctl mcp` composes `guiapi.New(service, operations, nil)` (no host-key confirmer) and serves it over stdio or, with `--http`, over Streamable HTTP. The package never imports an adapter.

- Tools are registered with the SDK's low-level `Server.AddTool`; the advertised schema comes from `jsonschema.For`, and handlers decode arguments exactly with `encoding/json/v2` (case-sensitive names, no unknown or duplicate members), so no error text can quote an argument name or value. A saved password is never combined with an agent-supplied host key pin. Each tool has an annotation class: R (no side effect), L (router untouched, local vault or files may change), W (router write or cancel). Router writes (`guide_apply`, `soft_remove`) are registered only while writes are enabled and re-check the switch when called.
- Every use-case call becomes an operation owned by the server, not by the request: a call waits at most 50 seconds, and a client cancellation or timeout only ends the wait. `operation_cancel`, `Stop`/`Shutdown` and process exit cancel operations; a stop also ends waiting calls at once. Events are redacted when they arrive and results when the operation ends, value by value, after which the operation keeps no password. Guided plans are kept server-side under a one-shot `plan_id`.
- HTTP wraps the SDK handler in cross-origin protection and a bearer-token check (SHA-256, constant-time), keeps the SDK's DNS-rebinding protection, caps bodies at 1 MiB and serves only `/mcp`.
- Each tool call writes a `call` audit record and each operation a `finish` record (before it is marked done); calls the SDK rejects before any handler are audited by a receiving middleware. The CLI prints the records as JSON lines on stderr. No record carries arguments, passwords, tokens or report bodies.
- The desktop composes the same server through `internal/agentaccess`: `cmd/routergui/app.go` passes its own `guiapi.Service` (whose emitter fans out to the window and to `mcpserver.Operations`) to `agentaccess.New`, which builds the server, an `mcpserver.Listener` over `mcpserver.SettingsStore` (`wiring.Paths.MCP`), a 200-record `AuditRing`, host setups and skill installation. The listener applies a change as bind, store, serve, stop-old, and rolls back on failure; its lifetime is separate from the operations, which only `Shutdown` at app exit cancels. Operations carry `guiapi.WithSource(ctx, "agent")`, so their history entries and host-key prompts name the agent.
- `skills` embeds the `router-tools` skill (`SKILL.md` first in `skills.Files()`). `internal/agentsetup` is pure: a table of agent hosts, `Build` for the per-host snippet (no file access), and `InstallSkill` for the temp-directory-and-rename install. `internal/cli/skill.go` adds `skill show|install` and `mcp config` on top; nothing edits another program's configuration.

## Local integrity and journals

Backup verification rejects absolute paths, traversal, symlink escape, non-regular files, trailing JSON, missing/extra sums, layout differences, length changes, and hash changes. `SHA256SUMS` covers every partition, `mtd.txt`, and `manifest.json`. During creation, the formal manifest is committed last; a sums failure cannot leave a new `complete=true` manifest.

Deep journals use stable stage names and verified old/new boot transitions. Files are 0600, directories 0700, and updates are fsynced and atomically renamed. Existing journals are never overwritten. There is no `resume` command.

CLI run reports are unique, redacted 0600 JSON documents under `run-dir`; unspecified directories are created under the OS temporary directory. Backup writes its own report and manifest in the new backup directory.

## Error and frontend model

Typed requests/results/events are shared across frontends. Stable string error codes retain device/transport meaning; CLI maps them to exit codes 0–12 and 130. Human text is not a machine API. JSON stdout contains one report object, with progress on stderr.

Wails discards the return value of a bound method that returns an error. Desktop bindings therefore resolve with the report for failed, partial, and unknown results. A canceled run carries code `CANCELED` and keeps the status its use case assigns. Enable and deep persistence report partial or unknown once a write may have happened, and the UI keeps those statuses with their non-retry warning; the other use cases report failed for every error, so their cancellations show as canceled even after a write. A binding rejects when the call fails before the application runs (for example an invalid password or a duplicate operation ID), or when an error comes with a report that claims success; deep persistence does this if its final journal write fails, and the report then appears only in operation history. The same happens when a guarded change succeeds but its final outcome record cannot be stored (`STATE_STORE_FAILED`); the router then stays blocked until the change is acknowledged.

## Adapter acceptance requirements

A new adapter must provide exact/unknown/ambiguous compatibility fixtures, read-only probe proof, capability-specific loopback/fake tests, secret-free reports, and real-device evidence before marking a profile verified. Similar hardware is not proof that write commands or layouts are reusable.
