# TDD log

This file records the initial red-green slices. Compilation failures caused only by missing production packages/functions are valid red states; test-fixture syntax errors were corrected first and were not treated as behavioral red tests.

## 1. Registry and RD08 profile

Red:

```bash
go test ./internal/device/... ./internal/devices/xiaomi/rd08/v1_1_96/...
# no non-test Go files
```

Green after minimal registry, errors, profile, and matcher implementation:

```bash
go test ./internal/device/... ./internal/devices/xiaomi/rd08/v1_1_96/...
# ok both packages
```

## 2. Login algorithm vectors

Red: `go test ./internal/devices/xiaomi/common/webauth/...` reported no non-test Go files.

Green: both fixed SHA-1/SHA-256 vectors, nonce bounds, and unknown-mode rejection passed.

## 3. Login page and init_info parsing

Red: `go test ./internal/devices/xiaomi/common/routerweb/...` reported no non-test Go files after correcting test fixture quoting.

Green: exact fields, conflicting values, numeric/string API codes, encryption types, top-level JSON, and trailing-value rejection passed. An implementation-side regexp escaping defect was found by these tests and fixed without weakening them.

## 4. HTTP client

Red: `NewClient` was undefined in client tests.

Green: fixed GET paths, cookie reuse, method checks, redirect rejection, response limit, and Base URL validation passed.

## 5. Doctor application slice

Red: Target, ProbeResult, Service, and RD08 Prober were undefined.

Green:

```bash
go test ./internal/device/... ./internal/app/... ./internal/devices/xiaomi/rd08/v1_1_96/...
go test ./...
```

Tests verify no probing after invalid input, no fallback for explicit profiles, stable failed reports, event order, skipped optional authentication, and Web-TCP-before-HTTP ordering.

## 6. CLI

Red: the `buildinfo`/CLI implementation was absent.

Green: `go test ./internal/cli/...` passed for version output, single-object JSON, progress on stderr, failed JSON reports, argument exit 2, compatibility exit 5, cancellation exit 130, and human authentication disclaimers.

## 7. Cancellation propagation regression

Red: new tests referenced the missing `CANCELED`/`WrapError` contract and covered cancellation through both Application and RD08 Prober rather than only a direct CLI error.

Green: cancellation retains `errors.Is(err, context.Canceled)`, reports `CANCELED`, and maps to CLI exit 130. Ordinary request timeouts remain network/protocol failures.

## 8. Password and redaction primitives

Red: the `internal/secret` package had only tests.

Green: exact password preservation, UTF-8/control/length checks, non-serializable passwords, and raw/Base64/URL/structured-field redaction passed.

## 9. Nonce generation and Web login POST

Red: `GenerateNonce`, `NewClientWithOptions`, and `Client.Login` were undefined.

Green: the fixed nonce drives the exact login form; Cookie reuse, SHA-256 hash, auth/protocol error separation, invalid-password network prevention, and Session JSON/format redaction passed.

## 10. Application admin authentication

Red: `CapabilityWebAuth`, `AdminVerifier`, and the Doctor admin fields were undefined.

Green: compatibility-before-login, capability checks, verified/failed reports, event ordering, exact password forwarding, and application-boundary error redaction passed.

## 11. CLI admin secret input

Red: `ExecuteWithInput` was undefined.

Green: LF/CRLF/EOF handling, whitespace and Unicode preservation, non-interactive failure, non-TTY warnings, auth exit 4, and final stdout/stderr redaction passed.

## 12. Shell and host-key policy

Red: Shell and host-key packages had only tests. Green: POSIX quoting, exact root UID, TOFU persistence, pin matching, changed-key rejection, and file permissions passed. Tests also caught a nil `net.Addr` panic and an initially missing pin-only acceptance path.

## 13. Loopback SSH and Telnet

Red: SSH/Telnet production transports were absent. Green: wrong-password classification, large concurrent stderr, consecutive commands, nonzero evidence, timeout closure, binary streams, incremental IAC decoding, login prompts, command markers, and connection invalidation passed without executing fixture commands.

## 14. Verify application and CLI

Red: `Verify`, `RootVerifier`, and CLI `verify` were undefined. Green: profile gating, SSH/Telnet report separation, restricted fallback, host-key errors without fallback, password input, and exit mappings passed.

## 15. MAC channel and temporary enable

Red: MAC methods and `EnableSSH` were undefined. Green: escaped `stok` paths, exact forms, fixed commands, Base64 password payload, attempted cleanup, no unknown-write retry, already-verified idempotency, and partial/unknown reporting passed.

## 16. Soft persistence and reboot

Red: exact soft script and reboot workflow were absent. Green: script bytes/Base64/SHA-256, UCI-after-file ordering, syntax/hash failure stops, conflicts, idempotency, managed removal, hash-read failure handling, canonical boot IDs, down observation, and changed-boot/root checks passed.

## 17. MTD backup and streaming

Red: parser, verifier, creator, and SSH stream API were absent. Green: duplicate names/index validation, binary preservation, short-read retry, free-space gate, `.partial` behavior, path/traversal/symlink/tamper rejection, sums coverage, and final-manifest commit ordering passed.

## 18. Deep persistence and journal

Red: journal, Application deep gates, and adapter state machine were absent. Green: atomic journal stages, backup acknowledgement/verification, existing-journal rejection, exactly three successful transitions, missing crash/invalid boot/no-down/unchanged boot/bad flag/second-reboot/erase/final-SSH failures, and unknown-write non-retry passed. A test-isolation issue that attempted a real Telnet dial was found and removed.

## 19. Application operations and locks

Red: soft/reboot/backup Application APIs and flock were absent. Green: confirmation gates, final evidence reports, offline backup verification, non-blocking lock/release, and production flock wiring passed.

## 20. Configuration and full CLI command tree

Red: config loader and operation commands were absent. Green: unknown/secret field rejection, CLI-over-file precedence, two-line secret reads, all command flags, partial exit 10, deep flag placement, and private run reports passed.

## 21. Online backup verification

Online verification was added on top of local integrity checks, reusing the same identity/layout/boot comparison required by deep persistence. The existing offline suite remained green.

## 22. Final failure-closed review

Regression tests/inspection covered hash-command failure versus missing files, cancellation propagation, trailing JSON, manifest-last commit semantics, incomplete adapter results, hardware fallback identity, and success/error status contradictions.

## 23. Desktop design system and Wails

The user selected a full `DESIGN.md` before UI implementation. Naive theme tokens were extracted from installed source before components. Primitive tests preceded product views. GUI API tests covered secret conversion, cancellation, real report history, and host-key confirmation; Vitest covered primitives and operation state. Browser QA caught and fixed missing page gutters and modal default-focus behavior, then verified three breakpoints, long-report scrolling, deep confirmation, and host-key dialogs. Wails Linux and Windows/NSIS artifacts were built; macOS cross-build is explicitly unsupported and assigned to a native runner.

## 24. Desktop failure reports and online-verify redaction

Wails rejects the promise and drops the return value whenever a bound method returns an error. `guiapi` returned the application error next to failed, partial, and unknown reports, so the desktop UI showed only the error text, labelled partial and unknown results as `failed`, and hid the "do not retry from the beginning" warning. The preview mock resolved those reports, so browser QA could not see the difference.

Red: new `guiapi` tests failed because `EnableSSH` returned `MAC_CLEANUP_FAILED` instead of the report, a canceled `Doctor` returned `context canceled`, and an invalid password left an empty report in history. `BackupVerifyOnline` put the unredacted root password in its report. Vitest failed because a `CANCELED` report showed as `failed` and the preview had no `failed` scenario.

Green: `guiapi` resolves failed, partial, and unknown reports with a nil binding error and keeps the error for every other status. Failures before the application call reject without a history entry and free the operation ID. `BackupVerifyOnline` now copies its error and redacts the root password, as `Doctor`, `Verify`, `EnableSSH`, soft persistence, and deep persistence already did. The frontend shows a failed `CANCELED` report as `canceled`, and the preview `failed` scenario resolves a report the same way the bindings do.

## 25. Integer ports in desktop requests

Browser QA against the real bindings (`wails dev`) found that editing any port field broke every operation. `TextInput` emits the input's string value even for `type="number"`, the request carried `"web_port": "1"`, and Wails could not decode a string into the Go `int`. The native runtime rejects such a call with an argument-parse error; the `wails dev` browser runtime cannot route that error and leaves the call pending. The string was also saved to localStorage, so the failure survived restarts.

Red: a Vitest case set string ports on the store and observed them unchanged in the `Doctor` request; a second case showed that a 20-digit value still passed an integer check and would overflow the Go `int`. Green: `makeRequest` converts every port to an integer in 1–65535, and anything else becomes 0, which application validation rejects before network access. A rerun through `wails dev` returned the `WEB_UNREACHABLE` report and showed it inline.

## 26. Review follow-ups: canceled writes and root-password redaction

An independent review reproduced, with the real RD08 adapter and Web client against `httptest`, that canceling `EnableSSH` after a MAC write was accepted produced a `partial` or `unknown` report with code `CANCELED`. The frontend mapped every `CANCELED` report to `canceled` and hid the "do not retry from the beginning" warning again. The review also found that `RebootVerify` and `BackupCreate` returned the adapter error without redacting the root password.

Red: a Vitest case showed a `partial` report with code `CANCELED` as `canceled`. A table test returned the root password in the `RebootVerify` and `BackupCreate` reports. Green: only a failed `CANCELED` report is shown as `canceled`, so partial and unknown cancellations keep their status and warning. Both use cases copy their error and redact the root password.

## 27. Redesign: 白瓷 default theme and 石墨 dark theme

The user chose the Voltip-derived option B (白瓷) as the default and option C (石墨) as the dark theme. `DESIGN.md` was rewritten first: two theme token blocks, bundled fonts, the scale, the shell, the primitive list, the appearance preference, and a contrast gate.

Red: Vitest cases failed for the missing theme controller (default light, stored preference, live system scheme, Windows title-bar sync, storage of the preference only), the Naive UI token mapping, the dashboard summaries (device, port tiles that keep port state apart from authentication, the next step in the documented order, recent operations), and the new primitives (toggle, segmented control, badge, stat tile, stepper, icon button, history list, Chinese labels for idle/canceling/canceled). `tokens.test.ts` failed until both theme blocks existed; it computes WCAG contrast for every text pair and status indicator in both themes. `tokenTrace.test.ts` failed on every component that still used the old `--rt-color-*` names.

Green: tokens, fonts, primitives and every view were rebuilt on the new system, the theme is applied before mount, and Naive dialogs receive the dark theme plus token overrides. The deep-persistence checklist names the checks the backend performs and never shows them as passed before a run. Browser QA at 375 / 768 / 1280 in both themes found and fixed a full-width wrapped navigation row and a stretched theme selector.

## 28. Redesign review follow-ups

An independent review confirmed 26 defects in the redesign; the main ones were safety-relevant. The deep flow's Telnet acknowledgement shared `store.allowTelnet` with the ordinary option on other pages, so it could already be on. Passwords stayed in the shared store after a canceled confirmation or leaving a page and appeared in other pages' fields. The standing "do not retry from the beginning" instruction was gone from the deep page after an app restart. The next-step tile kept recommending 开启 SSH after a partial or unknown enable. Port tiles and the sidebar labelled old evidence with the edited form. The stage list showed finished stages as running. The deep acknowledgement input had no associated label.

Red: Vitest cases failed for per-run request overrides, per-run target snapshots, clearing on request, history refresh after a rejected call, the collapsed stage list, password-blame only for `WEB_AUTH_FAILED`, the history-aware next step, canceled history rows, the sidebar status in words, host and port validation, per-form validation on the dashboard and enable page, the deep flow's own Telnet switch and standing instruction, dialog labelling and DOM order, `aria-required`, the selected-segment cue, the icon button's busy state, the report viewer's accessible names, and transitions suspended during a theme switch.

Green: each case passes; `DESIGN.md` was updated where the review found the contract imprecise (icon sizes, trust-on-first-use host keys, selection cues, focus ring on fields, the 480 px rule, clearing rules).

## 29. Windows test portability

The new Windows CI job runs `go test ./...` natively. Three tests asserted `Mode().Perm() == 0o600` for the run report, the journal, and the accepted `known_hosts` file. Windows has no POSIX permission bits, and `os.Stat` reports `0666` for any writable file, so these assertions could only fail there; the production code still creates the files with `0o600`. The assertions now apply on every platform except Windows, and the tests keep every other check. A review of the Windows path handling found a fourth case: the backup verification fixture built its partition path with `filepath.Join`, so on Windows the manifest and `SHA256SUMS` carried `partitions\mtd00_0_BOOT.bin`, which `Verify` correctly rejects as non-portable. The fixture now records the slash-separated path that `Create` writes and converts it only to write the file. `GOOS=windows go vet ./...` compiles the tests; the native Windows result comes from CI.

## 30. Device catalog and per-profile dispatch

Every capability call used to type-assert the one RD08 prober, so a second registered firmware would have run its writes through the 1.1.96 code, and the deep phrase `RD08-DEEP` and its three reboots were hard-coded in the application, CLI and GUI.

Red: registry tests failed for the missing `device.Adapter`, `Lookup`, `Candidates`, `Catalog`, the registration rules (profile ID `ModelID-Firmware`, one metadata set per model, `deep_persist` iff a `DeepPolicy`, implementations for declared capabilities) and two firmwares of one model (auto picks the exact one, an explicit profile does not move to its sibling). Application tests failed for dispatch to the resolved adapter with the wrong adapter's and the discovery prober's call counts at zero, an unregistered explicit profile with zero probes and no lock, an implementation lacking the interface, Doctor candidates on `UNSUPPORTED_FIRMWARE`, and the deep phrase and reboot count per profile (another profile's phrase passes the offline check but stops before any journal or adapter call). CLI tests failed for `routerctl profiles`, the supported-firmware hint and the deep prompt phrase; guiapi and Vitest failed for the `Catalog` binding and the preview catalog.

Green: `device.NewRegistry(...Adapter)`, `internal/devices/builtin` as the only registration list, `app.Service` with a discovery prober and `adapter.Operations` dispatch, `checkProfileID` before lock and network, `Profile.Deep` for the phrase and reboot count, `routerctl profiles`, and the `Catalog()` binding. The 24 existing app test registrations were wrapped mechanically (`newTestRegistry`) without changing an assertion.

An independent review then showed that only Verify and DeepPersist had a two-adapter test, so reverting any write path to the discovery prober would have stayed green, and that the rewritten deep test no longer told a right phrase from a wrong one. Red: a table test for every capability (Doctor admin and root checks, Verify, EnableSSH, PersistSoft, PersistRemove, RebootVerify, BackupCreate, BackupVerifyOnline, DeepPersist) with counting implementations on two firmwares and on the discovery prober, an unregistered explicit profile for every use case with zero probes, locks and adapter calls, and a deep test that registers a deep profile and proves a wrong phrase stops offline while the right one reaches the next offline gate. Mutation runs confirmed both catch the regressions. The CLI deep warning again states the exact reboot count from the profile, and `--json profiles` prints one JSON object.

## 31. Outcome guard and guided plan/apply

A failed or interrupted write left no durable trace: GUI history lived in memory, the locks and the deep journal directory were keyed by host and port, and an empty deep run directory created a fresh temporary directory, so changing a port or the directory restarted from step one.

Red: `device.RecoveryKey` table test (IPv4, IPv6 spellings, host-name case, trailing dot, ports ignored); both lockers refuse a second Acquire with any port changed; adapter tests for stage evidence (`WriteStarted` before the first soft, reboot and deep write, kept after a reboot that is followed by `HOST_KEY_MISMATCH`; no evidence for a Web login failure or a soft precheck failure); the state store's strict decode, size cap, 0600 file and secret-free content; every mutating use case, called directly, refuses `RECOVERY_REVIEW_REQUIRED` while blocked, aborts with `STATE_STORE_FAILED` before the adapter when the in-progress record cannot be stored, keeps blocking when the final record fails, and is unblocked by `AcknowledgeRecovery`; managed backup and journal directories, including the display-name-only identity branch. Green: `internal/state`, `beginWrite`/`settleReport`, `internal/wiring` for both mains, the managed roots, and `SoftInspector`/`DeepInspector` built from the existing read-only commands.

Red for the guided flow: a planner table test for every rule (SSH closed, root verified, missing or rejected root password, explicit reset, each soft state, managed backup, reboot, plan-level blocks, missing capabilities and backup root), goal normalization, and a digest test that changes when any confirmed field changes. Service tests against a simulated router whose state follows the writes: a read-only plan with no SSH login on a closed port; nothing to do on a configured router; blocks for the recovery record and a busy lock before any probe, and for unsupported firmware (with candidates) and a host-key mismatch; a soft drift naming its maintenance; apply refusing an unconfirmed, tampered, changed-identity, more-writes, not-ready or SSH-opened-meanwhile plan before any step or record; run -> skip allowed; stop at the first failure with later steps `not_run`; deferred soft decided after SSH works (skip, run, or stop on conflict); events tagged with their step; and every guided write refusing a stale confirmed identity. All failed first for the missing API. Green: `GuidePlan`, `GuideApply`, `ExpectedIdentity` in the four use cases. Eleven injected regressions (accepting more writes, skipping the digest or identity check, not stopping, running a conflict, running a skipped step, dropping the backup identity check, ignoring the recovery record, treating a rejected password as verified, silently resetting the root password) each turned a test red.

CLI red: `setup` plan-only rendering, goals and reset flag, stdin passwords, the new root password asked twice (mismatch exits 2), the current root password asked once followed by a second detection, non-interactive missing input, typed confirmation, nothing to do, a blocked plan's exit code and acknowledgement hint, a stopped run's per-step output, `recovery ack` confirmation, and exit codes 9 and 11 for the two new error codes. guiapi and Vitest red: the three bindings, no history for plans, one entry per run and acknowledgement, no secrets in history, and the preview scenarios.

An independent review (three lenses and an adversarial verifier) confirmed 16 findings, none blocking. Red, then green: managed directory names and the deep journal file name now follow `device.CanonicalHost`, so another spelling of the same router (host-name case, an IPv6 form) meets the existing journal (`DEEP_JOURNAL_EXISTS`) instead of starting from step one after an acknowledgement; unfinished, unacknowledged records no longer serialize zero timestamps (`omitzero`); failed plans and refused runs carry empty `steps`/`goals` lists instead of `null`; the outcome record keeps `write_evidence` (write started, accepted steps, attempted and uncleaned MAC rules, deep journal path, the journal already in the in-progress record) and drops the never-filled `operation_id`; `InspectDeep` also reports whether the crash partition is listed and readable, using only `cat /proc/mtd` and `test -r`. Regression tests added for code that was correct but unguarded: the RD08 `RebootVerify` adapter keeps `WriteStarted` after a submitted reboot whose reconnect fails with `HOST_KEY_MISMATCH` (deleting the propagation line now fails), the guided backup scan with the display-name identity branch and with a foreign device's backup, and a guided reboot step that never allows Telnet. Mutation runs confirmed each new test catches its regression. Deferred to the GUI redesign with documentation: the desktop recovery acknowledgement entry and sending the managed journal directory from the desktop deep flow.

## 32. Saved passwords in an app-encrypted vault

The binding spec allowed passwords only in process memory, so every page and command asked again. Contract first: spec §4.3(5), §16.1(1) and the new §16.7 allow exactly one persisted form, opt-in AES-256-GCM ciphertext in the app vault, with the threat model in RISKS.md.

Red: the vault round trip with only ciphertext and metadata on disk (0600 files, 0700 directory on POSIX), refusal of tampered ciphertext and nonces, of an entry moved to another router (additional data), and of another vault's key; strict decoding (unknown field, schema, trailing data, kind, port, non-canonical host, nonce length, duplicates, size); a missing key with existing entries is reported and never replaced, while their metadata stays listable; rejection, re-save clearing it, filtered and full forgetting; twenty concurrent writers on separate instances keep every entry; DPAPI round trip and tamper refusal (Windows CI only). Green: `internal/credentials` with `FileVault`, `Memory`, DPAPI and POSIX key files. Mutations that drop the additional-data binding, regenerate the key over entries, remove the cross-process lock, or accept trailing data each fail a test.

Red for the rules: Verify fills the saved root password after resolution and turns off `AcceptNew`; typed passwords win; nothing saved, a rejected entry, another model's or port's entry, a Telnet-capable run, and a broken vault all stop before the adapter; a refused saved value is marked rejected after one attempt and never leaks into the error; Remember saves only accepted passwords, a failed save is a note, and a partial enable never overwrites; every root-password use case fills and remembers; Doctor and EnableSSH handle the admin password while the root password EnableSSH sets stays typed; GuidePlan verifies with a saved root password, marks a refused one, counts a saved admin password as provided, and remembers a typed root password only after verification; GuideApply runs with saved passwords but requires a typed new root password before any write. CLI red: the flags reach the application without reading stdin, interactive prompts only without a usable entry, `enable` always asks for the root password, `deep-persist` refuses both flags, `credentials list|forget` with filter rules, exit codes 2 and 11, and `setup` skipping prompts its plan covers. guiapi and Vitest red: the DTO flags, the three credential bindings with metadata only and no history, and the preview vault.

An independent review (three lenses and an adversarial verifier) confirmed 13 findings, none blocking. Red, then green: a rejection marks only the value that was refused (a newer value saved meanwhile stays usable), the vault and the outcome store reject trailing `}` or `]`, an interactive Telnet-capable run asks for the root password instead of failing later, the CLI's redactor learns filled saved values, human output shows the credential notes, backup creation and online verification are covered by the fill-and-remember table, and the preview vault refuses an empty filter and remembers only typed passwords. The documents now state the actual remember rule (a password is saved once the step that used it was accepted; later failures do not undo it), the threat model on every platform, and that the desktop remember switch follows with the guided redesign.

## 33. Guided desktop flow and read-only deep status

The desktop still offered one page per operation, so the user had to know the order and repeat inputs. Contract first: `DESIGN.md` defines the navigation (向导, 维护, 深度固化, 运行报告, 设置; `/primitives` only in development builds), the device selector, plan step list and credential states, one confirmation per guided run with 取消 focused, and the password switches.

Red for the deep page, which needed facts before its first step: `DeepStatus` reports the managed journal path and whether a journal exists, the verified managed backups newest first, the root SSH result and the boot-flag inspection, and lists the problems it found (recovery record, existing journal, flags already or partly set, unreadable crash partition, no verified backup); without a root password it skips the inspection, a closed SSH port is reported without any SSH attempt, and an unsupported device returns its candidates. The guiapi binding resolves the report and records no history. Green: `internal/app/deep_status.go` and the binding, reusing the managed-backup scan of `GuidePlan`.

Red for the window: the primitives (automatic detection or one exact firmware of a model, a stale profile ID flagged and kept, step actions and results in words, credential states without values, `not_run`), the guided store (selected goals, reboot dropped without soft persistence, only the inputs the plan needs, a saved admin password counted as provided, the confirmation passed back unchanged, every password cleared after dispatch and absent from `localStorage`, progress per step, acknowledgement then a fresh detection, a changed plan shown as a new detection), the 向导 page (address validated before detecting, required passwords before the one confirmation, the recovery acknowledgement), and the deep page (phrase and reboot count from the profile and never prefilled, its own Telnet acknowledgement, the managed journal always used, secrets cleared when left). Green: `GuideView`, `ToolsView`, the reworked `DeepView` and `SettingsView`, and the removal of the dashboard and the step-by-step views, whose routes now redirect. Summary helpers that only the dashboard used were removed with their tests.

Browser QA at 375/768/1280 in both themes found three defects; each got a red test or check first. The root document scrolled because a visually hidden stepper label escaped the content column (fixed by making the column the containing block); the preview deep run reported a journal path that differed from the managed path its status showed; and the 向导 page kept 记住密码 on and 使用已保存的密码 off across visits, against `DESIGN.md` §4. `vue-tsc` caught a removed helper that the sidebar status still called. The real bindings were then exercised with `wails dev` under Xvfb (docs/gui.md).

An independent review (three lenses and an adversarial verifier) confirmed 21 findings, two of them blocking; each got a red test first. Blocking: canceling the guided, maintenance or deep confirmation left the typed passwords and the deep acknowledgement in their fields, and the current root password typed for detection stayed on the page after `GuidePlan` was sent; both are now cleared as `DESIGN.md` §4 requires, and the run asks for the root password again. Should fix: a `CONFIRMATION_REQUIRED` after earlier steps had written no longer discards the partial result as "nothing written"; the guided flow can be canceled; a plan whose address, ports, device, goals or saved-password choice changed after detection cannot run; the confirmation names the router; failed and rejected runs and failed acknowledgements show their error; the acknowledgement goes to the router the blocked plan belongs to, and the recovery panel shows the record's times; the deep page preselects no backup, tells a closed SSH port and a refused root password apart from a missing one (the report now carries the SSH state) and shows the credential notes; the settings page keeps a failed forget visible; the device selector links its error to the select. The documents now separate the deep-status problems that make a run refuse from the warnings, point to `routerctl backup create` for a backup of the current boot, and no longer describe the removed dashboard.

## 34. MCP server for agents (`routerctl mcp`)

Agents had to drive the human CLI. Contract first: spec §23 (tools and their annotation classes, the write switch, never-exposed capabilities, passwords in tool arguments, HTTP authentication and protections, the operation model, audit) and §1.4, §4.1, §4.3, §16.1, §16.4 and §17; `RISKS.md` "Agent access (MCP)"; `docs/usage.md`; `docs/architecture.md`.

Red for `internal/mcpserver` against an in-memory SDK client and a fake `Backend`: the tool list without and with writes and `tools/list_changed` when the switch flips; each tool pinned to its class (R no side effect, L local vault or files only, W router write or cancel); forced fields (SSH only, no Telnet, default known_hosts, managed directories, the server's tool version and `agent-` operation IDs) while credential and host-key choices pass through; strict arguments (unknown fields, wrong types, bad host or ports, `wait_seconds` outside 0-50, smuggled paths) refused before the backend; `soft_remove` refused without `confirm`; a ready plan stored under a one-shot `plan_id` that another session can apply once, a wrong digest refused without consuming it; `done=false` after the wait and the final report through `operation_wait`; a canceled request that leaves its operation running; `operation_cancel` and `operation_wait` limited to this server's IDs; at most four running operations; shutdown canceling them and reporting the ones that do not stop; failed, partial, unknown and success-with-error results marked as errors; one `call` audit record per tool call and one `finish` per operation; progress forwarded as `notifications/progress`. The secret scan put the passwords into every password field, into an unknown field's name, a wrongly typed field and a goal, and made the fake backend quote them in messages: results, progress and audit records never contain them; password properties are write-only and unconstrained. A first green run failed that scan because a goal value reached the report; goals are now validated in the MCP layer with a fixed message. Internal tests cover plan expiry and eviction, the write switch checked again inside the write path, bounded events, a contained panic whose value is not reported, and the finished-operation bound.

HTTP tests run the handler on a loopback `httptest` server: missing, wrong and non-Bearer tokens get 401 with `WWW-Authenticate`; the SDK client works with the token; a rotated token refuses the old one; a rebinding `Host`, a cross-site browser POST, a 2 MiB body and other paths get 403, 403, 413 and 404; `ParseListen` accepts IP literals and `localhost` only; tokens have the `rtmcp_` format and are never echoed by validation; `Serve` stops with its context. Fourteen mutations (operation bound to the request, reusable plan, Telnet allowed, no redaction, L marked read-only, no write re-check, no cross-origin protection, rebinding protection off, no body limit, any token accepted, goal echo, no concurrency cap, shutdown not canceling, unknown fields accepted) each made a test fail.

Red for the CLI: `routerctl mcp token`, `--http` without a token, with a short one, with a hostname, a port out of range or a group-readable token file (exit 2, nothing echoed), `--token-file` without `--http`, a served HTTP endpoint that answers 401 and then initializes with the token and stops with exit 0, the plaintext warning for `0.0.0.0`, and stdio whose stdout carries only JSON-RPC lines while audit lines and the write notice go to stderr; `AGENT_WRITES_DISABLED` maps to exit 9. Green: `internal/cli/mcp.go` composes `guiapi.New(service, operations, nil)` and the server. End-to-end tests in `cmd/routerctl` re-execute the test binary as the real `main` with a temporary home and drive it over stdio (`CommandTransport`) and over HTTP: `list_profiles`, and `preflight` to a closed loopback port returning a failed report.

An independent review (security, correctness and documentation lenses, each checked by an adversarial verifier) confirmed these defects; each got a red test first. An agent-supplied `host_key_sha256` replaced the known_hosts check while `use_saved_password` filled the vault's root password, so a pinned impostor could have received it: the combination is now refused before any network access. Argument decoding with `encoding/json` matched names case-insensitively (including Unicode folding) and accepted duplicates, although the schema forbids both: decoding now uses `encoding/json/v2` with exact names, and errors never echo a client's name. Whole-text redaction could corrupt a result when a password was as short as `1` or `true`: results are now redacted value by value when the operation ends, events when they arrive, and the operation then drops its redactor, so finished operations hold no password. On SIGINT or SIGTERM the server waited for every pending call's wait before canceling operations: `Stop` now ends waits at once (stdio and HTTP). Progress emitted right before the end could be dropped, and the `finish` audit record could be written after Shutdown returned; both are fixed. Calls the SDK rejects before any handler (a write tool while writes are off) are now audited by a receiving middleware. New end-to-end tests send passwords through stdio and HTTP and scan stdout, stderr and every HTTP response body. Eleven further mutations each made a test fail. The documents now list `operation_cancel` as able to stop a write, describe token and token-file checks exactly, and no longer mention the desktop switch and skill of later phases.

## 35. Agent skill and per-host setup (`routerctl skill`, `routerctl mcp config`)

Agents needed the flow (plan, approval, one apply, `operation_wait`, stop on `partial` or `unknown`) and each host its own configuration format. Contract first: spec §4.1 and §23.9, `docs/agents.md` (one section per host with its sources), `docs/usage.md`, `RISKS.md` (a write switch kept in a host's configuration).

Red for `skills`: the embedded `SKILL.md` starts with frontmatter whose `name` matches the directory rules and whose description stays within 1024 characters, and it mentions `guide_plan`, `guide_apply`, `operation_wait`, `use_saved_password` and `RECOVERY_REVIEW_REQUIRED`. Red for `internal/agentsetup`: golden snippets per host and transport (Claude Code JSON and `claude mcp add`, Codex TOML and `codex mcp add`, opencode JSON, pi JSON with `"exposure": "direct"`, a dsh YAML insert, WorkBuddy and generic `mcpServers`), HTTP snippets that read `ROUTERCTL_MCP_TOKEN` and never name the binary, quoting per platform (TOML and JSON escapes, YAML single quotes, POSIX and Windows command quoting, no JSON-in-shell command on Windows), refusals (unknown host, WorkBuddy over HTTP, non-HTTP URLs, no binary for stdio), `/mcp` added to a bare URL, no snippet or command that enables writes, and the documented skill directories. `InstallSkill`: a new skill through a temporary directory and a rename, an identical copy left alone, a changed copy refused without `--force` and left untouched, `--force` rewriting only shipped files and keeping the user's others, and symbolic links or files in the way refused, including a symbolic link inside an installed skill. Red for the CLI: `unknown command "skill"` and `unknown flag: --agent` first; then `skill show`, `skill install` (exit 9 for a changed copy without `--force`, exit 2 for `workbuddy`, `generic`, both or neither of `--agent` and `--dir`) and `mcp config` naming the running binary.

Real hosts in isolated home directories: Codex CLI 0.171.0 wrote the same TOML as the snippets and listed the server; pi 1.6.0 reported both servers connected with their tools; Claude Code 2.1.332 connected over stdio and HTTP with the printed commands and warned about the missing token variable without it. opencode 1.18.18 rejected the first snippet, which used opencode 2.x's `mcp.servers`, with `Configuration is invalid ... mcp.servers`; the snippet test was changed first to require servers directly under `mcp` (the form the 2.x migration guide still supports), then the builder, and opencode 1.18.18 connected over stdio and over HTTP with the token (401 without it). opencode 2.4.2 read the configuration but listed no servers in the headless test, even one written by its own `opencode mcp add`, so its connection is recorded as unverified. Seventeen mutations (identical copy rewritten, changed copy replaced without force, force deleting the user's files, a symbolic link inside the skill followed, WorkBuddy over HTTP, no `/mcp`, any URL scheme, POSIX quoting on Windows, unescaped TOML backslash, undoubled YAML quote, the JSON command on Windows, `mcp.servers` for opencode, `--allow-writes` in a snippet, pi without direct exposure, `--agent` with `--dir`, exit 2 for an install failure, a bare `routerctl` command) each made a test fail. Reordering `skills.Files()` is an equivalent mutant while the skill ships one file.

An independent review (install safety, host formats, documentation and skill lenses, then an adversarial verifier) confirmed these defects; each got a red test first where code changed. The main flow failed on a new router: `enable_ssh` writes and then verifies root over SSH, and `routerctl mcp` has no host-key prompt, so without `accept_new_host_key` the run ended `partial` and blocked the router. The server instructions and the skill now tell the agent to ask before `guide_apply` and pass it; tests pin both, the skill's guided section, and every offered tool name in `SKILL.md`. The skill also covers `needs_input` plans (call `guide_plan` again with the current root password, or `reset_root_password` on request), names the router in `routerctl --host <address> recovery ack`, separates `SSH_HOST_KEY_REJECTED` from `HOST_KEY_MISMATCH`, and no longer claims the CLI exit code alone tells `partial`. Codex does not pass `XDG_CONFIG_HOME` to stdio servers, so on Linux the agent's `routerctl` used other locks, recovery records, known hosts and saved passwords; when the variable is set, the Codex entry now sets it the way `codex mcp add --env` writes it (checked: both forms produce the same TOML), and macOS and Windows get no such entry. `mcp config` and `skill install --agent` follow the hosts' documented override variables (`CLAUDE_CONFIG_DIR`, `CODEX_HOME`, opencode's `XDG_CONFIG_HOME`, `PI_CODING_AGENT_DIR`, `DSH_HOME`, `DSH_AGENTS_HOME`). Windows commands are dropped when a path or URL contains `$`, a backtick or `%`, which PowerShell or cmd.exe would change inside double quotes. Install refusals are marked (`ErrRefused`, exit 9) apart from file-system errors (exit 12). The spec now states the symbolic-link rule the code implements, and `docs/agents.md` says that the Codex check wrote and listed the entry without connecting. Claude Code 2.1.332, pi 1.6.0 and opencode 1.18.18 were checked to pass their whole environment to stdio servers. Twenty further mutations each made a test fail. Rejected as out of scope: backslash paths in a caller-supplied `fs.FS` (only the embedded skill reaches `InstallSkill`), URLs such as `/mcp/` or `LOCALHOST` that the server rejects as typed, and pinning every error code in the skill.

A second round checked closure and looked for defects the fixes introduced. All round-1 items were closed; the new defects each got a red test. Two CLI tests read the developer's own `CLAUDE_CONFIG_DIR` and `CODEX_HOME`, and with the first set a test installed the skill into the real Claude Code directory: the tests now clear every override variable and point the home directory at a temporary one. The override test used `\work\claude`, which is not absolute on Windows and would have failed the Windows CI job: it uses temporary directories now. A named pipe in place of an installed file blocked the comparison: only regular files are read, anything else counts as changed. opencode 2.x takes `OPENCODE_CONFIG_DIR` as its global directory (source) and 1.18.18 loaded a server from it (checked), so it now wins over `XDG_CONFIG_HOME`. An `XDG_CONFIG_HOME` equal to `$HOME/.config` splits nothing and no longer gets a Codex entry or warning. The host-key texts overstated their premises: an already known key needs no flag, a run with SSH already on stops before any write, and a changed key is refused either way; the skill, the server instructions, the documentation and the spec now say so, and the refused-entry wording in the spec and usage matches the code.

## Required final gates

```bash
CGO_ENABLED=1 go test ./... -count=1 -race -cover
go vet ./...
go build ./cmd/routerctl
make build-all
```

Real-device tests are intentionally not claimed or inferred from these offline gates.
