# Self-Hosted CWD — Phase 1 (NWS Alerts End-to-End) — Design Document

**Date:** 2026-05-02
**Status:** Approved (verbal, all 8 brainstorm questions)
**Parent design:** [docs/plans/2026-05-01-self-hosted-cwd-design.md](2026-05-01-self-hosted-cwd-design.md)
**Phase 0 implementation:** [docs/plans/2026-05-01-self-hosted-cwd-phase0-skeleton-implementation.md](2026-05-01-self-hosted-cwd-phase0-skeleton-implementation.md)
**Recon dependency:** [docs/recon/2026-05-01-ncep-cwd-status-recon.md](../recon/2026-05-01-ncep-cwd-status-recon.md) — §5.3 covers `api.weather.gov/alerts/active`
**Phase 0 baseline:** main at commit `8a43b7a`

---

## 1. Goal & scope

Wire `https://api.weather.gov/alerts/active` end-to-end so the dashboard goes from "AntD shell" to "live alert badges + tsunami pane". Establishes every interface the parent design names (`Source`, `Fetcher`, `Cache`, `Store`, `Hub`, `/api/snapshot`, `/api/history`, `/api/stream`) using exactly one source. Phase 2 adds four more sources by following the same shape — no contract changes.

**Demo bar (per parent design §11):** Live alert badges + Tsunamis pane work.

---

## 2. Decisions log

Captured during the 2026-05-02 brainstorm.

| # | Question | Decision |
|---|---|---|
| 1 | Storage granularity | **Envelope-blob, diff-on-write.** One row per fetch where the response body changed. No per-alert decomposition in v1; the envelope blob preserves the full upstream JSON so per-alert tables can be added later without backfill. |
| 2 | Polygon parsing | **Skip in v1.** Typed `Alert` carries `id`, `event`, `awips`, `headline`, `severity`, `effective`, `expires`, `onset`, `ends`, `sent`, `areas`, `wfo`, `url`, `vtecEtn`, plus parsed UGC + SAME codes. Polygons recoverable later from the envelope blob. |
| 3 | Region filter | **Server-side predicate, default match-all.** `derived.thresholds.region_filter.{ugcs,wfos}` evaluated at `/api/snapshot`, `/api/history`, and SSE emit. Empty lists → match all. Store stays unfiltered. Badge counts reflect the filtered set. |
| 4 | SSE envelope shape | **Full §5.2 `Snapshot` with the four unimplemented sources omitted entirely** (`omitempty`). `status` and `outlook` also omitted (Phase 2/6). Frontend types code against `snapshot.sources.nws_alerts?` from day 1, which is the right pattern even when more sources arrive (any source can be missing in steady state). |
| 5 | Tsunami pane | **Overview page hosts both `AlertBadgeBar` and `TsunamiPanel`.** `/events` stays as the Phase 0 `<Empty>` placeholder; we promote the tsunami list there in Phase 2 when earthquakes + volcanoes go live. |
| 6 | Event-name → badge map | **Hard-coded map** in `internal/sources/nws_alerts.go` + `slog.Warn("nws_alerts.unmapped_severe_event", …)` on Extreme/Severe alerts whose `event` text is not in the map. Tsunami uses the AWIPS-prefix rule (`awips[0:3] == "TSU"`) rather than event-text matching. |
| 7 | Phase 0 reviewer carryovers | **All five in scope** as discrete tasks. |
| 8 | `/api/sources` + footer indicator | **In scope.** Single-tag footer for `nws_alerts` now; grows to N tags in Phase 2 with no shape change. |

---

## 3. Upstream constraint worth calling out

`api.weather.gov/alerts/active` returns **no `ETag` and no `Last-Modified`** (recon §8) — only `Cache-Control: public, max-age=9, s-maxage=30`. Conditional GET (`If-None-Match` / `If-Modified-Since`) is therefore **not available for this source**. Diff-on-write must use a content hash (sha256 over the raw response body, hex-encoded, stored in the same `etag` column for a uniform shape — `etag = "sha256:" + hex(sha256(body))`). Future sources with real ETags use them directly. The fetcher abstraction handles both: a `Source` returns `(payload, validator string)` and the fetcher treats the validator opaquely.

Other recon §8 constraints already addressed in Phase 0 but worth re-stating:
- Politeness: `User-Agent: cwd-self-host/<version> (<contact>)` on every request. `contact` from config; boot WARN if unset (Phase 0 wired the warning; Phase 1 threads the UA into the fetcher).
- Polling cadence: 30s, matching the upstream `s-maxage=30`. Per-source override via env var (carryover #1).

---

## 4. Architecture additions

Three concurrent subsystems per parent design §3 — Phase 0 shipped only the third's skeleton. Phase 1 brings the first two online and wires them to the third for one source.

```
                 ┌───────── single Go binary ─────────┐
                 │                                     │
   nws_alerts ──▶│  Fetcher ─▶ Cache ─▶ SSE Hub  ──┐  │──▶ EventSource (browser)
   /alerts/active│     │         │                  │  │
                 │     ▼         ▼                  ▼  │
                 │  SQLite store (30d ring)   /api/* ──│──▶ fetch() (browser)
                 │                                     │
                 │  embed.FS (Phase 0 SPA shell)       │
                 └─────────────────────────────────────┘
```

### 4.1 Package additions

```
internal/
├── sources/
│   ├── source.go              NEW  type Source interface
│   ├── nws_alerts.go          NEW  fetch + parse + category map + filter helper
│   ├── nws_alerts_test.go     NEW  fixture-driven parser tests
│   ├── filter.go              NEW  Filter struct + Match predicate
│   ├── filter_test.go         NEW
│   └── testdata/
│       └── alerts_active_*.json   NEW captured fixtures (multi-case)
├── fetcher/
│   ├── fetcher.go             NEW  ticker + content-hash diff + backoff + health stats
│   └── fetcher_test.go        NEW  uses httptest.Server
├── cache/
│   ├── cache.go               NEW  Get / Set / Subscribe per source name
│   └── cache_test.go          NEW
├── store/
│   ├── schema.sql             NEW  //go:embed
│   ├── store.go               NEW  modernc.org/sqlite wrapper
│   ├── snapshots.go           NEW  Append / Latest / At(t) / Prune
│   └── store_test.go          NEW  t.TempDir() round-trip
├── sse/
│   ├── hub.go                 NEW  broadcast + per-client buffered chan
│   ├── event.go               NEW  typed event marshaling
│   └── hub_test.go            NEW  multi-subscriber + slow-client backpressure
├── api/
│   ├── snapshot.go            NEW  GET /api/snapshot (region filter applied)
│   ├── snapshot_test.go       NEW
│   ├── history.go             NEW  GET /api/history?at=…
│   ├── history_test.go        NEW
│   ├── stream.go              NEW  GET /api/stream (SSE)
│   ├── stream_test.go         NEW
│   ├── sources.go             NEW  GET /api/sources
│   ├── sources_test.go        NEW
│   ├── healthz.go             MOD  add HEAD handlers
│   ├── healthz_test.go        MOD  cover HEAD method
│   ├── router.go              MOD  wire new routes; per-route WriteTimeout middleware
│   └── router_test.go         MOD
├── config/
│   ├── config.go              MOD  env walker reaches sources.<name>.{interval,enabled};
│   │                                slog.Warn on parse failure
│   └── config_test.go         MOD
└── server/
    └── server.go              MOD  wire fetchers→cache→store→hub on start;
                                    hot-start cache from store.Latest;
                                    /readyz predicate = first-success per enabled source
```

```
web/
├── package.json               MOD  add vitest, @testing-library/react, jsdom, @vitest/ui
├── vitest.config.ts           NEW
├── src/
│   ├── api/
│   │   ├── stream.ts          NEW  EventSource + reconnect + /api/snapshot fallback
│   │   └── types.ts           MOD  Alert, Envelope, Snapshot (Phase 1 subset)
│   ├── store/
│   │   └── snapshot.ts        NEW  Zustand: snapshot + connection state
│   ├── components/
│   │   ├── AlertBadgeBar.tsx           NEW
│   │   ├── AlertBadgeBar.test.tsx      NEW
│   │   ├── TsunamiPanel.tsx            NEW
│   │   ├── TsunamiPanel.test.tsx       NEW
│   │   └── SourceHealthIndicator.tsx   NEW
│   └── pages/
│       └── Overview.tsx       MOD  replace <Empty> with badge bar + tsunami panel
```

`Taskfile.yml` gains `web:test` (runs `pnpm --filter web test`); CI's existing `test-web` reusable workflow already invokes it once Vitest is installed.

---

## 5. Wire types

Phase 1 surface; matches parent design §5.2 with absent fields elided.

```go
// internal/sources/nws_alerts.go

type Severity string
const (
    SevExtreme Severity = "Extreme"
    SevSevere  Severity = "Severe"
    SevModerate Severity = "Moderate"
    SevMinor    Severity = "Minor"
    SevUnknown  Severity = "Unknown"
)

type Category string
const (
    CatTornado            Category = "Tornado"
    CatSevereThunderstorm Category = "SevereThunderstorm"
    CatFlashFlood         Category = "FlashFlood"
    CatTropical           Category = "Tropical"
    CatHighWind           Category = "HighWind"
    CatRedFlag            Category = "RedFlag"
    CatWinter             Category = "Winter"
    CatExtremeHeat        Category = "ExtremeHeat"
    CatExtremeCold        Category = "ExtremeCold"
    CatTsunami            Category = "Tsunami"
    CatUnknown            Category = "Unknown"
)

type Alert struct {
    ID        string    `json:"id"`
    Event     string    `json:"event"`
    AWIPS     string    `json:"awips"`              // properties.parameters.AWIPSidentifier[0]
    Headline  string    `json:"headline"`
    Severity  Severity  `json:"severity"`
    Sent      time.Time `json:"sent"`
    Effective time.Time `json:"effective"`
    Onset     time.Time `json:"onset,omitempty"`
    Expires   time.Time `json:"expires"`
    Ends      time.Time `json:"ends,omitempty"`
    Areas     []string  `json:"areas"`              // properties.areaDesc, split on "; "
    UGCs      []string  `json:"ugcs,omitempty"`     // properties.geocode.UGC
    SAMEs     []string  `json:"sames,omitempty"`    // properties.geocode.SAME
    WFO       string    `json:"wfo,omitempty"`      // 3-letter, parsed from properties.senderName or id
    VTEC_ETN  string    `json:"vtecEtn,omitempty"`  // properties.parameters.eventTrackingNumber[0]
    URL       string    `json:"url,omitempty"`
    Category  Category  `json:"category"`           // mapped; Unknown if no match
}
```

```go
// internal/api or shared types package

type Envelope[T any] struct {
    Source    string    `json:"source"`
    FetchedAt time.Time `json:"fetchedAt"`
    ETag      string    `json:"etag,omitempty"`
    Payload   T         `json:"payload"`
}

type Snapshot struct {
    ServerTime time.Time `json:"serverTime"`
    Sources    Sources   `json:"sources"`
    // Status, Outlook: Phase 2/6 — not present yet
}

type Sources struct {
    NWSAlerts *Envelope[[]Alert] `json:"nws_alerts,omitempty"`
    // SWPCScales, SWPCAlerts, USGSQuakes, USGSVolcanoes: Phase 2
}
```

Frontend mirror in `web/src/api/types.ts` uses TS optional chaining (`Snapshot.sources.nws_alerts?`).

### 5.1 Category map

Hard-coded in `internal/sources/nws_alerts.go`. Match by exact `properties.event` string except Tsunami, which matches `awips[0:3] == "TSU"`.

| `properties.event` | Category |
|---|---|
| `Tornado Warning` | Tornado |
| `Severe Thunderstorm Warning` | SevereThunderstorm |
| `Flash Flood Warning` | FlashFlood |
| `Storm Surge Warning`, `Hurricane Warning`, `Typhoon Warning`, `Tropical Storm Warning` | Tropical |
| `High Wind Warning`, `Extreme Wind Warning` | HighWind |
| `Red Flag Warning` | RedFlag |
| `Winter Storm Warning`, `Blizzard Warning`, `Ice Storm Warning`, `Snow Squall Warning` | Winter |
| `Extreme Heat Warning` | ExtremeHeat |
| `Extreme Cold Warning` | ExtremeCold |
| (any alert with `awips[0:3] == "TSU"`) | Tsunami (overrides any text match) |
| (otherwise) | Unknown |

**Drift canary:** when `Category == Unknown` AND (`Severity == Extreme` OR `Severity == Severe`), emit
`slog.Warn("nws_alerts.unmapped_severe_event", "event", a.Event, "severity", a.Severity, "id", a.ID)`.
Operators reading logs see the moment NWS renames a categorized event under HazSimp.

---

## 6. Data flow

### 6.1 Boot

1. `server.Run` → `store.Open(cfg.Store.Path)` → `store.Migrate()`.
2. For each enabled source: `cache.Set(name, store.Latest(name))` (hot start — SSE clients connecting in the cold-start window see real last-known data instead of "loading…").
3. Spawn `fetcher.Run(ctx, source, cache, store, hub)` per enabled source as a goroutine.
4. SSE hub registers one cache subscriber per source; fan-outs to connected clients.
5. HTTP listener starts last so `/readyz` is observable immediately (returns 503 until first success).

### 6.2 Fetch loop (per source)

```
ticker(interval, fire-immediately)
  ├─ Source.Fetch(ctx) — HTTP GET with UA header
  │   ├─ on 2xx: parse → typed []Alert
  │   │   ├─ validator = "sha256:" + hex(sha256(rawBody))
  │   │   ├─ if validator == cache.lastValidator: update lastSuccess only, no broadcast
  │   │   └─ else: cache.Set(name, env) → triggers subscribers; store.Append(name, env)
  │   ├─ on 4xx/5xx: log WARN; increment consecutiveFailures; backoff ×2 (cap 5×interval)
  │   └─ on parse error: log ERROR with body length + content-type; cache untouched
  └─ on success: reset backoff to interval
```

Backoff is per-source state on the fetcher; `consecutiveFailures` is exposed via `/api/sources`.

### 6.3 Cache → SSE

`cache.Subscribe(name)` returns `<-chan Envelope[any]` (or per-source typed chan via generics if ergonomic). Hub holds one sub per source. On envelope receipt, hub marshals an SSE `event: <name>.update` frame and writes to every connected client's per-client buffered chan (buf=8). Slow clients that fill their chan are disconnected; not blocking the broadcast loop.

### 6.4 HTTP

| Path | Behavior |
|---|---|
| `GET /api/snapshot` | Read cache; build `Snapshot` with only present sources; apply region filter to alerts; respond JSON. 5s `WriteTimeout`. |
| `GET /api/history?at=RFC3339` | `store.At(name, t)` returns nearest envelope at-or-before `t`; build Snapshot; apply filter; respond JSON. 5s `WriteTimeout`. |
| `GET /api/stream` | SSE. Write `event: snapshot` frame from current cache; subscribe to hub; stream `event: <name>.update` frames; `:` ping every 25s. **No `WriteTimeout`** (would kill the stream). |
| `GET /api/sources` | Per-source health JSON. 5s `WriteTimeout`. |
| `GET /healthz`, `HEAD /healthz` | 200 always. |
| `GET /readyz`, `HEAD /readyz` | 200 iff every enabled source has `lastSuccess != zero`. 503 otherwise. |
| (existing) `GET /api/version`, `GET /api/uiconfig`, `GET /` (SPA) | Unchanged from Phase 0. |

### 6.5 Region filter

```go
// internal/sources/filter.go

type Filter struct {
    UGCs map[string]struct{}  // empty = match-all
    WFOs map[string]struct{}  // empty = match-all
}

func (f Filter) Match(a Alert) bool {
    if len(f.UGCs) == 0 && len(f.WFOs) == 0 {
        return true
    }
    for _, u := range a.UGCs {
        if _, ok := f.UGCs[u]; ok {
            return true
        }
    }
    if a.WFO != "" {
        if _, ok := f.WFOs[a.WFO]; ok {
            return true
        }
    }
    return false
}
```

Union semantics (UGC OR WFO match), not intersection — operators with both filters get the broader set, which is the safer default when filtering severe-weather data. Built once at boot from `derived.thresholds.region_filter`.

### 6.6 SSE wire format

```
event: snapshot
data: {"serverTime":"2026-05-02T12:00:00.000Z","sources":{"nws_alerts":{...}}}

event: nws_alerts.update
data: {"source":"nws_alerts","fetchedAt":"...","etag":"sha256:abc...","payload":[...]}

: ping
```

Initial frame is the full Snapshot. Subsequent frames are per-source `Envelope<T>`. Comment-only `:` ping every 25s keeps intermediaries from idle-timing the connection.

---

## 7. Frontend behavior

### 7.1 Overview page

Replaces Phase 0 `<Empty>` with two blocks:

1. **`AlertBadgeBar`** — horizontal `Space` of 10 `Badge`-wrapped `Tag`s in fixed order:
   Tornado, Severe Thunderstorm, Flash Flood, Tropical, High Wind, Red Flag, Winter, Extreme Heat, Extreme Cold, Tsunami.
   Color from `theme.ts` severity-token mapping (count > 0 → category color; count == 0 → token `colorTextDisabled`). `Tooltip` on hover with category name. Click on Tsunami badge scrolls to `TsunamiPanel`.
2. **`TsunamiPanel`** — `ProList` of currently-active alerts where `awips` starts with `"TSU"`. Hidden when count is 0. Each item:
   - Avatar: tsunami icon
   - Title: `headline`
   - Description: `areas` joined by `", "`; `sent` timestamp formatted as UTC + relative ("5m ago")
   - Action: external link to `https://forecast.weather.gov/product.php?site=NWS&product=TSU&issuedby=<wfo>`

Badge counts derive from the `payload` array's `Category` field, computed per render via a memoized `useMemo`.

### 7.2 Live data plumbing

```ts
// web/src/api/stream.ts — pseudocode
function connect(onEvent: (e: SnapshotOrUpdate) => void) {
  // 1. initial paint
  fetch("/api/snapshot").then(r => r.json()).then(onEvent);
  // 2. live stream
  const es = new EventSource("/api/stream");
  es.addEventListener("snapshot", e => onEvent(JSON.parse(e.data)));
  es.addEventListener("nws_alerts.update", e => onEvent(JSON.parse(e.data)));
  es.onerror = () => { /* reconnect with exp backoff; on >3 failures, fall back to 60s polling /api/snapshot */ };
}
```

Zustand store holds `snapshot: Snapshot | null` and `connection: 'connecting' | 'live' | 'polling' | 'error'`. Components read via selectors.

### 7.3 Footer indicator

`SourceHealthIndicator` mounted in the existing Phase 0 `ProLayout` `footerRender`. Polls `/api/sources` every 15s (lightweight; not on the SSE path because per-source health is meta-data about the stream itself). Renders one `Tag` per source with color:
- green: `consecutiveFailures == 0` AND `ageSec ≤ 2 × intervalSec`
- yellow: `consecutiveFailures == 0` AND `ageSec > 2 × intervalSec` (stale)
- red: `consecutiveFailures > 0`
Tooltip: `<lastSuccess RFC3339> (age: Xs, failures: N)`.

In Phase 1 there is one tag (`nws_alerts`); Phase 2 adds four more without changing the component.

---

## 8. Phase 0 reviewer carryovers

All five in scope as discrete tasks in the implementation plan.

1. **Per-source env override walker.** `applyEnvOverrides` extended to walk `map[string]SourceConfig` (key = source name, fields = `Interval`, `Enabled`). New env vars: `CWD_SOURCES_NWS_ALERTS_INTERVAL` (Go `time.Duration` string), `CWD_SOURCES_NWS_ALERTS_ENABLED` (bool). Pattern: `CWD_SOURCES_<UPPER_SOURCE_NAME>_<FIELD>`. Documented in `README.md`.
2. **WARN log on env-parse failure.** Replace silent `strconv.ParseInt`/`ParseBool` swallowing with `slog.Warn("config.env_parse_failed", "key", k, "value", v, "err", err)`. Misconfigured env vars log loudly at boot instead of silently reverting to YAML defaults.
3. **HEAD handlers on `/healthz` + `/readyz`.** `r.Method(http.MethodHead, "/healthz", h)` (or equivalent chi pattern) so HEAD probes from load balancers don't 405. Tests cover both methods.
4. **Vitest + RTL scaffold.** Add `vitest`, `@testing-library/react`, `@testing-library/jest-dom`, `jsdom`, `@vitest/ui` to `web/package.json`. `vitest.config.ts` with jsdom env. `task web:test` target. First tests on `AlertBadgeBar` and `TsunamiPanel` ride this scaffold.
5. **Per-route `WriteTimeout` policy.** chi route-group middleware: 5s `WriteTimeout` for `/api/snapshot`, `/api/history`, `/api/sources`, `/api/version`, `/api/uiconfig`, `/healthz`, `/readyz`; **0 (unbounded)** for `/api/stream` (SSE). Implemented as `middleware.Timeout(5 * time.Second)` on the JSON group; `/api/stream` registered outside that group. Comment in `router.go` explains the policy.

---

## 9. Configuration

No new config keys. Phase 0 already wired:
- `sources.nws_alerts.{interval,enabled}` (Phase 1: actually consumed for the first time)
- `derived.thresholds.region_filter.{ugcs,wfos}` (Phase 1: actually consumed)
- `server.contact` (Phase 1: actually threaded into the User-Agent)
- `store.path`, `store.retention_days` (Phase 1: store exists for the first time)

Validation at boot (extends Phase 0):
- If `sources.nws_alerts.enabled == true` AND `server.contact == ""`: WARN (not fatal — matches Phase 0 contract).
- If `sources.nws_alerts.interval < 10s`: WARN ("polling more often than the upstream `s-maxage` is impolite"). Not fatal.
- If `derived.thresholds.region_filter.ugcs` or `.wfos` contain entries that don't match the format (`/^[A-Z]{2}[CZ]\d{3}$/` for UGC, `/^[A-Z]{3}$/` for WFO): WARN with the offending entry, drop it from the filter.

---

## 10. Testing

### 10.1 Backend (Go)

| Layer | Coverage |
|---|---|
| `internal/sources/nws_alerts_test.go` | Table-driven against captured fixtures in `internal/sources/testdata/`. Cases: empty `features`, single TOR warning, multi-event mix (TOR + SVR + FFW + TSU), tsunami-only (TSU AWIPS prefix), unmapped severe event (asserts WARN log via `slog` test handler), MultiPolygon geometry (parsed-but-not-stored — confirms no panic), missing `properties.parameters` (defensive; should not panic), HazSimp historical name in fixture (locks behavior on the canary log). |
| `internal/sources/filter_test.go` | Empty filter → match all; UGC-only filter; WFO-only filter; both populated → union; alert with no UGCs and no WFO → no match when filter is non-empty. |
| `internal/cache/cache_test.go` | Get/Set round-trip; Subscribe receives Set events; no event on identical Set; multi-subscriber; subscriber close. |
| `internal/store/store_test.go` | `t.TempDir()` SQLite file. Migrate; Append + Latest round-trip; At(t) returns nearest-prior; Prune removes rows older than retention. Concurrent Append from two goroutines (database is serialized; no panic). |
| `internal/fetcher/fetcher_test.go` | `httptest.Server` with canned responses. 200 → cache populated, store appended. 200 with same body → no broadcast (validator match). 500 → backoff doubles. 4xx → backoff doubles, error reflected in health. 200-then-200-different-body → broadcast fires. UA header asserted. |
| `internal/sse/hub_test.go` | Two subscribers receive same event. Slow subscriber (full chan) gets disconnected, fast one keeps receiving. Ping comment emitted on idle. |
| `internal/api/snapshot_test.go` | Cache populated → JSON shape matches §5; cache empty → `sources` is empty object; region filter applied. |
| `internal/api/history_test.go` | Store with multiple appends; `?at=` returns nearest-prior; `?at=` before any append returns empty; invalid `at` → 400. |
| `internal/api/stream_test.go` | EventSource-style client connects, receives `snapshot` then `nws_alerts.update` after a fetcher tick; reconnect tolerated. |
| `internal/api/sources_test.go` | Returns one entry per enabled source with all expected fields. |
| `internal/api/healthz_test.go` | GET and HEAD both 200; `/readyz` 503 before first success, 200 after. |
| `internal/config/config_test.go` (extend) | New env walker reaches `sources.nws_alerts.interval`; bad env value logs WARN and falls back to YAML. |
| `internal/server/server_test.go` (extend) | Boot with httptest fake-NOAA → cache hot-starts from store, fetcher fires, SSE hub broadcasts, `/readyz` flips to 200 within configured interval + epsilon. |

Coverage target unchanged: 80% on `internal/`.

### 10.2 Frontend (Vitest + RTL)

| Layer | Coverage |
|---|---|
| `AlertBadgeBar.test.tsx` | Empty payload → all badges render at 0 in disabled color. Mixed payload → counts match per category, severity colors applied. Tsunami AWIPS-prefixed alert appears under Tsunami badge regardless of `event` text. |
| `TsunamiPanel.test.tsx` | 0 TSU alerts → component returns null (panel hidden). 2 TSU alerts → 2 ProList items rendered, link href = `https://forecast.weather.gov/product.php?site=NWS&product=TSU&issuedby=<wfo>`. |

`task web:test` is added to `Taskfile.yml` and called by the `test-web` reusable workflow.

### 10.3 Smoke (manual, opt-in)

`task smoke` — boots the binary against real `api.weather.gov` for 5 minutes; asserts `/api/sources` shows green for `nws_alerts`. Not in CI (courtesy to NWS).

---

## 11. `/api/sources` shape

```json
{
  "nws_alerts": {
    "intervalSec": 30,
    "lastAttempt": "2026-05-02T12:00:00Z",
    "lastSuccess": "2026-05-02T12:00:00Z",
    "lastError":   null,
    "etag":        "sha256:abc...",
    "ageSec":      12,
    "consecutiveFailures": 0
  }
}
```

`lastError` is a string when present (most recent error message, truncated to 256 bytes).

---

## 12. Definition of done

- [ ] Design doc + implementation plan committed under `docs/plans/`
- [ ] All work in `.worktrees/phase1-nws-alerts` on branch `feature/phase1-nws-alerts` off main (`8a43b7a`)
- [ ] All tests pass (Go + frontend); `golangci-lint` clean
- [ ] `task build:all && ./bin/cwd serve` shows live alert data and a green `nws_alerts` tag in the footer
- [ ] Independent comprehensive review passes
- [ ] PR opened to main, CI green, merged with per-task history preserved
- [ ] Worktree cleaned up

---

## 13. Explicitly out of scope

- Polygon parsing / map rendering
- Per-alert row decomposition in the store
- Status banner (NORMAL / CWD / ENHANCED_CAUTION) — Phase 6
- 3-day Outlook timeline — Phase 2/6
- Image proxy `/img/*` — Phase 3
- History page slider UI — Phase 4
- PWA service worker, Web Push — Phase 5
- The four other sources (`swpc_scales`, `swpc_alerts`, `usgs_quakes`, `usgs_volcanoes`) — Phase 2
- Settings drawer turning region filter from read-only display into a writable form — design §6.2 already says read-only in v1
- VTEC supersession dedup logic for badge counts — design intentionally mirrors original CWD page; revisit if/when ops complain about double-counting

---

## 14. Risks (Phase 1 specific)

1. **Fixture drift.** NWS event names change under HazSimp without notice. Captured fixtures lock behavior at capture time; the `slog.Warn("nws_alerts.unmapped_severe_event")` canary is the production tripwire. Recapture fixtures during Phase 6 hardening.
2. **No upstream validators.** Content-hash diffing on `api.weather.gov` is slightly more CPU than ETag comparison and slightly more sensitive (a payload reorder with no content change still triggers a "diff"). Acceptable; real ETag-bearing sources in Phase 2 will use the original path.
3. **SSE through reverse proxies.** Some HTTP/2-capable proxies buffer event streams; `:` ping every 25s mitigates. Document in README that proxies should be configured for streaming on `/api/stream`.
4. **SQLite write contention under burst.** Single source at 30s polling cadence with diff-on-write is well below contention threshold. Becomes a Phase 2 concern when 5 sources poll simultaneously.
