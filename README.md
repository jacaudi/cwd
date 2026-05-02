# cwd — Self-Hosted Critical Weather Day Status

A self-hostable replacement for [https://www.nco.ncep.noaa.gov/status/cwd/](https://www.nco.ncep.noaa.gov/status/cwd/).

**Status:** Phase 0 (skeleton). No live data yet — see `docs/plans/`.

## Quick start

```bash
make build-all     # builds frontend + backend, produces ./cwd
./cwd serve        # binds 127.0.0.1:8765 by default
open http://127.0.0.1:8765
```

## Configuration

Default config path: `$XDG_CONFIG_HOME/cwd/config.yaml` (or `--config <path>`, or `$CWD_CONFIG`). See `docs/plans/2026-05-01-self-hosted-cwd-design.md` §8 for the full schema.

## Development

```bash
make test          # backend unit tests
make lint          # golangci-lint
make run           # backend only (no frontend rebuild)
make build-web     # frontend only → internal/webdist/dist/
```

## Documentation

- Design: `docs/plans/2026-05-01-self-hosted-cwd-design.md`
- Recon: `docs/recon/2026-05-01-ncep-cwd-status-recon.md`
- License: MIT
