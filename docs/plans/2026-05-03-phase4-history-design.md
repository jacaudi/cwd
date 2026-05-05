# Self-Hosted CWD — Phase 4 (History page — per-source operational time series) — Design Document

**Date:** 2026-05-03
**Status:** Approved (verbal, all 8 brainstorm questions)
**Parent design:** [docs/plans/2026-05-01-self-hosted-cwd-design.md](2026-05-01-self-hosted-cwd-design.md)
**Phase 1 design:** [docs/plans/2026-05-02-phase1-nws-alerts-design.md](2026-05-02-phase1-nws-alerts-design.md) — Source/Fetcher/Cache/Store/Hub contracts
**Phase 2 design:** [docs/plans/2026-05-02-phase2-multi-source-design.md](2026-05-02-phase2-multi-source-design.md)
**Phase 3 design:** [docs/plans/2026-05-03-phase3-image-proxy-design.md](2026-05-03-phase3-image-proxy-design.md)
**Phase 3 baseline:** main at the merge commit of PR #5 (`7f0e85d`)

---

## 1. Goal & scope

Replace the placeholder `/history` page with an **operational time-series dashboard** showing trend data for the 5 data sources Phase 1+2 already store snapshots of: NWS alerts, SWPC scales, SWPC alerts, USGS quakes, USGS volcanoes. Each source gets one full-width chart row (layout B from the brainstorm); a window-length toggle (24h / 7d / 30d) sits in the page header. The page uses `@ant-design/charts` for rendering, hover tooltips only, and live-tails new data points via the existing SSE pipeline.

Phase 4 also extends the existing `/api/history` endpoint — currently NWS-only and point-in-time — to return per-source aggregated time series.

**Demo bar (per parent design §11):** *trend visibility for each of the 5 data sources over the past 24 hours, 7 days, or 30 days.*

**Explicitly NOT in scope (deferred):**
- Time travel for the rest of the site (other pages stay live; `/history` does not freeze them).
- Calendar / arbitrary date-range picker — the 24h/7d/30d toggle is enough.
- Click-to-expand modals on chart rows — hover tooltips are the sole interaction in v1.
- Historical image bytes — the Phase 3 image proxy stays "latest only"; `/history` is data charts, not historical maps.
- Per-region (state, WFO) chart filters — out of v1 (we chart aggregates).
- Cross-source overlay charts ("alerts AND quakes on the same axis") — out of v1.
- Climatological comparisons ("this month vs same month last year") — out of v1; the 30d window is the longest baseline.
- PWA service-worker — Phase 5.

---

## 2. Decisions log

Captured during the 2026-05-03 brainstorm.

| # | Question | Decision |
|---|---|---|
| 1 | Primary user story | **C: per-source time-series charts.** No global slider, no time travel. The page is an operational trends dashboard. |
| 2 | Operational vs climatological | **Operational, with a window-length toggle (24h / 7d / 30d).** Stretches into climatological without a redesign; aligns with Phase 1's existing `retention_days: 30`. |
| 3 | Page layout | **B: stacked full-width rows.** One row per source. Generous X-axis room; mobile collapses cleanly to a tall scroll. |
| 4 | Per-source chart contents | **Approved as proposed.** NWS area, SWPC scales stepped + dashed prob lines, SWPC alerts stacked hourly bars, USGS quakes magnitude scatter, USGS volcanoes diff-feed (NOT a chart). |
| 5 | Storage / retention / sampling | **Approved as proposed.** Retention 30d (unchanged), snapshot cadence = source poll cadence (no separate timer), three fixed window options, server-side downsampling to ≤200 buckets per response, live-tail re-uses existing `<source>.update` SSE events. |
| 6 | Chart library | **`@ant-design/charts`.** Matches existing AntD theme; ~200-300 KB gzipped to bundle is acceptable for a self-host single-user binary. |
| 7 | Interaction depth | **A: hover tooltips only.** No click-to-expand modal; no click-to-pin. Operational dashboard tone. |
| 8 | Five small defaults | Images out of scope (latest-only stays), UTC timestamps, SSE live-tail (no new event names), "data starts here →" left-edge marker when window > available history. **Skeleton-during-load explicitly rejected** — initial paint is hot per existing prewarm/config; no Phase-4 loading state needed. |

---

## 3. Upstream constraints worth calling out

### 3.1 Storage already covers all 5 sources

`internal/store/snapshots.go` exposes `Append(source, fetchedAt, validator, payload)`, `Latest(source)`, `At(source, t)`, `Prune(retention)`. The schema is `snapshots(source TEXT, fetched_at INTEGER, etag TEXT, payload BLOB)` with PK `(source, fetched_at)` — source-agnostic.

**All 5 sources already persist on every successful fetch.** Phase 1's generic fetcher loop (`internal/fetcher/fetcher.go:139`) calls `store.Append(ctx, f.src.Name(), now, res.Validator, payload)` for any `Source` whose validator changed since the prior fetch. There is no NWS-specific store wiring; every source passing through the fetcher writes its history automatically. Phase 3's README note ("`/api/history` currently surfaces only `nws_alerts`") was a handler-side limitation — the storage has had all 5 sources since Phase 2 merged.

Phase 4 therefore needs only:
- **Range-query helpers** in the store. The current `At(source, t)` returns one point. Phase 4 adds `Range(source, from, to)` returning all rows in a window, plus a downsampling-friendly `RangeBuckets(source, from, to, n)` that returns ≤ n buckets pre-aggregated by SQL.
- **A rewritten `/api/history` handler** that fans out to all 5 sources and returns per-source typed responses.

**Validator-gated writes mean duplicate fetches are not stored.** The fetcher only calls `Append` when the upstream validator (ETag or body hash) changed. Snapshot density therefore equals `min(poll_cadence, content_change_rate)`. For slow-moving sources (volcanoes change state rarely; SWPC scales updates hourly even though we poll every 5m) actual row counts are well under the table in §3.2. This is fine — it makes downsampling cheaper, not harder.

### 3.2 Per-source poll cadence (already configured)

| Source | Default interval | Max snapshots in 24h | Max snapshots in 30d |
|---|---|---|---|
| `nws_alerts` | 60s | ~1,440 | ~43,200 |
| `swpc_scales` | 5m | ~288 | ~8,640 |
| `swpc_alerts` | 5m | ~288 | ~8,640 |
| `usgs_quakes` | 60s | ~1,440 | ~43,200 |
| `usgs_volcanoes` | 5m | ~288 | ~8,640 |
| **All sources, 30d (upper bound)** | | | **~112,320 rows** |

The numbers above are upper bounds (every poll changes the validator). In practice the validator-gate at `fetcher.go:139` drops duplicate fetches, so actual row counts are much lower for slow-moving sources (e.g. volcanoes might land 5-20 rows in 30d, not 8,640). Gzipped JSON payloads average ~2-15 KB each → 30d worst-case storage ~500 MB, typical ~10-100 MB. Safely under any modern Pi-class disk.

### 3.3 Downsampling discipline

A 30d window with NWS at 60s = 43,200 points. A chart at 1200 px wide can resolve ~600 distinct X positions; sending 43,200 wastes bandwidth and overwhelms the chart. Server-side downsampling caps every `/api/history` response at **200 buckets per source per request**. Bucket size derives from window:

- 24h → 432-second bucket (~7 minutes)
- 7d → 50-minute bucket
- 30d → 3.6-hour bucket

Bucket aggregator is per-chart-shape (see §5.3); the SQL query selects rows by `fetched_at BETWEEN from AND to` and `GROUP BY (fetched_at - from) / bucket_seconds`.

### 3.4 SSE live-tail re-uses existing event names

When a source fetcher writes a new snapshot, the existing cache→hub broadcast emits `<source>.update`. The History page subscribes to all 5 sources and appends the new latest data point to the right edge of each chart. No new SSE event names; no changes to `internal/cache/cache.go` or `internal/sse/hub.go`.

### 3.5 Volcanoes is the odd one out

USGS volcanoes data is a list of currently-elevated volcanoes (~5-15 typically), not a high-frequency time series. Per Decision 4, the volcanoes "row" is a **diff-feed of state-change events** between snapshots, not a chart. Implementation:
- Compute state diffs server-side: for each adjacent snapshot pair in the window, emit one event for any volcano that appeared, disappeared, or changed alert level.
- Cap at ~50 events per response (state changes are rare; this is a generous upper bound).
- Render as a vertical timeline, not a chart.

---

## 4. Package additions / file map

### 4.1 New Go files

```
internal/store/
├── range.go              # Range(source, from, to) and RangeBuckets(source, from, to, n)
├── range_test.go
├── volcanoes_diff.go     # VolcanoStateChanges(from, to) — diff between adjacent snapshots
├── volcanoes_diff_test.go
internal/api/
├── history.go            # REWRITE — multi-source aggregated time series
├── history_test.go       # extend
```

### 4.2 New TypeScript files

```
web/src/components/
├── HistoryChartRow.tsx           # one full-width row (header + chart container + window-aware empty marker)
├── HistoryChartRow.test.tsx
├── charts/
│   ├── NWSAlertsArea.tsx         # @ant-design/charts <Area>
│   ├── NWSAlertsArea.test.tsx
│   ├── SWPCScalesLine.tsx        # <Mix> (stepped + dashed)
│   ├── SWPCScalesLine.test.tsx
│   ├── SWPCAlertsBars.tsx        # <Column> stacked
│   ├── SWPCAlertsBars.test.tsx
│   ├── USGSQuakesScatter.tsx     # <Scatter>
│   ├── USGSQuakesScatter.test.tsx
│   ├── VolcanoesTimeline.tsx     # custom (not a chart)
│   ├── VolcanoesTimeline.test.tsx
web/src/pages/
├── History.tsx                   # rewrite of placeholder; 5 HistoryChartRow + window toggle
├── History.test.tsx              # rewrite
web/src/api/
├── history.ts                    # client for GET /api/history (typed per-source response)
├── history.test.ts
web/src/store/
├── history.ts                    # Zustand store: window selection + per-source series cache
├── history.test.ts
```

### 4.3 Touched files

- `internal/api/router.go` — wire the rewritten `NewHistoryHandler`. Same route (`GET /api/history`); query parameters change. The handler's constructor signature changes (it now needs the store, not the filter, since downsampling happens at SQL).
- `internal/server/server.go` — adjust the `NewHistoryHandler` call site to pass the store. No other changes; the source-fetcher wiring already writes to the store via the generic fetcher loop (verified at `internal/fetcher/fetcher.go:139`).
- `internal/cache/cache.go` — **NO CHANGE.** History reads from the store, not the cache.
- `internal/sse/hub.go` — **NO CHANGE.** Live-tail re-uses the existing per-source events.
- `web/package.json` — add `@ant-design/charts` (and its peer deps if any).
- `web/src/App.tsx` — `/history` route already exists pointing at the placeholder; no router change.
- `Taskfile.yml` smoke task — add an assertion that `GET /api/history?source=nws_alerts&window=24h` returns a non-error response with at least one bucket after the 90s warm-up.
- `README.md` — document the rewritten `/api/history` shape, the chart library dependency, and the retention/sampling semantics. Replace the "Per-source history for the SWPC and USGS sources is a Phase 3+ follow-up" paragraph (which was inaccurate — see §3.1).

### 4.4 The Source contract is **not** extended

Phase 1's `internal/sources.Source` interface remains unchanged. Phase 4 adds storage helpers and a new HTTP handler, plus a frontend page; no new abstractions in the source pipeline.

---

## 5. Internal contracts

### 5.1 Store — range + bucket queries (Go)

```go
// internal/store/range.go
package store

// Bucket is one downsampled point in a time series.
type Bucket struct {
    BucketStart time.Time // start of the bucket (UTC)
    Count       int       // number of source-snapshots that fell in this bucket
    Payload     []byte    // bucket-aggregated payload (shape depends on the source's aggregator)
}

// Range returns every snapshot row for `source` whose fetched_at is in [from, to].
// Rows are returned in ascending time order. Use Range when the chart needs raw
// rows (e.g. quakes-scatter); use RangeBuckets for everything else.
func (s *Store) Range(ctx context.Context, source string, from, to time.Time) ([]Row, error)

// RangeBuckets returns at most n buckets covering [from, to], aggregating each
// source's payload via its registered aggregator. Aggregators live alongside
// the source definition (sources.NWSAlertsName → aggregateNWSAlerts) so the
// store doesn't need to know about per-source schema details.
func (s *Store) RangeBuckets(ctx context.Context, source string, from, to time.Time, n int, agg Aggregator) ([]Bucket, error)

// Aggregator combines N raw payloads into a single per-bucket payload.
// Examples: max(activeCount) for NWS, last for SWPC scales, sum-by-severity for SWPC alerts.
type Aggregator func(rows []Row) ([]byte, error)
```

Bucket size: `(to.Unix() - from.Unix()) / n` rounded UP to whole seconds. `RangeBuckets` issues a single SQL query:

```sql
SELECT
  (fetched_at - ?from_ms) / ?bucket_ms AS bucket_idx,
  MIN(fetched_at) AS bucket_start_ms,
  COUNT(*) AS row_count,
  -- aggregator runs in Go; SQL just groups + collects the gzipped payloads
  GROUP_CONCAT(HEX(payload), '|') AS payloads_hex
FROM snapshots
WHERE source = ? AND fetched_at BETWEEN ?from_ms AND ?to_ms
GROUP BY bucket_idx
ORDER BY bucket_idx
```

The aggregator decompresses each row in a bucket and reduces to a single payload. For `nws_alerts`, the aggregator computes `max(len(parsed_alerts))` and emits `{"activeCount": N}`. For `swpc_scales`, the aggregator decompresses the LAST row in the bucket and emits its parsed `today` block.

### 5.2 Volcanoes diff (Go)

```go
// internal/store/volcanoes_diff.go
package store

// VolcanoStateChange captures one volcano transition between two snapshots.
type VolcanoStateChange struct {
    At         time.Time // fetched_at of the snapshot in which the change first appears
    Volcano    string    // observatory + volcano name, e.g. "AVO Great Sitkin"
    Prior      string    // alert level before, "" if newly elevated
    Current    string    // alert level after, "" if removed from elevated list
}

// VolcanoStateChanges walks adjacent snapshot pairs in [from, to] and returns
// every transition. Capped at maxEvents (default 50). Returns events in
// ascending time order.
func (s *Store) VolcanoStateChanges(ctx context.Context, from, to time.Time, maxEvents int) ([]VolcanoStateChange, error)
```

### 5.3 HTTP API surface

The current `GET /api/history?at=RFC3339` is replaced. New shape:

```
GET /api/history?source=<name>&window=24h|7d|30d
    200 OK + per-source typed payload (see below)
    400 if source is unknown or window is unsupported
    500 on store error
```

One source per request, so the SPA fires 5 parallel requests on page load. (We considered a single multi-source endpoint and rejected it: per-source caching headers + per-source SSE live-tail line up better when each source has its own URL.)

Response shape per source:

```ts
// NWS alerts: bucketed area
type NWSAlertsHistory = {
  source: "nws_alerts";
  window: "24h" | "7d" | "30d";
  buckets: Array<{ at: string; activeCount: number }>; // at = ISO-8601 UTC, bucket start
  windowStart: string;  // requested start of window
  dataStart: string;    // earliest available snapshot in store; ≥ windowStart usually
};

// SWPC scales: bucketed last-value of "today" probabilities + G-scale
type SWPCScalesHistory = {
  source: "swpc_scales";
  window: ...;
  buckets: Array<{ at: string; gScale: number; r1: number; s1: number }>;
  ...
};

// SWPC alerts: bucketed counts by severity
type SWPCAlertsHistory = {
  source: "swpc_alerts";
  window: ...;
  buckets: Array<{ at: string; warning: number; watch: number; alert: number }>;
  ...
};

// USGS quakes: raw events, capped at 500, M ≥ 4 only
type USGSQuakesHistory = {
  source: "usgs_quakes";
  window: ...;
  events: Array<{ at: string; mag: number; place: string; depthKm: number }>;
  ...
};

// USGS volcanoes: state-change diff feed
type USGSVolcanoesHistory = {
  source: "usgs_volcanoes";
  window: ...;
  changes: Array<{ at: string; volcano: string; prior: string; current: string }>;
  ...
};
```

`Cache-Control: public, max-age=60` on all responses (the right edge of the chart updates via SSE; the bulk of the response is stable for at least the next minute).

### 5.4 SSE live-tail — no new events

The History page subscribes to `nws_alerts.update`, `swpc_scales.update`, `swpc_alerts.update`, `usgs_quakes.update`, `usgs_volcanoes.update` (already broadcast by Phase 1+2). On each event, the page invokes the source's per-bucket aggregator on the new payload + the most recent bucket and either (a) extends the rightmost bucket's count if the new snapshot falls in the same bucket window, or (b) appends a new bucket on the right edge if the bucket boundary has crossed. No round-trip to `/api/history`.

---

## 6. Data flow

### 6.1 Initial page load

```
Browser                  Server                           Store
   │                        │                               │
   ├─GET /history──────────▶│ (SPA renders shell from /api/uiconfig + /api/snapshot — already hot)
   │                        │
   ├─5x parallel:           │                               │
   ├─GET /api/history?source=nws_alerts&window=24h─────────▶│
   │                        ├─ RangeBuckets ───────────────▶│ SELECT … GROUP BY bucket_idx
   │                        │                  rows  ◀──────┤
   │                        ├─ aggregate to NWSAlertsHistory│
   ◀──200 OK────────────────┤                               │
   │                        │ (similar for the other 4)
   │
   ├─GET /api/stream (already open from App-mount)
   ◀── SSE: nws_alerts.update / swpc_*.update / usgs_*.update (live-tail)
```

### 6.2 Window-toggle change (e.g. user clicks 7d)

```
Browser                  Server
   │                        │
   ├─5x parallel re-fetch with window=7d
   │                        ├─ RangeBuckets (different bucket size) per source
   ◀── 5x 200 OK ───────────┤
   │
   │ (existing SSE subscription unchanged; live-tail keeps appending)
```

### 6.3 SSE live-tail tick

```
Server (nws_alerts fetcher succeeds)        Browser (History page open)
   │                                              │
   ├─cache.Set(envelope) ──────────────────────▶  │ stream.ts dispatch
   │                                              │
   │  (hub broadcasts nws_alerts.update)          ├─ history-store: parse new active count
   │                                              ├─ append/extend rightmost bucket
   │                                              ├─ AntD <Area> re-renders (animated)
```

### 6.4 Window > available history

```
Browser asks for window=30d but binary booted 3 days ago
   │
   ├─GET /api/history?source=nws_alerts&window=30d
   │
Server: RangeBuckets returns 200 buckets
        but the leftmost N buckets have row_count == 0
        Response includes dataStart = "<3 days ago>" so client can mark.
   │
Browser: HistoryChartRow renders the chart with all 200 buckets;
         a "data starts here →" marker overlays at the dataStart X position;
         buckets to the left of dataStart render as a muted band.
```

---

## 7. Frontend

### 7.1 History page composition

```tsx
<History>
  <PageHeader extra={<WindowToggle value={window} onChange={setWindow} />} />
  <HistoryChartRow source="nws_alerts" window={window} />
  <HistoryChartRow source="swpc_scales" window={window} />
  <HistoryChartRow source="swpc_alerts" window={window} />
  <HistoryChartRow source="usgs_quakes" window={window} />
  <HistoryChartRow source="usgs_volcanoes" window={window} />
</History>
```

Each `HistoryChartRow`:
- Owns its own data-fetch (`useHistory(source, window)`).
- Owns its SSE subscription (subscribes to that source's `.update` event).
- Renders a per-source chart component from `web/src/components/charts/`.
- Renders the "data starts here →" overlay if `dataStart > windowStart`.
- Renders a small `Tag` showing the row's headline number ("147 active", "G2", "12 in 24h", "8 M4+", "3 elevated").

### 7.2 Window toggle

```tsx
<Segmented
  options={['24h', '7d', '30d']}
  value={window}
  onChange={(v) => setWindow(v as Window)}
/>
```

State lives in a small Zustand store (`web/src/store/history.ts`) so it persists across remounts but resets on refresh. URL param (`?w=7d`) is synced to support deep links and browser-back. Default = `24h`.

### 7.3 Per-chart components (one file each in `web/src/components/charts/`)

| File | AntD Charts type | Notes |
|---|---|---|
| `NWSAlertsArea` | `Area` | xField: `at`, yField: `activeCount`. Smooth, area fill 15% opacity. |
| `SWPCScalesLine` | `Mix` (multi-series) | Stepped line for `gScale`, dashed line for `r1`, dashed for `s1`. Three Y-axis series sharing the X axis. |
| `SWPCAlertsBars` | `Column` (stacked) | xField: `at`, yField: `count`, seriesField: `severity` (warning/watch/alert). Color map fixed. |
| `USGSQuakesScatter` | `Scatter` | xField: `at`, yField: `mag`, sizeField: `mag`. Reference lines at M4/M5/M6. |
| `VolcanoesTimeline` | (custom — not a chart) | Vertical list of state-change rows. AntD `Timeline` component. |

All chart components accept `data`, `loading`, `dataStart` props; render a `Skeleton` placeholder ONLY if explicitly told to (per Decision 8, no implicit skeleton — initial paint is hot).

### 7.4 Settings drawer (no change)

Phase 6 settings drawer will expose retention + window defaults. Phase 4 just exposes them via `/api/uiconfig` so the drawer has the data when it ships.

---

## 8. Configuration

No new top-level config block in v1. Phase 4 reuses:

- `store.retention_days` (existing, default 30) — bounds how far back any chart can show.
- `sources.<name>.interval` (existing) — drives snapshot density.

Two derived values that will appear in `/api/uiconfig` for the SPA to read (so the page doesn't have to hardcode them):

```yaml
ui:
  history:
    windows: ["24h", "7d", "30d"]   # the toggle options
    default_window: "24h"
    max_buckets_per_request: 200    # informational; server enforces
```

---

## 9. Testing

### 9.1 Backend

- `internal/store/range_test.go` — Range round-trip; RangeBuckets honors n; bucket boundaries correct; empty range returns empty; aggregator errors propagate.
- `internal/store/volcanoes_diff_test.go` — diff between two synthetic snapshots produces correct transitions; cap at maxEvents respected; volcanoes appearing/disappearing/changing level all detected.
- `internal/api/history_test.go` — 400 on unknown source; 400 on bad window; 200 returns the typed shape per source; Cache-Control header; live-tail event composition.
- `internal/server/server_test.go` — extend smoke boot test: after the 5 sources warm up, `GET /api/history?source=<each>&window=24h` returns a non-empty response (modulo allowed empties for sources with no events yet).

### 9.2 Frontend

- `web/src/components/HistoryChartRow.test.tsx` — fetches data on mount; subscribes to SSE; updates rightmost bucket on `<source>.update`; renders "data starts here →" marker when dataStart > windowStart; window-toggle change re-fetches.
- One test per chart component — renders with sample data; tooltip text composition; no error on empty data array.
- `web/src/pages/History.test.tsx` — renders all 5 rows; window toggle changes URL param + re-fetches all 5.
- `web/src/api/history.test.ts` — types parse correctly; URL composition correct.

### 9.3 Smoke

`task smoke` extension: assert `GET /api/history?source=nws_alerts&window=24h` returns 200 + at least one bucket within the existing 90s warm-up window (NWS at 60s gives time for ≥1 snapshot).

---

## 10. Definition of done

- [ ] Design doc + implementation plan committed under `docs/plans/`
- [ ] All work in a worktree on branch `feature/phase4-history` off main at `7f0e85d` (Phase 3 merge)
- [ ] All Go tests pass with `-race`; `golangci-lint run ./...` clean
- [ ] All frontend tests pass (`task web:test`); `task web:typecheck` clean; `task web:build` succeeds
- [ ] `task build:all && ./bin/cwd serve` shows live data on `/history`: 5 rows render; window toggle (24h/7d/30d) re-fetches all 5; tooltip-on-hover works for the line/bar/scatter/area rows; volcanoes timeline renders; live-tail extends the right edge of each chart on the next source poll
- [ ] `task smoke` passes the new history-endpoint assertion
- [ ] `internal/webdist/dist/index.html` is the placeholder (`git diff <merge-base> -- internal/webdist/dist/index.html` empty)
- [ ] `internal/cache/cache.go` and `internal/sse/hub.go` are unchanged in this branch
- [ ] All 5 sources confirmed writing to `internal/store` (sanity check: `SELECT source, COUNT(*) FROM snapshots GROUP BY source` after smoke warm-up shows ≥1 row per source). The fetcher already wires this; this DoD item is verification, not implementation.
- [ ] Independent comprehensive review passes
- [ ] PR opened to main, CI green, merged with per-task history preserved
- [ ] Worktree cleaned up

---

## 11. Out of scope

Verbatim from §1, expanded:

- Time travel for non-history pages — Phase 6 if ever.
- Calendar / arbitrary date-range picker — Phase 6 if ever.
- Click-to-expand modals on chart rows — Phase 5+ if ever.
- Historical image bytes — Phase 5 PWA SW MAY cache last-N images for offline; non-goal here.
- Per-region (state, WFO) chart filters — out of v1.
- Cross-source overlay charts — out of v1.
- Climatological comparisons — out of v1; 30d window is the longest baseline.
- New chart types beyond the 5 in §5.3 — Phase 6 settings panel may expose toggles for variants.

---

## 12. Risks

1. **`@ant-design/charts` bundle size.** Adds ~200-300 KB gzipped. Acceptable for the self-host single-user binary. If we ever ship a public web build, evaluate tree-shaking the unused chart types. Not blocking.

2. **SSE live-tail bucket-extension correctness.** When the rightmost bucket spans a window boundary, naive append-on-every-update can produce off-by-one bars. Mitigation: HistoryChartRow's SSE handler computes the current bucket index from `(now − windowStart) / bucketSeconds`; if it equals the index of the rightmost bucket, extend in place; otherwise append a new bucket and (for fixed-window charts) drop the leftmost. Test this explicitly with mocked timers.

3. **Store growth with 5 sources × 30d.** Worst case ~500 MB of gzipped JSON. Default Pi-class disk handles it; the existing `Prune` ticker keeps it bounded. Operator-tunable via `store.retention_days`. Document in README. Not blocking.

4. **Volcanoes diff churn.** Most days have zero state changes. The diff-feed row will display "no changes in window" most of the time. UX-acceptable: shows the operational reality (volcano alerts ARE rare). Document.

5. **SWPC scales chart legibility.** Three series (G-scale stepped + R1 dashed + S1 dashed) on one chart can read busy. Mitigation: subdued colors for the dashed probability lines, bold for the G-scale step. If it remains hard to scan, fall back to "G-scale only" with R1/S1 as tooltip-on-hover values.

6. **`/api/history` cardinality on first paint.** 5 parallel requests at 200 buckets each = ~1000 data points on first render. ~50 KB total. Fine.

7. **Server-side bucket aggregation cost.** A 30d window query touches ~43k rows for NWS/quakes. Each row's gzipped payload is ~5 KB; SQLite returns them in one query, the aggregator decompresses + parses each. Worst case ~200 ms server-side per source. Fine for the SPA's parallel-fetch pattern. If it ever drags, we add a materialized-view layer (Phase 5+).

---

## 13. Open questions

None remaining. All 8 brainstorm decisions answered. Implementation plan should reference §5.1 (range queries), §5.3 (HTTP shape), §7.3 (chart components) as the load-bearing contracts.
