# Phase 4 — History page polish + per-event-type counters — Design

**Date:** 2026-05-05
**Status:** Approved (4/4 open questions resolved 2026-05-05)
**Parent design:** `2026-05-03-phase4-history-design.md`
**Branch:** continues on `feature/phase4-history` (or a follow-up branch off it)

---

## 1. Goal

Close out the visual + behavioral gaps that the Phase 4 v1 implementation surfaced once it landed in a real browser, and add a per-event-type counter strip to the NWS row so the operator can see Tornado / Severe Thunderstorm / Flash Flood activity at a glance.

Two motivations:
1. **Polish.** The committed charts work but are unusable in dark mode, missing the headline-stat tag the parent design called for, and not live-tailing — they go stale until the user refreshes or toggles the window.
2. **Per-event-type visibility.** Aggregate `activeCount` collapses high-impact events (tornadoes) with low-impact ones (winter weather advisories). The operator wants to see the counts of the three highest-criticality NWS events as their own line.

---

## 2. Decisions to confirm before implementation

| # | Question | Proposed |
|---|---|---|
| 1 | Live-tail strategy: full re-fetch on each `.update` event, or in-place bucket extension? | **Full re-fetch.** `Cache-Control: max-age=60` on the response + the existing 5 parallel-fetch pattern means the cost is small and we avoid bucket-edge drift bugs the design §5.4 flagged. |
| 2 | Where to surface per-event-type counters? | **Inside the existing NWS row**, as a 3-tag strip above the area chart — Tornado / Severe Thunderstorm / Flash Flood, each with current count + 24h delta sparkline. |
| 3 | Which event types? | **Start with the user's three: Tornado, Severe Thunderstorm, Flash Flood.** Make the set extensible (config-driven on the server, hard-coded list on the client v1). |
| 4 | Server-side: extend `/api/history?source=nws_alerts` payload, or new endpoint? | **Extend the existing payload** with an `eventCounts: {tornado, severeTstorm, flashFlood}` field per bucket. One round-trip per source stays the rule. |
| 5 | Headline-stat tag content per row | NWS = "300 active"; SWPC scales = current G-scale label ("G0"); SWPC alerts = "12 in 24h" (cumulative count of severities); Quakes = "3 M4+"; Volcanoes = "2 elevated". |
| 6 | "Phase 0 — skeleton" replacement | **Drop the tag entirely.** The shell is past Phase 0 and the build version + uiconfig already convey "what's live". |
| 7 | Footer `image:*` debug tags visibility | **Gate behind `import.meta.env.DEV`.** They're useful for development but noise in production. |
| 8 | Page max-width on wide displays | **Stretch the chart row card to `width: 100%`** with `marginInline: 0`. Existing AntD `Layout.Content` already provides the page padding. |

---

## 3. Tasks (proposed — pending approval)

### Task A — Dark-mode chart theme

**Files:** `web/src/components/charts/{NWSAlertsArea,SWPCScalesLine,SWPCAlertsBars,USGSQuakesScatter}.tsx`

**Why:** The committed components inherit AntD's dark theme tokens for their wrapper cards but `@ant-design/charts` (G2) draws axis labels, gridlines, and legend with its own hard-coded "classic" theme — near-black text on a dark card → invisible. Verified via Playwright: the same page in light mode renders all axes legibly.

**Approach:** Read AntD's current algorithm via `theme.useToken()` (or the project's existing dark-mode signal — locate during implementation), then pass `theme={isDark ? 'academy' : 'classic'}` to each chart. v2 supports both string-presets and a custom token map; we start with the preset.

**Test:** Vitest assertion that the component passes `theme="academy"` when `isDark` is true, plus a Playwright smoke that the axis-tick text computed-style `fill` is light-on-dark when `prefers-color-scheme: dark`.

### Task B — Headline-stat tag per row

**Files:** `web/src/components/HistoryChartRow.tsx`, `web/src/api/history.ts` (extract latest-stat helpers)

**Why:** Parent design §7.1 specifies a row-summary tag. Currently the only header tag is "data starts here →".

**Approach:** Extract the "latest meaningful number" from each source's response: NWS = max activeCount across buckets; SWPC scales = last bucket's gScale formatted as `G{n}`; SWPC alerts = sum of warning+watch+alert across all buckets; Quakes = `events.length`; Volcanoes = `changes.length`. Render as a second `Tag` in the card's `extra`.

**Test:** Vitest renders the row with mocked responses and asserts the headline number text is present.

### Task C — SSE live-tail

**Files:** `web/src/components/HistoryChartRow.tsx`, possibly a small `useSourceUpdate(source)` hook

**Why:** Plan §5.4 + §6.3 deliverable. Without it, the chart goes stale.

**Approach:** Subscribe via the existing `web/src/api/stream.ts` connect() pipeline (already in use by the snapshot store). Easiest hook: read the per-source `lastUpdate` timestamp from the snapshot store, depend on it in the row's `useEffect`, and re-fetch `/api/history?source=…&window=…` whenever that timestamp ticks. Decision 1: full re-fetch, not bucket-extension.

**Test:** Vitest mocks the snapshot store, asserts re-fetch fires when the per-source timestamp changes; integration test (Playwright) leaves the page open for 70s on a real backend and asserts the right-edge data point has moved.

### Task D — Per-event-type counters (NEW)

**Files (server):** `internal/api/history.go` (extend NWS aggregator), `internal/api/history_test.go`
**Files (client):** `web/src/api/history.ts` (extend type), `web/src/components/charts/NWSEventCountsStrip.tsx` (new), `HistoryChartRow.tsx` (compose)

**Why:** Operator-facing — collapses the noise of activeCount-as-a-blob into three tracked event categories.

**Approach:**

*Server.* The NWS aggregator currently emits `{activeCount: max}` per bucket. Extend to also emit `{eventCounts: {tornado, severeTstorm, flashFlood}}` where each is the **max concurrent count of that event type within the bucket** (so it stays a "how bad did it get" measure consistent with `activeCount`). Each alert in the upstream NWS payload has an `event` field; we string-match against:

```go
var nwsEventCounters = map[string][]string{
    "tornado":      {"Tornado Warning", "Tornado Watch", "Tornado Emergency"},
    "severeTstorm": {"Severe Thunderstorm Warning", "Severe Thunderstorm Watch"},
    "flashFlood":   {"Flash Flood Warning", "Flash Flood Watch", "Flash Flood Emergency"},
}
```

(Initial list — refine during implementation against actual NWS event taxonomy.)

*Client.* Add a strip above the area chart:

```
┌─ NWS alerts — active count over time ─────────── [300 active] [data starts here →] ─┐
│                                                                                      │
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐                   │
│  │ 🌪 Tornado    2 │  │ ⛈ Severe Tstorm 7│  │ 🌊 Flash Flood  4│                   │
│  │ ──/──\─/────/── │  │ ─────/\__/───── │  │ ───\__/─\__/── │                   │
│  └──────────────────┘  └──────────────────┘  └──────────────────┘                   │
│                                                                                      │
│  ┌────── area chart of TOTAL activeCount over the window (existing) ──────┐         │
│  │                                                                        │         │
│  │       300 ─────────●──●───────●──────●─────●─────────●──────●          │         │
│  │       200                                                              │         │
│  │       100                                                              │         │
│  │         0 ──────────────────────────────────────────────────────────── │         │
│  │           07:50      07:55      08:00      08:05      08:10      08:15│         │
│  └────────────────────────────────────────────────────────────────────────┘         │
└──────────────────────────────────────────────────────────────────────────────────────┘
```

Each per-event card shows:
- An icon (AntD icon or emoji — pick during implementation)
- The category label
- The current (latest bucket) count, large
- A small inline sparkline of that category's value across the window — `<TinyArea>` from `@ant-design/charts` (~70×24px)

The strip is a single horizontal `Space` of three `Card`s with `bordered`, sized to be visually subordinate to the main area chart.

**Test:** Backend — extend `internal/api/history_test.go` with a fixture containing one of each event type and assert the per-bucket counts. Frontend — Vitest renders `NWSEventCountsStrip` with sample data and asserts the three labels + counts + that each sparkline gets a non-empty data prop.

### Task E — Replace "Phase 0 — skeleton" tag

**Files:** `web/src/App.tsx:127`

**Why:** Outdated; per Decision 6 we drop it.

**Approach:** Remove the `<Tag color="default">Phase 0 — skeleton</Tag>` line. (Optional: replace with a build-version tag wired off `/api/version`.)

**Test:** Vitest assertion in `App.test.tsx` that the deprecated text is gone.

### Task F — Hide footer `image:*` debug tags in non-dev

**Files:** likely `web/src/components/SourceHealthIndicator.tsx` (locate during implementation)

**Why:** Decision 7. Prod users don't need to see the prewarm-image source tags inline.

**Approach:** Wrap the `image:*` tag rendering in an `import.meta.env.DEV` guard, OR a more general "show extended diagnostics" toggle that defaults off. Simplest first: env guard.

**Test:** Vitest snapshot in dev vs production-mocked env (vitest can override `import.meta.env.DEV`).

### Task G — X-axis time formatting

**Files:** all 4 chart components

**Why:** Currently the SWPC alerts row shows ticks like `"Tue May 05 2026 07:47:25 GMT-0700 (Pacific Daylight Time)"` because we hand G2 a Date and let it pick the default toString. We want compact ticks: `HH:mm` for 24h, `MM-dd HH:mm` for 7d, `MM-dd` for 30d.

**Approach:** Pass the `window` prop down to each chart and use `axis.x.labelFormatter` with a dayjs-style mask. Or use a small `formatTimeForWindow(window)` helper that returns the right format string and feed it to `axis.x.labelFormatter`.

**Test:** Vitest asserts the formatter is set to the right format string per window prop.

### Task H — Page max-width / row stretch

**Files:** `web/src/pages/History.tsx`, `web/src/components/HistoryChartRow.tsx`

**Why:** Decision 8. Currently rows render at ~800px on a 1920px display, leaving huge whitespace.

**Approach:** Either set `style={{ width: '100%' }}` on the Card or wrap History in a `Layout.Content` with `maxWidth: 1280, margin: '0 auto'` shell. Pick the simpler one during implementation.

**Test:** Playwright at 1920×1080 — assert the chart card's bounding-rect width is the viewport minus the sidebar minus padding.

### Task I — Tooltip-on-hover smoke check

**Files:** new Playwright test under `web/tests/e2e/` (if no infra exists, defer to a vitest+jsdom assertion that the chart gets a `tooltip` config object — partial coverage but cheaper)

**Why:** Parent design §7 mandates tooltips as the only interaction.

**Approach:** Either a Playwright test that hovers over a known data point and asserts a tooltip element appears, or a unit-level check that `tooltip={{...}}` is passed to each chart with a non-empty formatter.

**Test:** itself.

---

## 4. File map

```
internal/api/
  history.go        (modify — extend NWS aggregator)
  history_test.go   (modify — fixture for event-type counts)

web/src/
  App.tsx                                       (modify — drop "Phase 0 — skeleton")
  api/history.ts                                (modify — extend NWSAlertsHistory.buckets[].eventCounts)
  components/
    HistoryChartRow.tsx                         (modify — headline tag + SSE-driven re-fetch + dark-theme prop wiring + max-width)
    HistoryChartRow.test.tsx                    (modify — assertions)
    SourceHealthIndicator.tsx                   (modify — env-gate image:* tags)
    charts/
      NWSAlertsArea.tsx                         (modify — theme + time-formatter)
      SWPCScalesLine.tsx                        (modify — same)
      SWPCAlertsBars.tsx                        (modify — same)
      USGSQuakesScatter.tsx                     (modify — same)
      NWSEventCountsStrip.tsx                   (new — 3-card strip)
      NWSEventCountsStrip.test.tsx              (new)
  pages/
    History.tsx                                 (modify if needed for max-width)
```

---

## 5. Test plan

- **Backend:** extend `internal/api/history_test.go` with `TestHistoryHandler_NWSAlerts_PerEventTypeCounts` — one fixture mixing tornado / severe-tstorm / flash-flood / unrelated events, assert bucket payload.
- **Frontend unit:** vitest tests per task as above.
- **Frontend integration (Playwright):** dark-mode contrast test, tooltip-on-hover smoke, post-SSE-tick right-edge update.
- **Smoke:** existing `task smoke` already covers the basic API; no extension needed for this round.

---

## 6. Out of scope

- New per-source charts beyond NWS event-types (no equivalent for SWPC alerts severity-by-product yet).
- Configurable per-event-type list at runtime (server-side hardcoded for v1).
- Climatological comparisons / cross-source overlays (still deferred).
- Restoring history-page state across hard refresh beyond the existing `?w=` URL param (Phase 6 if ever).
- Runtime theme toggle that switches charts without a page reload (Task A reads at mount; if you toggle theme, you reload).

---

## 7. Risks

1. **G2's `theme="academy"` may not be the exact string in v2.** If wrong, fall back to passing a token object. Implementation will probe before committing.
2. **NWS event-name string-matching is brittle.** Names occasionally vary ("Severe Thunderstorm Warning" vs "Severe Thunderstorm Statement"). The list is a starting point; we add a small unit test fixture covering the canonical names and treat unmatched events as "other" (not silently bucketed).
3. **SSE re-fetch storm.** If 5 sources tick simultaneously (rare but possible), 5 simultaneous /api/history requests fire. Each is small (~50KB) and has `Cache-Control: max-age=60` so browser cache absorbs many of them. Still — debounce in the row at, say, 250ms.
4. **Per-event-type strip mobile layout.** Three side-by-side cards may wrap awkwardly under 600px. AntD `Space` with `wrap` handles this; verify with Playwright at narrow viewports.

---

## 8. Mockup — desktop dark mode (target visual)

```
┌────────────────────────────────────────────────────────────────────────────────────────────────┐
│  cwd                                                                  [v0.4.0]  [⚙]            │
├──────┬─────────────────────────────────────────────────────────────────────────────────────────┤
│ Nav  │                                                                                          │
│ Over │  History                                                       [ 24h | 7d | 30d ]        │
│ Haz  │                                                                                          │
│ Spw  │  ┌──── NWS alerts — active count over time ─────── [300 active] [data starts here →] ─┐ │
│ Evt  │  │                                                                                     │ │
│ ▶Hst │  │  ┌──── 🌪 Tornado ──┐  ┌──── ⛈ Severe Tst ──┐  ┌──── 🌊 Flash Flood ──┐           │ │
│      │  │  │     2            │  │     7              │  │     4                │           │ │
│      │  │  │   ─/\─/─\──/─    │  │   ──/────\──/─     │  │   ─\__/─\─/────       │           │ │
│      │  │  └──────────────────┘  └────────────────────┘  └──────────────────────┘           │ │
│      │  │                                                                                     │ │
│      │  │  active alerts                                                                      │ │
│      │  │       300 ●─●───────●──────●─────●─────────●──────●  ← AntD Charts Area            │ │
│      │  │       200                                                                           │ │
│      │  │       100                                                                           │ │
│      │  │         0 ──────────────────────────────────────────────────────────────            │ │
│      │  │           07:50      07:55      08:00      08:05      08:10      08:15              │ │
│      │  └─────────────────────────────────────────────────────────────────────────────────────┘ │
│      │                                                                                          │
│      │  ┌──── SWPC scales — today's G / R / S ─────── [G0]  [data starts here →] ─────────┐ │
│      │  │  • G-scale  • R1  • S1                                                            │ │
│      │  │       1 ────────────────────────────────────────────────────────────              │ │
│      │  │       0 ●─●─●─●─●─●─●─●─●─●─●─●─●─●─●─●─●─●─●─●─●─●─●─●─●─●─●─●─●                │ │
│      │  │           07:50    07:55    08:00    08:05    08:10    08:15                      │ │
│      │  └────────────────────────────────────────────────────────────────────────────────────┘ │
│      │                                                                                          │
│      │  ┌──── SWPC alerts — by severity ─────── [0 in 24h]  [data starts here →] ─────────┐ │
│      │  │  ■ warning  ■ watch  ■ alert                                                       │ │
│      │  │  (empty — no current alerts)                                                       │ │
│      │  └────────────────────────────────────────────────────────────────────────────────────┘ │
│      │                                                                                          │
│      │  ┌──── USGS quakes — magnitude scatter ─────── [0 M4+]  [data starts here →] ──────┐ │
│      │  │  (no events in this window)                                                        │ │
│      │  └────────────────────────────────────────────────────────────────────────────────────┘ │
│      │                                                                                          │
│      │  ┌──── USGS volcanoes — state-change events ───── [0 elevated] [data starts here] ──┐ │
│      │  │  (no state changes in this window)                                                 │ │
│      │  └────────────────────────────────────────────────────────────────────────────────────┘ │
└──────┴──────────────────────────────────────────────────────────────────────────────────────────┘
```

Key visual changes from the current state:
- Headline-stat tag in each card extra (`[300 active]`, `[G0]`, `[0 in 24h]`, `[0 M4+]`, `[0 elevated]`)
- Per-event-type strip above the NWS area chart (Tornado / Severe Thunderstorm / Flash Flood)
- Axis labels, gridlines, and legend rendered in light text against the dark card
- Compact X-axis time format (`HH:mm` for 24h)
- Cards stretch to full content width
- "Phase 0 — skeleton" tag in the global header is gone

---

## 9. Resolved questions (2026-05-05)

- [x] **Live-tail strategy: full re-fetch.** Re-fetch on each `<source>.update` event with a 250ms debounce. In-place bucket extension is rejected — its complexity, server/client drift risk, and bucket-edge edge cases outweigh the saved HTTP cost given `Cache-Control: max-age=60`.
- [x] **Event-type list: Tornado / Severe Thunderstorm / Flash Flood only** for v1. Additional categories deferred (out of scope this round to control creep).
- [x] **Headline-stat source: history response.** Use the over-window aggregates the `/api/history` payload already returns (e.g. NWS = `max(activeCount)` across the window's buckets, SWPC alerts = sum across buckets). This keeps the row's stat consistent with the chart's data on the same screen.
- [x] **"Phase 0 — skeleton" replacement: build-version tag.** Replace with a version tag that reads from the existing `/api/version` endpoint (or whatever the SPA already plumbs). If the SPA doesn't already have a version-fetch hook, add a small `useBuildVersion()` reading from `/api/version` once at mount.

Implementation: one wave-bundle of Tasks A–I in the same subagent-driven pattern as the original Phase 4 plan.
