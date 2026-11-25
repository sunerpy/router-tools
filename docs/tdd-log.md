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

## Required final gates

```bash
CGO_ENABLED=1 go test ./... -count=1 -race -cover
go vet ./...
go build ./cmd/routerctl
make build-all
```

Real-device tests are intentionally not claimed or inferred from these offline gates.
