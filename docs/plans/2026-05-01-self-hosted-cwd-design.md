# Self-Hosted CWD — Design Document

**Date:** 2026-05-01
**Status:** Approved (verbal, all 5 design sections)
**Recon dependency:** [docs/recon/2026-05-01-ncep-cwd-status-recon.md](../recon/2026-05-01-ncep-cwd-status-recon.md)
**Stack:** Go (single binary, `embed.FS`) backend; React + TypeScript + AntD + AntD Pro Components frontend; SSE for live push; SQLite (`modernc.org/sqlite`, no CGO) for 30-day history; PWA shell.

---

## 1. Goal & scope

A self-hostable replacement for [https://www.nco.ncep.noaa.gov/status/cwd/](https://www.nco.ncep.noaa.gov/status/cwd/) — the NWS/NCO Critical Weather Day Status dashboard — that:

- Mirrors the original's panels and behavior (faithful UX), AND
- Adds a small set of operator wins ("modest superset"): configurable alert thresholds, regional filtering, 30-day history with a time slider, dark mode, PWA install, accessibility/keyboard improvements.

**Explicitly out of scope for v1** (see also §10):

- Web Push (Phase 2 add-on)
- Multi-user / per-user accounts (auth handled by an external OIDC proxy or mTLS)
- Postgres (SQLite is the storage tier)
- Server-side write API for config (`PUT /api/config`)
- Cross-pipeline integration with `noaa-data-processor`
- Prometheus metrics
- Translations / i18n

---

## 2. Decisions log (from brainstorm)

| # | Question | Decision |
|---|---|---|
| 1 | Mirror vs superset | **Modest superset.** Same panels + configurable thresholds, region filter, history slider, dark mode, PWA. |
| 2 | Server cache strategy | **Normalized in-memory cache.** One fetcher per source, parses to typed Go structs, all viewers share. |
| 3 | Image strategy | **Build for B (background pollers + persistent cache), default to C (lazy on-demand).** Per-source `prewarm` flag flips into B. |
| 4 | Auth | **None in-app.** Bind to localhost; auth lives upstream (OIDC proxy / mTLS). Trust the network. |
| 5 | History / persistence | **SQLite ring buffer**, 30-day cap, via `modernc.org/sqlite` (pure Go). |
| 6 | Push to browser | **SSE.** Single `/api/stream`. `/api/snapshot` for initial paint and as poll fallback. |
| 7 | Theming / AntD posture | **AntD Pro Components.** ProLayout/ProCard/ProList/ProForm. Small `theme.token` override for brand cue. Default theme **dark**. |
| 7b | PWA Web Push | **No Web Push in v1.** Installable shell + offline asset caching only. |

---

## 3. Architecture

```
┌────────────────────────── single Go binary ──────────────────────────┐
│                                                                       │
│  ┌──────────────┐    ┌─────────────────┐    ┌────────────────────┐    │
│  │  Fetchers    │───▶│  In-mem Cache   │───▶│  SSE Hub           │───▶ browsers
│  │  (1 per src) │    │   (typed)       │    │  /api/stream       │    │
│  └──────┬───────┘    └────────┬────────┘    └────────────────────┘    │
│         │                     │                                        │
│         ▼                     ▼                                        │
│  ┌──────────────┐    ┌─────────────────┐    ┌────────────────────┐    │
│  │ Image proxy  │    │  SQLite store   │    │  HTTP API          │───▶ browsers
│  │ /img/*       │    │ (30d ring)      │    │  /api/snapshot     │    │
│  │ on-demand    │    │ modernc.org/    │    │  /api/history?t=…  │    │
│  │ +opt poller  │    │ sqlite (no CGO) │    │  /img/*  (cached)  │    │
│  └──────────────┘    └─────────────────┘    └────────────────────┘    │
│                                                                       │
│  ┌─────────────────────────────────────────────────────────────────┐  │
│  │  embed.FS — built React/AntD-Pro SPA, served from /             │  │
│  └─────────────────────────────────────────────────────────────────┘  │
└───────────────────────────────────────────────────────────────────────┘
```

Three concurrent subsystems inside one process:

1. **Fetchers** — one goroutine per upstream source. Conditional GET (`If-None-Match`, `If-Modified-Since`). Exponential backoff on error (1s → 5× interval, cap). Diff-on-write: only mutate cache + emit SSE if `etag` (or content-hash fallback) actually changed.
2. **Cache + Store + Hub** — in-memory `Cache` is the read source of truth; every successful fetch also `Append`s to SQLite (history) and `Broadcast`s through the SSE hub.
3. **HTTP API + image proxy + SPA serving** — the public surface. Image proxy is lazy by default; opt-in pre-warm pollers per registered image.

**Per-source poll cadences:**

| Source | Interval | Upstream `Cache-Control` |
|---|---|---|
| `nws_alerts` (api.weather.gov) | 30s | `public, max-age=9, s-maxage=30` |
| `swpc_scales` | 60s | `max-age=60` |
| `swpc_alerts` | 60s | `max-age=60` |
| `usgs_quakes` | 60s | `public, max-age=60` |
| `usgs_volcanoes` (RSS) | 5m | (none; ETag/Last-Modified only) |

**Hot start:** on boot, `store.Latest(name)` fills the cache before fetchers tick — SSE clients connecting in the cold-start window see real (last-known) data instead of "loading…".

**Single binary, ~15 MB, no CGO.** Binds to `127.0.0.1:8765` by default. Logging via stdlib `log/slog` (format configurable: JSON default, text optional). No Prometheus in v1.

---

## 4. Package layout

```
cwd/                                   # repo root (will be `git init`-ed)
├── go.mod                             # module path TBD at scaffold time
├── cmd/
│   └── cwd/
│       └── main.go                    # flag parsing → config.Load → server.Run
├── internal/
│   ├── config/
│   │   ├── config.go                  # YAML+env, defaults, validation
│   │   └── config_test.go
│   ├── sources/                       # one file per upstream; all implement Source iface
│   │   ├── source.go                  # type Source interface { Name, Fetch(ctx)(Snapshot,error) }
│   │   ├── nws_alerts.go
│   │   ├── swpc_scales.go
│   │   ├── swpc_alerts.go
│   │   ├── usgs_quakes.go
│   │   ├── usgs_volcanoes.go
│   │   ├── testdata/                  # fixtures captured during recon
│   │   └── *_test.go
│   ├── fetcher/
│   │   ├── fetcher.go                 # generic loop: ticker + ETag/IMS + backoff
│   │   └── fetcher_test.go
│   ├── cache/
│   │   ├── cache.go                   # Cache: Get/Set/Subscribe per source
│   │   └── cache_test.go
│   ├── store/
│   │   ├── schema.sql                 # //go:embed
│   │   ├── store.go                   # modernc.org/sqlite wrapper
│   │   ├── ringbuffer.go              # 30-day prune
│   │   ├── snapshots.go               # Insert / Latest / At(t time.Time)
│   │   └── store_test.go
│   ├── imgproxy/
│   │   ├── proxy.go                   # lazy fill, on-disk cache, per-source TTL
│   │   ├── prewarm.go                 # opt-in poller per registered source
│   │   ├── registry.go                # the 22 image URLs from recon §4
│   │   └── proxy_test.go
│   ├── sse/
│   │   ├── hub.go                     # broadcast + per-client buffered chan
│   │   ├── event.go                   # typed event marshaling
│   │   └── hub_test.go
│   ├── api/
│   │   ├── router.go                  # net/http (+ chi)
│   │   ├── snapshot.go                # GET /api/snapshot
│   │   ├── history.go                 # GET /api/history{,/range}
│   │   ├── stream.go                  # GET /api/stream  (SSE)
│   │   ├── images.go                  # GET /img/{source}/{name}
│   │   ├── sources.go                 # GET /api/sources
│   │   ├── uiconfig.go                # GET /api/uiconfig (subset of config served to SPA)
│   │   ├── healthz.go                 # GET /healthz, /readyz
│   │   ├── version.go                 # GET /api/version
│   │   └── api_test.go
│   ├── server/
│   │   ├── server.go                  # wires fetchers→cache→store→sse, signals, shutdown
│   │   └── server_test.go
│   ├── webdist/
│   │   ├── embed.go                   # //go:embed dist/*
│   │   └── dist/                      # populated by `pnpm build`; gitignored except .gitkeep
│   └── version/
│       └── version.go                 # build-time ldflags
├── web/                               # React + AntD Pro SPA (Vite)
│   ├── package.json                   # antd, @ant-design/pro-components, react, vite, ts,
│   │                                  # vite-plugin-pwa, workbox-window, zustand
│   ├── tsconfig.json
│   ├── vite.config.ts                 # outDir: ../internal/webdist/dist; VitePWA({...})
│   ├── index.html
│   ├── public/
│   │   ├── icons/                     # 192, 512, maskable-192, maskable-512, apple-touch
│   │   ├── manifest.webmanifest       # generated by vite-plugin-pwa
│   │   └── offline.html               # SW fallback page
│   └── src/
│       ├── main.tsx
│       ├── App.tsx                    # ProLayout + routes
│       ├── theme.ts                   # ConfigProvider tokens, dark/light/auto
│       ├── api/
│       │   ├── client.ts
│       │   ├── stream.ts              # EventSource + reconnect + fallback to poll
│       │   └── types.ts               # mirrors Go DTOs
│       ├── store/
│       │   └── snapshot.ts            # Zustand: current snapshot + history cursor
│       ├── pwa/
│       │   ├── register.ts            # workbox-window registration + update prompt
│       │   └── installPrompt.tsx
│       ├── pages/
│       │   ├── Overview.tsx
│       │   ├── Hazards.tsx
│       │   ├── SpaceWeather.tsx
│       │   ├── Events.tsx
│       │   ├── History.tsx
│       │   └── Settings.tsx
│       └── components/
│           ├── StatusBanner.tsx
│           ├── AlertBadgeBar.tsx
│           ├── HazardImageCard.tsx
│           ├── SWPCScaleCard.tsx
│           ├── EventListItem.tsx
│           ├── SourceHealthIndicator.tsx
│           ├── ZClock.tsx
│           └── LocalTimesStrip.tsx
├── docs/
│   ├── recon/                         # already populated
│   └── plans/                         # this design + later implementation plan
├── Makefile                           # build, build-web, build-all, test, lint, run, smoke
├── .golangci.yml
└── README.md
```

**Notes:**

- **`internal/webdist`** is the seam for embedding. Keeps `embed.FS` out of `internal/api` so backend tests don't need a frontend build.
- **`sources/` is the boundary.** Each file maps 1:1 to one upstream URL — easy to add/remove sources without touching the rest.
- **No domain-model package.** Per-source DTOs live in `sources/`; the cache stores them as `interface{}` keyed by source name, decoded on read. Avoids a shared "everything is an Alert" abstraction we'd regret.
- **Frontend stack:** Vite + React + TS + AntD + AntD Pro Components + Zustand + vite-plugin-pwa + workbox-window. `react-router-dom` is a transitive peer of AntD Pro.

---

## 5. Data flow

### 5.1 Per-source fetch loop

```
                      ┌─────────────────────────────────────────────┐
                      │            Per-source goroutine             │
                      │  ┌───────────────────────────────────────┐  │
                      │  │ ticker(interval) ── tick ──┐          │  │
                      │  │                            ▼          │  │
                      │  │   GET upstream                        │  │
                      │  │     If-None-Match / If-Modified-Since │  │
                      │  │   ├─ 304 ── update cache.lastSeen     │  │
                      │  │   ├─ 200 ── parse → typed Snapshot ──┐│  │
                      │  │   └─ err ── exponential backoff      ││  │
                      │  │                                      ▼│  │
                      │  │              cache.Set(name, snap, etag) │
                      │  └────────────────────────┬────────────────┘ │
                      │                           │                  │
                      │       store.Append(name, snap, ts)           │
                      │                           │                  │
                      │       sse.Broadcast({type, source, payload}) │
                      └───────────────────────────┴──────────────────┘
```

- **Backoff:** doubling from 1s, capped at 5× the configured interval. Resets on success.
- **Diff-on-write:** `cache.Set` only fires SSE/store-append if `etag` changed (or `payload` JSON hash if no etag). Prevents 60-second heartbeat spam when nothing actually changed upstream.
- **Per-source TTL on cache reads:** `/api/snapshot` always reads from cache; cache returns a `staleness` field (`age = now - fetchedAt`) so the UI can show "as of HH:MMZ".

### 5.2 Wire types

```ts
type SourceName =
  | "nws_alerts" | "swpc_scales" | "swpc_alerts"
  | "usgs_quakes" | "usgs_volcanoes";

type Envelope<T> = {
  source: SourceName;
  fetchedAt: string;           // RFC3339Nano
  etag?: string;
  payload: T;
};

type Snapshot = {
  serverTime: string;
  sources: {
    nws_alerts:     Envelope<Alert[]>;
    swpc_scales:    Envelope<SWPCForecast>;
    swpc_alerts:    Envelope<SWPCAlert[]>;
    usgs_quakes:    Envelope<Earthquake[]>;
    usgs_volcanoes: Envelope<VolcanoAlert[]>;
  };
  status: { mode: "NORMAL" | "CWD" | "ENHANCED_CAUTION"; effective?: string };
  outlook: { day: string; status: string }[];   // 3-day
};

type Alert = {
  id: string;
  event: string;                // e.g. "Tornado Warning"
  awips: string;                // e.g. "TSUNHC"
  headline: string;
  severity: "Extreme" | "Severe" | "Moderate" | "Minor" | "Unknown";
  effective: string; expires: string;
  areas: string[];
  wfo?: string;
  url?: string;
};

type SWPCForecast = {
  days: { date: string; r1: number; r3: number; s1: number;
          g: { scale: number; text: string } }[];
};

type SWPCAlert  = { productId: "K08A"|"K09A"|"P12A"|"P13A"|string;
                    issuedAt: string; message: string; };
type Earthquake = { id: string; title: string; time: string;
                    lat: number; lon: number; depthM: number; mag: number; url: string; };
type VolcanoAlert = { id: string; volcano: string;
                      alertLevel: "WATCH"|"WARNING"|"ADVISORY"|"NORMAL";
                      colorCode:  "GREEN"|"YELLOW"|"ORANGE"|"RED"|"UNASSIGNED";
                      description: string; updated: string; url: string; };
```

### 5.3 SSE event shapes

`text/event-stream` with named events; the browser dispatches via `addEventListener(name, …)`:

```
event: snapshot              ← initial full snapshot on connect (one frame)
data: { …Snapshot… }

event: nws_alerts.update     ← any source's envelope when it changes
data: { source:"nws_alerts", fetchedAt:"…", etag:"…", payload:[…] }

event: status.update         ← derived field changes (mode, outlook)
data: { mode:"CWD", effective:"2026-05-02T12:00:00Z" }

event: image.invalidate      ← image proxy refreshed something
data: { source:"spc", name:"day1otlk.png", fetchedAt:"…" }

: ping                       ← comment line every 25s, keeps proxies happy
```

### 5.4 HTTP API surface

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/snapshot`                              | Latest full `Snapshot` (initial paint + SSE fallback) |
| `GET` | `/api/history?at=RFC3339`                    | Snapshot as it was at `at` (nearest-prior in store) |
| `GET` | `/api/history/range?from=…&to=…&source=…`    | For the time-slider sparkline |
| `GET` | `/api/stream`                                | SSE; emits `snapshot` then `*.update` events |
| `GET` | `/img/{source}/{name}`                       | Lazy image proxy (Q3-C); pollers can pre-warm |
| `GET` | `/api/sources`                               | Per-source `{ intervalSec, lastAttempt, lastSuccess, lastError, etag, ageSec }` |
| `GET` | `/api/uiconfig`                              | Subset of config served to the SPA (theme default, region filter, feature flags) |
| `GET` | `/api/version`                               | git SHA, build time, Go version |
| `GET` | `/healthz`                                   | 200 always |
| `GET` | `/readyz`                                    | 200 only after first successful fetch of every enabled source |
| `GET` | `/`                                          | SPA (served from `embed.FS`) |

### 5.5 Error handling

- **Upstream 4xx/5xx:** WARN log; backoff retry; cache keeps last good snapshot. UI shows a small `Tag color="warning"` per source ("stale 4 min") rather than blanking the panel.
- **Parse failure:** ERROR log with raw body length + content-type; cache untouched; surfaced via `/api/sources`.
- **Volcano RSS XML:** parsed with `encoding/xml` into a typed struct; namespaced fields handled via xml-tag namespaces.
- **NWS API politeness:** `User-Agent: cwd-self-host/<version> (<contact>)`. `contact` from config; required-with-warning at boot.

**Tradeoff:** all derived state (`status`, `outlook`) is server-computed so the SPA stays dumb. Single source of truth, easy to test in Go, no rule drift between server and client.

---

## 6. AntD Pro component map

### 6.1 Chrome (everywhere)

- **`ProLayout`** — sidebar + top header + breadcrumbs + content area; collapsible; mobile hamburger built-in.
  - `title="CWD"`, `logo={<NoaaMark/>}`, route config drives nav.
  - `rightContentRender` → status pill (`Tag` colored by mode), `Z`-clock, theme switcher, settings drawer trigger.
  - `footerRender` → small attribution + build version + last-fetch summary (`SourceHealthIndicator` row).
- **`ConfigProvider`** — wraps app; supplies theme tokens.
  - `theme={{ algorithm: themeMode === "dark" ? darkAlgorithm : defaultAlgorithm,
              token: { colorPrimary: '#0a4f8a', borderRadius: 6,
                       fontFamilyCode: 'ui-monospace, …' } }}`
  - **Default mode: dark** (per config `ui.default_theme: "dark"`); user toggle persists in `localStorage`.
- **`App`** (AntD's wrapper) — provides `message`, `notification`, `Modal` contexts; the SW update prompt routes through `notification.info`.
- **Settings drawer** — `Drawer` + `ProForm` for: theme (light/dark/auto), default landing page, region/WFO filter (read-only display in v1), threshold display (read-only in v1), image prewarm display (read-only in v1).

### 6.2 Pages

| Page | Components | Content |
|---|---|---|
| **Overview** (`/`) | `ProCard.Group`, `Statistic`, `Tag`, `Alert`, `Timeline`, `Skeleton`, `Empty` | Status banner (NORMAL/CWD/ENHANCED_CAUTION) as colored `Alert`; `AlertBadgeBar`; 3-day outlook as `Timeline`; 6-zone time strip. |
| **Hazards** (`/hazards`) | `ProCard`, `Image.PreviewGroup`, `Image`, `Segmented` (Day 1/2/3), `Tag`, `Skeleton.Image` | One `ProCard` per category (Severe Storms, Wildfire, Excessive Rainfall, Winter, Heat, Tropical, Flooding). Stacks on mobile, 2-col tablet, 3-col desktop. `Image` provides lazy + fullscreen preview with proper alt props. |
| **Space Weather** (`/space`) | `ProCard.StatisticCard.Group`, `ProCard.StatisticCard`, `GColorChip` | 3-day SWPC forecast: one `StatisticCard` per day, R1/R3/S1 percentages and the G-scale chip. |
| **Events** (`/events`) | `ProList`, `Avatar`, `Tag`, `Empty` | Three `ProList` panes via `ProList.toolBar.tabs`: Tsunamis, Earthquakes, Volcanoes. |
| **History** (`/history`) | `ProCard`, `Slider` (with `marks`), `DatePicker.RangePicker`, `Switch` ("follow live"), reuse Overview/Hazards/Events components in read-only mode | Time slider over the SQLite ring buffer. Slider drag → `GET /api/history?at=…`. |
| **Settings** (`/settings`, also a Drawer) | `ProForm`, `ProFormSwitch`, `ProFormSelect`, `ProFormDigit`, `ProFormText`, `ProFormGroup` | Read-only display of server-side config in v1; client-only prefs (theme, default landing) writable to `localStorage`. |

### 6.3 Reusable components (`web/src/components/`)

| Component | Built on | Notes |
|---|---|---|
| `StatusBanner` | `Alert`, `Statistic` | Mirrors "Current Status: NORMAL" line |
| `AlertBadgeBar` | `Space`, `Badge`, `Tooltip`, `Tag` | Maps `get_count_alerts.js` logic 1:1, typed |
| `HazardImageCard` | `ProCard`, `Image`, `Skeleton.Image`, `Tag` ("stale 4m") | One per `(source, day)` triple |
| `SWPCScaleCard` | `ProCard.StatisticCard`, custom `GColorChip` | G/S/R triplet for one forecast day |
| `EventListItem` | `ProList.Item`, `Avatar`, `Tag` | Generic; specialized via props |
| `SourceHealthIndicator` | `Tag` + `Tooltip` | Layout footer; one per source |
| `ZClock` | `Statistic.Countdown` styling | UTC clock with second precision |
| `LocalTimesStrip` | `Space`, `Statistic` | 6-zone strip (HI/AK/PT/MT/CT/ET) |

### 6.4 UX choices

- **Severity → color is consistent:** Extreme=`colorError`, Severe=`colorErrorActive`, Moderate=`colorWarning`, Minor=`colorInfo`. Centralized in `theme.ts`, accessed via `useToken()`.
- **Hazard cards have a 16:10 aspect-ratio wrapper** around `Image` to kill cumulative layout shift when lazy-loading.
- **`Image.PreviewGroup` per category** lets users arrow-key through Day 1/2/3 in a fullscreen lightbox.
- **Skeletons** while the first SSE `snapshot` event is in flight — never an empty page.
- **Reduced motion** respected: `@media (prefers-reduced-motion: reduce)` disables hover lifts and badge pulses.
- **Keyboard nav:** Pro components ship proper focus rings; `?` opens a Modal listing shortcuts (`G H` → Hazards, etc.).
- **Color-blind safe G-scale chip:** color + `G0..G5` text inside the chip.
- **Print stylesheet:** sidebar hidden, panels reflow to single column, timestamps in UTC + local.

**Tradeoff:** AntD Pro Components adds ~150 KB gzipped beyond bare AntD. For a self-hosted dashboard served from the same binary on LAN, this is a non-issue.

---

## 7. PWA

Service worker generated by `vite-plugin-pwa` (Workbox under the hood).

| Asset class | Strategy | Why |
|---|---|---|
| SPA shell (`/index.html`, JS, CSS, fonts) | `precache` | Built-time hashed; instant cold loads |
| `/api/snapshot` | `NetworkFirst`, 5s timeout, 60s cache | Live data preferred; cache covers offline |
| `/api/history?at=…` | `CacheFirst`, 1 day | History snapshots are immutable per `(at)` key |
| `/img/*` | `StaleWhileRevalidate`, 1 day | Maps tolerate brief staleness; refresh in background |
| `/api/stream` (SSE) | **bypass SW** | SW must not buffer SSE — would break live push |

- `display: "standalone"`, `theme_color` matches the AntD Pro header color, `background_color` matches initial paint.
- Update prompt via `workbox-window` `controlling`/`waiting` events → AntD `notification.info` with a "Reload" button.
- "Add to Home Screen" via AntD `Modal` triggered on `beforeinstallprompt`, deferred until ≥30s on app, shown once, remembered in `localStorage`.

**Backend implications:** none. `vite-plugin-pwa` generates everything at build time; ships inside `embed.FS`. Backend only needs `Cache-Control` headers on `/api/*` and `/img/*` aligned with the SW strategies (we control both).

---

## 8. Configuration

One `config.yaml` (path discoverable via `--config`, `$CWD_CONFIG`, then `$XDG_CONFIG_HOME/cwd/config.yaml`). Every key also overridable by `CWD_*` env. Validation at boot; refuses to start on bad values.

```yaml
server:
  bind: "127.0.0.1:8765"
  contact: "you@example.com"          # appended to NWS User-Agent (required-with-warning)
  log_level: "info"                   # debug|info|warn|error
  log_format: "json"                  # json|text

store:
  path: "${XDG_STATE_HOME}/cwd/cwd.db"
  retention_days: 30

cache:
  image_dir: "${XDG_CACHE_HOME}/cwd/img"
  image_max_bytes: 524288000          # 500 MB hard cap

sources:
  nws_alerts:     { interval: "30s", enabled: true }
  swpc_scales:    { interval: "60s", enabled: true }
  swpc_alerts:    { interval: "60s", enabled: true }
  usgs_quakes:    { interval: "60s", enabled: true }
  usgs_volcanoes: { interval: "5m",  enabled: true }

images:
  default_mode: "lazy"                # lazy | prewarm
  prewarm:                            # opt-in per source (Q3: build for B, default C)
    - spc.day1otlk
    - wpc.wssi.day1
    # …names come from internal/imgproxy/registry.go

derived:
  thresholds:
    swpc_alert_window_hours: 24
    swpc_alert_products: [K08A, K09A, P12A, P13A]
    region_filter:
      ugcs: []
      wfos: []

ui:                                   # served to the SPA via /api/uiconfig
  default_theme: "dark"               # dark | light | auto
  default_landing: "/"
  enable_history: true
```

---

## 9. Observability

- **`log/slog`** JSON to stdout; one structured line per fetch attempt with `source`, `status`, `etag_match`, `bytes`, `duration_ms`, `next_in`.
- **`/api/sources`** — per-source health JSON; UI footer renders a `SourceHealthIndicator` row from this.
- **`/healthz`** = process up. **`/readyz`** = first successful fetch from every enabled source.
- **No Prometheus** in v1.
- **Build info** (`/api/version`) carries git SHA + build time + Go version via `-ldflags`.

---

## 10. Testing

| Layer | Tool | Scope |
|---|---|---|
| **Unit** | Go stdlib `testing` (table-driven) | Each `sources/*.go` parses real captured fixtures from `internal/sources/testdata/` (recon payloads). Cache, store, fetcher (with `httptest.Server`), imgproxy (synthetic origin), SSE hub. Frontend: Vitest + React Testing Library on presentational components only. |
| **Integration** | Go `testing` + `httptest` + in-process server | Boot the full server pointed at a fake-NOAA `httptest.Server` serving canned 200/304/5xx/parse-failure bodies. Verify snapshot, SSE event stream, history, image lazy fill + cache hit, prewarm poller. |
| **Smoke (manual, opt-in)** | `make smoke` | Runs binary against real upstreams for 5 min; asserts every source goes green; prints `/api/sources`. Not run in CI — courtesy to NOAA. |

Coverage target: 80% on `internal/`, no target on `cmd/` or `web/components/`. `golangci-lint` for vet/staticcheck/govet/errcheck.

**TDD applies** (per `superpowers:test-driven-development`): each subagent writes the failing test first. Recon fixtures (`cwd-*.html`, `cwd-*.js`) seed `internal/sources/testdata/`.

---

## 11. Rollout phases

> **For Claude:** REQUIRED EXECUTION WORKFLOW (follow in order):
> 1. `superpowers:using-git-worktrees` — Isolate work in a dedicated worktree
> 2. `superpowers:subagent-driven-development` — Dispatch a fresh subagent per task
> 3. `superpowers:test-driven-development` — All subagents use TDD
> 4. `superpowers:verification-before-completion` — Verify all tests pass per task
> 5. `superpowers:requesting-code-review` — Code review after each task (built in)
> 6. After all tasks: comprehensive code review on full diff from branch point (automatic)
> 7. `superpowers:finishing-a-development-branch` — Complete the branch
>
> Skills carry their own model and effort settings. Do not override them.

Each phase ships its own implementation document and its own subagent batch.

| Phase | Goal | Demo |
|---|---|---|
| **0 — Skeleton** | `git init`, Go module, Vite app, Makefile, embed wiring, CI lint+test | `cwd serve` → AntD Pro shell renders |
| **1 — One source end-to-end** | `nws_alerts`: source → fetcher → cache → store → snapshot endpoint → SSE → AntD page. Establishes every interface. | Live alert badges + Tsunamis pane work |
| **2 — Remaining four sources** | SWPC scales, SWPC alerts, USGS quakes, USGS volcanoes. Adds Space Weather page + Events page. | Feature parity with original page's *dynamic* sections |
| **3 — Image proxy** | Lazy `/img/*` + prewarm config knob. Hazards page goes live. | Feature parity with original page's *static maps* |
| **4 — History + slider** | SQLite store reads, History page, time slider, "follow live" toggle | Click back to "what was active at 14:00Z" and see dashboard re-render |
| **5 — PWA + polish** | Service worker, manifest, install prompt, offline fallback, print stylesheet, keyboard shortcuts modal, reduced-motion paths | Install to home screen on phone; works offline-stale |
| **6 — Hardening** | Backoff tuning, error UX pass, accessibility audit (axe-core in CI), README + ops doc, single-binary release artifact (linux/amd64, linux/arm64, darwin/arm64) | Tagged v0.1.0 release |

---

## 12. Risks

1. **NWS API politeness.** They've publicly grumped about clients without User-Agents. The `contact` config field is required-with-warning; boot log is loud if missing; `User-Agent` always carries `cwd-self-host/<ver>`.
2. **AntD Pro version churn.** Pro Components has had breakage between minors. Pin exact versions in `package.json`; Renovate PRs reviewed manually.
3. **`embed.FS` + frontend build ordering.** If `web/dist` is empty, `go build` succeeds but serves a blank shell. CI runs `pnpm build` before `go build`; build fails loudly if `dist/index.html` is missing.
4. **SQLite + 30-day retention.** Pruning runs at boot + every 6 h. A long-running process under hurricane-week ingest spikes can briefly outgrow the cap. Acceptable.
5. **Image cache disk usage.** `image_max_bytes` enforced by LRU eviction; not a hard inotify quota. Drift possible during burst. 500 MB default fits a Pi SD card with margin.

---

## 13. Open items deferred to implementation phase

- **Go module path** (depends on where this repo lands — GitHub vs GitLab/n7knightone vs personal namespace).
- **License** (likely MIT to match other personal projects, but to confirm at scaffold time).
- **CI provider** — GitHub Actions vs GitLab CI; depends on §13 module-path choice.
- **Release tooling** — semantic-release vs goreleaser vs hand-rolled `make release`. Decide in Phase 6.
- **Exact AntD/Pro versions to pin** — pin to latest at scaffold time; document in Phase 0 plan.
