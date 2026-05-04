# Self-Hosted CWD — Phase 3 (Image proxy + Hazards page) — Design Document

**Date:** 2026-05-03
**Status:** Approved (verbal, all 7 brainstorm questions)
**Parent design:** [docs/plans/2026-05-01-self-hosted-cwd-design.md](2026-05-01-self-hosted-cwd-design.md)
**Phase 1 design:** [docs/plans/2026-05-02-phase1-nws-alerts-design.md](2026-05-02-phase1-nws-alerts-design.md) — Source/Fetcher/Cache/Store/Hub contracts
**Phase 1 implementation:** [docs/plans/2026-05-02-phase1-nws-alerts-implementation.md](2026-05-02-phase1-nws-alerts-implementation.md) — structural template
**Phase 2 design:** [docs/plans/2026-05-02-phase2-multi-source-design.md](2026-05-02-phase2-multi-source-design.md)
**Phase 2 implementation:** [docs/plans/2026-05-02-phase2-multi-source-implementation.md](2026-05-02-phase2-multi-source-implementation.md)
**Recon dependency:** [docs/recon/2026-05-01-ncep-cwd-status-recon.md](../recon/2026-05-01-ncep-cwd-status-recon.md) §4 (per-image inventory)
**Phase 2 baseline:** main at the merge commit of PR #4 (`86b2d1b`)

---

## 1. Goal & scope

Stand up a server-side image proxy in front of the ~20 third-party static maps the original NCEP CWD page hot-links, and bring the Phase 0 placeholder `/hazards` page to life with seven category cards (Severe Storms, Wildfire, Excessive Rainfall, Winter, Heat, Tropical, Flooding). The proxy is **lazy by default with an opt-in pre-warm poller per image** (parent design Q3, decision: build for B, default to C). All images flow through the same `Source/Fetcher/Cache` discipline used by Phase 1/2 — no new architectural primitives.

**Demo bar (per parent design §11):** *Feature parity with the original NCEP CWD page's static maps section.*

**Explicitly NOT in scope (deferred):**
- History page + time slider — Phase 4.
- PWA service-worker + offline image cache — Phase 5 (the SW reads `/img/*` with `StaleWhileRevalidate` per parent design §7; backend just needs to ship correct `Cache-Control` headers, which Phase 3 does).
- Image transcoding (PNG↔WEBP, resize, optimize) — out of v1; pass upstream bytes through unchanged.
- Image-level region/CONUS-vs-state cropping — out of v1.
- Map-tile rendering or polygon overlay — Phase 4 at the earliest.
- Authentication / per-user pre-warm — out of v1 (the binary is single-tenant local).
- Adding new image keys via runtime config — out of v1 (registry is hardcoded; operator can override interval and pre-warm membership but not add a new `(source, name)` pair without code change). See Decision 5 rationale.

---

## 2. Decisions log

Captured during the 2026-05-03 brainstorm.

| # | Question | Decision |
|---|---|---|
| 1 | Storage backend | **Disk-backed cache + in-memory hot tier, both operator-tunable.** Disk LRU is the durable store (cold-restart safe, Pi-friendly); in-memory hot tier is the latency accelerator. Both bound by separate config knobs (`disk_max_bytes`, `hot_max_bytes`, `hot_max_entries`). |
| 2 | Refresh cadence | **Global default + per-image override map.** A single `refresh_interval` (default 5m) applies to every image; `image_intervals: { "spc.day1otlk": "2m", … }` overrides per-image. Conditional GET (`If-None-Match`/`If-Modified-Since`) on every poll regardless. |
| 3 | Failure mode on upstream 4xx/5xx/timeout | **Serve last-good cached bytes** with `Warning: 110 cwd "stale Xm"` HTTP header, log WARN, surface failure in `/api/sources` so the SPA shows a "stale Xm" Tag (mirrors Phase 1/2). 502 only if no cached copy ever existed for that key. |
| 4 | Default pre-warm list | **Ship sensible defaults, fully operator-editable.** `defaults()` seeds `[spc.day1otlk, spc.day2otlk, spc.day3otlk, nhc.atl_7d]` (4 highest-traffic maps); operator overrides via `images.prewarm` in YAML or `CWD_IMAGES_PREWARM` env (comma-separated keys) to add/remove freely. |
| 5 | Image registry shape | **Hardcoded in Go (`internal/imageproxy/registry.go`)** because the URL set rarely changes (a year between SPC product churn). Each entry carries a descriptive code comment identifying the image (category, source agency, what the map depicts) so the registry doubles as documentation. Operator overrides `interval` and `prewarm` membership; no runtime add/remove of keys. |
| 6 | SSE `image.invalidate` trigger | **Diff-on-write only.** Fires only when the conditional GET produces actually-different bytes (ETag/Last-Modified delta or content-hash diff). Same discipline as Phase 1/2 cache writes — no spurious refresh on identical body. |
| 7 | URL format | **`/img/{source}/{name}`** — bare key, source-grouped path, content-type returned in HTTP header. Maps `(source="spc", name="day1otlk")` → `https://www.spc.noaa.gov/products/outlook/day1otlk.png`. The `images.prewarm` config uses dot-key form (`spc.day1otlk`) for terseness; the URL form (`/img/spc/day1otlk`) is REST-ish. Both refer to the same registry entry. |

---

## 3. Upstream constraints worth calling out

### 3.1 Image inventory (20 entries across 7 categories + 1)

Per recon §4, the original page hot-links these PNGs/GIFs/JPGs:

| Category | Count | Sources |
|---|---|---|
| Severe Storms | 3 | SPC convective Day 1/2/3 PNGs |
| Wildfire | 3 | SPC fire wx Day 1/2 PNGs + Day 3-8 experimental GIF |
| Excessive Rainfall | 3 | WPC ERO Day 1/2/3 GIFs |
| Winter | 3 | WPC WSSI Day 1/2/3 PNGs |
| Heat | 3 | WPC HeatRisk Day 1/2/3 PNGs |
| Tropical | 4 | NHC Atlantic / East Pacific / Central Pacific 7-day PNGs + Navy JTWC ABPW JPG |
| Flooding | 1 | WPC NWC FHO PNG |
| **Total** | **20** | |

Full URL list lives in recon §4; the registry table in §5.1 below mirrors it 1:1.

### 3.2 Politeness contract carries forward

- Same `User-Agent: cwd-self-host/<version> (<contact>)` threading as Phase 1/2 (built once in `internal/server/server.go buildUserAgent`).
- Conditional GET on every poll (`If-None-Match` if registry has an ETag from prior poll; `If-Modified-Since` from prior `Last-Modified`; both if both available).
- Per-image `interval` floor of 60s (matches Phase 1/2 source-interval floor; bumped to 60s rather than 10s because images are larger and most update on multi-minute schedules).
- Pre-warm pollers run only for keys explicitly listed in `images.prewarm` — no implicit poll-everything.
- Lazy fetch on `/img/*` request: if cache has no entry, do a synchronous fetch (with timeout); subsequent requests for the same key serve from cache and only refetch when the per-image `interval` has elapsed since last successful fetch.

### 3.3 Validators

Most upstreams (NWS/SPC/WPC/NHC/Navy) ship unstable or no ETag. Strategy mirrors Phase 1's NWS handling: store **either** the upstream ETag/Last-Modified pair if present **or** a sha256 of the response body as the validator. `/api/sources` reports the validator opaquely; pollers send `If-None-Match` when the registry has a real upstream ETag and rely on body-hash diff otherwise.

### 3.4 Content types

PNG (`image/png`), GIF (`image/gif`), JPEG (`image/jpeg`). All three preserved as-is from upstream `Content-Type` header (verified at fetch time; if upstream omits it, fall back to extension-based MIME from the registry entry's `MIME` field). No transcoding.

---

## 4. Package additions / file map

### 4.1 New Go files

```
internal/imageproxy/
├── registry.go         # The 20-image table; one comment block per entry describing what the map shows
├── store.go            # Disk LRU (filesystem-backed, atomic write-then-rename, on-restart scan)
├── hot.go              # In-memory LRU hot tier (sync.Map + container/list)
├── proxy.go            # Public API: Get(ctx, key) → (bytes, contentType, validator, fetchedAt, error)
├── prewarm.go          # Per-key poller goroutine (one per registered prewarm key); uses internal/fetcher
├── *_test.go           # tests for each
internal/api/
├── images.go           # GET /img/{source}/{name} handler
├── images_test.go
```

### 4.2 New TypeScript files

```
web/src/components/
├── HazardCategoryCard.tsx      # one ProCard per category; Segmented Day 1/2/3; Image.PreviewGroup
├── HazardCategoryCard.test.tsx
web/src/pages/
├── Hazards.tsx                 # rewrite of the placeholder; renders 7 category cards in responsive grid
├── Hazards.test.tsx
```

### 4.3 Touched files

- `internal/config/config.go` — add `Images` config block + defaults; add new validation: per-image-interval floor (60s), unknown prewarm key WARN-and-drop, disk_max_bytes / hot_max_bytes positive-int validation
- `internal/config/config_test.go` — extend
- `internal/server/server.go` — wire ImageProxy into the existing `Run` flow: construct registry, store, hot tier; spawn one prewarm poller per `images.prewarm` key (using `internal/fetcher` exactly as Phase 1/2 sources do); register `/img/*` route on the chi router; surface image-source health in the existing `/api/sources` collection (each prewarm key gets a `image:<key>` entry alongside the 5 data sources)
- `internal/sse/hub.go` — **NO CHANGE.** The existing `applyFilter` already passes through any non-`nws_alerts` env unchanged; image events just need to be `Broadcast`ed by the proxy via the existing hub interface. Same deferral as Phase 2 §6.5.
- `internal/cache/cache.go` — **NO CHANGE.** Image proxy maintains its own typed cache (the in-memory hot tier doubles as the broadcast trigger); we don't store image blobs in the typed `cache.Cache` because that cache is shaped around small JSON payloads. The SSE event for `image.invalidate` is fired by `internal/imageproxy/proxy.go` directly into the hub.
- `web/src/api/types.ts` — add `ImageInvalidate` type for the SSE event payload
- `web/src/api/stream.ts` — subscribe to `image.invalidate` events, route them into a new `useImageStore` (or extend the existing snapshot store with a `lastImageRefresh: Record<string, string>` map)
- `web/src/store/snapshot.ts` — add `imageRefresh: Record<string, string>` (key → ISO timestamp); set on `image.invalidate`. Components key their `<img>` element on this so a new fetch triggers a browser-side reload.
- `Taskfile.yml` smoke task — extend to assert all `images.prewarm` keys have `lastSuccess` populated within their interval window
- `README.md` — document the `/img/*` proxy, the registry shape, the operator override knobs

### 4.4 The Source contract is **not** extended

Phase 1/2's `internal/sources.Source` interface remains unchanged. Image proxy is its own subsystem (`internal/imageproxy/`) that internally reuses the `internal/fetcher` package's politeness machinery (conditional GET, exponential backoff, UA threading) but exposes a different API surface (`Get(ctx, key)` returning bytes+content-type, vs `Source.Fetch` returning a typed payload). This keeps the typed-snapshot side of the system clean — `/api/snapshot` and the SPA's typed Zustand store don't need to model image bytes.

---

## 5. Internal contracts

### 5.1 Registry (Go)

```go
// internal/imageproxy/registry.go
package imageproxy

import "time"

// Image describes one upstream map proxied by /img/{source}/{name}.
type Image struct {
    Source       string        // URL-path segment 1; e.g. "spc"
    Name         string        // URL-path segment 2; e.g. "day1otlk"
    URL          string        // upstream absolute URL
    MIME         string        // fallback content-type if upstream omits
    DefaultInterval time.Duration // poll cadence absent operator override
    Description  string        // human-readable "what this map shows"
}

// Key returns the dot-key form used in config (images.prewarm, image_intervals).
func (i Image) Key() string { return i.Source + "." + i.Name }

// Path returns the URL form served at /img/{source}/{name}.
func (i Image) Path() string { return "/img/" + i.Source + "/" + i.Name }

// Registry is the immutable list of all proxied images.
//
// Each entry's comment names the upstream agency, the product, and the time
// horizon it depicts so the registry doubles as documentation. To add a new
// image, append a new struct literal and add its description comment;
// no other code changes required (the proxy and prewarm machinery iterate this).
//
//nolint:gochecknoglobals // intentional package-level immutable config
var Registry = []Image{
    // ── Severe Storms — SPC Convective Outlooks ────────────────────────────
    // Day 1: today's convective threat (tornado / wind / hail) over CONUS.
    {Source: "spc", Name: "day1otlk", URL: "https://www.spc.noaa.gov/products/outlook/day1otlk.png", MIME: "image/png", DefaultInterval: 2 * time.Minute, Description: "SPC Convective Outlook — Day 1 (today)"},
    // Day 2: tomorrow's convective threat.
    {Source: "spc", Name: "day2otlk", URL: "https://www.spc.noaa.gov/products/outlook/day2otlk.png", MIME: "image/png", DefaultInterval: 5 * time.Minute, Description: "SPC Convective Outlook — Day 2 (tomorrow)"},
    // Day 3: the day after tomorrow.
    {Source: "spc", Name: "day3otlk", URL: "https://www.spc.noaa.gov/products/outlook/day3otlk.png", MIME: "image/png", DefaultInterval: 10 * time.Minute, Description: "SPC Convective Outlook — Day 3"},

    // ── Wildfire — SPC Fire Weather Outlooks ───────────────────────────────
    // Day 1: today's fire-weather concern.
    {Source: "spc", Name: "day1otlk_fire", URL: "https://www.spc.noaa.gov/products/fire_wx/day1otlk_fire.png", MIME: "image/png", DefaultInterval: 5 * time.Minute, Description: "SPC Fire Weather Outlook — Day 1"},
    // Day 2: tomorrow.
    {Source: "spc", Name: "day2otlk_fire", URL: "https://www.spc.noaa.gov/products/fire_wx/day2otlk_fire.png", MIME: "image/png", DefaultInterval: 10 * time.Minute, Description: "SPC Fire Weather Outlook — Day 2"},
    // Day 3-8 experimental: a longer outlook; animated GIF.
    {Source: "spc", Name: "day38otlk_fire", URL: "https://www.spc.noaa.gov/products/exper/fire_wx/imgs/day38otlk_fire.gif", MIME: "image/gif", DefaultInterval: 30 * time.Minute, Description: "SPC Fire Weather Outlook — Day 3-8 (experimental, animated)"},

    // ── Excessive Rainfall — WPC ERO ───────────────────────────────────────
    // Day 1: today's flash-flood / heavy-rain risk.
    {Source: "wpc", Name: "ero_day1", URL: "https://www.wpc.ncep.noaa.gov/qpf/94ewbg.gif", MIME: "image/gif", DefaultInterval: 5 * time.Minute, Description: "WPC Excessive Rainfall Outlook — Day 1"},
    // Day 2.
    {Source: "wpc", Name: "ero_day2", URL: "https://www.wpc.ncep.noaa.gov/qpf/98ewbg.gif", MIME: "image/gif", DefaultInterval: 10 * time.Minute, Description: "WPC Excessive Rainfall Outlook — Day 2"},
    // Day 3.
    {Source: "wpc", Name: "ero_day3", URL: "https://www.wpc.ncep.noaa.gov/qpf/99ewbg.gif", MIME: "image/gif", DefaultInterval: 15 * time.Minute, Description: "WPC Excessive Rainfall Outlook — Day 3"},

    // ── Winter — WPC WSSI Overall CONUS ────────────────────────────────────
    // Day 1: today's overall winter-storm impact.
    {Source: "wpc", Name: "wssi_day1", URL: "https://www.wpc.ncep.noaa.gov/wwd/wssi/images/WSSI_Overall_Day1_CONUS_Day1.png", MIME: "image/png", DefaultInterval: 10 * time.Minute, Description: "WPC Winter Storm Severity Index — Day 1 (overall, CONUS)"},
    // Day 2.
    {Source: "wpc", Name: "wssi_day2", URL: "https://www.wpc.ncep.noaa.gov/wwd/wssi/images/WSSI_Overall_Day2_CONUS_Day2.png", MIME: "image/png", DefaultInterval: 15 * time.Minute, Description: "WPC Winter Storm Severity Index — Day 2 (overall, CONUS)"},
    // Day 3.
    {Source: "wpc", Name: "wssi_day3", URL: "https://www.wpc.ncep.noaa.gov/wwd/wssi/images/WSSI_Overall_Day3_CONUS_Day3.png", MIME: "image/png", DefaultInterval: 20 * time.Minute, Description: "WPC Winter Storm Severity Index — Day 3 (overall, CONUS)"},

    // ── Heat — WPC HeatRisk ────────────────────────────────────────────────
    // Day 1: today's heat-related health risk.
    {Source: "wpc", Name: "heatrisk_day1", URL: "https://www.wpc.ncep.noaa.gov/heatrisk/graphics/HeatRisk_Day1_CONUS.png", MIME: "image/png", DefaultInterval: 15 * time.Minute, Description: "WPC HeatRisk — Day 1 (CONUS)"},
    // Day 2.
    {Source: "wpc", Name: "heatrisk_day2", URL: "https://www.wpc.ncep.noaa.gov/heatrisk/graphics/HeatRisk_Day2_CONUS.png", MIME: "image/png", DefaultInterval: 20 * time.Minute, Description: "WPC HeatRisk — Day 2 (CONUS)"},
    // Day 3.
    {Source: "wpc", Name: "heatrisk_day3", URL: "https://www.wpc.ncep.noaa.gov/heatrisk/graphics/HeatRisk_Day3_CONUS.png", MIME: "image/png", DefaultInterval: 30 * time.Minute, Description: "WPC HeatRisk — Day 3 (CONUS)"},

    // ── Tropical — NHC + Navy JTWC ─────────────────────────────────────────
    // NHC 7-day Atlantic basin tropical outlook.
    {Source: "nhc", Name: "atl_7d", URL: "https://www.nhc.noaa.gov/xgtwo/two_atl_7d0.png", MIME: "image/png", DefaultInterval: 30 * time.Minute, Description: "NHC Tropical Weather Outlook — Atlantic (7 day)"},
    // NHC 7-day East Pacific.
    {Source: "nhc", Name: "epac_7d", URL: "https://www.nhc.noaa.gov/xgtwo/two_pac_7d0.png", MIME: "image/png", DefaultInterval: 30 * time.Minute, Description: "NHC Tropical Weather Outlook — East Pacific (7 day)"},
    // NHC 7-day Central Pacific.
    {Source: "nhc", Name: "cpac_7d", URL: "https://www.nhc.noaa.gov/xgtwo/two_cpac_7d0.png", MIME: "image/png", DefaultInterval: 30 * time.Minute, Description: "NHC Tropical Weather Outlook — Central Pacific (7 day)"},
    // US Navy Joint Typhoon Warning Center — Western Pacific basin tropical advisory.
    {Source: "navy", Name: "jtwc_abpw", URL: "https://www.metoc.navy.mil/jtwc/products/abpwsair.jpg", MIME: "image/jpeg", DefaultInterval: 30 * time.Minute, Description: "Navy JTWC ABPW — Western Pacific tropical synopsis"},

    // ── Flooding — WPC NWC National Flood Hazard Outlook ──────────────────
    // National flood hazard outlook from the National Water Center.
    {Source: "nwc", Name: "fho_national", URL: "https://www.weather.gov/images/owp/FHO/National/National_FHO.png", MIME: "image/png", DefaultInterval: 30 * time.Minute, Description: "NWC National Flood Hazard Outlook"},
}
```

Note that `DefaultInterval` is the per-image registry default; operator overrides via `images.image_intervals` in YAML always win. The defaults are tiered (Day 1 polled tighter than Day 3) because today's-product churn is the operationally-relevant signal.

### 5.2 Disk store (Go)

```go
// internal/imageproxy/store.go
package imageproxy

type DiskStore struct { /* ... */ }

func NewDiskStore(dir string, maxBytes int64, logger *slog.Logger) (*DiskStore, error)

// Get returns bytes + metadata for the key, or os.ErrNotExist.
func (s *DiskStore) Get(key string) (bytes []byte, contentType, validator string, fetchedAt time.Time, err error)

// Put writes bytes + metadata atomically; evicts LRU entries if maxBytes is exceeded.
func (s *DiskStore) Put(key string, bytes []byte, contentType, validator string, fetchedAt time.Time) error

// Stats returns current usage for /api/sources surfacing.
func (s *DiskStore) Stats() (entries int, bytesUsed int64)
```

Layout under `cache_dir`:
```
<cache_dir>/
├── <key-hash-prefix-2>/<key-hash-rest>.bin   # raw bytes
├── <key-hash-prefix-2>/<key-hash-rest>.json  # {contentType, validator, fetchedAt, originalKey}
├── manifest.json                              # full registry of (key → bytes, lastAccess); written on every Put + once per minute
```

Atomic write = write to `*.tmp` then rename. On boot, `NewDiskStore` walks the directory, reads `manifest.json`, validates each entry against its `.bin/.json` pair, drops orphans. LRU eviction = sort by `lastAccess` ascending, delete until under cap.

Key hashing: `sha256(key)[:32]` hex; prefix-2 directory split keeps any one directory under ~1k entries even with 100k images (we have 20).

### 5.3 Hot tier (Go)

```go
// internal/imageproxy/hot.go
package imageproxy

type HotTier struct { /* ... */ }

func NewHotTier(maxBytes int64, maxEntries int) *HotTier

func (h *HotTier) Get(key string) (bytes []byte, contentType, validator string, fetchedAt time.Time, ok bool)
func (h *HotTier) Put(key string, bytes []byte, contentType, validator string, fetchedAt time.Time)
func (h *HotTier) Stats() (entries int, bytesUsed int64)
```

Standard LRU: `container/list` doubly-linked list + `map[string]*list.Element`. Bounded by both `maxBytes` AND `maxEntries`. Put with bytes that alone exceed `maxBytes` is a no-op (the disk still has it).

### 5.4 Proxy (Go)

```go
// internal/imageproxy/proxy.go
package imageproxy

type Proxy struct {
    registry map[string]Image  // key → Image
    hot      *HotTier
    disk     *DiskStore
    fetcher  *fetcher.Client   // reuses Phase 1/2 conditional-GET + UA + backoff
    hub      *sse.Hub          // for image.invalidate broadcast
    intervals map[string]time.Duration  // operator-override map
    nowFn     func() time.Time
}

func New(...) *Proxy

// Get returns the cached or freshly-fetched bytes for a registered key.
// On cache miss with no prior cache: synchronous fetch (with timeout).
// On cache hit: returns hot-tier bytes if present, else disk bytes (and warms the hot tier).
// Returns os.ErrNotExist if the key is unknown.
func (p *Proxy) Get(ctx context.Context, key string) (bytes []byte, contentType, validator string, fetchedAt time.Time, isStale bool, staleSince time.Duration, err error)

// Refresh attempts a conditional GET for the key. On success-with-change, writes
// hot+disk and broadcasts image.invalidate. On 304 / unchanged body, no-ops.
// On upstream failure, writes a stale-warning marker to be reflected in /api/sources
// but does NOT mutate the cache. Used by prewarm pollers and by Get's lazy-refresh path.
func (p *Proxy) Refresh(ctx context.Context, key string) error

// Stats per registered image for /api/sources surfacing.
func (p *Proxy) Stats() map[string]ImageHealth

type ImageHealth struct {
    LastAttempt          time.Time
    LastSuccess          time.Time
    Validator            string
    AgeSec               int64
    ConsecutiveFailures  int
    BytesOnDisk          int64
    InHotTier            bool
}
```

### 5.5 Prewarm poller (Go)

```go
// internal/imageproxy/prewarm.go
package imageproxy

// StartPrewarm spawns one goroutine per key that calls proxy.Refresh on
// the per-image interval (registry default, overridden by operator).
// Returns a stop function.
func StartPrewarm(ctx context.Context, p *Proxy, keys []string, logger *slog.Logger) func()
```

Implementation reuses the existing `internal/fetcher.Client` ticker pattern from Phase 1/2 — same exponential-backoff-on-error, same context-cancel discipline.

### 5.6 SSE event payload

```typescript
// web/src/api/types.ts (additions)
export type ImageInvalidate = {
  source: string;       // e.g. "spc"
  name: string;         // e.g. "day1otlk"
  fetchedAt: string;    // ISO-8601 UTC
};
```

Go-side wire shape on the SSE bus:
```
event: image.invalidate
data: {"source":"spc","name":"day1otlk","fetchedAt":"2026-05-03T22:14:33Z"}
```

### 5.7 `/api/sources` extension

Existing 5 source entries unchanged. Each `images.prewarm` key gets a sibling entry keyed `image:<source>.<name>` (e.g. `image:spc.day1otlk`) with the same shape:

```json
{
  "intervalSec": 120,
  "lastAttempt": "...",
  "lastSuccess": "...",
  "etag": "sha256:...",     // or upstream ETag if available
  "ageSec": 47,
  "consecutiveFailures": 0
}
```

All registered image keys appear in `/api/sources` from boot, prewarm and lazy alike. Lazy-only entries are emitted with zero-valued `lastAttempt`/`lastSuccess`/`ageSec` until their first observation, at which point those fields populate. This makes the full registry visible to operators and dashboards from process start without waiting for first-touch on every key. (Phase 3 cleanup #6 N6 — supersedes the original "do not appear until first lazy fetch" wording, which was operator-unfriendly.)

### 5.8 HTTP API surface

```
GET /img/{source}/{name}     200 OK + Content-Type + Cache-Control: public, max-age=60, stale-while-revalidate=900
                             304 Not Modified (browser conditional GET)
                             404 if registry has no such (source, name)
                             502 only if no cached copy exists AND upstream fails
                             200 + "Warning: 110 cwd \"stale Xm\"" if cached but upstream is currently failing
```

The handler honors the browser's `If-None-Match` header for 304 responses against our stored validator (whether sha256 or upstream ETag).

---

## 6. Data flow

### 6.1 Lazy GET (cache miss, no prior copy)

```
Browser                  Server                                     Upstream
   │                        │                                          │
   ├─GET /img/spc/day1otlk──▶                                          │
   │                        ├─ HotTier.Get → miss                      │
   │                        ├─ DiskStore.Get → miss                    │
   │                        ├─ Proxy.Refresh                           │
   │                        │   conditional GET (no validator)─────────▶
   │                        │                          200 OK + bytes ◀┤
   │                        ├─ DiskStore.Put                           │
   │                        ├─ HotTier.Put                             │
   │                        ├─ hub.Broadcast(image.invalidate)         │
   ◀──200 OK + bytes────────┤                                          │
```

### 6.2 Lazy GET (hot-tier hit, fresh)

```
Browser                  Server
   │                        │
   ├─GET /img/spc/day1otlk──▶
   │                        ├─ HotTier.Get → hit, ageSec < interval
   ◀──200 OK + bytes────────┤
```

### 6.3 Lazy GET (disk hit, stale)

```
Browser                  Server                                     Upstream
   │                        │                                          │
   ├─GET /img/spc/day1otlk──▶                                          │
   │                        ├─ HotTier.Get → miss                      │
   │                        ├─ DiskStore.Get → hit, ageSec ≥ interval  │
   │                        ├─ HotTier.Put (warm from disk)            │
   ◀──200 OK + bytes────────┤                                          │
   │                        ├─ async Proxy.Refresh ──────conditional GET▶
   │                        │       (304 or new bytes; if new,         │
   │                        │        broadcast image.invalidate so     │
   │                        │        the SPA re-fetches via key bump)  │
```

i.e. **stale-while-revalidate** at the proxy layer. The browser's request never blocks on the upstream re-check.

### 6.4 Prewarm tick

```
Ticker fires for spc.day1otlk
    └─ Proxy.Refresh
        ├─ conditional GET (with stored validator)
        ├─ if 304 or identical body → no-op (sources entry's lastAttempt updates; lastSuccess does NOT)
        ├─ if new bytes → DiskStore.Put + HotTier.Put + hub.Broadcast(image.invalidate)
        └─ if upstream error → log WARN, increment consecutiveFailures, do NOT mutate cache
```

### 6.5 Upstream failure on lazy GET (no cached copy)

```
Browser                  Server                                     Upstream
   │                        │                                          │
   ├─GET /img/spc/day1otlk──▶                                          │
   │                        ├─ HotTier.Get → miss                      │
   │                        ├─ DiskStore.Get → miss                    │
   │                        ├─ Proxy.Refresh                           │
   │                        │     conditional GET ─────────────────────▶
   │                        │                            500 Internal ◀┤
   ◀──502 Bad Gateway───────┤                                          │
```

### 6.6 Upstream failure on lazy GET (cached copy exists)

```
Browser                  Server                                     Upstream
   │                        │                                          │
   ├─GET /img/spc/day1otlk──▶                                          │
   │                        ├─ DiskStore.Get → hit, ageSec ≥ interval  │
   ◀──200 OK + bytes────────┤  (with Warning: 110 cwd "stale 12m")    │
   │                        ├─ async Proxy.Refresh ──────GET───────────▶
   │                        │                              503 Unav ◀──┤
   │                        ├─ log WARN, /api/sources reflects failure │
```

### 6.7 SSE-driven re-render

```
Server                                                     Browser
   │                                                          │
   ├─image.invalidate {source:"spc",name:"day1otlk",...}─────▶ stream.ts
   │                                                          │   │
   │                                                          │   └─ snapshot.ts: imageRefresh["spc.day1otlk"] = fetchedAt
   │                                                          │       │
   │                                                          │       └─ HazardCategoryCard re-renders with new key on the <Image>
   │                                                          │           │
   ◀─GET /img/spc/day1otlk────────────────────────────────────┤◀──────────┘
   │                                                          │
   ├─200 OK + bytes (HotTier hit) ───────────────────────────▶│
```

---

## 7. Frontend

### 7.1 Hazards page composition

```
<Hazards>
  <Row gutter={[16,16]}>
    <Col xs={24} sm={12} md={8}>  <HazardCategoryCard category="severe-storms" />  </Col>
    <Col xs={24} sm={12} md={8}>  <HazardCategoryCard category="wildfire" />        </Col>
    <Col xs={24} sm={12} md={8}>  <HazardCategoryCard category="excessive-rainfall" /></Col>
    <Col xs={24} sm={12} md={8}>  <HazardCategoryCard category="winter" />          </Col>
    <Col xs={24} sm={12} md={8}>  <HazardCategoryCard category="heat" />            </Col>
    <Col xs={24} sm={12} md={8}>  <HazardCategoryCard category="tropical" />        </Col>
    <Col xs={24} sm={12} md={8}>  <HazardCategoryCard category="flooding" />        </Col>
  </Row>
</Hazards>
```

Responsive: stacks on mobile (xs=24 → 1-col), 2-col on tablet (sm=12), 3-col on desktop (md=8). Verbatim from parent design §6.2.

### 7.2 HazardCategoryCard

```tsx
type Category = "severe-storms" | "wildfire" | "excessive-rainfall" | "winter" | "heat" | "tropical" | "flooding";

const CATEGORIES: Record<Category, {title: string; icon: React.ReactNode; images: ImageRef[]}> = {
  "severe-storms": { title: "Severe Storms", icon: <ThunderboltOutlined />, images: [
    { key: "spc.day1otlk", label: "Day 1", path: "/img/spc/day1otlk", alt: "SPC Convective Outlook Day 1" },
    { key: "spc.day2otlk", label: "Day 2", path: "/img/spc/day2otlk", alt: "SPC Convective Outlook Day 2" },
    { key: "spc.day3otlk", label: "Day 3", path: "/img/spc/day3otlk", alt: "SPC Convective Outlook Day 3" },
  ]},
  // ... 6 more entries, mirroring the 7 categories
};

function HazardCategoryCard({ category }: { category: Category }) {
  const { title, icon, images } = CATEGORIES[category];
  const [day, setDay] = useState(0);
  const refresh = useSnapshotStore((s) => s.imageRefresh[images[day].key]);
  // refresh changes → key on <Image> bumps → browser re-fetches /img/* → server returns fresh hot-tier bytes
  return (
    <ProCard title={<><span>{icon}</span> {title}</>} extra={<Tag color="default">stale Xm</Tag>}>
      <Segmented options={images.map((i, idx) => ({ label: i.label, value: idx }))} value={day} onChange={(v) => setDay(Number(v))} />
      <Image.PreviewGroup>
        <Image src={images[day].path} alt={images[day].alt} key={refresh ?? "init"} placeholder={<Skeleton.Image style={{ width: "100%", height: 240 }} />} />
      </Image.PreviewGroup>
    </ProCard>
  );
}
```

The `key={refresh}` on `<Image>` is the SSE-driven re-fetch trigger: when `image.invalidate` arrives for this key, the store's `imageRefresh[key]` ticks, the `<Image>` element gets a new key, React unmounts/remounts, browser does a fresh GET — and the server's hot tier serves the new bytes immediately.

`Image.PreviewGroup` per category lets users arrow-key through Day 1/2/3 in fullscreen lightbox per parent design §6.2.

The `Tag` extra ("stale Xm") consults `/api/sources` (extended in §5.7) for the `image:<key>` entry's `consecutiveFailures > 0` state.

### 7.3 Settings drawer surface

Read-only display of `images.prewarm` list and current cache stats (`disk_max_bytes`, `disk_bytes_used`, entries count) — phase 6 settings expansion. Phase 3 just exposes the data via `/api/uiconfig`; the UI bindings can be deferred.

---

## 8. Configuration

```yaml
# Phase 3 additions to config.yaml
images:
  cache_dir: "/var/lib/cwd/images"      # required if not lazy-only; created with 0700 if missing
  disk_max_bytes: 524288000             # 500 MiB
  hot_max_bytes: 67108864               # 64 MiB
  hot_max_entries: 256                  # bound on entry count (cheap upper limit on small images)
  refresh_interval: "5m"                # global default; per-image registry default still wins if tighter
  image_intervals:                      # operator overrides per-image, dot-key form
    spc.day1otlk: "90s"                 # tighter than the registry default during severe-weather days
    nhc.atl_7d: "1h"                    # looser during quiet period
  prewarm:                              # list of keys (dot form); add/remove freely
    - spc.day1otlk
    - spc.day2otlk
    - spc.day3otlk
    - nhc.atl_7d
```

**Defaults shipped in `internal/config/config.go defaults()`:**
- `cache_dir`: `<store_dir>/images` if not set (where `store_dir` is the existing Phase 1 SQLite store dir parent)
- `disk_max_bytes`: `524_288_000` (500 MiB)
- `hot_max_bytes`: `67_108_864` (64 MiB)
- `hot_max_entries`: `256`
- `refresh_interval`: `5 * time.Minute`
- `image_intervals`: empty map (registry defaults apply)
- `prewarm`: `["spc.day1otlk", "spc.day2otlk", "spc.day3otlk", "nhc.atl_7d"]`

**Env override pattern (extends Phase 1 walker):**
- `CWD_IMAGES_CACHE_DIR` — string
- `CWD_IMAGES_DISK_MAX_BYTES` — int64
- `CWD_IMAGES_HOT_MAX_BYTES` — int64
- `CWD_IMAGES_HOT_MAX_ENTRIES` — int
- `CWD_IMAGES_REFRESH_INTERVAL` — duration
- `CWD_IMAGES_PREWARM` — comma-separated dot keys (replaces the entire list; not a delta)
- `CWD_IMAGES_IMAGE_INTERVALS_<KEY>` — duration; e.g. `CWD_IMAGES_IMAGE_INTERVALS_SPC_DAY1OTLK=90s`. The walker treats `_` in the key segment as a literal underscore in the dot form (e.g. `SPC_DAY1OTLK_FIRE` → `spc.day1otlk_fire`); decision: use a `__` separator to disambiguate, so `CWD_IMAGES_IMAGE_INTERVALS_SPC__DAY1OTLK_FIRE` → `spc.day1otlk_fire`. Documented in README.

**Validation at boot (extends Phase 2 §11):**
- `disk_max_bytes <= 0` → fatal error.
- `hot_max_bytes < 0` → fatal error. Zero is valid (disables hot tier).
- `hot_max_entries < 0` → fatal error.
- `refresh_interval < 60s` → clamp to 60s + WARN (politeness floor, matches per-source minimum from Phase 2).
- `image_intervals[key] < 60s` → clamp to 60s + WARN.
- `prewarm[i]` not in `Registry` keys → WARN + drop (mirrors Phase 1's `validateRegionFilter` and Phase 2's `validateSWPCProducts`).
- `cache_dir` not writable → fatal error at boot (don't start a process that can't persist). Created with `0700` if missing.

---

## 9. Testing

### 9.1 Backend

- `internal/imageproxy/registry_test.go` — registry has no duplicate keys; every entry's URL is well-formed; every entry has a non-empty Description.
- `internal/imageproxy/store_test.go` — Put/Get round-trip; restart load (manifest read); LRU eviction at byte cap; atomic write semantics (no half-files left after concurrent crashes simulated with truncation).
- `internal/imageproxy/hot_test.go` — Put/Get round-trip; LRU eviction at both byte cap AND entry cap; oversized-entry Put is no-op.
- `internal/imageproxy/proxy_test.go` — lazy-fetch happy path (httptest upstream); 304 path; stale-while-revalidate (cache hit serves immediately, async refresh fires); upstream-failure-with-cached-copy serves stale + Warning header; upstream-failure-no-cache returns 502; SSE invalidate fires only on real content change.
- `internal/imageproxy/prewarm_test.go` — N pollers run on their cadence; context cancel stops cleanly; failure backoff matches `internal/fetcher` semantics.
- `internal/api/images_test.go` — handler dispatches to proxy; 404 for unknown registry key; 304 honors browser If-None-Match against our validator; Cache-Control header shape; `image:` entries appear in `/api/sources`.
- `internal/server/server_test.go` — extend boot test to verify all 4 default-prewarm keys go live (using httptest backends), `/api/sources` lists them, `/img/{source}/{name}` serves bytes for each.
- `internal/config/config_test.go` — extend with the new validation rules per §8.

### 9.2 Frontend

- `web/src/components/HazardCategoryCard.test.tsx` — renders with each of the 7 categories; Day 1/2/3 segmented switch swaps the `<Image>`; `imageRefresh` store change bumps the `key` and triggers re-fetch (assert via spy on `<Image>` `key` prop).
- `web/src/pages/Hazards.test.tsx` — renders all 7 cards; responsive grid (smoke test).
- `web/src/api/stream.test.ts` — extend: `image.invalidate` event routes into `imageRefresh` store correctly.
- `web/src/store/snapshot.test.ts` — extend: `applyImageInvalidate` sets the per-key timestamp.

### 9.3 Smoke

`task smoke` extension: after the existing 5-source health gate, also assert each `images.prewarm` key has a `lastSuccess` populated within 2× its interval.

### 9.4 Test fixtures

- A small (≤10 KB) PNG and GIF in `internal/imageproxy/testdata/` for httptest server bodies.
- No live upstream captures needed for fixtures (image bytes are opaque to the proxy; the test fixture just needs to be a valid HTTP body).

---

## 10. Definition of done

- [ ] Design doc + implementation plan committed under `docs/plans/`
- [ ] All work in `.worktrees/phase3-image-proxy` on branch `feature/phase3-image-proxy` off main at `86b2d1b` (the Phase 2 merge commit)
- [ ] All Go tests pass with `-race`; `golangci-lint run ./...` clean
- [ ] All frontend tests pass (`task web:test`); `task web:typecheck` clean; `task web:build` succeeds
- [ ] `task build:all && ./bin/cwd serve` shows live data on `/hazards`: 7 ProCards render; each card's Day 1 image loads through `/img/*`; switching to Day 2/3 loads next image; PreviewGroup fullscreen works
- [ ] `task smoke` reports all 5 data sources + all `images.prewarm` keys `consecutiveFailures: 0`
- [ ] `internal/webdist/dist/index.html` IS the placeholder (`git diff <merge-base> -- internal/webdist/dist/index.html` empty)
- [ ] `internal/cache/cache.go` and `internal/sse/hub.go` are unchanged in this branch (Phase 1's race fix preserved; Phase 2's hub-passes-through invariant preserved)
- [ ] Independent comprehensive review passes
- [ ] PR opened to main, CI green, merged with per-task history preserved
- [ ] Worktree cleaned up

---

## 11. Out of scope (deferred to later phases)

- History page (slider + range scrubber over historic snapshots) — Phase 4.
- PWA service-worker for offline image fallback — Phase 5.
- Image transcoding / WEBP conversion / resize — out of v1.
- Operator UI for editing `images.prewarm` (currently YAML/env only) — Phase 6 settings panel.
- Automatic registry refresh from a published NCEP product index — out of v1; manual code update if SPC/WPC churn.
- Per-region (state, WFO) image variants — out of v1 (we serve CONUS-only maps).

---

## 12. Risks

1. **Upstream reshape.** SPC/WPC have been known to retire products without notice (e.g. Day 3-8 fire wx is "experimental"). When that happens, `Proxy.Refresh` returns 404 forever for that key, the `image:` source entry shows growing `consecutiveFailures`, and the SPA shows "stale Xm" indefinitely. Detection is via `/api/sources` + smoke task; recovery is a config edit (drop the key from prewarm) or a code change (drop the registry entry). Not blocking.

2. **Disk usage drift.** Default 500 MiB cap is comfortable for 20 images at typical PNG sizes (~100-300 KB each → 6 MB total) plus history headroom. If we ever add high-resolution radar mosaics in a later phase the cap may need bumping. Operator-tunable via `CWD_IMAGES_DISK_MAX_BYTES`. Not blocking.

3. **Pre-warm thundering herd.** Four default pre-warm pollers + 16 lazy keys on a busy `/hazards` page first load → up to 20 concurrent upstream GETs against 4 distinct hosts (spc.noaa.gov, wpc.ncep.noaa.gov, nhc.noaa.gov, weather.gov, plus metoc.navy.mil). The existing `internal/fetcher` already serializes per-host (one in-flight request per source) via the existing `http.Client` connection pool — we inherit that. Initial paint latency on first-ever boot may be 2-5 seconds while lazy fetches complete; subsequent loads serve from cache. Acceptable.

4. **Browser cache vs server cache.** Server sends `Cache-Control: public, max-age=60, stale-while-revalidate=900` so the browser may serve a stale image for up to 15 min without consulting the server. The SSE `image.invalidate` event triggers a key-bump on the `<Image>` element which forces a fresh request that bypasses the browser cache (React unmount-remount of the `<img>` tag with a different `key`). Documented; not blocking.

5. **`Warning: 110` HTTP header.** RFC 7234 defines this; modern browsers don't surface it to the page (so the SPA still relies on `/api/sources` for the "stale Xm" Tag). This is fine — the header is for CLI/debug visibility.

6. **HeatRisk `<img alt"...">` upstream HTML bug noted in recon.** Doesn't affect us — we proxy the image, not the upstream HTML. Recon's note is informational.

7. **Image content licensing.** All 20 images are NOAA/NWS/NCEP/USNavy products in the public domain; no licensing constraint on caching or proxying for this self-host use case. (Verified during recon §4.)

---

## 13. Open questions

None remaining. All 7 brainstorm decisions answered; recon §4 nails the upstream inventory.
