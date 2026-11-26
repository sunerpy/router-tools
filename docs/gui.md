# Desktop GUI (Wails)

The desktop application reuses `internal/app` through `internal/guiapi`. It does not start a local HTTP service, execute the CLI, or parse human CLI output. Every device operation uses the same profile checks, operation locks, recovery record, backups, host-key policy, and deep-persist journal rules as `routerctl`.

## Design system and screens

`DESIGN.md` is the visual and interaction authority. Vue 3 + Naive UI + Lucide are the only UI component/icon stacks. Development builds add the `/primitives` route, the primitive showcase used before product screens, with its own theme switch; production builds do not register it.

Two themes share one component set: 白瓷 (light, the default) and 石墨 (dark), both taken from the Voltip design system. The sidebar's 主题 button toggles them and 设置 › 界面主题 offers light, dark or follow-system; only that choice is stored, under `router.theme`. Instrument Sans, JetBrains Mono and Noto Sans SC are bundled, so the app looks the same offline. On Windows the native title bar follows the theme.

Views (`DESIGN.md` §2):

- 向导 (`/`): choose 自动识别 or a model and one of its registered firmware versions from `Catalog()`, enter the address, and pick the goals. 开启并验证 SSH is always included; 备份全部分区 and 软固化 are on by default; 重启验证 is off by default and needs 软固化. 检测 is read-only (`GuidePlan`); a current root password typed for it is cleared as soon as detection is sent, and asked again when the run needs it. The page shows the router's state and the plan, asks only for the passwords the plan needs (the new root password twice), and runs the plan after one confirmation that names the router (address, ports, hardware, firmware, profile) and lists every write and its effects (`GuideApply`). Changing the address, ports, device, goals or 使用已保存的密码 after detection withholds the run until the next detection. Progress is shown per step; a detection or a run can be canceled; the run stops at the first step that does not succeed, and the result shows its error and links to the reports. When an uncertain earlier change blocks detection, the page names the router and the record's times and offers 我已检查路由器 (`AcknowledgeRecovery`) for that router. When the firmware is not supported, the page lists the model's registered firmware and offers a switch only to an exact registered match, on click.
- 维护 (`/tools`): remove soft persistence (with its own confirmation), verify root SSH login, verify a backup locally or against the router, and run the raw preflight.
- 深度固化 (`/deep`): a read-only status card (`DeepStatus`: profile, acknowledgement phrase, reboot count, the SSH port and root login, boot flags, the managed journal path and whether a journal exists, the verified managed backups, and the problems it found). An uncertain earlier change, an existing journal or an unreadable crash partition make the run refuse before any write; boot flags that are already or partly set and a missing managed backup are warnings, and the backup's boot match, the lock and the root shell are checked again when the run starts. Then come the evidence steps with an explicit backup choice (nothing is preselected), the page's own Telnet acknowledgement, the exact phrase from the profile, and the final confirmation. The backup must come from the router's current boot: after a reboot, including 重启验证, create a new one with `routerctl backup create`, because the guided flow skips the backup while a verified managed backup exists. The journal always goes to the managed directory.
- 运行报告 (`/reports`): this session's operations as a list, with the selected report beside it.
- 设置 (`/settings`): default connection and device, saved passwords (list, forget one, forget all), host keys, and theme.

The step-by-step routes `/enable`, `/persistence` and `/backup` redirect to `/`.

`GuidePlan` and `DeepStatus` keep no history because detection changes nothing; a guided run and an acknowledgement each add one history entry. The window passes the plan's `confirmation` back unchanged, since `plan_digest` covers it. Guided steps never use Telnet. The preview mock plans a fresh device by default and accepts the scenarios `configured`, `recovery`, `unsupported` and `failed`; `partial`, `unknown` and `failed` also shape the run result, and the deep status preview also accepts `journal`.

Requests carry `UseSavedPassword` and `RememberPassword`. 使用已保存的密码 starts on in 向导, 维护 and the deep status check, and 记住密码, offered only in 向导, starts off each time the page opens; neither choice is stored. Deep persistence itself never uses a saved password. The bindings `CredentialStatus`, `ForgetCredential` and `ForgetAllCredentials` return saved-password metadata only, and `GuidePlan` reports `saved_admin`/`saved_root` so the page asks only for missing passwords. The preview mock keeps an in-memory vault of metadata that a successful guided run with `RememberPassword` fills.

Passwords and the deep acknowledgement are cleared once the call that uses them is dispatched, when a confirmation or the run is canceled, and when their page is left; they are never persisted in localStorage. Only target/profile/public host-key preferences and the theme choice are stored. Unknown host keys display the full public fingerprint in a modal; changed keys cannot be approved away. Deep/unknown/partial results retain their reports and non-retry warnings.

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

The make targets write development artifacts to `cmd/routergui/build/bin/`, which Git ignores; rebuild them after any frontend or Go change before treating them as current.

- `make gui-build`: `routergui`, the Linux amd64 desktop binary, linked to WebKit2GTK 4.1.
- `make gui-build-windows`: `routergui-windows-amd64.exe`, the Windows amd64 desktop executable.
- `make gui-installer-windows`: `Router Tools-amd64-installer.exe`, the Windows amd64 NSIS installer, together with `routergui-windows-amd64-setup.exe`, the executable it packages.

Windows UI execution is verified by hand on a Windows machine, not in CI. macOS packaging needs a native macOS runner, because Wails does not cross-compile to macOS. These builds and the published release assets are unsigned and not notarized; code signing and notarization are not configured yet (see README).

Real bindings, checked on 2026-11-26 with `wails dev -tags webkit2_41` under Xvfb and a temporary `XDG_CONFIG_HOME`: `docs/qa/routergui-linux-xvfb-1280.png` is the WebKitGTK window, which carries no preview label. From Chrome connected to the dev server, against `127.0.0.1` with every port set to the closed port 1, `Catalog()` listed RD08 1.1.96; `GuidePlan` and `DeepStatus` resolved with `WEB_UNREACHABLE` and added no history entry; the raw preflight added one failed entry; `CredentialStatus` returned an empty vault; and `localStorage` held no password. With `-devserver 0.0.0.0:34115`, Wails dev rejected the window's requests (`Host: wails.localhost:34115`) until `WAILS_DEV_ALLOWED_HOSTS=wails.localhost` was set.

## Frontend tests and browser QA

- 327 Vitest cases across 16 files. 179 are parameterized design checks: `tokens.test.ts` computes WCAG contrast for every text pair (at least 4.5:1) and every status indicator (at least 3:1) in both themes, and `tokenTrace.test.ts` rejects undefined tokens, literal colours and raw pixel sizes in every component. The rest cover primitives (including the device selector, plan step list and credential states), the theme controller, the report summaries, exact acknowledgement, operation state, resolved failed/partial/unknown reports, canceled runs with and without writes, rejected calls, integer ports, the guided flow (goals, only the inputs a plan needs, the confirmation passed back unchanged and naming the router, secrets cleared after dispatch and when a confirmation is canceled, the password switches reset on each visit, a stale plan withheld, cancellation, failed and rejected runs shown, a partial run kept when a later step finds the router changed, recovery acknowledgement for the blocked router with its times, profile switch on click), the deep page (phrase and reboot count from the profile, never prefilled, no preselected backup, closed SSH and refused root password told apart, secrets cleared on cancel, managed journal), the maintenance and settings pages (secrets cleared on cancel, a failed forget kept visible), preview scenarios including the guided-flow plans and runs and the saved-password vault, cancellation, and host-key confirmation.
- `vue-tsc` and Vite production build pass.
- Real Chrome captures at 375 / 768 / 1280 in both themes show no horizontal document overflow, and the document itself never scrolls; only the content column does. The target form switches layout by its own width (container queries).
- Long report evidence: the JSON region is bounded at 320 px and scrolls independently, leaving operation controls intact.
- The guided confirmation, the deep final confirmation and the unknown-host-key dialog open with 取消 focused; the dark theme reaches the Naive dialog and its input.
- Preview mode is explicitly labelled as simulated data; production without Wails bindings fails instead of simulating success. `window.__ROUTER_PREVIEW_SCENARIO__` selects `failed`, `partial`, `unknown`, or `long` (default: success), plus the guided and deep status scenarios above; like the bindings, the preview resolves failed, partial, and unknown reports instead of rejecting.

Evidence files under `docs/qa/` (2026-11-26 guided redesign, preview data unless noted):

- `guide-plan-1280.png`, `guide-confirm-1280.png`, `guide-result-dark-1280.png`, `guide-recovery-768.png`, `guide-unsupported-375.png`, `guide-stale-dark-375.png`
- `tools-dark-768.png`, `deep-status-1280.png`, `deep-confirm-1280.png`, `deep-unknown-dark-1280.png`
- `reports-dark-1280.png`, `settings-saved-1280.png`, `hostkey-dialog-1280.png`
- `primitives-guide-1280.png`, `primitives-guide-dark-1280.png`
- `routergui-linux-xvfb-1280.png` (real bindings, see above)

Browser QA verifies rendering/interactions only. It does not establish router compatibility or real-device deep-persist success; that remains governed by `docs/compatibility.md`.
