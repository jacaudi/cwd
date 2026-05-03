# cwd — Self-Hosted Critical Weather Day Status

A self-hostable replacement for [https://www.nco.ncep.noaa.gov/status/cwd/](https://www.nco.ncep.noaa.gov/status/cwd/).

**Status:** Phase 2 — all 5 sources live end-to-end. `nws_alerts`, `swpc_scales`, `swpc_alerts`, `usgs_quakes`, `usgs_volcanoes` all running on the Phase 1 pipeline. SpaceWeather page (`/space`) renders a 3-day forecast cards block + alerts list. Events page (`/events`) renders the tsunami panel + significant earthquakes list + elevated volcanoes list. Overview's Tsunami badge deep-links to `/events#tsunami`. Footer SourceHealthIndicator shows 5 source tags.

The `/api/history` endpoint currently surfaces only `nws_alerts`. Per-source history for the SWPC and USGS sources is a Phase 3+ follow-up; until then `/api/snapshot` and `/api/stream` are the canonical multi-source views.

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
