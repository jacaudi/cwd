# cwd — Self-Hosted Critical Weather Day Status

A self-hostable replacement for [https://www.nco.ncep.noaa.gov/status/cwd/](https://www.nco.ncep.noaa.gov/status/cwd/).

**Status:** Phase 3 — image proxy + Hazards page live · Phase 4 (history) live. All 5 data sources from Phase 2 (`nws_alerts`, `swpc_scales`, `swpc_alerts`, `usgs_quakes`, `usgs_volcanoes`) continue to run on the Phase 1 pipeline. SpaceWeather page (`/space`) renders a 3-day forecast cards block + alerts list. Events page (`/events`) renders the tsunami panel + significant earthquakes list + elevated volcanoes list. Overview's Tsunami badge deep-links to `/events#tsunami`. Footer SourceHealthIndicator shows 5 source tags. The new `/hazards` page renders 7 category cards (Severe Storms, Wildfire, Excessive Rainfall, Winter, Heat, Tropical, Flooding) backed by 20 server-proxied NCEP/NWS/NHC/Navy maps under `/img/{source}/{name}`.

The `/api/history` endpoint surfaces all 5 data sources as per-source aggregated time series. Query: `GET /api/history?source=<name>&window=24h|7d|30d`. The `/history` page renders one chart per source over the selected window, live-tailed via the existing `<source>.update` SSE events. Phase 4 adds `@ant-design/charts` to the SPA bundle (~250 KB gzipped). Storage retention is governed by `store.retention_days` (default 30); snapshots are written by the generic fetcher loop on every successful poll where the upstream validator changed.

The HazSimp category map expansion (Tornado Watch, Flood Warning, etc.) is tracked in [issue #3](https://github.com/jacaudi/cwd/issues/3) for a future PR.

## Quick start

```bash
task build:all      # builds frontend + backend, produces ./bin/cwd
./bin/cwd serve     # binds 127.0.0.1:8765 by default
open http://127.0.0.1:8765
```

Requires [`task`](https://taskfile.dev), Go 1.26+, Node 24 LTS, and pnpm 9+.

## Configuration

Default config path: `$XDG_CONFIG_HOME/cwd/config.yaml` (or `--config <path>`, or `$CWD_CONFIG`). See `docs/plans/2026-05-01-self-hosted-cwd-design.md` §8 for the full schema.

### Politeness contract

`api.weather.gov` requires a contact-bearing `User-Agent` on every request. Set `server.contact` in your config (e.g. `you@example.com`) — the server logs `server.contact_unset` at boot if you leave it empty, but still serves with a placeholder UA. See [https://www.weather.gov/documentation/services-web-api](https://www.weather.gov/documentation/services-web-api) for the upstream rules.

### Per-source environment overrides

Source intervals and enable/disable can be overridden without touching YAML:

```bash
CWD_SOURCES_NWS_ALERTS_INTERVAL=45s
CWD_SOURCES_NWS_ALERTS_ENABLED=false
CWD_SOURCES_SWPC_SCALES_INTERVAL=120s
CWD_SOURCES_SWPC_SCALES_ENABLED=false
CWD_SOURCES_SWPC_ALERTS_INTERVAL=120s
CWD_SOURCES_SWPC_ALERTS_ENABLED=false
CWD_SOURCES_USGS_QUAKES_INTERVAL=60s
CWD_SOURCES_USGS_QUAKES_ENABLED=false
CWD_SOURCES_USGS_VOLCANOES_INTERVAL=10m
CWD_SOURCES_USGS_VOLCANOES_ENABLED=false
```

Pattern: `CWD_SOURCES_<UPPER_SNAKE_NAME>_<FIELD>`. Bad values log `config.env_parse_failed` (WARN) and fall back to the YAML value.

### Per-source cadence

| Source | Default interval | Validator | Notes |
|---|---|---|---|
| `nws_alerts` | 30s | sha256 content hash | api.weather.gov politeness |
| `swpc_scales` | 60s | sha256 content hash | matches upstream max-age=60 |
| `swpc_alerts` | 60s | sha256 content hash | 24h window + product allowlist |
| `usgs_quakes` | 60s | upstream ETag | If-None-Match passthrough |
| `usgs_volcanoes` | 5m | upstream ETag | NORMAL filtered server-side |

### Phase 3 — Image proxy + Hazards page

Live now: server-side proxy in front of 20 NCEP/NWS/NHC/Navy static maps,
served at `/img/{source}/{name}`. Lazy-by-default with an opt-in pre-warm
poller per image. The `/hazards` page renders seven category cards (Severe
Storms, Wildfire, Excessive Rainfall, Winter, Heat, Tropical, Flooding) with
Segmented Day 1/2/3 controls and AntD `Image.PreviewGroup` lightbox.

#### Operator knobs (env)

```
CWD_IMAGES_CACHE_DIR=/var/lib/cwd/images
CWD_IMAGES_DISK_MAX_BYTES=524288000
CWD_IMAGES_HOT_MAX_BYTES=67108864
CWD_IMAGES_HOT_MAX_ENTRIES=256
CWD_IMAGES_PREWARM=spc.day1otlk,spc.day2otlk,spc.day3otlk,nhc.atl_7d
# Per-image interval override; "__" is the dot separator in the registry key.
CWD_IMAGES_IMAGE_INTERVALS_SPC__DAY1OTLK=90s
CWD_IMAGES_IMAGE_INTERVALS_NHC__ATL_7D=1h
```

#### Image registry summary

| Category            | Keys                                                    | Default cadence |
|---------------------|---------------------------------------------------------|-----------------|
| Severe Storms       | `spc.day1otlk`, `spc.day2otlk`, `spc.day3otlk`         | 2m / 5m / 10m   |
| Wildfire            | `spc.day1otlk_fire`, `spc.day2otlk_fire`, `spc.day38otlk_fire` | 5m / 10m / 30m |
| Excessive Rainfall  | `wpc.ero_day1..3`                                       | 5m / 10m / 15m  |
| Winter              | `wpc.wssi_day1..3`                                      | 10m / 15m / 20m |
| Heat                | `wpc.heatrisk_day1..3`                                  | 15m / 20m / 30m |
| Tropical            | `nhc.atl_7d`, `nhc.epac_7d`, `nhc.cpac_7d`, `navy.jtwc_abpw` | 30m each |
| Flooding            | `nwc.fho_national`                                      | 30m             |

`/api/sources` surfaces one `image:<key>` entry per registered key; the SPA
displays a "stale Xm" Tag when `consecutiveFailures > 0`.

`/img/{source}/{name}` returns the upstream bytes verbatim with
`Cache-Control: public, max-age=60, stale-while-revalidate=900`. When upstream
omits a `Content-Type` header, the registry's declared MIME type for the key
is substituted; otherwise the upstream `Content-Type` is preserved as-is.
Responses whose `Content-Type` is non-image (e.g. an HTML rate-limit page)
or whose body is empty are rejected and the prior cached bytes (if any)
remain in place. When the proxy is serving stale bytes (upstream currently
failing), the response includes `Warning: 110 cwd "stale Xm"`.

#### SSE invalidation

When the proxy's diff-on-write detects a real content change, it broadcasts
an `image.invalidate.update` SSE event with `{source, name, fetchedAt}`. The
SPA's `useSnapshotStore.imageRefresh[<source>.<name>]` map updates, the
`<Image>` element keys on it, React unmount-remounts, the browser refetches
`/img/...`, and the server's hot tier serves the fresh bytes immediately.

### Reverse proxy note (SSE)

`/api/stream` is a long-lived Server-Sent Events connection. If you front the server with nginx, Caddy, or another reverse proxy, **disable response buffering on that route** so events reach the browser as they're emitted. Examples:

- nginx: `proxy_buffering off;` and `proxy_cache off;` on the `/api/stream` location (the server already sets `X-Accel-Buffering: no` to hint this).
- Caddy: streaming works by default; no extra config needed.
- A 25-second comment-ping (`: ping`) is emitted on the stream to keep idle proxies from timing out the connection.

## Development

```bash
task                # list all available tasks
task test           # backend unit tests
task lint           # golangci-lint
task run            # backend only (no frontend rebuild)
task web:install    # install frontend deps (frozen lockfile)
task web:build      # frontend only → internal/webdist/dist/
task web:dev        # Vite dev server (proxies /api to localhost:8765)
task web:test       # frontend Vitest suite
task web:typecheck  # TypeScript typecheck only
task smoke          # boot against real upstreams for ~5 min; assert all 5 sources healthy
task clean          # remove build artifacts
```

## Documentation

- Design (parent): `docs/plans/2026-05-01-self-hosted-cwd-design.md`
- Phase 1 design: `docs/plans/2026-05-02-phase1-nws-alerts-design.md`
- Phase 2 design: `docs/plans/2026-05-02-phase2-multi-source-design.md`
- Phase 2 implementation: `docs/plans/2026-05-02-phase2-multi-source-implementation.md`
- Recon: `docs/recon/2026-05-01-ncep-cwd-status-recon.md`
- License: MIT
