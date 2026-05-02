# cwd — Self-Hosted Critical Weather Day Status

A self-hostable replacement for [https://www.nco.ncep.noaa.gov/status/cwd/](https://www.nco.ncep.noaa.gov/status/cwd/).

**Status:** Phase 1 — `nws_alerts` source live end-to-end (badge bar + tsunami panel + SSE stream + history endpoint + per-source health footer). Other sources (SWPC scales/alerts, USGS quakes/volcanoes) arrive in Phase 2.

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
CWD_SOURCES_NWS_ALERTS_INTERVAL=45s   # Go time.Duration; default 30s
CWD_SOURCES_NWS_ALERTS_ENABLED=false  # disable a source entirely
```

Pattern: `CWD_SOURCES_<UPPER_SNAKE_NAME>_<FIELD>`. Bad values log `config.env_parse_failed` (WARN) and fall back to the YAML value.

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
task smoke          # boot against real api.weather.gov for ~5 min and report /api/sources
task clean          # remove build artifacts
```

## Documentation

- Design (parent): `docs/plans/2026-05-01-self-hosted-cwd-design.md`
- Phase 1 design: `docs/plans/2026-05-02-phase1-nws-alerts-design.md`
- Recon: `docs/recon/2026-05-01-ncep-cwd-status-recon.md`
- License: MIT
