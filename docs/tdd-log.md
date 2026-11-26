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

## Required final gates

```bash
CGO_ENABLED=1 go test ./... -count=1 -race -cover
go vet ./...
go build ./cmd/routerctl
make build-all
```

Real-device tests are intentionally not claimed or inferred from these offline gates.
