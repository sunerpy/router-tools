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

- Sidebar: brand row, grouped navigation (设备 / 高风险 / 记录; only the navigation list scrolls), the current-target card, and three square buttons (连接设置, 主题, 设计原语).
- Page header: title and one-sentence description on the left, the operation status badge on the right. It never scrolls away.
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
- `StatusBadge`: maps an operation status to `AppBadge`: succeeded 成功, running 运行中, partial 部分完成, unknown 结果未知, failed 失败, skipped 已跳过, idle 未运行, canceling 正在取消, canceled 已取消; anything else shows the raw value in neutral.
- `AppToggle`: switch built on a native checkbox with `role="switch"`, persistent label and optional note.
- `AppSegmented`: native radio group drawn as a segmented control.
- `FormField`, `TextInput`, `SecretInput`: persistent label, hint, required marker, linked error, disabled and read-only states, show/hide secret control.
- `AppCard`: optional eyebrow, title, actions and body; flat.
- `AppAlert`: info / warning / error banner with icon, title and an actionable sentence.
- `StatTile`: icon chip, title, optional mono meta, value and sentence; tones ok / info / neutral / warn / danger and the single highlight tile.
- `AppStepper`: ordered steps with done / current / pending and `aria-current="step"`.
- `HistoryList`: operation rows with icon chip, title, mono meta, status badge and an optional action slot.
- `ConfirmDialog`: ordinary and deep variants, focus management, explicit cancel, no default destructive action.
- `OperationProgress`: stage list with completed / running / failed states, bounded overflow and the non-retry warning.
- `ReportViewer`: formatted JSON, copy and download actions, long-content wrapping and independent scroll.
- `EmptyState`: explains the next concrete action rather than displaying a spinner.

## 4. Interaction and state

- Buttons disable while their own operation is pending; navigation remains available unless an operation explicitly blocks unsafe context switching.
- Form validation is per-form before calling Go or opening a confirmation (`useFieldErrors`): failed rules show under their fields, link through `aria-describedby`, and focus moves to the first invalid field. Persistent labels remain after input; required inputs carry `aria-required`.
- Option switches are `AppToggle`; a switch that enables a plaintext protocol carries the note 明文.
- Ordinary changes require a confirmation dialog each time; there is no remembered "always allow".
- Deep persistence is a dedicated flow shown with `AppStepper`: evidence checklist and backup/run paths → credentials and Telnet risk acknowledgement → exact `RD08-DEEP` → final destructive confirmation. The checklist names the checks the tool performs before any write; it never shows them as passed before a run has produced that evidence. No default selection and no ordinary-yes shortcut.
- Host-key confirmation displays address, algorithm, and full SHA-256 fingerprint with accept-and-save (trust on first use: the key is written to known_hosts) or cancel. Changed keys cannot be confirmed away.
- Partial/unknown results keep the report visible and show "do not retry from the beginning" where applicable; a canceled run that may have written keeps its partial or unknown status.
- Password fields and the deep acknowledgement are cleared immediately after the backend call is dispatched, when a confirmation is canceled, and when the page that holds them unmounts. The deep flow's Telnet acknowledgement is its own switch, unset whenever the page opens. None of these is stored in localStorage or report JSON.
- Only target/profile/host-key preferences and the appearance preference may use localStorage. No secrets or deep acknowledgement are persisted.
- Device summaries, port tiles, the next-step suggestion and recent operations are derived from reports the backend returned in this session and from the target each run was sent to, never from the edited form; before a report exists they show an empty state, never placeholder data. A newer partial or unknown write makes the next step "view the report, do not start over".
- The operation stage list shows one row per stage: a stage is completed once the next one starts, and the terminal result belongs to the stage that was running.

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
