# Desktop GUI (Wails)

The desktop application reuses `internal/app` through `internal/guiapi`. It does not start a local HTTP service, execute the CLI, or parse human CLI output. Every device operation uses the same profile checks, operation locks, recovery record, backups, host-key policy, and deep-persist journal rules as `routerctl`. Until the guided redesign, the desktop deep flow writes its journal to the run directory you choose rather than the managed journal directory, and there is no in-app recovery acknowledgement yet; use `routerctl recovery ack`.

## Design system and screens

`DESIGN.md` is the visual and interaction authority. Vue 3 + Naive UI + Lucide are the only UI component/icon stacks. The `/primitives` route contains the primitive showcase used before product screens, with its own theme switch.

Two themes share one component set: 白瓷 (light, the default) and 石墨 (dark), both taken from the Voltip design system. The sidebar's 主题 button toggles them and 连接设置 › 界面主题 offers light, dark or follow-system; only that choice is stored, under `router.theme`. Instrument Sans, JetBrains Mono and Noto Sans SC are bundled, so the app looks the same offline. On Windows the native title bar follows the theme.

Implemented views:

- Device discovery and Web/root verification
- Temporary SSH enable with ordinary confirmation
- Soft install/remove/reboot verification
- MTD backup create/local/online verification
- High-risk deep flow with evidence steps, exact acknowledgement, and final confirmation
- Operation history as a list with the selected report beside it
- Connection, host-key and appearance settings
- Development primitive gallery

The guided-flow bindings `GuidePlan`, `GuideApply` and `AcknowledgeRecovery` exist in `internal/guiapi`, `cmd/routergui/app.go`, `src/services/api.ts` and both APIs in `src/services/wails.ts`; the wizard that uses them replaces the step-by-step views in the next redesign. `GuidePlan` keeps no history because detection changes nothing; a guided run and an acknowledgement each add one history entry. The window passes the plan's `confirmation` back unchanged, since `plan_digest` covers it. The preview mock plans a fresh device by default and accepts the scenarios `configured`, `recovery`, `unsupported` and `failed`; `partial`, `unknown` and `failed` also shape the run result.

Requests carry `UseSavedPassword` and `RememberPassword`; the bindings `CredentialStatus`, `ForgetCredential` and `ForgetAllCredentials` return saved-password metadata only, and `GuidePlan` reports `saved_admin`/`saved_root` so a window can ask only for missing passwords. The current views neither use nor save passwords; the remember switch, saved badges and settings card arrive with the guided redesign. The preview mock keeps an in-memory vault of metadata that a successful guided run with `RememberPassword` fills.

Passwords and the deep acknowledgement are cleared after dispatch/cancel and are never persisted in localStorage. Only target/profile/public host-key preferences and the theme choice are stored. Unknown host keys display the full public fingerprint in a modal; changed keys cannot be approved away. Deep/unknown/partial results retain their reports and non-retry warnings.

## Development requirements

- Go 1.27.1
- Wails CLI 2.17.1
- Node.js and pnpm
- Linux: GTK3 and WebKit2GTK 4.1 development packages (`libgtk-3-dev`, `libwebkit2gtk-4.1-dev`)

Useful commands:

```bash
make gui-test                 # Vitest, vue-tsc, Vite, and Go guiapi tests
make gui-build                # Linux binary, webkit2_41 tag
make gui-build-windows        # Windows amd64 executable
make gui-installer-windows    # Windows amd64 executable + NSIS installer
```

Run directly:

```bash
cd cmd/routergui
wails dev -tags webkit2_41
wails build -clean -tags webkit2_41
```

`make build-all` remains the CGO-free six-platform CLI build. GUI packaging has platform WebView/CGO requirements and must not reuse those static CLI flags.

## Release packaging

`scripts/package-gui.sh windows|darwin [--version X.Y.Z] [--out DIR]` builds the release assets with `-skipbindings` and checks them before it writes them to `DIR` (default `dist-gui/`):

| Platform  | Built on                                                             | Assets                                                                                    | Checks                                                                                                                                           |
| --------- | -------------------------------------------------------------------- | ----------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| `windows` | Linux or macOS (cross-build; needs `makensis`, `7z` or `7zz`, `zip`) | `router-tools_<v>_windows_amd64_setup.exe`, `router-tools_<v>_windows_amd64_portable.zip` | the installer's `Router Tools.exe` has the same SHA-256 as the built executable; the zip contains it with `LICENSE` and `THIRD_PARTY_NOTICES.md` |
| `darwin`  | macOS only (`ditto`, `lipo`, `plutil`)                               | `router-tools_<v>_darwin_universal.zip`                                                   | the executable contains `x86_64` and `arm64`; `CFBundleShortVersionString` equals the version                                                    |

`<v>` is `info.productVersion` in `cmd/routergui/wails.json`, which release-please updates in the release pull request. `--version` makes the script fail unless the two agree, which the release workflow relies on. CI runs both platforms on every pull request without `--version`; the release workflow runs them on the release tag and attaches the assets to the draft Release together with the `routerctl` archives. Asset names contain no spaces because GitHub rewrites spaces in release asset names.

## Produced artifacts

Development artifacts in `cmd/routergui/build/bin/`, built on 2026-11-25 at 17:10 with `make gui-build`, `make gui-build-windows` and `make gui-installer-windows`. They contain the 白瓷/石墨 redesign and every fix through `docs/tdd-log.md` §28. Git ignores `cmd/routergui/build/`, so rebuild them after any frontend or Go change before treating them as current.

- `routergui`: Linux amd64 desktop binary, linked to WebKit2GTK 4.1.
- `routergui-windows-amd64.exe`: Windows amd64 desktop executable.
- `routergui-windows-amd64-setup.exe`: the executable that `make gui-installer-windows` builds before packaging it; byte-identical to `routergui-windows-amd64.exe`.
- `Router Tools-amd64-installer.exe`: Windows amd64 NSIS installer. The `Router Tools.exe` it installs has the same SHA-256 as `routergui-windows-amd64-setup.exe` (checked by extracting it with 7-Zip).
- `SHA256SUMS`: checksums of the four files above with repository-relative paths. Verify them from the repository root with `sha256sum -c cmd/routergui/build/bin/SHA256SUMS`.

The Linux binary was launched under Xvfb and kept running for 15 seconds without a startup crash. `docs/qa/routergui-linux-xvfb-1280.png` is its window as WebKitGTK 2.52.6 rendered it through the real bindings, so it carries no preview label. Windows executables and the NSIS installer were generated successfully; Windows UI execution was not tested in this environment. macOS packaging was not produced because Wails reports that cross-compiling to macOS is unsupported; use a native macOS runner. These and the published release assets are unsigned and not notarized; code signing and notarization are not configured yet (see README).

## Frontend tests and browser QA

- 296 Vitest cases across 14 files. 176 are parameterized design checks: `tokens.test.ts` computes WCAG contrast for every text pair (at least 4.5:1) and every status indicator (at least 3:1) in both themes, and `tokenTrace.test.ts` rejects undefined tokens, literal colours and raw pixel sizes in every component. The rest cover primitives, the theme controller, the dashboard summaries, exact acknowledgement, operation state, resolved failed/partial/unknown reports, canceled runs with and without writes, rejected calls, integer ports, preview scenarios including the guided-flow plans and runs and the saved-password vault, cancellation, secret clearing, and host-key confirmation.
- `vue-tsc` and Vite production build pass.
- Real Chrome captures at 375 / 768 / 1280 in both themes show no horizontal document overflow; the target form switches layout by its own width (container queries).
- Long report evidence: the JSON region is bounded at 320 px and scrolls independently, leaving operation controls intact.
- Deep final confirmation and unknown-host-key dialogs open with 取消 focused; the dark theme reaches the Naive dialog and its input.
- Preview mode is explicitly labelled as simulated data; production without Wails bindings fails instead of simulating success. `window.__ROUTER_PREVIEW_SCENARIO__` selects `failed`, `partial`, `unknown`, or `long` (default: success); like the bindings, the preview resolves failed, partial, and unknown reports instead of rejecting.

Evidence files under `docs/qa/` (2026-11-25 redesign, preview data):

- `primitives-{375,768,1280}.png`, `primitives-dark-1280.png`, `primitives-dialog-1280.png`
- `dashboard-{375,768,1280}.png`, `dashboard-dark-{375,1280}.png`
- `dashboard-failed-{375,768,1280}.png`, `dashboard-unknown-375.png`, `dashboard-long-report-375.png`
- `deep-step2-1280.png`, `deep-success-1280.png`, `deep-unknown-dark-1280.png`
- `hostkey-dialog-1280.png`, `reports-dark-1280.png`, `settings-1280.png`

The failed-report delivery through the real Go bindings was checked with `wails dev` before the redesign (Doctor against the closed port `127.0.0.1:1`, `WEB_UNREACHABLE`); the redesign changes presentation only, not the binding path.

Browser QA verifies rendering/interactions only. It does not establish router compatibility or real-device deep-persist success; that remains governed by `docs/compatibility.md`.
