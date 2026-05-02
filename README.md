# cwd — Self-Hosted Critical Weather Day Status

A self-hostable replacement for [https://www.nco.ncep.noaa.gov/status/cwd/](https://www.nco.ncep.noaa.gov/status/cwd/).

**Status:** Phase 0 (skeleton). No live data yet — see `docs/plans/`.

## Quick start

```bash
task build:all      # builds frontend + backend, produces ./bin/cwd
./bin/cwd serve     # binds 127.0.0.1:8765 by default
open http://127.0.0.1:8765
```

Requires [`task`](https://taskfile.dev), Go 1.26+, Node 22+, and pnpm 9+.

## Configuration

Default config path: `$XDG_CONFIG_HOME/cwd/config.yaml` (or `--config <path>`, or `$CWD_CONFIG`). See `docs/plans/2026-05-01-self-hosted-cwd-design.md` §8 for the full schema.

## Development

```bash
task                # list all available tasks
task test           # backend unit tests
task lint           # golangci-lint
task run            # backend only (no frontend rebuild)
task web:install    # install frontend deps (frozen lockfile)
task web:build      # frontend only → internal/webdist/dist/
task web:dev        # Vite dev server (proxies /api to localhost:8765)
task web:typecheck  # TypeScript typecheck only
task clean          # remove build artifacts
```

## Documentation

- Design: `docs/plans/2026-05-01-self-hosted-cwd-design.md`
- Recon: `docs/recon/2026-05-01-ncep-cwd-status-recon.md`
- License: MIT
