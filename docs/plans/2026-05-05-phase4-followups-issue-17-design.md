# Phase 4 — follow-ups (issue #17): dark-mode chart theme + UX polish — Design

**Date:** 2026-05-05
**Status:** Draft, awaiting user approval
**Issue:** [jacaudi/cwd#17](https://github.com/jacaudi/cwd/issues/17) — "Phase 4 polish — UX issues found in dark/light review"
**Parent designs:**
- `docs/plans/2026-05-03-phase4-history-design.md` (Phase 4 v1)
- `docs/plans/2026-05-05-phase4-history-polish-design.md` (polish round)
**Branch:** `feature/p4-followups-issue-17` off `origin/main` at `e5dba8d` (Phase 4 polish-round merge)

---

## 1. Goal & scope

Close out the five UX issues that the post-merge dark/light Playwright review captured under issue #17:

1. **Major** — dark-mode chart axes/legend/gridlines barely visible across all four chart rows on `/history`. The polish-round attempt to set `theme={isDark ? 'academy' : 'classic'}` per chart wired the prop but didn't fix contrast, because `'academy'` is a stylistic preset — not a dark-mode adapter. The fix is to use the `ConfigProvider` that `@ant-design/charts` exports and pass a built-in theme `type: 'dark' | 'light'`.
2. **Minor** — SWPC alerts column chart renders empty bars over a populated time axis when every bucket sums to zero severities; should render `<Empty>`.
3. **Minor** — three "Missing encoding for channel: x." console errors emitted on `/history` in dark mode, caused by the explicit `scale: { x: { type: 'time' } }` config introduced in commit `ee0b9f0`. v2 should infer the time scale from `Date` data.
4. **Minor** — Active Alerts hazard chips on Overview have count badges that overlap the chip text, especially the gray `0` badges (visible on most categories most of the time).
5. **Minor** — Each volcano on `/events` shows two side-by-side chips (`WATCH` + `ORANGE`) that encode the same information; the alert-level chip is already colored by level.

**Not in scope (explicitly):**
- Custom G2 themes / hand-rolled token maps. The user has explicitly directed that no custom theme palettes be created — Ant Design built-in `type: 'dark' | 'light'` only.
- New chart types or new pages.
- Restructuring the AlertBadgeBar layout beyond the zero-badge fix.
- Restructuring the VolcanoList beyond the duplicate-chip drop.
- Theme-toggle persistence changes (already shipped).
- Per-chart color palette overrides — the existing series-color choices (e.g. `#52c41a` on the NWS area) stay; only G2's axis/legend/gridline rendering is re-themed via the chart-package ConfigProvider.

**Demo bar (parent §11):** the dark-mode History page is legible end-to-end; Overview's Active Alerts strip and Events' Volcanoes list are visually clean in both themes.

---

## 2. Decisions log

Resolved during the 2026-05-05 brainstorm. The user constraint "no custom themes — use Ant Design built-in themes only" is load-bearing and applies across all chart-related decisions.

| # | Question | Decision |
|---|---|---|
| 1 | Where does the chart-package `ConfigProvider` wrap? | **Page-level** in `web/src/pages/History.tsx`, around the `<HistoryChartRow>` map. Charts only live on `/history` today; if a future page adds charts, hoisting to App.tsx is a one-line change. With `key={isDark ? 'dark' : 'light'}` belt-and-suspenders to force a remount on theme toggle in case G2 memoizes its theme on mount. |
| 2 | Per-chart `theme={…}` prop disposition | **Drop in the same PR.** Once `ConfigProvider` owns theming, the prop is dead code. The `useIsDark` import in each chart component is also dropped. Per-chart tests asserting `theme=academy` get the assertion removed (along with the prop's appearance in the test mock). |
| 3 | Explicit `scale: { x: { type: 'time' } }` removal (finding 3) | **Drop entirely from `NWSAlertsArea`, `SWPCScalesLine`, `USGSQuakesScatter`.** v2 infers the time scale from the `Date` objects passed in the `at` field. **Fallback:** if Task 9 (verify) shows the missing-encoding errors persist in the browser console, switch to `axis: { x: { type: 'time' } }` per current v2 docs. (`SWPCAlertsBars` does not have `scale.x.type` today — leave it alone.) |
| 4 | SWPC alerts empty-state semantics (finding 2) | **Render `<Empty>` when `buckets.every(b => b.warning + b.watch + b.alert === 0)`.** The `every` check is vacuously true for empty arrays, so it strictly subsumes the existing `data.length === 0` guard. |
| 5 | AlertBadgeBar zero-badge handling (finding 4) | **`<Badge showZero={false}>`.** The chip's gray `'default'` color (line `AlertBadgeBar.tsx:47`) already conveys the inactive state; the gray `0` corner badge added clutter without information. The now-unused `color={n > 0 ? undefined : '#999'}` prop on `Badge` is dropped. |
| 6 | VolcanoList chip merge (finding 5) | **Drop the `COLOR_CHIP` chip.** The `ALERT_COLOR` mapping and `COLOR_CHIP` mapping are 1:1 (NORMAL/GREEN, ADVISORY/YELLOW, WATCH/ORANGE, WARNING/RED), so the second chip is always a redundant restatement. The level chip's color carries the same signal. |
| 7 | Test strategy under chart-package `ConfigProvider` | **Per-chart tests:** `vi.mock('@ant-design/charts', ...)` continues to mock only the chart primitives (Area, Line, Column, Scatter); each chart component no longer imports `ConfigProvider`, so no mock entry needed. **HistoryChartRow.test.tsx:** add a passthrough `ConfigProvider` to its existing `vi.mock('@ant-design/charts', ...)` because it imports real chart components transitively. **History.test.tsx (or a new wrapper-focused test):** uses a recording mock that asserts the wrapper receives `common.theme.type === 'dark'` when `useIsDark` returns true. |
| 8 | Scope discipline | **One cohesive plan.** All five findings share the chart-package familiarity (1, 2, 3) and AntD-Tag styling familiarity (4, 5). Splitting into multiple plans would add coordination overhead without changing implementation rhythm. Sized similarly to the polish round. |
| — | Conventional-commit prefix | `feat(p4-followups):`, `fix(p4-followups):`, `chore(p4-followups):`, `docs(p4-followups):`, `test(p4-followups):` — distinguishes follow-up work from the original Phase 4 + polish (`feat(p4):`) commits in `git log --grep`. |
| — | Branch | `feature/p4-followups-issue-17` off `origin/main` at `e5dba8d`. |

---

## 3. Upstream constraints worth calling out

### 3.1 The chart-package `ConfigProvider` API is documented and version-correct

`@ant-design/charts@2.6.7` (already in `web/package.json`) exports `ConfigProvider` at the top level. The supported shape:

```tsx
import { ConfigProvider as ChartsConfigProvider } from '@ant-design/charts';

<ChartsConfigProvider common={{ theme: { type: 'dark' } }}>
  …chart subtree…
</ChartsConfigProvider>
```

`type` accepts the built-in presets `'dark'` and `'light'` (and supports overrides like `color`, `category10`, `axis.labelFill` — which we deliberately do NOT use, per the no-custom-themes constraint). Verified via context7 against the upstream `site/docs/manual/config-provider.en.md` doc.

Aliasing the import as `ChartsConfigProvider` is a readability convention — there's no name clash with antd's `ConfigProvider` in `web/src/pages/History.tsx` today, but the alias makes the call site self-documenting and prevents accidental confusion if a future `History.tsx` ever needs both.

### 3.2 `useIsDark()` continues to work inside the wrapper

`useIsDark()` in `web/src/theme.ts` calls `theme.useToken()` from `antd`, which reads tokens from the nearest enclosing **antd** `ConfigProvider`. The chart-package `ConfigProvider` is a different React Context — it sits inside antd's wrapper (which is in `App.tsx:116`), not in place of it. Wrapping the `<HistoryChartRow>` map in `<ChartsConfigProvider>` does not displace antd's tokens; `useIsDark()` returns the same value it returned before.

The single `useIsDark()` call lives in `History.tsx` (the wrapper's parent component). After Decision 2, no chart component or `HistoryChartRow` calls `useIsDark()` directly.

### 3.3 Theme toggle re-applies cleanly

When the user toggles theme via `SettingsDrawer`, antd's algorithm flips → `colorBgBase` token changes → `useIsDark()` re-evaluates → `History.tsx` re-renders with a new `common.theme.type`. The chart-package's `ConfigProvider` is a normal React component, so it should re-render its subtree on prop changes. Belt-and-suspenders: pass `key={isDark ? 'dark' : 'light'}` so the wrapper unmounts + remounts on toggle, forcing G2 to pick up the new theme even if it caches its theme on first mount. Cost: a brief flash on toggle (5 chart instances re-mount). Acceptable — theme toggle is a rare event.

### 3.4 Protected files contract (Phases 1–4 carryover)

The following files MUST remain byte-identical to BASE (`origin/main` at `e5dba8d`) for the entire branch life:

- `internal/cache/cache.go`
- `internal/sse/hub.go`
- `internal/webdist/dist/index.html` (committed Vite-build placeholder)
- `internal/sources/*.go` (every file)

This round changes only frontend code and (potentially) frontend tests + docs. No backend changes are required by any of the five findings. The protected-files contract is honored trivially.

### 3.5 `web/node_modules` is missing in the local checkout

The implementation worktree must run `task web:install` (or the project-equivalent `pnpm install`) before running `task web:test` / `task web:typecheck`. Note in plan setup; not a design-level concern.

---

## 4. File map

```
docs/plans/
  2026-05-05-phase4-followups-issue-17-design.md          (this file — new)
  2026-05-05-phase4-followups-issue-17-implementation.md  (sibling — written next session)

web/src/
  pages/
    History.tsx                                            (modify — wrap row map in ChartsConfigProvider)
    History.test.tsx                                       (modify — recording-mock test for the wrapper)
  components/
    AlertBadgeBar.tsx                                      (modify — Badge showZero={false}, drop dead color prop)
    AlertBadgeBar.test.tsx                                 (modify or new — assert no zero badges render)
    VolcanoList.tsx                                        (modify — drop the COLOR_CHIP chip)
    VolcanoList.test.tsx                                   (modify or new — assert single chip per row)
    HistoryChartRow.test.tsx                               (modify — add ConfigProvider passthrough mock)
    charts/
      NWSAlertsArea.tsx                                    (modify — drop useIsDark, theme prop, scale.x.type)
      NWSAlertsArea.test.tsx                               (modify — drop theme=academy assertion + theme mock)
      SWPCScalesLine.tsx                                   (modify — drop useIsDark, theme prop, scale.x.type)
      SWPCScalesLine.test.tsx                              (modify — same)
      SWPCAlertsBars.tsx                                   (modify — drop useIsDark, theme prop; replace empty guard with every-zero check)
      SWPCAlertsBars.test.tsx                              (modify — same + new every-zero test)
      USGSQuakesScatter.tsx                                (modify — drop useIsDark, theme prop, scale.x.type)
      USGSQuakesScatter.test.tsx                           (modify — same)
```

**No backend (Go) files change.** No new components are added; no components are deleted. `useIsDark` stays in `web/src/theme.ts` (still used by other potential consumers; only the per-chart usages go away).

---

## 5. Internal contracts

### 5.1 History page — `ChartsConfigProvider` wrapping pattern

```tsx
// web/src/pages/History.tsx (sketch — final form lives in implementation plan)
import { ConfigProvider as ChartsConfigProvider } from '@ant-design/charts';
import { useIsDark } from '../theme';

export default function History() {
  const isDark = useIsDark();
  // ...existing window-toggle + URL-sync logic unchanged...
  return (
    <div style={{ padding: 24 }}>
      {/* header + Segmented unchanged */}
      <ChartsConfigProvider
        key={isDark ? 'dark' : 'light'}
        common={{ theme: { type: isDark ? 'dark' : 'light' } }}
      >
        {SOURCES.map((s) => (
          <HistoryChartRow key={s} source={s} window={window} />
        ))}
      </ChartsConfigProvider>
    </div>
  );
}
```

The `key` prop forces a remount on theme toggle — guards against the (unverified) possibility that G2 caches its theme on first mount and ignores subsequent `common.theme.type` changes. If a future investigation proves the package reacts correctly to prop changes alone, the `key` can be removed; until then it's cheap insurance.

### 5.2 Per-chart simplification pattern

Each of `NWSAlertsArea`, `SWPCScalesLine`, `SWPCAlertsBars`, `USGSQuakesScatter`:

- **Drop** the `import { useIsDark } from '../../theme';` line.
- **Drop** the `const isDark = useIsDark();` line.
- **Drop** the `theme={isDark ? 'academy' : 'classic'}` prop on the chart primitive.
- **Drop** the `scale.x.type: 'time'` clause (Area, Line, Scatter only — `SWPCAlertsBars`/`Column` doesn't have it). Other `scale` clauses (e.g. `y.domainMin`, `color.range`, `size.range`) stay.
- Existing series-color styles (`fill: '#52c41a'`, `colorField` palettes, etc.) stay unchanged.

### 5.3 SWPC alerts empty-state

```tsx
// web/src/components/charts/SWPCAlertsBars.tsx
if (data.every((b) => b.warning + b.watch + b.alert === 0)) {
  return <Empty description="No alerts in this window" />;
}
```

Replaces the existing `if (data.length === 0) return <Empty …/>;`. The `every` predicate is vacuously true on an empty array, so the new guard subsumes the old one.

### 5.4 AlertBadgeBar zero-badge

```tsx
// web/src/components/AlertBadgeBar.tsx (chip render block)
<Tooltip key={key} title={label}>
  <span aria-label={label}>
    <Badge count={n} showZero={false}>
      <Tag
        color={color}
        onClick={onClick}
        style={{ cursor: onClick && n > 0 ? 'pointer' : 'default' }}
      >
        {label}
      </Tag>
    </Badge>
  </span>
</Tooltip>
```

Diff from current:
- `showZero` flips from `true` (implicit when `count={0}` was rendered) to `false`.
- The `color={n > 0 ? undefined : '#999'}` prop on `Badge` is removed (dead now).
- `n` is still computed and passed as `count`; the badge just suppresses itself when zero.
- The aria-label, Tooltip, and chip color logic are unchanged.

### 5.5 VolcanoList — drop the color chip

```tsx
// web/src/components/VolcanoList.tsx (Space block)
<Space size="small" wrap>
  <Typography.Text type="secondary">{v.region}</Typography.Text>
  <Tag
    color={ALERT_COLOR[v.alert] ?? 'default'}
    data-alert={v.alert}
    data-testid="volcano-alert-tag"
  >
    {v.alert}
  </Tag>
  {v.updatedAt && (
    <Typography.Text type="secondary">· {relative(v.updatedAt)}</Typography.Text>
  )}
</Space>
```

The `<Tag color={COLOR_CHIP[v.color] ?? 'default'}>{v.color}</Tag>` line is deleted. The `COLOR_CHIP` map at the top of the file becomes unused — delete it. The `ColorCode` import from `../api/types` may also become unused; verify and drop accordingly. Do NOT remove `ColorCode` from `api/types` itself (out of scope; other files may still reference it).

---

## 6. Frontend layout / composition changes

| Page | Component | Change |
|---|---|---|
| `/history` | `pages/History.tsx` | Wrap `<HistoryChartRow>` map in `<ChartsConfigProvider key={isDark} common={{theme:{type}}}>`. |
| `/history` | `components/charts/{NWSAlertsArea,SWPCScalesLine,SWPCAlertsBars,USGSQuakesScatter}.tsx` | Drop `useIsDark`, drop `theme={…}` prop, drop `scale.x.type: 'time'` (where present). |
| `/history` | `components/charts/SWPCAlertsBars.tsx` | Replace `data.length === 0` empty-state guard with `data.every(b => b.warning + b.watch + b.alert === 0)`. |
| `/` (Overview) | `components/AlertBadgeBar.tsx` | `<Badge showZero={false}>`; drop the dead `color={…}` prop. |
| `/events` | `components/VolcanoList.tsx` | Drop the second (`COLOR_CHIP`) `<Tag>`. Delete the unused `COLOR_CHIP` map. |

All other layout, typography, and component composition stays the same. `HistoryChartRow.tsx` itself does NOT change (it doesn't import `useIsDark` or set chart `theme`).

---

## 7. Configuration

No new config. No changes to `/api/uiconfig`, no changes to backend YAML, no changes to env vars. The `ChartsConfigProvider` reads its `type` value from `useIsDark()` at runtime — the existing `cwd.themeMode` localStorage key + `defaultTheme` from `/api/uiconfig` continue to drive the theme as before.

---

## 8. Testing

### 8.1 Unit (vitest + Testing Library + jsdom)

**Per-chart tests** (`charts/*.test.tsx`):
- Drop the `vi.mock('../../theme', () => ({ useIsDark: () => true }))` line — no longer used.
- Drop the `theme=academy` substring assertion.
- Drop the `theme?: string` field from each test's `Area`/`Line`/`Column`/`Scatter` mock signature.
- For `SWPCAlertsBars.test.tsx` only: add a new test asserting the `<Empty>` placeholder renders when `data` is non-empty but every bucket sums to zero.

**`HistoryChartRow.test.tsx`:**
- Add `ConfigProvider: ({children}: {children: React.ReactNode}) => <>{children}</>` to its `vi.mock('@ant-design/charts', ...)` shape, since `HistoryChartRow` now renders inside the wrapper supplied by the parent. (HistoryChartRow itself doesn't import `ConfigProvider`, but its tests render it inside a wrapper-providing parent — see History.test.tsx pattern below.)

**`pages/History.test.tsx` (or a new wrapper-focused test):**
- Use a recording mock for the chart-package `ConfigProvider`:
  ```ts
  let recordedTheme: string | undefined;
  vi.mock('@ant-design/charts', () => ({
    ConfigProvider: (props: { common?: { theme?: { type?: string } }; children: React.ReactNode }) => {
      recordedTheme = props.common?.theme?.type;
      return <>{props.children}</>;
    },
    // chart primitives stub through to data-testid divs as before
  }));
  ```
- Assert: `useIsDark` mocked to return `true` ⇒ History renders ⇒ `recordedTheme === 'dark'`.
- Assert: `useIsDark` mocked to return `false` ⇒ `recordedTheme === 'light'`.

**`AlertBadgeBar.test.tsx`:**
- Assert: when a category's count is zero, no `Badge`-rendered count text appears next to the chip (i.e. the chip text "Tornado" is not adjacent to a "0" textnode under the same wrapper). Cheapest form: query for the chip's `<Tag>` element by aria-label and assert its parent doesn't contain a "0" textnode.
- Assert: when a category's count is positive, the count number IS rendered.

**`VolcanoList.test.tsx`:**
- Assert: a single `<Tag>` per volcano (find tags scoped to one `<List.Item>` and assert length 1).
- Assert: the alert-level chip's color matches `ALERT_COLOR[level]`.

### 8.2 Integration / smoke

No backend changes → no `task smoke` extension. Existing smoke continues to pass.

### 8.3 Manual verification (Task 9 of the implementation plan)

After `task web:build && ./bin/cwd serve`:
- Open `/history` in dark mode in a real browser (Chromium via Playwright is fine). Assert axes, legend text, gridlines, Y-axis titles, legend swatches, and tooltip text are all legible against the dark card background. Take a screenshot and visually compare to the polish-round dark capture under `.playwright-mcp/`. **Stop-and-escalate gate:** if any element is still illegible after the `ChartsConfigProvider type:'dark'` switch, halt the verify pass, capture a screenshot of the specific failing element(s), and surface to the user with a description of what's still wrong. Resolution happens **in this branch** (not in a follow-up), within the no-custom-themes constraint — see Risk 1 for the resolution paths.
- Open DevTools console on `/history` in dark mode. Assert no "Missing encoding for channel: x." errors appear at mount or after a window-toggle + SSE tick. **Fallback gate:** if the errors persist, switch the affected charts (`NWSAlertsArea`, `SWPCScalesLine`, `USGSQuakesScatter`) from dropping `scale.x.type` to using `axis: { x: { type: 'time' } }`. Re-run; if errors then clear, commit the fallback as a separate `fix(p4-followups):` commit and update the design's Open Questions section.
- Open `/` (Overview) in both themes. Assert no `0` corner badges on inactive Active Alerts chips; the chip itself remains gray.
- Open `/events` in both themes. Assert each volcano row shows exactly one chip (the alert level), no `ORANGE`/`YELLOW`/etc. second chip.
- Toggle theme via `SettingsDrawer` while on `/history`. Assert charts re-theme correctly (dark → light → dark) without a stale-state stuck render.
- Open `/history` in light mode. Assert nothing regressed (the polish-round was passing in light; this round must not break it).

---

## 9. Definition of done

- [ ] Design doc + implementation plan committed under `docs/plans/` (this file + sibling implementation file).
- [ ] All work in worktree on branch `feature/p4-followups-issue-17` off `origin/main` at `e5dba8d`.
- [ ] All `task web:test` tests pass; `task web:typecheck` clean; `task web:build` succeeds.
- [ ] `task lint` clean; `task test` passes (Go suite untouched, but verify nothing broke).
- [ ] Manual verification (§8.3) all green in BOTH themes on `/history`, `/`, and `/events`.
- [ ] Protected-files contract intact: `git diff origin/main -- internal/cache/cache.go internal/sse/hub.go internal/webdist/dist/index.html 'internal/sources/*.go'` outputs nothing.
- [ ] Console on `/history` is free of "Missing encoding" errors at mount, after window toggle, after theme toggle, and after SSE tick.
- [ ] Any residual dark-mode legibility issue surfaced to the user during Task 9 has been resolved IN THIS BRANCH (not deferred), with the chosen resolution path approved by the user — see Risk 1.
- [ ] Independent comprehensive code review on the full diff vs `e5dba8d` passes; review findings addressed.
- [ ] PR opened to `main`, CI green.
- [ ] Worktree cleaned up post-merge.

---

## 10. Out of scope

- Custom G2 theme tokens (axis label colors, gridline colors, legend colors, etc.). The user explicitly directed Ant-Design built-ins only. If `type: 'dark'` proves insufficient for a specific element, the resolution lands in **this branch** (not a follow-up) per Risk 1 — surface to the user, choose a resolution path that respects the constraint, apply it. Hand-rolled palettes remain off-limits unless the user explicitly relaxes the constraint.
- New chart types (heatmap, density, etc.).
- Per-region NWS alert filters; cross-source overlays; climatological comparisons (still deferred to Phase 5+).
- Refactoring `useIsDark` itself, despite the per-chart consumers going away. It still lives in `web/src/theme.ts` for future use.
- Restructuring Overview's Active Alerts row beyond the zero-badge fix (no chip-layout changes; no new sort order; no new categories).
- Restructuring VolcanoList beyond the duplicate-chip drop (region text, USGS link, relative-time text all unchanged).
- Changes to `SettingsDrawer`, the theme algorithm, or theme-mode persistence.

---

## 11. Risks

1. **`type: 'dark'` may not be sufficient for every G2 element.** The built-in dark preset adapts axis text, legend, gridlines, and tooltip; a specific element (tooltip arrow, legend swatch, etc.) may remain poorly contrasted. **The resolution must land in this branch — NOT deferred to a follow-up issue.** Task 9's verify pass surfaces any residual problem to the user immediately, with a screenshot of the failing element. Resolution paths, all within the no-custom-themes constraint:
    1. Identify whether the element is configurable via a non-palette `ConfigProvider` field that still avoids hand-rolled colors (e.g. a documented sizing/opacity knob).
    2. Pin or upgrade `@ant-design/charts` to a version where the dark preset covers the element. (Confirm with the user before changing the version.)
    3. If neither (a) nor (b) is viable: discuss with the user. If the only path is custom palette tokens, that's a constraint-revisit decision the user must explicitly make — do not apply it unilaterally. In that case the user may also choose to accept the limitation as a documented caveat in `README.md` and merge the branch with the residual issue called out.

   **Mitigation:** Task 9's manual verification is the gate; we don't paint over residual issues silently, and we don't kick the can to a separate issue either.

2. **Dropping `scale.x.type` could surface a different inference bug.** v2 has shipped many minor versions; auto-inference from `Date` data is documented but not guaranteed for every chart variant. **Mitigation:** Decision 3's fallback path — switch to `axis.x.type='time'` per current v2 docs, gated by Task 9's console check.

3. **`key={isDark ? …}`-driven remount causes a brief flash on theme toggle.** Five chart cards unmount + remount; G2 re-instantiates each. User perceives a flicker for ~100-300ms on toggle. **Mitigation:** Theme toggle is rare (typically once per session). The flicker is preferable to an inconsistent theme. If a future investigation proves G2 reacts to `common.theme.type` changes alone, the `key` can be dropped.

4. **`HistoryChartRow.test.tsx` mock surface area grows.** The test must add a `ConfigProvider` passthrough to its existing `vi.mock('@ant-design/charts', ...)`. Risk: the test mock drifts from the real component surface; future regressions (e.g. an upgrade of `@ant-design/charts` that changes the export shape) are caught at runtime, not at unit-test time. **Mitigation:** the recording-mock test in `History.test.tsx` is the canary — it reads `common.theme.type`, so a real-package shape change would break it.

5. **`AlertBadgeBar` zero-badge change loses the "we know it's zero, not unfetched" signal.** Before: a gray `0` corner badge conveyed "fetched, zero". After: no badge at all. The chip's gray `default` color is the substitute signal. **Mitigation:** the chip color was already conveying this — the user noted it as redundant clutter — so the substitute is a clean upgrade, not a degradation.

6. **VolcanoList drop loses the literal USGS color-code term.** Operators familiar with the USGS taxonomy ("ORANGE" is its own well-known classification) lose the explicit label. The level-chip's color is the substitute. **Mitigation:** the level word ("WATCH") is the operationally important signal; color codes are derivative. Color-blind A11y is unaffected because the level word remains.

---

## 12. Open questions

None remaining at design time. Two areas have explicit fallback paths inside the design (Decisions 3 and 1's `key` discussion) and will resolve at Task 9 verify time.

---

## 13. Appendix — Issue #17 finding-to-decision crosswalk

| Issue finding | Severity | Resolved by |
|---|---|---|
| 1. Dark-mode chart contrast | Major | Decisions 1 + 2 (page-level `ChartsConfigProvider` + drop per-chart prop) |
| 2. SWPC alerts empty axes when all zero | Minor | Decision 4 (`every`-zero guard) |
| 3. Console "Missing encoding" errors ×3 | Minor | Decision 3 (drop `scale.x.type`; fallback to `axis.x.type` if needed) |
| 4. AlertBadgeBar 0-badge overlap | Minor | Decision 5 (`showZero={false}`) |
| 5. VolcanoList duplicate chips | Minor | Decision 6 (drop color chip) |
