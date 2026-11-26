# Router Tools design system

Status: **locked 2026-11-25**, replacing the Naive-derived v1 system. The user chose option B (白瓷) as the default theme and option C (石墨) as the dark theme from the design drafts of 2026-11-25. Colour, radius, shadow and font values come from the Voltip design system (`packages/ui/src/tokens.css`, themes `light` and `graphite`). Values marked _project_ are additions for this app; every value below is a token in `cmd/routergui/frontend/src/styles/tokens.css`, and components use only these tokens.

## 1. Visual foundation

### Themes

- Two themes share one component set: `light` (白瓷, the default) and `dark` (石墨). `data-theme` on `<html>` selects the palette; `color-scheme` follows it.
- The appearance preference is `light`, `dark` or `system`. New installs use `light`. `system` follows `prefers-color-scheme` and updates live. The preference is stored under the localStorage key `router.theme`.
- On Windows the native title bar follows the resolved theme through the Wails runtime (`WindowSetLightTheme`, `WindowSetDarkTheme`, `WindowSetSystemDefaultTheme`).
- Naive UI components (dialogs and their inputs) receive the same palette through `NConfigProvider`: Naive's dark theme in `dark`, and overrides read from the tokens.

### Colour tokens

| Token                      | 白瓷 `light`          | 石墨 `dark`           | Use                                              |
| -------------------------- | --------------------- | --------------------- | ------------------------------------------------ |
| `--rt-canvas`              | `#f2f3f6`             | `#22252b`             | Window background                                |
| `--rt-surface`             | `#ffffff`             | `#2a2e35`             | Cards, dialogs, report panels                    |
| `--rt-sidebar`             | `#f7f8fa`             | `#262a30`             | Floating sidebar (_project_)                     |
| `--rt-inset`               | `#f7f8fa`             | `#333840`             | JSON, code, chips, step chips                    |
| `--rt-inset-strong`        | `#eef0f3`             | `#3a4048`             | Neutral badges, icon chips                       |
| `--rt-field`               | `#ffffff`             | `#262a30`             | Inputs, secondary buttons (_project_ dark value) |
| `--rt-nav-active`          | `#e6e9ee`             | `#333840`             | Current navigation row, selected list row        |
| `--rt-border`              | `#e1e4ea`             | `#414750`             | Card and row hairlines                           |
| `--rt-border-strong`       | `#d6dae1`             | `#4c535d`             | Field and secondary-button edges                 |
| `--rt-fg`                  | `#1b1d22`             | `#eceef1`             | Body text, titles                                |
| `--rt-fg-muted`            | `#5f6570`             | `#a5acb6`             | Secondary text, hints, labels of meta rows       |
| `--rt-primary`             | `#1b1d22`             | `#eceef1`             | Primary action fill (ink)                        |
| `--rt-primary-fg`          | `#ffffff`             | `#22252b`             | Text on the primary fill                         |
| `--rt-accent`              | `#339cff`             | `#339cff`             | Accent fills that carry no text                  |
| `--rt-accent-soft`         | `#e7f3ff`             | `#2c4b6a`             | Info badges, accent chips                        |
| `--rt-accent-text`         | `#036dc0`             | `#83befe`             | Links, focus ring, info text, active nav icon    |
| `--rt-toggle-on`           | `#036dc0`             | `#339cff`             | Switch track when on (_project_ light value)     |
| `--rt-toggle-off`          | `#c4c9d2`             | `#4c535d`             | Switch track when off (_project_)                |
| `--rt-ok`                  | `#2e9e62`             | `#3eddb2`             | Status lamps                                     |
| `--rt-ok-soft`             | `#e4f4ea`             | `#263d3a`             | Success badges                                   |
| `--rt-ok-text`             | `#1f7a4a`             | `#3eddb2`             | Success text and icons                           |
| `--rt-warning`             | `#b7791f`             | `#f2c14e`             | Warning lamps                                    |
| `--rt-warning-soft`        | `#fbf1dc`             | `#3d3826`             | Partial / unknown badges and callouts            |
| `--rt-warning-text`        | `#8a5a10`             | `#f2c14e`             | Warning text and icons (_project_ light value)   |
| `--rt-danger`              | `#c93a33`             | `#f0736a`             | Destructive action fill, failure lamps           |
| `--rt-danger-soft`         | `#f9e7e5`             | `#3f2b2a`             | Failure badges, high-risk banner                 |
| `--rt-danger-text`         | `#b3261e`             | `#f0736a`             | Failure text and icons (_project_ light value)   |
| `--rt-danger-fg`           | `#ffffff`             | `#22252b`             | Text on the destructive fill                     |
| `--rt-highlight-from`/`to` | `#262c3a` → `#1b1d22` | `#2c4b6a` → `#2a2e35` | The single "next step" tile                      |
| `--rt-highlight-fg`        | `#ffffff`             | `#eceef1`             | Text on the highlight tile                       |
| `--rt-highlight-muted`     | `#b4bcc8`             | `#b2c2d6`             | Secondary text on the highlight tile             |

Derived tokens: `--rt-hover` and `--rt-pressed` are `color-mix(in srgb, var(--rt-fg) 6% / 10%, transparent)`, so one rule serves every surface in both themes. Stat tiles tint their surface with their tone: `color-mix(in srgb, <tone> var(--rt-tile-tint), var(--rt-surface))` with a border mixed at `--rt-tile-edge` (light 5 % / 22 %, dark 8 % / 28 %). The highlight tile adds one radial accent glow (light 55 %, dark 40 %) over its gradient; it is the only gradient in the app.

Status is always icon + text, never colour alone. Destructive colours appear only on destructive actions, failure states and the deep-persistence risk banner.

### Typography

- UI: `"Instrument Sans Variable", "Noto Sans SC Variable", "PingFang SC", "Microsoft YaHei UI", system-ui, sans-serif`.
- Mono (addresses, ports, profile IDs, stages, JSON, `boot_id`): `"JetBrains Mono Variable", "Noto Sans SC Variable", ui-monospace, "Cascadia Mono", Menlo, Consolas, monospace`, with tabular numbers.
- All three families are bundled through `@fontsource-variable/*` (SIL OFL 1.1); nothing is fetched from the network.
- Sizes: 12 auxiliary and badges, 13 labels and meta rows, 14 body and controls, 16 card titles, 18 dialog titles, 22 tile values, 26 page titles. Weights 400 regular, 500 medium, 600 strong. Line height 1.5; titles 1.25.
- Long identifiers and JSON wrap with `overflow-wrap: anywhere`; truncation alone is not an accessible full-value strategy.

### Radius, spacing, controls, elevation

- Radius: `--rt-r-panel` 10 px (cards, tiles, sidebar, dialogs, icon chips), `--rt-r-control` 6 px (fields, buttons, rows, chips), pill 999 px (badges, switches).
- Spacing scale: 4, 8, 12, 16, 20, 24, 32 px. The same relationship uses the same step; 16 px is the default gap between cards and the shell gutter.
- Controls: 40 px fields and buttons, 32 px compact icon buttons, at least 44 px on coarse pointers.
- Icons: Lucide only, stroke 1.75 (set once in `base.css`): 14 px in badges, compact buttons, inline links and checkmarks; 16 px inline with body text and in stat-tile chips; 18 px in navigation, list and empty-state chips; 20 px in alert and dialog headings. An icon is always next to a label unless its control has an accessible name.
- Cards are flat: 1 px `--rt-border`, no shadow. Only overlays are elevated: `--rt-shadow-pop` for menus and `--rt-shadow-dialog` for dialogs.
- Selected states never rely on a background tint alone: a selected segment adds a checkmark, a strong weight and a `--rt-border-strong` outline; a selected list row adds a `--rt-accent-text` indicator bar and a strong title.
- The current step of `AppStepper` uses the accent and primary colours; required markers use `--rt-fg-muted`. Destructive colours stay reserved for destructive actions, failures and the deep-persistence banner.
- No glow, texture, decorative illustration, ambient animation or marketing hero.

## 2. Layout and scroll ownership

Primitives: `fixed-sidenav-shell`, `scroll-body-shell`, `split`, `tile-grid`, `cluster`, `list-detail`, `overlay-stack`.

```text
.app-shell        grid-template-columns: 232px minmax(0, 1fr); gap 16; padding 16; height 100vh
.app-sidebar      floating panel: grid-template-rows: auto minmax(0, 1fr) auto auto
.app-workspace    grid-template-rows: auto minmax(0, 1fr)
```

- Sidebar: brand row, grouped navigation (设备: 向导, 维护 / 高风险: 深度固化 / 集成: Agent 接入 / 记录: 运行报告, 设置; only the navigation list scrolls), the current-target card, and square buttons (主题, and 设计原语 in development builds only). The Agent 接入 row shows its state as a word in the count position: 已开启 while the agent service listens, 可写入 when router writes are on too, nothing when it is off. The step-by-step routes of v1 (`/enable`, `/persistence`, `/backup`) redirect to `/`.
- Page header: title and one-sentence description on the left, the operation status badge on the right: the guided run on 向导, 运行中 while an agent operation runs (else 未运行) on Agent 接入, and the window's current operation elsewhere. It never scrolls away.
- `.operation-content` is the only routine page-scroll owner (`min-height: 0`, `overflow: auto`), centred at `--rt-content-max` 1200 px.
- `split`: `minmax(0, 1.65fr) minmax(0, 1fr)` above 1024 px, one column below. `tile-grid`: four tracks above 1024 px, two up to 480 px, one below.
- `list-detail` (运行报告): history list and report side by side above 1024 px, stacked below.
- Report JSON: independent bounded scroll at 320 px.
- Grid and flex tracks use `minmax(0, 1fr)`; every bounded scroll child has `min-height: 0`.
- At 768 px and below the shell becomes one column: the sidebar turns into a top region (brand and square buttons on one row, navigation wrapping beneath, target card hidden) and there is no horizontal page scrollbar. At 480 px and below forms are one column and primary and destructive actions fill the width. The target form switches layout by its own width (container queries) because it appears in both a narrow column and a full-width card.
- Dialogs: `--rt-dialog-width` 480 px with a 32 px viewport margin.
- Content stress requirements: empty state, 80-character unbroken identifier, long error paragraph, 2,000-line JSON, open dialog, and busy / partial / unknown / canceled states, in both themes.

## 3. Reusable primitives and showcase

`/primitives` shows every primitive in the current theme, has its own theme switch, and must be verified in both themes before product screens.

- `AppButton`: primary (ink) / secondary (outline) / ghost / danger; default and `sm`; default, hover, focus, active, disabled and loading states; accessible busy state.
- `AppIconButton`: 32 px square (44 px on coarse pointers), required accessible name, same states as `AppButton` including loading with `aria-busy`.
- `AppBadge`: pill with icon + text; tones ok / warn / danger / info / neutral.
- `StatusBadge`: maps an operation status to `AppBadge`: succeeded 成功, running 运行中, partial 部分完成, unknown 结果未知, failed 失败, skipped 已跳过, not_run 未执行, idle 未运行, canceling 正在取消, canceled 已取消; anything else shows the raw value in neutral.
- `AppToggle`: switch built on a native checkbox with `role="switch"`, persistent label and optional note.
- `AppSegmented`: native radio group drawn as a segmented control.
- `FormField`, `TextInput`, `SecretInput`: persistent label, hint, required marker, linked error, disabled and read-only states, show/hide secret control.
- `AppCard`: optional eyebrow, title, actions and body; flat.
- `AppAlert`: info / warning / error banner with icon, title and an actionable sentence.
- `StatTile`: icon chip, title, optional mono meta, value and sentence; tones ok / info / neutral / warn / danger and the single highlight tile.
- `AppStepper`: ordered steps with done / current / pending and `aria-current="step"`.
- `HistoryList`: operation rows with icon chip, title, mono meta, status badge and an optional action slot. A row an agent started carries an `AppBadge` 「Agent」 (info, Bot icon) before its status; agent tool rows use the icon of the use case they call.
- `CodeBlock`: a labelled monospace block for text the user copies (configuration snippets, commands, a new access token): a heading row with the label and a 复制 button (`AppButton` `sm`, accessible name 复制<label>, 已复制 with a check icon after copying, announced politely), and the content in `--rt-inset` with `white-space: pre-wrap`, `overflow-wrap: anywhere`, and independent scroll beyond `--rt-report-max-height`. It never truncates.
- `ConfirmDialog`: ordinary and deep variants, focus management, explicit cancel, no default destructive action.
- `OperationProgress`: stage list with completed / running / failed states, bounded overflow and the non-retry warning.
- `ReportViewer`: formatted JSON, copy and download actions, long-content wrapping and independent scroll.
- `EmptyState`: explains the next concrete action rather than displaying a spinner.
- `DeviceSelector`: two native selects, 型号 (自动识别 or a catalog model) and 固件 (that model's registered firmware), producing `auto` or one exact profile ID. A stored profile ID that is no longer registered shows an inline warning and is kept until the user picks another; it is never rewritten silently.
- `PlanStepList`: the ordered guided steps as a list. Each row has an icon chip, the step title, an action badge (将执行 / 已满足 / SSH 可用后判断 / 需要输入 / 已阻止 / 不支持), the reason as a sentence, and the effects the step would have. In result mode the badge is the step's `StatusBadge` (成功, 失败, 部分完成, 结果未知, 已跳过, 未执行) and a failed step shows its error code and message.
- `CredentialState`: one badge per saved password kind: 已保存 (ok), 已被拒绝 (warn) or 未保存 (neutral), with the kind as text. It never shows or reveals a value.

## 4. Interaction and state

- Buttons disable while their own operation is pending; navigation remains available unless an operation explicitly blocks unsafe context switching.
- Form validation is per-form before calling Go or opening a confirmation (`useFieldErrors`): failed rules show under their fields, link through `aria-describedby`, and focus moves to the first invalid field. Persistent labels remain after input; required inputs carry `aria-required`.
- Option switches are `AppToggle`; a switch that enables a plaintext protocol carries the note 明文.
- Ordinary changes started from 维护 require a confirmation dialog each time; there is no remembered "always allow".
- A guided run has exactly one confirmation dialog. It lists every planned write with its effects in plain words (including the root password change, the committed NVRAM switches, the managed boot script and firewall include, and the reboot), marks steps decided once SSH works, and states that the run stops at the first step that does not succeed. It names the router the plan was detected for (address and ports, hardware, firmware, profile). Default focus is 取消, and the confirmation is never remembered. Changing the address, ports, device, goals or 使用已保存的密码 after detection withholds the run until the next detection; a running detection or run offers 取消. The run sends the plan's confirmation back unchanged; when the run reports that the plan changed, the window shows the fresh detection instead of retrying.
- The guided page asks only for the inputs the current plan needs: the Web admin password and the new root password (twice, `autocomplete="new-password"`) when the enable step runs, otherwise the current root password (`autocomplete="current-password"`). A password the vault provides is shown as `CredentialState`, not as a field. Resetting an unknown root password is its own explicit choice with the effect stated next to it.
- 使用已保存的密码 is on and 记住密码 is off when the page opens; neither is stored. Remembering happens only after the router accepted the password.
- When detection is blocked by an unacknowledged uncertain change, the page shows the router, the recorded operation, its status and times, the instruction to check the router as RECOVERY.md describes, and an acknowledgement action with its own confirmation. When the firmware is not registered, the page lists the model's registered firmware; switching to a profile is offered only for an exact registered match and only on the user's click.
- Deep persistence is a dedicated flow shown with `AppStepper`: evidence checklist and backup choice → credentials and Telnet risk acknowledgement → the resolved profile's exact acknowledgement phrase → final destructive confirmation. The phrase and the number of reboots come from the profile `DeepStatus` resolved; they are displayed and never prefilled. The journal directory is the managed one and is shown read-only; the backup is chosen from the verified managed backups or another directory picked with the system dialog. The checklist names the checks the tool performs before any write and shows what `DeepStatus` read (journal present, flags already set, crash partition readable); it never shows a run's checks as passed before the run produced that evidence. No default selection and no ordinary-yes shortcut.
- Host-key confirmation displays address, algorithm, and full SHA-256 fingerprint with accept-and-save (trust on first use: the key is written to known_hosts) or cancel. Changed keys cannot be confirmed away.
- Partial/unknown results keep the report visible and show "do not retry from the beginning" where applicable; a canceled run that may have written keeps its partial or unknown status.
- Password fields and the deep acknowledgement are cleared immediately after the backend call is dispatched, when a confirmation is canceled, and when the page that holds them unmounts. The deep flow's Telnet acknowledgement is its own switch, unset whenever the page opens. None of these is stored in localStorage or report JSON.
- Only target/profile/host-key preferences and the appearance preference may use localStorage. No secrets or deep acknowledgement are persisted by the window. Saved passwords exist only in the Go vault, saved after the user opts in and the router accepted them; the window receives only whether a password is saved or was rejected, never its value.
- The sidebar's target status and the report list are derived from reports the backend returned in this session and from the target each run was sent to, never from the edited form; before a report exists they show an empty state, never placeholder data. Partial or unknown results keep "do not start over" on the guided result card and the deep-persistence record card.
- The operation stage list shows one row per stage: a stage is completed once the next one starts, and the terminal result belongs to the stage that was running.

### Agent 接入 (`/agents`)

- The page is one `.stack` of cards: 服务, 访问令牌, 连接 Agent, Agent 操作, 最近的调用. The service card edits a draft: 允许 Agent 连接 (`AppToggle`), 监听地址 (a native select: 仅本机 127.0.0.1, 所有网卡 0.0.0.0, and each address of this computer), 端口 (1024–65535), and 允许 Agent 写入路由器 (`AppToggle`). 应用更改 applies the whole form in one call and 撤销更改 restores the stored settings; both are disabled while nothing changed, and 应用更改 shows its pending state. The status line above the form always reports what actually listens (正在监听 with the address and whether writes are on, 已开启，但没有在监听, or 已关闭), never the draft; the listener's last error appears in an `AppAlert` below it (上次的设置没有生效), and a failed apply keeps that line and shows its error once in an `AppAlert` (没有应用更改).
- An address other than loopback shows a warning `AppAlert` (明文: the token and tool arguments, passwords included, cross the network unencrypted). Turning writes on shows a warning `AppAlert` that names what agents may then do (run a guided plan, remove soft persistence) and what stays human-only (deep persistence, the recovery acknowledgement, forgetting passwords).
- A new access token appears exactly once: after the first 应用更改 that turns the service on without a token, or after 生成新令牌 (its `ConfirmDialog` says the old token stops working at once). It is shown in a `CodeBlock` labelled 访问令牌 under a warning that names the variable `ROUTERCTL_MCP_TOKEN`, cleared by 我已保存 and when the page unmounts, and never stored by the window. Afterwards the card shows only whether a token exists and when it was created.
- 连接 Agent: an `AppSegmented` host picker (Claude Code, Codex CLI, opencode, pi, DeepSeek Harness, WorkBuddy, 其他客户端) above the host's file as text, its snippet and command as `CodeBlock`s, and its notes as text. The snippets reference `ROUTERCTL_MCP_TOKEN` and never contain the token. WorkBuddy shows that it documents only stdio and names `routerctl mcp config --agent workbuddy` instead of a snippet. 安装 Skill (hosts with a documented skills directory) names the target directory; an installed copy that differs asks before replacing it (`ConfirmDialog`, the user's other files are kept). WorkBuddy and 其他客户端 offer 导出到文件夹… (system folder picker) instead. The outcome is an `AppAlert` with the path.
- Agent 操作 lists the agent operations with a 取消 action for running ones; canceling asks first, because a canceled write can end partial and block the router. While agent operations run and the draft turns the service off (or it is off), an info `AppAlert` under the switch says they continue and points to 取消: the listener and the operations have separate lifetimes. 最近的调用 lists the newest agent calls (tool, call or finish, status, error code, time). Both use `HistoryList`, with `EmptyState` sentences when empty. They update from the `agent:status` event, not by polling.
- The unknown-host-key dialog names an operation an agent started (title 「Agent 操作遇到未知的 SSH 主机密钥」 and the operation ID); acceptance stays the user's. An agent that passed `accept_new_host_key` after asking in chat (spec §23.9(4)) skips the dialog, like the window's own 自动接受 setting. Prompts queue: one dialog at a time; every answered or withdrawn prompt is followed by a 400 ms pause, queued or not, so a double click cannot answer the next; a line says how many more wait; a prompt Go stopped waiting for (two minutes, or its operation was canceled or ended; the key is refused) is withdrawn.

## 5. Motion

Use `cubic-bezier(.4,0,.2,1)`. Routine hover, background and switch-thumb changes are 120 ms; dialogs use Naive defaults rather than custom choreography. No stage-list animated layout shifting and no theme cross-fade: transitions are suspended (`rt-theme-switching`) while the palette swaps. Under `prefers-reduced-motion: reduce`, all project transitions and animations are disabled.

## 6. Accessibility

- Full keyboard access; visible 2 px `--rt-accent-text` outline with 2 px offset, on fields too (drawn around the field box).
- Text contrast is at least 4.5:1 in both themes, and status icons, lamps and the active switch track are at least 3:1 against their surfaces. `src/styles/tokens.test.ts` computes these pairs from `tokens.css` for both themes.
- Status never depends on colour alone.
- Dialogs use Naive modal focus trapping, Escape/cancel behaviour, a labelled title and `aria-describedby` for the description; the actions come last in DOM order. Destructive confirm is never focused by default.
- Inputs have associated labels and error descriptions; placeholders never replace labels. Switches and segmented controls are native checkboxes and radios.
- Secret reveal buttons have an accessible name and do not expose values in logs/events.
- Reports are available as text for copy/download rather than screenshot-only evidence.

## 7. Accepted debt

- Hairline borders (`--rt-border`, `--rt-border-strong`) are below 3:1 against their surfaces, as in Voltip's classic themes. Fields are identified by persistent labels and their background, and the focus ring meets 3:1.
- Only the Windows title bar follows the in-app theme; Wails v2 implements the window theme calls only on Windows, so the macOS title bar follows the system appearance.
- The native window background is the light canvas, so a dark preference can show one light frame before the WebView paints.
- The bundled Noto Sans SC adds about 4.8 MB to the embedded frontend assets.
- Native packages are unsigned development artifacts until code signing/notarization is separately configured. No update channel is implied.
- Browser QA covers responsive layout and interactions, not real router behaviour. Real-device acceptance remains governed by `docs/compatibility.md`.
