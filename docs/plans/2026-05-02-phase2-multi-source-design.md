# Self-Hosted CWD — Phase 2 (Remaining four sources) — Design Document

**Date:** 2026-05-02
**Status:** Approved (verbal, all 9 brainstorm questions)
**Parent design:** [docs/plans/2026-05-01-self-hosted-cwd-design.md](2026-05-01-self-hosted-cwd-design.md)
**Phase 1 design:** [docs/plans/2026-05-02-phase1-nws-alerts-design.md](2026-05-02-phase1-nws-alerts-design.md)
**Phase 1 implementation:** [docs/plans/2026-05-02-phase1-nws-alerts-implementation.md](2026-05-02-phase1-nws-alerts-implementation.md)
**Recon dependency:** [docs/recon/2026-05-01-ncep-cwd-status-recon.md](../recon/2026-05-01-ncep-cwd-status-recon.md) §5 (per-source endpoints + cadences)
**Phase 1 baseline:** main at the merge commit of PR #2

---

## 1. Goal & scope

Wire the remaining four sources end-to-end (`swpc_scales`, `swpc_alerts`, `usgs_quakes`, `usgs_volcanoes`) using exactly the contracts Phase 1 established. Bring the Phase 0 placeholder pages `/space` and `/events` to life. Migrate the Phase 1 TsunamiPanel from Overview to the new Events page (Phase 1 design Q5).

**Demo bar (per parent design §11):** *Feature parity with the original NCEP CWD page's dynamic sections.* Static maps (Hazards page) remain Phase 3.

**Explicitly NOT in scope (deferred):**
- Status banner (NORMAL / CWD / ENHANCED_CAUTION) — Phase 6.
- 3-day Outlook timeline on Overview — Phase 6.
- Per-source teaser summaries on Overview that match the upstream "everything at a glance" landing — Phase 6.
- Image proxy + Hazards page — Phase 3.
- Polygon/map rendering — Phase 4 (history slider) at the earliest.
- HazSimp category map expansion (Tornado Watch, Flood Warning, Freeze Watch, etc.) — tracked in [issue #3](https://github.com/jacaudi/cwd/issues/3).
- USGS quake magnitude/depth threshold knobs — comment-noted in code; not wired in v1.
- USGS volcanoes RSS-feed fallback — comment-noted in code; not wired in v1.

---

## 2. Decisions log

Captured during the 2026-05-02 brainstorm.

| # | Question | Decision |
|---|---|---|
| 1 | PR scope | **Single PR for all four sources.** They share Phase 1's `Source`/`Fetcher`/`Cache`/`Store` contract; the work is structurally symmetric and the per-task subagent flow gives per-source review checkpoints. |
| 2 | Overview page | **Phase-1-shaped, unchanged in this round.** New sources land on `/space` and `/events`. Phase 6 is when Overview becomes the "everything at a glance" landing that matches the upstream NCEP CWD page (status banner, 3-day outlook, per-source teasers). |
| 3 | SWPC scales surface | **3-day forecast cards only (days 1, 2, 3).** Matches the upstream page. Day-0 ("now") and day-(-1) ("yesterday") are skipped in v1; can be added in Phase 6 alongside the status banner if needed. |
| 4 | SWPC alerts filter | **Honor the parent design's config knobs.** `derived.thresholds.swpc_alert_window_hours` (default 24) AND `derived.thresholds.swpc_alert_products` (default `[K08A, K09A, P12A, P13A]`) applied server-side. Comment block in `internal/sources/swpc_alerts.go` enumerates the wider product code namespace so operators can extend their config without spelunking SWPC docs. |
| 5 | USGS quakes feed | **`significant_day.geojson`, no extra threshold knob.** USGS already curates this feed to the events worth surfacing. Comment block in `internal/sources/usgs_quakes.go` enumerates the alternative feeds (`4.5_day`, `2.5_day`, `1.0_day`, `all_day`) and the future `derived.thresholds.quake_{min_magnitude, max_depth_km}` knob shape we'd wire when the upgrade path is needed. |
| 6 | USGS volcanoes feed | **JSON `getElevatedVolcanoes` endpoint.** Cleaner contract than RSS; no new XML dependency. Comment block in `internal/sources/usgs_volcanoes.go` notes the legacy `elevatedRSS` feed for the future fallback path the parent design originally listed. |
| 7 | HazSimp category map | **Unchanged in Phase 2.** The Phase 1 drift canary surfaces real Severe/Extreme events the badge bar treats as `Unknown` (Tornado Watch, Flood Warning, Freeze Watch, Fire Weather Watch, Special Marine Warning), but expanding the map diverges from upstream parity and is a separate axis from Phase 2's source-add work. Tracked in [issue #3](https://github.com/jacaudi/cwd/issues/3) for a follow-up PR after Phase 2 lands. |
| 8 | Tsunami panel home | **Migrate to `/events`.** Per Phase 1 design Q5: "we promote the tsunami list there in Phase 2 when earthquakes + volcanoes go live." `Overview.tsx` drops the panel mount; the `AlertBadgeBar` Tsunami-badge `onClick` now navigates to `/events#tsunami` instead of scrolling within Overview. |
| 9 | Region filter scope | **NWS-only, unchanged.** The `internal/sse/hub.go applyFilter` keeps its `name == "nws_alerts"` guard. Space-weather and global quakes/volcanoes aren't UGC/WFO-coded — geographic filtering doesn't apply to them. |

---

## 3. Upstream constraints worth calling out

| Source | Endpoint | Cadence | ETag/Last-Modified? | Validator strategy |
|---|---|---|---|---|
| `swpc_scales` | `https://services.swpc.noaa.gov/products/noaa-scales.json` | 60s | **No** (only `Cache-Control: max-age=60`) | sha256 content hash, same as nws_alerts |
| `swpc_alerts` | `https://services.swpc.noaa.gov/products/alerts.json` | 60s | **No** | sha256 content hash |
| `usgs_quakes` | `https://earthquake.usgs.gov/earthquakes/feed/v1.0/summary/significant_day.geojson` | 60s | **Yes** | upstream ETag passthrough |
| `usgs_volcanoes` | `https://volcanoes.usgs.gov/hans-public/api/volcano/getElevatedVolcanoes` | 5m | **Yes** | upstream ETag passthrough |

The Phase 1 fetcher already treats the validator as opaque (`FetchResult.Validator string`), so a source returns whichever shape its upstream supports. SWPC sources reuse Phase 1's content-hash path; USGS sources exercise the original ETag path the parent design specified.

**Politeness:** every upstream request carries `User-Agent: cwd-self-host/<version> (<contact>)` (Phase 1 already wires this through `buildUserAgent` in `internal/server/server.go`; new sources just inherit). USGS doesn't loud-grump about UAs but accepts the same shape.

---

## 4. Architecture additions

### 4.1 Package additions

```
internal/
├── sources/
│   ├── swpc_scales.go              NEW
│   ├── swpc_scales_test.go         NEW
│   ├── swpc_alerts.go              NEW (carries SWPC product code namespace comment)
│   ├── swpc_alerts_test.go         NEW
│   ├── usgs_quakes.go              NEW (carries alternative-feed + threshold-knob comment)
│   ├── usgs_quakes_test.go         NEW
│   ├── usgs_volcanoes.go           NEW (carries RSS-fallback comment)
│   ├── usgs_volcanoes_test.go      NEW
│   └── testdata/
│       ├── swpc_scales_typical.json        NEW (live capture)
│       ├── swpc_alerts_typical.json        NEW (live capture)
│       ├── usgs_quakes_typical.geojson     NEW (live capture)
│       ├── usgs_quakes_empty.geojson       NEW (hand-crafted, zero features)
│       ├── usgs_volcanoes_typical.json     NEW (live capture)
│       └── README.md                       MOD (table extended)
├── api/
│   ├── snapshot.go                 MOD  (Sources map gains 4 typed slots)
│   └── snapshot_test.go            MOD  (asserts all 5 sources round-trip)
└── server/
    └── server.go                   MOD  (boot wires 4 more Source constructors,
                                          4 more fetchers, 4 more hub-source-names,
                                          4 more hot-start lookups)
```

```
web/
├── src/
│   ├── api/
│   │   └── types.ts                MOD  (4 typed Envelope slots; 5 source-payload types)
│   ├── components/
│   │   ├── SWPCForecast.tsx        NEW + .test.tsx
│   │   ├── SWPCAlerts.tsx          NEW + .test.tsx
│   │   ├── EarthquakeList.tsx      NEW + .test.tsx
│   │   ├── VolcanoList.tsx         NEW + .test.tsx
│   │   └── TsunamiPanel.tsx        UNCHANGED (consumed from Events instead of Overview)
│   └── pages/
│       ├── Overview.tsx            MOD  (drop TsunamiPanel mount; redirect badge click)
│       ├── Overview.test.tsx       MOD  (drop the tsunami-related assertion)
│       ├── SpaceWeather.tsx        REWRITE (replace <Empty> with forecast + alerts blocks)
│       ├── SpaceWeather.test.tsx   NEW
│       ├── Events.tsx              REWRITE (replace <Empty> with tsunami + quakes + volcanoes blocks)
│       └── Events.test.tsx         NEW
```

`Taskfile.yml` `smoke` target gains a multi-source `jq` loop that asserts all 5 `/api/sources` entries show `consecutiveFailures: 0` and a recent `lastSuccess`.

### 4.2 Source contract continuity

Every new source implements the Phase 1 `internal/sources.Source` interface verbatim:

```go
type Source interface {
    Name() string
    Interval() time.Duration
    Fetch(ctx context.Context) (FetchResult, error)
}
```

No interface change. Each constructor follows the Phase 1 pattern (`NewSWPCScales(url, ua string, logger *slog.Logger)`, etc.) and exposes `SetInterval(d)` for the config env walker (already wired in Phase 1's `applyEnvOverrides`).

---

## 5. Wire types

Phase 2 surface; matches parent design §5.2 with all five source slots populated.

```go
// internal/sources/swpc_scales.go

type GScale string  // "G1"…"G5" or "" when zero

type SWPCDay struct {
    Date  string `json:"date"`            // "2026-05-03" UTC
    R1    int    `json:"r1"`              // % chance R1+ event
    R3    int    `json:"r3"`              // % chance R3+ event
    S1    int    `json:"s1"`              // % chance S1+ event
    G     GScale `json:"g,omitempty"`     // worst predicted G-scale
    GText string `json:"gText,omitempty"` // free-text note from SWPC
}

type SWPCForecast struct {
    Days [3]SWPCDay `json:"days"`         // tomorrow, +2, +3
}
```

```go
// internal/sources/swpc_alerts.go

type SWPCAlert struct {
    Code        string    `json:"code"`        // "K08A", "P12A", "WARK04W", …
    Series      string    `json:"series"`      // "K-Index", "Proton-Event", "Geomagnetic-Watch", …
    Description string    `json:"description"` // short human label resolved from code → name table
    Issued      time.Time `json:"issued"`
    Message     string    `json:"message"`     // first ~512 bytes of upstream free-text body
    URL         string    `json:"url,omitempty"`
}
```

```go
// internal/sources/usgs_quakes.go

type Quake struct {
    ID        string    `json:"id"`         // USGS event id
    Magnitude float64   `json:"magnitude"`
    Place     string    `json:"place"`
    Time      time.Time `json:"time"`       // event origin time
    UpdatedAt time.Time `json:"updatedAt"`
    Lat       float64   `json:"lat"`
    Lon       float64   `json:"lon"`
    DepthKm   float64   `json:"depthKm"`
    Tsunami   bool      `json:"tsunami"`    // USGS tsunami-flag bit
    Alert     string    `json:"alert,omitempty"` // "green"|"yellow"|"orange"|"red" PAGER level
    URL       string    `json:"url,omitempty"`
}
```

```go
// internal/sources/usgs_volcanoes.go

type AlertLevel string  // "NORMAL" | "ADVISORY" | "WATCH" | "WARNING"
type ColorCode  string  // "GREEN" | "YELLOW" | "ORANGE" | "RED"

type Volcano struct {
    ID        string     `json:"id"`           // USGS volcano id
    Name      string     `json:"name"`
    Region    string     `json:"region"`       // "Alaska", "CalVO", …
    Lat       float64    `json:"lat"`
    Lon       float64    `json:"lon"`
    Alert     AlertLevel `json:"alert"`
    Color     ColorCode  `json:"color"`
    UpdatedAt time.Time  `json:"updatedAt"`
    Synopsis  string     `json:"synopsis,omitempty"` // current activity note
    URL       string     `json:"url,omitempty"`
}
```

```go
// internal/api/snapshot.go (extended)

type SourcesMap struct {
    NWSAlerts      *Envelope[[]sources.Alert]      `json:"nws_alerts,omitempty"`
    SWPCScales     *Envelope[sources.SWPCForecast] `json:"swpc_scales,omitempty"`
    SWPCAlerts     *Envelope[[]sources.SWPCAlert]  `json:"swpc_alerts,omitempty"`
    USGSQuakes     *Envelope[[]sources.Quake]      `json:"usgs_quakes,omitempty"`
    USGSVolcanoes  *Envelope[[]sources.Volcano]    `json:"usgs_volcanoes,omitempty"`
}
```

(In practice the existing `map[string]Envelope` in `snapshot.go` continues to do the work; the explicit struct above is the conceptual wire-shape contract — present-source-only via map-key absence.)

The frontend `web/src/api/types.ts` mirrors these types verbatim with TypeScript:

```ts
export type GScale = 'G1' | 'G2' | 'G3' | 'G4' | 'G5';

export interface SWPCDay { date: string; r1: number; r3: number; s1: number; g?: GScale; gText?: string; }
export interface SWPCForecast { days: [SWPCDay, SWPCDay, SWPCDay]; }
export interface SWPCAlert    { code: string; series: string; description: string; issued: string; message: string; url?: string; }
export interface Quake        { id: string; magnitude: number; place: string; time: string; updatedAt: string; lat: number; lon: number; depthKm: number; tsunami: boolean; alert?: string; url?: string; }
export type AlertLevel = 'NORMAL' | 'ADVISORY' | 'WATCH' | 'WARNING';
export type ColorCode  = 'GREEN'  | 'YELLOW'   | 'ORANGE' | 'RED';
export interface Volcano { id: string; name: string; region: string; lat: number; lon: number; alert: AlertLevel; color: ColorCode; updatedAt: string; synopsis?: string; url?: string; }

export interface Snapshot {
  serverTime: string;
  sources: {
    nws_alerts?:     Envelope<Alert[]>;
    swpc_scales?:    Envelope<SWPCForecast>;
    swpc_alerts?:    Envelope<SWPCAlert[]>;
    usgs_quakes?:    Envelope<Quake[]>;
    usgs_volcanoes?: Envelope<Volcano[]>;
  };
}
```

---

## 6. Data flow per source

Each source plugs into the same Phase 1 pipeline: `fetcher` ticker → `Source.Fetch` → diff-on-write → `cache.Set` → `store.Append`. No fetcher changes. No cache changes. No SSE hub structural changes (only one new line: the hub's `enabledNames` list grows from 1 to 5).

### 6.1 SWPC scales (`internal/sources/swpc_scales.go`)

```
Fetch → GET noaa-scales.json → parse "1"/"2"/"3" keys
  → For each day: R[1], R[3], S[1] percentages + G summary
    → Derive worst G-scale from G map (highest G-rating with non-zero MajorProb)
    → Build SWPCForecast{Days: [3]SWPCDay{...}}
  → Validator = "sha256:" + hex(rawBody)
```

**Code-level comment** documents the shape of the upstream JSON (key per relative day, R/S/G subkeys, MajorProb fields) and notes that day-0 + day-(-1) are deliberately skipped in v1 — they're parsed-but-discarded so the future Phase 6 path can flip a flag.

### 6.2 SWPC alerts (`internal/sources/swpc_alerts.go`)

```
Fetch → GET alerts.json → array of {product_id, issue_datetime, message}
  → Resolve product_id → series + description via internal table
  → Filter: keep iff product_id ∈ swpc_alert_products AND issue_datetime within
    swpc_alert_window_hours of now
  → Truncate message to first 512 bytes
  → Validator = "sha256:" + hex(rawBody)
```

The product-code-to-description table lives next to the parser as a `var swpcCodeTable = map[string]struct{ Series, Description string }{...}` initialized at package load. Comment block immediately above documents:

```go
// SWPC product code namespace (operator menu — NOT all of these are in the
// default swpc_alert_products allowlist; extend your config to opt in).
//
// K-Index series (planetary geomagnetic, K = 0..9):
//   K04A — K-index of 4 (active conditions; not yet a storm)
//   K05A — K-index of 5 = G1 minor storm
//   K06A — K-index of 6 = G2 moderate storm
//   K07A — K-index of 7 = G3 strong storm
//   K08A — K-index of 8 = G4 severe storm                          [DEFAULT]
//   K09A — K-index of 9 = G5 extreme storm                         [DEFAULT]
//
// Geomagnetic warnings + watches (forecasts, not retrospective):
//   WARK04W — K-index ≥ 4 expected (warning)
//   WARK05W — K-index ≥ 5 expected
//   WARK06W — K-index ≥ 6 expected
//   WARK07W — K-index ≥ 7 expected
//   WARK08W — K-index ≥ 8 expected
//   WARK09W — K-index ≥ 9 expected
//   WATA50W — Geomagnetic A-index ≥ 50 watch
//
// Solar proton event (≥10 MeV flux at GOES, in pfu):
//   P10A — ≥ 10 pfu (S1)
//   P11A — ≥ 100 pfu (S2)
//   P12A — ≥ 1,000 pfu (S2/S3 boundary)                            [DEFAULT]
//   P13A — ≥ 10,000 pfu (S3)                                       [DEFAULT]
//   P14A — ≥ 100,000 pfu (S4)
//   P15A — ≥ 1,000,000 pfu (S5)
//
// Radio blackout / X-ray flare:
//   RWAR  — Radio blackout warning
//   X1XW  — X-ray flux ≥ X1 (R3 radio blackout)
//   X10XW — X-ray flux ≥ X10 (R5)
//
// Type II / Type IV radio sweeps (CME signatures):
//   SUM01R — Type II radio sweep
//   SUM02R — Type IV radio sweep
//
// SWPC summary / discussion:
//   SUM01D — Daily 3-day forecast discussion
//
// To extend coverage, add codes to derived.thresholds.swpc_alert_products
// and (if needed) add an entry to swpcCodeTable below for the human label.
```

### 6.3 USGS quakes (`internal/sources/usgs_quakes.go`)

```
Fetch → GET significant_day.geojson with If-None-Match: <lastETag>
  → On 304: return cached validator, no payload change (fetcher skips broadcast)
  → On 200: parse FeatureCollection
    → For each feature: extract magnitude, place, time, coords, depth, tsunami flag
    → Sort descending by magnitude (consumer expects newsworthy-first)
  → Validator = upstream ETag verbatim (already a sha-prefixed string from USGS)
```

**Code-level comment** documents the alternative feed paths and the future threshold-knob shape:

```go
// USGS earthquake feeds — operator menu.
//
// We use significant_day.geojson because USGS already curates it to
// newsworthy events (M4.5+ globally, plus felt events ≥ M2.5 in CONUS,
// plus tsunami-flagged quakes regardless of magnitude). Polled every 60s
// matches USGS Cache-Control. Each feed: same GeoJSON shape, different cutoffs.
//
// Alternatives (not used in v1; flip the URL constant to switch):
//   significant_hour.geojson      — past hour, ~1-3 events
//   significant_day.geojson       — past 24h, ~5-20 events                  [DEFAULT]
//   significant_week.geojson      — past 7 days
//   significant_month.geojson     — past 30 days
//   4.5_day.geojson               — all M4.5+, ~30-60/day
//   2.5_day.geojson               — all M2.5+, ~150/day
//   1.0_day.geojson               — all M1.0+, ~500/day (CONUS heavy)
//   all_day.geojson               — every detection, ~5000/day
//
// If you switch to a higher-volume feed, add a server-side threshold:
//   derived:
//     thresholds:
//       quake_min_magnitude: 5.0          # drop everything below
//       quake_max_depth_km:  300          # drop deep-focus events
// (Knob shape is reserved; not wired in v1.)
```

### 6.4 USGS volcanoes (`internal/sources/usgs_volcanoes.go`)

```
Fetch → GET getElevatedVolcanoes (JSON array) with If-None-Match
  → On 304: no payload change
  → On 200: parse array
    → Drop entries where alert_level == "NORMAL" (page only shows elevated)
    → Sort by Region then Name (stable display)
  → Validator = upstream ETag
```

**Code-level comment** notes the legacy RSS endpoint:

```go
// USGS volcano feeds — operator menu.
//
// Primary: hans-public/api/volcano/getElevatedVolcanoes (JSON, ETag).
// Cleaner contract than the legacy RSS feed: structured fields, no XML
// dependency, fewer regex-heavy edge cases.
//
// Legacy fallback (vhpss/api/volcano/elevatedRSS — ATOM/RSS, ETag) exists
// and is what the parent design originally listed. If the JSON endpoint is
// ever deprecated upstream, swap to the RSS feed and add a small XML-to-
// Volcano parser. Polling cadence stays 5min.
```

### 6.5 SSE hub change

`internal/sse/hub.go` already supports an arbitrary `sources []string` list; only the `applyFilter` helper hardcodes `name == "nws_alerts"`. That guard stays — region filter is geographic, not space-weather-applicable. No code change to the hub itself; `internal/server/server.go` just builds `enabledNames` from the new source map.

### 6.6 Hot-start

`internal/server/server.go` already loops `for name := range srcs` and calls `store.Latest`. With Phase 2's source map having up to 5 entries, this just iterates more. The decode path needs a per-source type assertion:

```go
switch name {
case "nws_alerts":     var p []sources.Alert     /* json.Unmarshal */
case "swpc_scales":    var p sources.SWPCForecast
case "swpc_alerts":    var p []sources.SWPCAlert
case "usgs_quakes":    var p []sources.Quake
case "usgs_volcanoes": var p []sources.Volcano
}
```

A small helper (`hotStartDecode(name string, raw []byte) (any, error)`) keeps the switch contained to one function. Cache.Set takes the typed payload; downstream consumers (snapshot handler, hub, page components) already type-assert at point of use.

---

## 7. Frontend behavior

### 7.1 SpaceWeather page (`/space`)

Replaces the Phase 0 `<Empty>` placeholder. Two AntD `ProCard.Group` blocks stacked vertically (`Space direction="vertical"`):

**Block 1 — 3-day SWPC forecast.** Three `ProCard.StatisticCard`s side by side via `ProCard.Group`. Each card:
- **Title:** day label ("Tomorrow" / "+2 days" / "+3 days") + UTC date subtitle.
- **Body:** three labeled `Statistic` rows (R1, R3, S1) showing percentages with `Progress` bars colored by severity threshold (≥50 → orange, ≥75 → red).
- **Footer:** G-scale chip — colored Tag (`G1`-`G5` → green/yellow/orange/red/magenta), or "G-scale: none" muted text when absent.
- **Tooltip on hover:** the day's `gText` field plus `as of <fetchedAt UTC>`.

**Block 2 — Active SWPC alerts.** AntD `List` (no header — let card chrome carry the title). One row per alert:
- Code as colored Tag (color from a per-series palette: K-* magenta, P-* gold, WARK-* volcano).
- Series + Description as `List.Item.Meta.title`.
- Issued time as relative ("12m ago") via `dayjs.relative` (existing dep).
- First line of `message` truncated to ~120 chars as description.

Empty state: `<Empty description="No active SWPC alerts in window">`. Component gracefully renders `Skeleton` while the snapshot is loading.

### 7.2 Events page (`/events`)

Replaces the Phase 0 `<Empty>` placeholder. Three blocks, top-to-bottom; each block self-hides when its source has zero items.

**Block 1 — Tsunami panel.** The Phase 1 `TsunamiPanel` component, anchored at `id="tsunami"` for the deep-link from `AlertBadgeBar`. Same component, same data path (filtered subset of `nws_alerts.payload` where `awips` starts with `"TSU"`).

**Block 2 — Significant earthquakes.** `EarthquakeList` component. AntD `List` with header `"Significant earthquakes"`. One row per quake:
- **Magnitude Tag** — color thresholds: `M < 5.0` blue, `5.0 ≤ M < 6.0` orange, `6.0 ≤ M < 7.0` red, `M ≥ 7.0` magenta.
- **Place** as title.
- **Description** line: depth (`depthKm km`), time (relative), 🌊 icon if `tsunami: true`, optional PAGER alert level Tag.
- **Action:** external link to `url`.

Hidden entirely when the snapshot's `usgs_quakes.payload` is empty.

**Block 3 — Volcanoes at elevated alert.** `VolcanoList` component. AntD `List` with header `"Volcanoes at elevated alert"`. One row per volcano:
- **Title:** name.
- **Description:** region — alert level Tag (`ADVISORY` yellow, `WATCH` orange, `WARNING` red) — color code Tag — `synopsis` truncated to ~150 chars — `as of` relative timestamp.
- **Action:** external link.

Hidden when the snapshot's `usgs_volcanoes.payload` is empty.

### 7.3 Overview change

Single change: `Overview.tsx` drops the `<TsunamiPanel>` render (and the `tsuRef` + `onTsunamiClick` callback). The `AlertBadgeBar`'s `onTsunamiClick` is rebuilt as a `useNavigate` redirect to `/events#tsunami`. The existing `Overview.test.tsx` drops its tsunami-related assertion; smoke that the page renders the badge bar continues to pass.

### 7.4 Footer / source health

`SourceHealthIndicator` is unchanged. It already renders one Tag per key in `/api/sources` response; with Phase 2 the response carries 5 keys, so 5 Tags appear. Color rules (green / yellow / red) carry forward.

---

## 8. Configuration

No new config keys. Everything Phase 2 consumes is already in the schema (parent design §8) and was Phase 0-validated:

- `sources.{swpc_scales,swpc_alerts,usgs_quakes,usgs_volcanoes}.{interval,enabled}` — Phase 1 defined the type; Phase 2 actually consumes them.
- `derived.thresholds.swpc_alert_window_hours` — int, default 24.
- `derived.thresholds.swpc_alert_products` — []string, default `[K08A, K09A, P12A, P13A]`.

**Validation at boot** (extends Phase 1):
- For each enabled source with `interval < 10s` → WARN (matches Phase 1 NWS pattern). Already wired in Phase 1 for nws_alerts via the `interval` floor; Phase 2 generalizes the warning to all sources.
- `swpc_alert_window_hours <= 0` → clamp to 24 + WARN.
- `swpc_alert_products` entries failing `/^[A-Z][A-Z0-9]{2,7}$/` → WARN-and-drop (mirrors Phase 1's `validateRegionFilter`). Regex accepts the documented SWPC namespace (K-series, P-series, WARK*/WATA*/RWAR/X-series, SUM*) and every entry in `defaults()`. The pre-implementation draft `^[A-Z]{3,8}[0-9A-Z]?$` rejected all defaults (K08A/K09A/P12A/P13A) and was corrected before code was written.
- For each enabled source with `server.contact == ""` → WARN at boot (already wired via `cfg.MissingContact()`).

**Per-source env override pattern carries forward unchanged:**

```bash
CWD_SOURCES_SWPC_SCALES_INTERVAL=120s
CWD_SOURCES_USGS_VOLCANOES_ENABLED=false
CWD_SOURCES_USGS_QUAKES_INTERVAL=30s
```

Already supported by Phase 1 Task 14's `applyEnvOverrides` walker — Phase 2 adds zero env-walker code.

---

## 9. Testing

### 9.1 Backend (Go)

| Layer | Coverage |
|---|---|
| `internal/sources/swpc_scales_test.go` | Fixture-driven. Cases: typical 5-day shape (parses days 1/2/3, ignores 0/-1), missing day-1 (defensive), malformed JSON, validator deterministic for identical body, G-scale derivation across all 5 levels. |
| `internal/sources/swpc_alerts_test.go` | Fixture with mix of K-series, P-series, WARK warnings, type-II radio sweeps. Window filter (24h cutoff applied at fixed `time.Now()` via injected clock) verified independently of allowlist filter. Code-to-description resolver round-trips known codes; unknown code passes through with empty description (NOT canary-warned — drift on SWPC isn't operationally as urgent as NWS, and unknown codes are dropped by the allowlist anyway). |
| `internal/sources/usgs_quakes_test.go` | Empty `features` → empty result. Mixed magnitudes + places. `tsunami: true` flag round-trips. ETag passthrough verified via httptest. Sort-by-magnitude-descending verified. |
| `internal/sources/usgs_volcanoes_test.go` | Fixture with 8-12 volcanoes at various alert levels. `NORMAL`-level entries are filtered out before storage. ETag passthrough verified. Sort by region/name stable. |
| `internal/api/snapshot_test.go` (extend) | Cache populated with all 5 sources → JSON shape matches the parent design wire shape. Absent sources omitted (map-key absence, NOT null fields). |
| `internal/server/server_test.go` (extend) | Boot with httptest fake-NOAA + fake-SWPC + fake-USGS. All 5 fetchers fire. All 5 hot-start. `/readyz` flips to 200 within `max(intervals) + epsilon`. Hot-start works for each source's payload type (round-trips through json.Marshal/Unmarshal). |

Coverage target unchanged: 80% on `internal/`. Backend layer count is unchanged from Phase 1 (just 4× more sources).

### 9.2 Frontend (Vitest + RTL)

| Layer | Coverage |
|---|---|
| `web/src/components/SWPCForecast.test.tsx` | Empty payload → all 3 cards render with disabled/skeleton state. Mixed payload (G2 + G4) → correct chip color and percentage rendering. R/S percentages above thresholds → progress bars in correct color. |
| `web/src/components/SWPCAlerts.test.tsx` | Empty → `<Empty>` block. With alerts → list rows with relative time + description. Per-series color tag verified. |
| `web/src/components/EarthquakeList.test.tsx` | Empty → component returns `null` (block hidden). Mixed magnitudes → correct color tag per row. Tsunami flag renders 🌊 icon. PAGER alert tag renders when present. External link href is the source URL. |
| `web/src/components/VolcanoList.test.tsx` | Empty → component returns `null`. Mixed alert levels → correct AntD Tag color per row. Color-code chip rendered. External link href is the source URL. |
| `web/src/pages/Events.test.tsx` (NEW) | Smoke: page renders all three blocks given a populated snapshot. Renders compact (no `<Empty>` spam) given empty sources. Tsunami anchor `id="tsunami"` is reachable. |
| `web/src/pages/SpaceWeather.test.tsx` (NEW) | Smoke: forecast block always renders (with skeleton when no payload). Alerts block hides cleanly when empty. |
| `web/src/pages/Overview.test.tsx` (modify) | Drop the tsunami-panel-rendered assertion. Add: badge bar's Tsunami-badge click navigates to `/events#tsunami` (use `MemoryRouter` + `useLocation` spy). |

Frontend test count grows from 12 (Phase 1) to ~22.

### 9.3 Smoke (manual, opt-in)

`task smoke` extended:

```yaml
  smoke:
    desc: Boot against real upstreams for ~5 minutes; report all source health
    deps: [build:all]
    cmds:
      - |
        ./{{.BIN}} serve &
        PID=$!
        trap "kill $PID 2>/dev/null || true" EXIT
        echo "PID=$PID; sleeping 90s for first fetch of slowest source"
        sleep 90
        echo "---- /api/sources ----"
        curl -sS http://127.0.0.1:8765/api/sources | jq .
        echo "---- per-source counts ----"
        curl -sS http://127.0.0.1:8765/api/snapshot | jq '
          .sources | to_entries | map({
            (.key): (
              if (.value.payload | type) == "array" then (.value.payload | length)
              else (.value.payload.days | length)
              end
            )
          }) | add'
        echo "---- holding for 4 more minutes (Ctrl-C to stop early) ----"
        sleep 240
```

Asserts every enabled source shows `consecutiveFailures: 0` after warm-up; expected counts: nws_alerts ~250-350, swpc_alerts 0-20, usgs_quakes 0-20, usgs_volcanoes 5-15, swpc_scales (always 3 days).

---

## 10. `/api/sources` shape (Phase 2 example)

```json
{
  "nws_alerts":     { "intervalSec": 30,  "lastSuccess": "...", "ageSec":  12, "consecutiveFailures": 0, "etag": "sha256:..." },
  "swpc_scales":    { "intervalSec": 60,  "lastSuccess": "...", "ageSec":  45, "consecutiveFailures": 0, "etag": "sha256:..." },
  "swpc_alerts":    { "intervalSec": 60,  "lastSuccess": "...", "ageSec":  20, "consecutiveFailures": 0, "etag": "sha256:..." },
  "usgs_quakes":    { "intervalSec": 60,  "lastSuccess": "...", "ageSec":  35, "consecutiveFailures": 0, "etag": "W/\"abc...\"" },
  "usgs_volcanoes": { "intervalSec": 300, "lastSuccess": "...", "ageSec": 180, "consecutiveFailures": 0, "etag": "W/\"def...\"" }
}
```

Footer health indicator renders 5 colored Tags. Existing color rules (green / yellow / red) work unchanged.

---

## 11. Definition of done

- [ ] Design doc + implementation plan committed under `docs/plans/`
- [ ] All work in `.worktrees/phase2-multi-source` on branch `feature/phase2-multi-source` off main
- [ ] All Go tests pass with `-race`; `golangci-lint run ./...` clean
- [ ] All frontend tests pass (`task web:test`); `task web:typecheck` clean; `task web:build` succeeds
- [ ] `task build:all && ./bin/cwd serve` shows live data on `/space` (3-day forecast cards + alerts list) and `/events` (tsunami panel + significant earthquakes list + elevated volcanoes list); footer shows 5 green source tags
- [ ] Independent comprehensive review passes
- [ ] PR opened to main, CI green, merged with per-task history preserved
- [ ] Worktree cleaned up
- [ ] Issue #3 referenced in PR description as the tracked follow-up for category map expansion

---

## 12. Explicitly out of scope

- Status banner (NORMAL / CWD / ENHANCED_CAUTION) — Phase 6
- 3-day Outlook timeline on Overview — Phase 6
- Per-source teaser cards on Overview — Phase 6 (matches upstream NCEP CWD layout)
- Image proxy `/img/*` + Hazards page — Phase 3
- History page slider UI — Phase 4
- PWA service worker, Web Push — Phase 5
- HazSimp category map expansion (Tornado Watch, Flood Warning, etc.) — issue #3
- USGS quake threshold knobs (`quake_min_magnitude`, `quake_max_depth_km`) — comment-noted, not wired
- USGS volcanoes RSS-feed fallback — comment-noted, not wired
- Polygon parsing / map rendering — earliest Phase 4
- Settings drawer turning region filter from read-only display into a writable form — design §6.2 says read-only in v1
- Settings drawer turning per-source thresholds (SWPC product list, etc.) into form fields — read-only in v2 too
- VTEC supersession dedup logic for badge counts — open question; revisit if/when ops complain

---

## 13. Risks (Phase 2 specific)

1. **SWPC product code drift.** SWPC has been known to add new product codes (e.g. when scales-mapping changes). The `swpcCodeTable` lookup falls back to empty description for unknown codes — visible in the SpaceWeather page as "no description" rows but functionally harmless. No canary log; the allowlist filter naturally drops drift before it reaches the page.
2. **USGS feed shape changes.** `significant_day.geojson` and `getElevatedVolcanoes` are stable but documented-mostly-by-example. Parser tests against committed fixtures lock current behavior; recapture during Phase 6 hardening.
3. **5-source polling load on slow self-hosters.** Five concurrent fetchers + diff-on-write may briefly contend on the SQLite write lock during the boot window when all 5 fire `Append` near-simultaneously. SQLite serializes writes (Phase 1 Task 5 set `SetMaxOpenConns(1)` for exactly this); worst case is a few-millisecond stall, not data loss. Becomes a Phase 4 concern when history-slider reads compete with writes.
4. **Volcanic alert downgrades to NORMAL during steady state.** The source filters out NORMAL-level volcanoes server-side. If a volcano was at WATCH yesterday and is at NORMAL today, it disappears from the page silently. The history endpoint preserves the WATCH state for historical replay; live page shows current truth only. Acceptable per design.
5. **SSE bandwidth.** Five sources broadcasting on diff means 5× the SSE frame rate compared to Phase 1. Diff-on-write keeps actual frame count low (most fetches are no-ops); pessimistic upper bound: 5 frames per minute under steady state, well under the existing per-client buf=16 backpressure window.
