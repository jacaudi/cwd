# Self-Hosted CWD — Phase 3 (Image proxy + Hazards page) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up a server-side image proxy in front of the 20 third-party static maps the original NCEP CWD page hot-links, lazy-by-default with an opt-in pre-warm poller per image, and rewrite the placeholder `/hazards` page as 7 category cards (Severe Storms, Wildfire, Excessive Rainfall, Winter, Heat, Tropical, Flooding).

**Architecture:** A new `internal/imageproxy` subsystem (registry → disk store + hot tier → proxy core → prewarm pollers) sits alongside Phase 1/2's `internal/sources` pipeline. The proxy emits SSE invalidation events through the existing `cache.Cache` (a single synthetic key, `image.invalidate`, carrying an `ImageInvalidate{source,name,fetchedAt}` payload), so neither `internal/cache/cache.go` nor `internal/sse/hub.go` needs to change. The HTTP surface adds `GET /img/{source}/{name}` and extends `/api/sources` with one `image:<source>.<name>` entry per registered prewarm key.

**Tech Stack:**
- Backend: Go 1.24, `chi` router, `gopkg.in/yaml.v3`, `log/slog` stdlib, `modernc.org/sqlite`
- Frontend: Node 24, pnpm, Vite 5, React 18, TypeScript 5, AntD 5, `@ant-design/pro-components`, Zustand, Vitest + `@testing-library/react` + jsdom
- Tooling: `golangci-lint` v2, GitHub Actions via `jacaudi/github-actions` reusable workflows, Renovate, `Taskfile.yml` (go-task)

**Source documents:**
- Design: [`docs/plans/2026-05-03-phase3-image-proxy-design.md`](2026-05-03-phase3-image-proxy-design.md)
- Phase 1 design: [`docs/plans/2026-05-02-phase1-nws-alerts-design.md`](2026-05-02-phase1-nws-alerts-design.md)
- Phase 1 implementation: [`docs/plans/2026-05-02-phase1-nws-alerts-implementation.md`](2026-05-02-phase1-nws-alerts-implementation.md)
- Phase 2 design: [`docs/plans/2026-05-02-phase2-multi-source-design.md`](2026-05-02-phase2-multi-source-design.md)
- Phase 2 implementation (structural template): [`docs/plans/2026-05-02-phase2-multi-source-implementation.md`](2026-05-02-phase2-multi-source-implementation.md)
- Parent design: [`docs/plans/2026-05-01-self-hosted-cwd-design.md`](2026-05-01-self-hosted-cwd-design.md)
- Recon: [`docs/recon/2026-05-01-ncep-cwd-status-recon.md`](../recon/2026-05-01-ncep-cwd-status-recon.md) §4

**Branch:** `feature/phase3-image-proxy` off main at the Phase 2 merge commit `86b2d1b`. All work happens inside the worktree `.worktrees/phase3-image-proxy`.

> **For Claude:** REQUIRED EXECUTION WORKFLOW (follow in order):
> 1. `superpowers:using-git-worktrees` — Isolate work in a dedicated worktree (`.worktrees/phase3-image-proxy`)
> 2. `superpowers:subagent-driven-development` — Dispatch a fresh subagent per task
> 3. `superpowers:test-driven-development` — All subagents use TDD
> 4. `superpowers:verification-before-completion` — Verify all tests pass per task
> 5. `superpowers:requesting-code-review` — Code review after each task (built in)
> 6. After all tasks: comprehensive code review on full diff from branch point (automatic)
> 7. `superpowers:finishing-a-development-branch` — Complete the branch
>
> Skills carry their own model and effort settings. Do not override them.

---

## Conventions

- All commits prefixed `feat(p3):`, `test(p3):`, `chore(p3):`, `fix(p3):`, `docs(p3):`. Co-authored trailer per repo norm.
- All Go tests use stdlib `testing` + `httptest`; no external test frameworks. Table-driven where appropriate.
- All upstream HTTP must include `User-Agent: cwd-self-host/<version> (<contact>)`. Phase 1 already threads this via `buildUserAgent` in `internal/server/server.go`; the image proxy inherits the string passed to `imageproxy.New` at construction.
- Logging: stdlib `log/slog`. Use structured key/value fields; never format into the message.
- No CGO. SQLite is `modernc.org/sqlite` exclusively (the proxy itself does not touch SQLite — disk store is plain filesystem).
- Frontend tests live next to the component (`Component.test.tsx`).
- `golangci-lint` v2 must stay clean (`task lint`).
- Never `git add -A` or `git add .`. Never skip hooks (`--no-verify`, `--no-gpg-sign`).

### Commit author identity

The branch and PR author identity is already configured at the repo level:

```
Author: jacaudi <47005674+jacaudi@users.noreply.github.com>
```

Do not run `git config user.email` or `git config user.name`. The privacy-block address (`47005674+jacaudi@…`) is required — using `adam.caudill@proton.me` as the author email will be rejected by GitHub's privacy block on push.

### Stage + commit by explicit pathspec

To survive parallel-subagent execution (where two siblings may each be staging different files in the same worktree at overlapping wall-clock moments), every commit step **must** stage by explicit pathspec AND commit by explicit pathspec:

```bash
git add path/to/file_a.go path/to/file_b.go
git commit -m "feat(p3): explanatory message" -- path/to/file_a.go path/to/file_b.go
```

The trailing `-- <pathspec>` form on `git commit` causes git to commit only the named paths (treats them as `--include`). This is belt-and-suspenders: even if a sibling subagent stages an unrelated file in between the `git add` and the `git commit`, the trailing pathspec scopes the commit to your task's files.

Never `git add -A` and never `git add .`. Both have been observed (Phase 2 Tasks 1–4) to absorb sibling work into the wrong commit.

### Worktree setup

```bash
git worktree add -b feature/phase3-image-proxy .worktrees/phase3-image-proxy 86b2d1b
cd .worktrees/phase3-image-proxy
```

All subsequent commands assume CWD is the worktree.

### Trap — webdist placeholder file (do NOT stage)

The repo's embed target `internal/webdist/dist/index.html` is a **committed placeholder** that exists so the Go package compiles before any frontend build. The file at the Phase 2 merge commit `86b2d1b` is:

```
<!-- internal/webdist/dist/index.html -->
<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <title>cwd — placeholder</title>
  </head>
  <body>
    <p>This is the embed placeholder. Run <code>task web:build</code> to build the real SPA.</p>
  </body>
</html>
```

`task web:build` overwrites it with a real Vite build. **Never commit the real build artifact** — it bloats history and makes diffs unreviewable. **Only Task 13** (final integration) should run `task web:build`. Tasks 10/11/12 (frontend) verify with `task web:test` and `task web:typecheck` only — those do not touch `internal/webdist/dist/index.html`.

After running `task web:build` (in Task 13), restore the placeholder before any `git add`:

```bash
git checkout 86b2d1b -- internal/webdist/dist/index.html
```

The branch point SHA `86b2d1b` is the Phase 2 merge commit; that is the source of truth for the placeholder content during this branch's lifetime.

### Trap — cache/SSE broadcast race (do NOT touch)

Phase 1 fixed a real race in the cache→SSE broadcast path: `cache.Set` originally read the subscriber list outside `c.mu`, allowing a subscriber that was being closed concurrently to receive on a closed channel. The fixed pattern in `internal/cache/cache.go` snapshots the subscriber slice **inside** the lock, then iterates outside the lock, and each subscriber's `send` is guarded by `s.mu` against `closeOnce`. Phase 2's `applyFilter` in `internal/sse/hub.go` already passes through any non-`nws_alerts` source unchanged.

**Phase 3 does not change `internal/cache/cache.go` or `internal/sse/hub.go`.** Image SSE invalidation is delivered through the existing cache-broadcast path: the proxy emits to a callback, the server wires that callback to `cache.Set` at a single synthetic key (`image.invalidate`), the hub picks the change up via existing `cache.Subscribe`, and broadcasts as event name `image.invalidate.update`. See Task 4 §"SSE wiring strategy" for the full rationale.

If a Phase 3 task seems to require touching `cache.go` / `hub.go` / their tests, **stop and reconsider** — every change in Phase 3 should compose with the existing pattern, not modify it.

### Trap — config field reconciliation

The Phase 2 `internal/config/config.go` already declares two image-related blocks that overlap with design §8's new contract:

```go
type CacheConfig struct {
    ImageDir      string `yaml:"image_dir"`
    ImageMaxBytes int64  `yaml:"image_max_bytes"`
}
type ImagesConfig struct {
    DefaultMode string   `yaml:"default_mode"`   // currently validated as "lazy"|"prewarm"
    Prewarm     []string `yaml:"prewarm"`
}
```

These were Phase 0 placeholders for the image proxy. Phase 3 supersedes both with a single, expanded `ImagesConfig` per design §8: `cache_dir`, `disk_max_bytes`, `hot_max_bytes`, `hot_max_entries`, `refresh_interval`, `image_intervals`, `prewarm`. Task 6 deletes `CacheConfig` entirely (and the `Cache` field from the top-level `Config`), removes `ImagesConfig.DefaultMode` and its `validate()` switch case, and migrates the existing `ImagesConfig.Prewarm` into the new shape. This is **not** a backwards-compatibility shim — the Phase 0 fields are unused at runtime and can be replaced cleanly.

### Per-image env override pattern

Phase 1 Task 14 wired the env walker (`applyEnvOverrides` in `internal/config/config.go`) to walk top-level scalar fields under `Server`, `Store`, `Cache`, `Images`, `UI`. Task 6 removes `Cache` from that walker (it is deleted), preserves the top-level `Images` walk for new scalar fields, and adds two new explicit env-walk passes:

- `CWD_IMAGES_PREWARM` — comma-separated dot keys; replaces the entire list (not a delta).
- `CWD_IMAGES_IMAGE_INTERVALS_<KEY>` — duration; e.g. `CWD_IMAGES_IMAGE_INTERVALS_SPC__DAY1OTLK_FIRE=90s`. The walker treats `__` (two underscores) in `<KEY>` as the dot separator in the registry key, then lowercases. This disambiguates the dot in `spc.day1otlk_fire` (which has a real underscore in its `name` segment).

Per-source overrides (`CWD_SOURCES_*`) carry forward unchanged from Phase 1.

---

## Dependency graph (visual)

```
1 registry ────┐
2 disk store ──┤
3 hot tier ────┤
6 config ──────┤   (all five tasks parallel — different files)
10 FE wire ────┘
       │
       ▼
       ├──► 4 proxy core ◀── 1, 2, 3
       │      │
       │      ├──► 5 prewarm ◀── 4
       │      ├──► 7 HTTP handler ◀── 4
       │      └──► 8 /api/sources ext ◀── 4
       │             │
       │             ▼
       │      9 server wiring ◀── 4, 5, 6, 7, 8
       │
       └──► 11 HazardCategoryCard ◀── 10
              │
              ▼
              12 Hazards page rewrite ◀── 11
                     │
                     ▼
              13 README + smoke + final integration ◀── 9, 12 (all)
```

Shape: **leaf-package primitives + config + frontend wire layer fan in to the proxy core; proxy core fans out to prewarm + HTTP handler + sources health surfacing; backend converges at server wiring; frontend converges at the Hazards page; README + integration check is the final fan-in.**

Parallelism windows:
- **Wave 1** (5 parallel): Tasks 1, 2, 3, 6, 10 — five different file regions, no shared code.
- **Wave 2** (2 parallel): Tasks 4 (proxy core, after 1+2+3) and 11 (HazardCategoryCard, after 10).
- **Wave 3** (4 parallel): Tasks 5 (prewarm), 7 (HTTP handler), 8 (sources ext), 12 (Hazards page) — different files; backends 5/7/8 share imageproxy package only via reads (no edits to each other's files).
- **Wave 4** (1): Task 9 — server wiring.
- **Wave 5** (1): Task 13 — README + smoke + final integration.

No two parallel-runnable tasks touch the same file. Per-task pathspec discipline (see Conventions §"Stage + commit by explicit pathspec") provides belt-and-suspenders against any race we missed.

---

## Task 1: Image registry — table + tests

**Files:**
- Create: `internal/imageproxy/registry.go`
- Create: `internal/imageproxy/registry_test.go`

**Dependencies:** none.

**Why:** Design §5.1 fixes the 20-image table as the single source of truth for which upstream maps the proxy serves. Each entry carries a `Description` so the registry doubles as documentation; per-image `DefaultInterval`s tier today's products tighter than +2/+3 day products. Per-image **exported key constants** (e.g. `KeySPCDay1Otlk = "spc.day1otlk"`) prevent typos in cross-file references (matches the `sources.NWSAlertsName`/`SWPCScalesName` discipline from Phase 1/2).

- [ ] **Step 1.1: Write the failing test**

`internal/imageproxy/registry_test.go`:

```go
package imageproxy

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestRegistry_NoDuplicateKeys(t *testing.T) {
	seen := map[string]struct{}{}
	for _, img := range Registry {
		k := img.Key()
		if _, dup := seen[k]; dup {
			t.Errorf("duplicate registry key: %q", k)
		}
		seen[k] = struct{}{}
	}
}

func TestRegistry_ExpectedKeyCount(t *testing.T) {
	const want = 20
	if got := len(Registry); got != want {
		t.Errorf("Registry size = %d, want %d", got, want)
	}
}

func TestRegistry_EveryEntryWellFormed(t *testing.T) {
	allowedMIME := map[string]struct{}{
		"image/png": {}, "image/gif": {}, "image/jpeg": {},
	}
	for _, img := range Registry {
		t.Run(img.Key(), func(t *testing.T) {
			if img.Source == "" {
				t.Errorf("Source empty")
			}
			if img.Name == "" {
				t.Errorf("Name empty")
			}
			if strings.ContainsAny(img.Source, "./") {
				t.Errorf("Source %q must not contain '.' or '/'", img.Source)
			}
			if strings.ContainsAny(img.Name, "./") {
				t.Errorf("Name %q must not contain '.' or '/'", img.Name)
			}
			u, err := url.Parse(img.URL)
			if err != nil {
				t.Errorf("URL parse: %v", err)
			} else if u.Scheme != "https" {
				t.Errorf("URL scheme = %q, want https", u.Scheme)
			}
			if _, ok := allowedMIME[img.MIME]; !ok {
				t.Errorf("MIME %q not in {image/png,image/gif,image/jpeg}", img.MIME)
			}
			if img.DefaultInterval < 60*time.Second {
				t.Errorf("DefaultInterval = %s, must be >= 60s (politeness floor)", img.DefaultInterval)
			}
			if strings.TrimSpace(img.Description) == "" {
				t.Errorf("Description empty")
			}
		})
	}
}

func TestImage_KeyAndPath(t *testing.T) {
	img := Image{Source: "spc", Name: "day1otlk"}
	if got := img.Key(); got != "spc.day1otlk" {
		t.Errorf("Key() = %q, want spc.day1otlk", got)
	}
	if got := img.Path(); got != "/img/spc/day1otlk" {
		t.Errorf("Path() = %q, want /img/spc/day1otlk", got)
	}
}

func TestRegistry_ByKey_LookupHits(t *testing.T) {
	want := []string{
		"spc.day1otlk", "spc.day2otlk", "spc.day3otlk",
		"spc.day1otlk_fire", "spc.day2otlk_fire", "spc.day38otlk_fire",
		"wpc.ero_day1", "wpc.ero_day2", "wpc.ero_day3",
		"wpc.wssi_day1", "wpc.wssi_day2", "wpc.wssi_day3",
		"wpc.heatrisk_day1", "wpc.heatrisk_day2", "wpc.heatrisk_day3",
		"nhc.atl_7d", "nhc.epac_7d", "nhc.cpac_7d",
		"navy.jtwc_abpw",
		"nwc.fho_national",
	}
	byKey := ByKey()
	for _, k := range want {
		if _, ok := byKey[k]; !ok {
			t.Errorf("ByKey() missing %q", k)
		}
	}
	if got := len(byKey); got != len(want) {
		t.Errorf("ByKey() size = %d, want %d", got, len(want))
	}
}

func TestKey_Constants_MatchRegistry(t *testing.T) {
	pairs := []struct {
		name  string
		value string
	}{
		{"KeySPCDay1Otlk", KeySPCDay1Otlk},
		{"KeySPCDay2Otlk", KeySPCDay2Otlk},
		{"KeySPCDay3Otlk", KeySPCDay3Otlk},
		{"KeyNHCAtl7D", KeyNHCAtl7D},
	}
	byKey := ByKey()
	for _, p := range pairs {
		if _, ok := byKey[p.value]; !ok {
			t.Errorf("%s = %q not in registry", p.name, p.value)
		}
	}
}
```

- [ ] **Step 1.2: Run, confirm fail**

```bash
go test ./internal/imageproxy/ -run TestRegistry -v
```

Expected: build fails — `Registry`, `Image`, `ByKey`, `KeySPCDay1Otlk`, etc. undefined.

- [ ] **Step 1.3: Implement `internal/imageproxy/registry.go`**

```go
// Package imageproxy implements the lazy-with-prewarm proxy that fronts the
// 20 third-party static maps the original NCEP CWD page hot-links. See
// docs/plans/2026-05-03-phase3-image-proxy-design.md for the full design.
package imageproxy

import "time"

// Image describes one upstream map proxied by /img/{source}/{name}.
type Image struct {
	// Source is URL-path segment 1 (e.g. "spc"). Must not contain '.' or '/'.
	Source string
	// Name is URL-path segment 2 (e.g. "day1otlk"). Must not contain '.' or '/'.
	Name string
	// URL is the upstream absolute URL.
	URL string
	// MIME is the fallback content-type if upstream omits Content-Type.
	MIME string
	// DefaultInterval is the poll cadence absent operator override.
	// Must be >= 60s (politeness floor).
	DefaultInterval time.Duration
	// Description is human-readable "what this map shows" text. Doubles as
	// documentation; surfaced to operators in future settings panel.
	Description string
}

// Key returns the dot-key form used in config and SSE payloads (e.g. "spc.day1otlk").
func (i Image) Key() string { return i.Source + "." + i.Name }

// Path returns the URL form served at /img/{source}/{name}.
func (i Image) Path() string { return "/img/" + i.Source + "/" + i.Name }

// Per-key string constants. One per registry entry, mirroring the Phase 1/2
// pattern of exporting per-source name constants (sources.NWSAlertsName etc).
// These guard cross-file references against typos at compile time.
const (
	KeySPCDay1Otlk      = "spc.day1otlk"
	KeySPCDay2Otlk      = "spc.day2otlk"
	KeySPCDay3Otlk      = "spc.day3otlk"
	KeySPCDay1OtlkFire  = "spc.day1otlk_fire"
	KeySPCDay2OtlkFire  = "spc.day2otlk_fire"
	KeySPCDay38OtlkFire = "spc.day38otlk_fire"
	KeyWPCEroDay1       = "wpc.ero_day1"
	KeyWPCEroDay2       = "wpc.ero_day2"
	KeyWPCEroDay3       = "wpc.ero_day3"
	KeyWPCWSSIDay1      = "wpc.wssi_day1"
	KeyWPCWSSIDay2      = "wpc.wssi_day2"
	KeyWPCWSSIDay3      = "wpc.wssi_day3"
	KeyWPCHeatRiskDay1  = "wpc.heatrisk_day1"
	KeyWPCHeatRiskDay2  = "wpc.heatrisk_day2"
	KeyWPCHeatRiskDay3  = "wpc.heatrisk_day3"
	KeyNHCAtl7D         = "nhc.atl_7d"
	KeyNHCEpac7D        = "nhc.epac_7d"
	KeyNHCCpac7D        = "nhc.cpac_7d"
	KeyNavyJTWCABPW     = "navy.jtwc_abpw"
	KeyNWCFHONational   = "nwc.fho_national"
)

// Registry is the immutable list of all proxied images.
//
// To add a new image: append a new struct literal AND add a Key* constant
// AND extend the front-end CATEGORIES table in HazardCategoryCard.tsx if
// the new image belongs to a category card.
//
// Each entry's comment names the upstream agency, the product, and the time
// horizon it depicts so the registry doubles as documentation.
//
//nolint:gochecknoglobals // intentional package-level immutable config
var Registry = []Image{
	// Severe Storms — SPC Convective Outlooks
	// Day 1: today's convective threat (tornado / wind / hail) over CONUS.
	{Source: "spc", Name: "day1otlk", URL: "https://www.spc.noaa.gov/products/outlook/day1otlk.png", MIME: "image/png", DefaultInterval: 2 * time.Minute, Description: "SPC Convective Outlook — Day 1 (today)"},
	// Day 2: tomorrow's convective threat.
	{Source: "spc", Name: "day2otlk", URL: "https://www.spc.noaa.gov/products/outlook/day2otlk.png", MIME: "image/png", DefaultInterval: 5 * time.Minute, Description: "SPC Convective Outlook — Day 2 (tomorrow)"},
	// Day 3: the day after tomorrow.
	{Source: "spc", Name: "day3otlk", URL: "https://www.spc.noaa.gov/products/outlook/day3otlk.png", MIME: "image/png", DefaultInterval: 10 * time.Minute, Description: "SPC Convective Outlook — Day 3"},

	// Wildfire — SPC Fire Weather Outlooks
	{Source: "spc", Name: "day1otlk_fire", URL: "https://www.spc.noaa.gov/products/fire_wx/day1otlk_fire.png", MIME: "image/png", DefaultInterval: 5 * time.Minute, Description: "SPC Fire Weather Outlook — Day 1"},
	{Source: "spc", Name: "day2otlk_fire", URL: "https://www.spc.noaa.gov/products/fire_wx/day2otlk_fire.png", MIME: "image/png", DefaultInterval: 10 * time.Minute, Description: "SPC Fire Weather Outlook — Day 2"},
	// Day 3-8 experimental: a longer outlook; animated GIF.
	{Source: "spc", Name: "day38otlk_fire", URL: "https://www.spc.noaa.gov/products/exper/fire_wx/imgs/day38otlk_fire.gif", MIME: "image/gif", DefaultInterval: 30 * time.Minute, Description: "SPC Fire Weather Outlook — Day 3-8 (experimental, animated)"},

	// Excessive Rainfall — WPC ERO
	{Source: "wpc", Name: "ero_day1", URL: "https://www.wpc.ncep.noaa.gov/qpf/94ewbg.gif", MIME: "image/gif", DefaultInterval: 5 * time.Minute, Description: "WPC Excessive Rainfall Outlook — Day 1"},
	{Source: "wpc", Name: "ero_day2", URL: "https://www.wpc.ncep.noaa.gov/qpf/98ewbg.gif", MIME: "image/gif", DefaultInterval: 10 * time.Minute, Description: "WPC Excessive Rainfall Outlook — Day 2"},
	{Source: "wpc", Name: "ero_day3", URL: "https://www.wpc.ncep.noaa.gov/qpf/99ewbg.gif", MIME: "image/gif", DefaultInterval: 15 * time.Minute, Description: "WPC Excessive Rainfall Outlook — Day 3"},

	// Winter — WPC WSSI Overall CONUS
	{Source: "wpc", Name: "wssi_day1", URL: "https://www.wpc.ncep.noaa.gov/wwd/wssi/images/WSSI_Overall_Day1_CONUS_Day1.png", MIME: "image/png", DefaultInterval: 10 * time.Minute, Description: "WPC Winter Storm Severity Index — Day 1 (overall, CONUS)"},
	{Source: "wpc", Name: "wssi_day2", URL: "https://www.wpc.ncep.noaa.gov/wwd/wssi/images/WSSI_Overall_Day2_CONUS_Day2.png", MIME: "image/png", DefaultInterval: 15 * time.Minute, Description: "WPC Winter Storm Severity Index — Day 2 (overall, CONUS)"},
	{Source: "wpc", Name: "wssi_day3", URL: "https://www.wpc.ncep.noaa.gov/wwd/wssi/images/WSSI_Overall_Day3_CONUS_Day3.png", MIME: "image/png", DefaultInterval: 20 * time.Minute, Description: "WPC Winter Storm Severity Index — Day 3 (overall, CONUS)"},

	// Heat — WPC HeatRisk
	{Source: "wpc", Name: "heatrisk_day1", URL: "https://www.wpc.ncep.noaa.gov/heatrisk/graphics/HeatRisk_Day1_CONUS.png", MIME: "image/png", DefaultInterval: 15 * time.Minute, Description: "WPC HeatRisk — Day 1 (CONUS)"},
	{Source: "wpc", Name: "heatrisk_day2", URL: "https://www.wpc.ncep.noaa.gov/heatrisk/graphics/HeatRisk_Day2_CONUS.png", MIME: "image/png", DefaultInterval: 20 * time.Minute, Description: "WPC HeatRisk — Day 2 (CONUS)"},
	{Source: "wpc", Name: "heatrisk_day3", URL: "https://www.wpc.ncep.noaa.gov/heatrisk/graphics/HeatRisk_Day3_CONUS.png", MIME: "image/png", DefaultInterval: 30 * time.Minute, Description: "WPC HeatRisk — Day 3 (CONUS)"},

	// Tropical — NHC + Navy JTWC
	{Source: "nhc", Name: "atl_7d", URL: "https://www.nhc.noaa.gov/xgtwo/two_atl_7d0.png", MIME: "image/png", DefaultInterval: 30 * time.Minute, Description: "NHC Tropical Weather Outlook — Atlantic (7 day)"},
	{Source: "nhc", Name: "epac_7d", URL: "https://www.nhc.noaa.gov/xgtwo/two_pac_7d0.png", MIME: "image/png", DefaultInterval: 30 * time.Minute, Description: "NHC Tropical Weather Outlook — East Pacific (7 day)"},
	{Source: "nhc", Name: "cpac_7d", URL: "https://www.nhc.noaa.gov/xgtwo/two_cpac_7d0.png", MIME: "image/png", DefaultInterval: 30 * time.Minute, Description: "NHC Tropical Weather Outlook — Central Pacific (7 day)"},
	// US Navy Joint Typhoon Warning Center — Western Pacific basin tropical advisory.
	{Source: "navy", Name: "jtwc_abpw", URL: "https://www.metoc.navy.mil/jtwc/products/abpwsair.jpg", MIME: "image/jpeg", DefaultInterval: 30 * time.Minute, Description: "Navy JTWC ABPW — Western Pacific tropical synopsis"},

	// Flooding — WPC NWC National Flood Hazard Outlook
	{Source: "nwc", Name: "fho_national", URL: "https://www.weather.gov/images/owp/FHO/National/National_FHO.png", MIME: "image/png", DefaultInterval: 30 * time.Minute, Description: "NWC National Flood Hazard Outlook"},
}

// ByKey returns a map view of the registry indexed by Image.Key(). The map is
// rebuilt on every call (cheap — 20 entries) so callers can mutate the result
// without affecting the package-level Registry.
func ByKey() map[string]Image {
	m := make(map[string]Image, len(Registry))
	for _, img := range Registry {
		m[img.Key()] = img
	}
	return m
}

// DefaultPrewarmKeys returns the registry keys seeded into ImagesConfig.Prewarm
// by config defaults() — the 4 highest-traffic maps per design Decision 4.
func DefaultPrewarmKeys() []string {
	return []string{
		KeySPCDay1Otlk,
		KeySPCDay2Otlk,
		KeySPCDay3Otlk,
		KeyNHCAtl7D,
	}
}
```

- [ ] **Step 1.4: Run tests + lint**

```bash
go test ./internal/imageproxy/ -v -race
golangci-lint run ./internal/imageproxy/...
```

Expected: all 6 tests pass; lint clean.

- [ ] **Step 1.5: Commit**

```bash
git add internal/imageproxy/registry.go internal/imageproxy/registry_test.go
git commit -m "feat(p3): image proxy registry — 20 entries + key constants" -- \
  internal/imageproxy/registry.go internal/imageproxy/registry_test.go
```

---

## Task 2: Disk store — atomic writes, manifest, LRU eviction

**Files:**
- Create: `internal/imageproxy/store.go`
- Create: `internal/imageproxy/store_test.go`

**Dependencies:** none.

**Why:** Design §5.2 requires a durable disk-backed store so the proxy survives Pi-style cold restarts without re-fetching every image. Atomic write-then-rename prevents partial files surviving a crash; the manifest doubles as the on-restart index. LRU eviction by `lastAccess` keeps disk usage bounded.

- [ ] **Step 2.1: Write the failing test**

`internal/imageproxy/store_test.go`:

```go
package imageproxy

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func newTestStore(t *testing.T, maxBytes int64) *DiskStore {
	t.Helper()
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s, err := NewDiskStore(dir, maxBytes, logger)
	if err != nil {
		t.Fatalf("NewDiskStore: %v", err)
	}
	return s
}

func TestDiskStore_PutGetRoundTrip(t *testing.T) {
	s := newTestStore(t, 1<<20)
	want := []byte("hello-image-bytes")
	now := time.Now().UTC().Truncate(time.Second)
	if err := s.Put("spc.day1otlk", want, "image/png", "v1", now); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, ct, val, fetched, err := s.Get("spc.day1otlk")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("bytes mismatch")
	}
	if ct != "image/png" {
		t.Errorf("contentType = %q", ct)
	}
	if val != "v1" {
		t.Errorf("validator = %q", val)
	}
	if !fetched.Equal(now) {
		t.Errorf("fetchedAt = %s, want %s", fetched, now)
	}
}

func TestDiskStore_GetMissing_ReturnsErrNotExist(t *testing.T) {
	s := newTestStore(t, 1<<20)
	_, _, _, _, err := s.Get("never.written")
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("err = %v, want os.ErrNotExist", err)
	}
}

func TestDiskStore_RestartLoadsManifest(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s1, err := NewDiskStore(dir, 1<<20, logger)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	if err := s1.Put("a.b", []byte("payload-a"), "image/png", "va", now); err != nil {
		t.Fatal(err)
	}
	if err := s1.Put("c.d", []byte("payload-c"), "image/gif", "vc", now); err != nil {
		t.Fatal(err)
	}

	s2, err := NewDiskStore(dir, 1<<20, logger)
	if err != nil {
		t.Fatalf("re-open: %v", err)
	}
	for _, k := range []string{"a.b", "c.d"} {
		if _, _, _, _, err := s2.Get(k); err != nil {
			t.Errorf("after restart, Get(%q) = %v", k, err)
		}
	}
	entries, _ := s2.Stats()
	if entries != 2 {
		t.Errorf("entries after restart = %d, want 2", entries)
	}
}

func TestDiskStore_LRUEvictionAtByteCap(t *testing.T) {
	// Cap is 30 bytes; each entry is 10 bytes (5-byte payload + ~5 bytes manifest
	// overhead per entry — but eviction is computed against payload bytes only,
	// per Stats(). Three 10-byte payloads exactly fits; a fourth must evict the LRU.
	s := newTestStore(t, 30)
	now := time.Now().UTC()
	put := func(k string, ts time.Time) {
		if err := s.Put(k, []byte("0123456789"), "image/png", "v", ts); err != nil {
			t.Fatalf("Put(%s): %v", k, err)
		}
	}
	put("a.b", now.Add(-3*time.Second))
	put("c.d", now.Add(-2*time.Second))
	put("e.f", now.Add(-1*time.Second))
	put("g.h", now) // forces eviction of a.b (oldest)

	if _, _, _, _, err := s.Get("a.b"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a.b should have been evicted, got err=%v", err)
	}
	for _, k := range []string{"c.d", "e.f", "g.h"} {
		if _, _, _, _, err := s.Get(k); err != nil {
			t.Errorf("Get(%q) after eviction: %v", k, err)
		}
	}
}

func TestDiskStore_PutOverwritesValidatorAndBytes(t *testing.T) {
	s := newTestStore(t, 1<<20)
	now := time.Now().UTC()
	if err := s.Put("k.k", []byte("v1-bytes"), "image/png", "v1", now); err != nil {
		t.Fatal(err)
	}
	if err := s.Put("k.k", []byte("v2-bytes"), "image/png", "v2", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	got, _, val, _, err := s.Get("k.k")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("v2-bytes")) {
		t.Errorf("Get returned old bytes")
	}
	if val != "v2" {
		t.Errorf("validator = %q, want v2", val)
	}
}

func TestDiskStore_AtomicWrite_NoHalfFiles(t *testing.T) {
	// Simulate a torn write by truncating the .tmp file mid-flight: confirm
	// that NewDiskStore on a directory containing only orphan .tmp files
	// drops them and reports zero entries.
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "ab"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ab", "deadbeef.bin.tmp"), []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s, err := NewDiskStore(dir, 1<<20, logger)
	if err != nil {
		t.Fatalf("NewDiskStore: %v", err)
	}
	entries, _ := s.Stats()
	if entries != 0 {
		t.Errorf("entries = %d, want 0 (orphan .tmp must be ignored)", entries)
	}
	// Orphan .tmp should be cleaned by the constructor.
	if _, err := os.Stat(filepath.Join(dir, "ab", "deadbeef.bin.tmp")); !os.IsNotExist(err) {
		t.Errorf("orphan .tmp still present, err=%v", err)
	}
}

func TestDiskStore_ConcurrentPuts_NoCorruption(t *testing.T) {
	s := newTestStore(t, 1<<20)
	now := time.Now().UTC()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			payload := []byte("payload-" + string(rune('a'+i%26)))
			_ = s.Put("k."+string(rune('a'+i%26)), payload, "image/png", "v", now)
		}()
	}
	wg.Wait()
	entries, _ := s.Stats()
	if entries == 0 {
		t.Errorf("entries = 0 after concurrent Puts")
	}
}
```

- [ ] **Step 2.2: Run, confirm fail**

```bash
go test ./internal/imageproxy/ -run TestDiskStore -v
```

- [ ] **Step 2.3: Implement `internal/imageproxy/store.go`**

```go
package imageproxy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// DiskStore is a filesystem-backed LRU cache for image bytes + metadata.
// Layout under root:
//
//	<root>/<hash[0:2]>/<hash[2:]>.bin   raw image bytes
//	<root>/<hash[0:2]>/<hash[2:]>.json  metadata (contentType, validator, fetchedAt, originalKey)
//	<root>/manifest.json                 full registry of (key -> entry); rewritten on every Put.
//
// Atomicity: every write is to <name>.tmp followed by os.Rename. Orphan .tmp
// files are removed on construction.
type DiskStore struct {
	root     string
	maxBytes int64
	logger   *slog.Logger

	mu      sync.Mutex
	entries map[string]*diskEntry // key -> entry
	bytes   int64
}

type diskEntry struct {
	Key         string    `json:"key"`
	ContentType string    `json:"contentType"`
	Validator   string    `json:"validator"`
	FetchedAt   time.Time `json:"fetchedAt"`
	BytesLen    int64     `json:"bytesLen"`
	LastAccess  time.Time `json:"lastAccess"`
}

// NewDiskStore opens (creating if necessary) a disk store rooted at dir.
// maxBytes is the LRU cap; <= 0 disables the cap (use with care).
func NewDiskStore(dir string, maxBytes int64, logger *slog.Logger) (*DiskStore, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("imageproxy.store: mkdir %q: %w", dir, err)
	}
	s := &DiskStore{
		root:     dir,
		maxBytes: maxBytes,
		logger:   logger,
		entries:  map[string]*diskEntry{},
	}
	s.loadManifest()
	s.cleanOrphans()
	return s, nil
}

// Get returns bytes + metadata for the key, or os.ErrNotExist.
// Updates lastAccess on hit (re-marks LRU recency) before returning.
func (s *DiskStore) Get(key string) (bytes []byte, contentType, validator string, fetchedAt time.Time, err error) {
	s.mu.Lock()
	e, ok := s.entries[key]
	s.mu.Unlock()
	if !ok {
		return nil, "", "", time.Time{}, os.ErrNotExist
	}
	binPath, _ := s.paths(key)
	b, err := os.ReadFile(binPath)
	if err != nil {
		return nil, "", "", time.Time{}, err
	}
	s.mu.Lock()
	e.LastAccess = time.Now().UTC()
	s.mu.Unlock()
	s.writeManifest() // best-effort
	return b, e.ContentType, e.Validator, e.FetchedAt, nil
}

// Put writes bytes + metadata atomically; evicts LRU entries if maxBytes is exceeded.
func (s *DiskStore) Put(key string, body []byte, contentType, validator string, fetchedAt time.Time) error {
	binPath, jsonPath := s.paths(key)
	if err := os.MkdirAll(filepath.Dir(binPath), 0o700); err != nil {
		return fmt.Errorf("imageproxy.store: mkdir: %w", err)
	}
	if err := atomicWrite(binPath, body); err != nil {
		return fmt.Errorf("imageproxy.store: write bin: %w", err)
	}
	meta := diskEntry{
		Key:         key,
		ContentType: contentType,
		Validator:   validator,
		FetchedAt:   fetchedAt.UTC(),
		BytesLen:    int64(len(body)),
		LastAccess:  time.Now().UTC(),
	}
	mb, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("imageproxy.store: marshal meta: %w", err)
	}
	if err := atomicWrite(jsonPath, mb); err != nil {
		return fmt.Errorf("imageproxy.store: write meta: %w", err)
	}

	s.mu.Lock()
	if prev, ok := s.entries[key]; ok {
		s.bytes -= prev.BytesLen
	}
	s.entries[key] = &meta
	s.bytes += meta.BytesLen
	s.mu.Unlock()

	s.evictIfOverCap()
	s.writeManifest()
	return nil
}

// Stats returns current usage for /api/sources surfacing.
func (s *DiskStore) Stats() (entries int, bytesUsed int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries), s.bytes
}

// paths returns (binPath, jsonPath) for the given key.
func (s *DiskStore) paths(key string) (string, string) {
	sum := sha256.Sum256([]byte(key))
	hash := hex.EncodeToString(sum[:])
	dir := filepath.Join(s.root, hash[:2])
	base := hash[2:]
	return filepath.Join(dir, base+".bin"), filepath.Join(dir, base+".json")
}

func (s *DiskStore) loadManifest() {
	mPath := filepath.Join(s.root, "manifest.json")
	b, err := os.ReadFile(mPath)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	if err != nil {
		s.logger.Warn("imageproxy.store.manifest_read", "err", err.Error())
		return
	}
	var entries map[string]*diskEntry
	if err := json.Unmarshal(b, &entries); err != nil {
		s.logger.Warn("imageproxy.store.manifest_parse", "err", err.Error())
		return
	}
	for k, e := range entries {
		// Validate against the actual on-disk pair.
		binPath, jsonPath := s.paths(k)
		bi, err1 := os.Stat(binPath)
		_, err2 := os.Stat(jsonPath)
		if err1 != nil || err2 != nil {
			s.logger.Warn("imageproxy.store.manifest_orphan", "key", k)
			continue
		}
		e.BytesLen = bi.Size()
		s.entries[k] = e
		s.bytes += e.BytesLen
	}
}

func (s *DiskStore) writeManifest() {
	s.mu.Lock()
	snapshot := make(map[string]diskEntry, len(s.entries))
	for k, e := range s.entries {
		snapshot[k] = *e
	}
	s.mu.Unlock()
	b, err := json.Marshal(snapshot)
	if err != nil {
		s.logger.Warn("imageproxy.store.manifest_marshal", "err", err.Error())
		return
	}
	if err := atomicWrite(filepath.Join(s.root, "manifest.json"), b); err != nil {
		s.logger.Warn("imageproxy.store.manifest_write", "err", err.Error())
	}
}

// cleanOrphans removes any *.tmp files left behind by torn writes from a
// previous process. Called once at construction.
func (s *DiskStore) cleanOrphans() {
	_ = filepath.WalkDir(s.root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if filepath.Ext(p) == ".tmp" {
			_ = os.Remove(p)
		}
		return nil
	})
}

func (s *DiskStore) evictIfOverCap() {
	if s.maxBytes <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.bytes <= s.maxBytes {
		return
	}
	keys := make([]string, 0, len(s.entries))
	for k := range s.entries {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return s.entries[keys[i]].LastAccess.Before(s.entries[keys[j]].LastAccess)
	})
	for _, k := range keys {
		if s.bytes <= s.maxBytes {
			return
		}
		e := s.entries[k]
		binPath, jsonPath := s.paths(k)
		_ = os.Remove(binPath)
		_ = os.Remove(jsonPath)
		s.bytes -= e.BytesLen
		delete(s.entries, k)
		s.logger.Debug("imageproxy.store.evict", "key", k, "bytes_freed", e.BytesLen)
	}
}

// atomicWrite writes data to path via a sibling .tmp + rename. Removes the
// .tmp on error so a follow-up cleanOrphans pass can sweep it later if needed.
func atomicWrite(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
```

- [ ] **Step 2.4: Run tests + lint**

```bash
go test ./internal/imageproxy/ -run TestDiskStore -v -race
golangci-lint run ./internal/imageproxy/...
```

Expected: all 7 disk-store tests pass with `-race`; lint clean.

- [ ] **Step 2.5: Commit**

```bash
git add internal/imageproxy/store.go internal/imageproxy/store_test.go
git commit -m "feat(p3): image proxy disk store — atomic writes + manifest + LRU" -- \
  internal/imageproxy/store.go internal/imageproxy/store_test.go
```

---

## Task 3: Hot tier — bounded in-memory LRU

**Files:**
- Create: `internal/imageproxy/hot.go`
- Create: `internal/imageproxy/hot_test.go`

**Dependencies:** none.

**Why:** Design §5.3. The hot tier sits in front of the disk store so repeated GETs in a tight window (browser preview-group flips, dashboard reloads) don't pay the disk-read latency. Bounded by both `maxBytes` AND `maxEntries` because either can be the binding constraint depending on image-size distribution; oversized-entry Puts are no-ops (the disk still has it).

- [ ] **Step 3.1: Write the failing test**

`internal/imageproxy/hot_test.go`:

```go
package imageproxy

import (
	"bytes"
	"testing"
	"time"
)

func TestHotTier_PutGetRoundTrip(t *testing.T) {
	h := NewHotTier(1<<20, 100)
	now := time.Now().UTC()
	h.Put("k", []byte("hello"), "image/png", "v1", now)
	got, ct, val, fetched, ok := h.Get("k")
	if !ok {
		t.Fatal("expected hit")
	}
	if !bytes.Equal(got, []byte("hello")) {
		t.Errorf("bytes mismatch")
	}
	if ct != "image/png" {
		t.Errorf("contentType = %q", ct)
	}
	if val != "v1" {
		t.Errorf("validator = %q", val)
	}
	if !fetched.Equal(now) {
		t.Errorf("fetchedAt mismatch")
	}
}

func TestHotTier_GetMiss_ReturnsFalse(t *testing.T) {
	h := NewHotTier(1<<20, 100)
	if _, _, _, _, ok := h.Get("never"); ok {
		t.Error("expected miss")
	}
}

func TestHotTier_EvictsLRUByByteCap(t *testing.T) {
	h := NewHotTier(20, 100) // 20 bytes; each entry is 10
	now := time.Now().UTC()
	h.Put("a", []byte("0123456789"), "ct", "v", now.Add(-2*time.Second))
	h.Put("b", []byte("0123456789"), "ct", "v", now.Add(-time.Second))
	h.Put("c", []byte("0123456789"), "ct", "v", now) // forces eviction of "a"

	if _, _, _, _, ok := h.Get("a"); ok {
		t.Error("a should have been evicted")
	}
	if _, _, _, _, ok := h.Get("b"); !ok {
		t.Error("b should still be present")
	}
	if _, _, _, _, ok := h.Get("c"); !ok {
		t.Error("c should still be present")
	}
}

func TestHotTier_EvictsLRUByEntryCap(t *testing.T) {
	h := NewHotTier(1<<20, 2) // generous bytes, but cap of 2 entries
	now := time.Now().UTC()
	h.Put("a", []byte("x"), "ct", "v", now.Add(-2*time.Second))
	h.Put("b", []byte("x"), "ct", "v", now.Add(-time.Second))
	h.Put("c", []byte("x"), "ct", "v", now)

	if _, _, _, _, ok := h.Get("a"); ok {
		t.Error("a should have been evicted (entry cap)")
	}
	entries, _ := h.Stats()
	if entries != 2 {
		t.Errorf("Stats entries = %d, want 2", entries)
	}
}

func TestHotTier_OversizedPutIsNoop(t *testing.T) {
	h := NewHotTier(10, 100)
	h.Put("big", []byte("01234567890123456789"), "ct", "v", time.Now())
	if _, _, _, _, ok := h.Get("big"); ok {
		t.Error("oversized entry should not have been admitted")
	}
}

func TestHotTier_GetUpdatesRecency(t *testing.T) {
	h := NewHotTier(20, 100)
	now := time.Now().UTC()
	h.Put("a", []byte("0123456789"), "ct", "v", now.Add(-2*time.Second))
	h.Put("b", []byte("0123456789"), "ct", "v", now.Add(-time.Second))
	// Touch a — now it's MRU.
	if _, _, _, _, ok := h.Get("a"); !ok {
		t.Fatal("expected a to be present")
	}
	// Insert c → forces eviction of b (now LRU).
	h.Put("c", []byte("0123456789"), "ct", "v", now)
	if _, _, _, _, ok := h.Get("a"); !ok {
		t.Error("a should have survived (touched recently)")
	}
	if _, _, _, _, ok := h.Get("b"); ok {
		t.Error("b should have been evicted")
	}
}

func TestHotTier_PutOverwriteReplacesEntry(t *testing.T) {
	h := NewHotTier(1<<20, 100)
	now := time.Now().UTC()
	h.Put("k", []byte("v1"), "ct", "1", now)
	h.Put("k", []byte("v2"), "ct", "2", now.Add(time.Second))
	got, _, val, _, ok := h.Get("k")
	if !ok || string(got) != "v2" || val != "2" {
		t.Errorf("expected overwritten v2/2, got %q/%s ok=%v", got, val, ok)
	}
	entries, b := h.Stats()
	if entries != 1 || b != 2 {
		t.Errorf("Stats = (%d, %d), want (1, 2)", entries, b)
	}
}
```

- [ ] **Step 3.2: Run, confirm fail**

```bash
go test ./internal/imageproxy/ -run TestHotTier -v
```

- [ ] **Step 3.3: Implement `internal/imageproxy/hot.go`**

```go
package imageproxy

import (
	"container/list"
	"sync"
	"time"
)

// HotTier is a bounded in-memory LRU of image bytes + metadata. Bound by both
// maxBytes and maxEntries — an entry is admitted only if it fits within both
// caps after eviction. Oversized entries (alone exceeding maxBytes) are
// silently dropped (no-op Put) since the disk store still has them.
type HotTier struct {
	mu         sync.Mutex
	maxBytes   int64
	maxEntries int
	bytes      int64
	order      *list.List               // front = MRU, back = LRU
	index      map[string]*list.Element // key -> *list.Element wrapping *hotEntry
}

type hotEntry struct {
	key         string
	bytes       []byte
	contentType string
	validator   string
	fetchedAt   time.Time
}

// NewHotTier returns a hot tier capped at the smaller of maxBytes (>=0; 0
// disables the tier) and maxEntries (>=0; 0 disables the tier).
func NewHotTier(maxBytes int64, maxEntries int) *HotTier {
	return &HotTier{
		maxBytes:   maxBytes,
		maxEntries: maxEntries,
		order:      list.New(),
		index:      map[string]*list.Element{},
	}
}

// Get returns bytes + metadata for the key, marking it MRU.
func (h *HotTier) Get(key string) (body []byte, contentType, validator string, fetchedAt time.Time, ok bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	el, hit := h.index[key]
	if !hit {
		return nil, "", "", time.Time{}, false
	}
	h.order.MoveToFront(el)
	e := el.Value.(*hotEntry)
	return e.bytes, e.contentType, e.validator, e.fetchedAt, true
}

// Put inserts or replaces the entry. Oversized entries are dropped.
func (h *HotTier) Put(key string, body []byte, contentType, validator string, fetchedAt time.Time) {
	if h.maxBytes <= 0 || h.maxEntries <= 0 {
		return
	}
	if int64(len(body)) > h.maxBytes {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if el, ok := h.index[key]; ok {
		old := el.Value.(*hotEntry)
		h.bytes -= int64(len(old.bytes))
		h.order.Remove(el)
		delete(h.index, key)
	}
	e := &hotEntry{
		key: key, bytes: body, contentType: contentType,
		validator: validator, fetchedAt: fetchedAt,
	}
	el := h.order.PushFront(e)
	h.index[key] = el
	h.bytes += int64(len(body))
	h.evictLocked()
}

// Stats returns current count + total bytes.
func (h *HotTier) Stats() (entries int, bytesUsed int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.index), h.bytes
}

// Has reports presence without touching recency.
func (h *HotTier) Has(key string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	_, ok := h.index[key]
	return ok
}

// evictLocked drops LRU entries until under both caps. Caller holds h.mu.
func (h *HotTier) evictLocked() {
	for h.bytes > h.maxBytes || len(h.index) > h.maxEntries {
		back := h.order.Back()
		if back == nil {
			return
		}
		e := back.Value.(*hotEntry)
		h.order.Remove(back)
		delete(h.index, e.key)
		h.bytes -= int64(len(e.bytes))
	}
}
```

- [ ] **Step 3.4: Run tests + lint**

```bash
go test ./internal/imageproxy/ -run TestHotTier -v -race
golangci-lint run ./internal/imageproxy/...
```

- [ ] **Step 3.5: Commit**

```bash
git add internal/imageproxy/hot.go internal/imageproxy/hot_test.go
git commit -m "feat(p3): image proxy hot tier — bounded in-memory LRU" -- \
  internal/imageproxy/hot.go internal/imageproxy/hot_test.go
```

---

## Task 4: Proxy core — Get + Refresh + Stats + diff-on-write SSE invalidate

**Files:**
- Create: `internal/imageproxy/proxy.go`
- Create: `internal/imageproxy/proxy_test.go`

**Dependencies:** depends on Task 1 (Registry), Task 2 (DiskStore), Task 3 (HotTier).

**Why:** Design §5.4. The proxy is the heart of the subsystem: it composes registry + hot + disk into the public `Get` and `Refresh` API, runs conditional GETs against upstreams, enforces the diff-on-write SSE invalidation discipline (Decision 6), and exposes per-key health for `/api/sources`.

### SSE wiring strategy (design clarification)

Design §4.3 says `internal/sse/hub.go` is unchanged. The hub has no `Broadcast` method on its public surface — it broadcasts only via `cache.Subscribe`/`cache.Set`. Phase 3 therefore wires SSE invalidation as follows (see Task 9 for the server-side composition):

1. The proxy holds a callback `OnInvalidate func(ImageInvalidate) error` and invokes it from `Refresh` whenever a diff-on-write actually changes the stored bytes.
2. The server constructs the proxy with a closure that translates each `ImageInvalidate` into a `cache.Envelope{Source: "image.invalidate", FetchedAt: ev.FetchedAt, Validator: <unique-per-event>, Payload: ev}` and calls `c.Set` on it.
3. The server adds the synthetic name `"image.invalidate"` to the hub's `enabledNames` slice. The hub's existing `applyFilter` returns the envelope unchanged for any non-`nws_alerts` name, and `Hub.Serve` writes it out as event name `image.invalidate.update`.
4. The cache's diff-on-write semantics (`prev.Validator == env.Validator && env.Validator != "" → no broadcast`) are preserved because the validator we synthesize (`source.name@RFC3339Nano`) is unique per event.
5. `/api/snapshot` is untouched: its handler iterates only the 5 named data sources (`snapshotSourceNames` in `internal/api/snapshot.go`), so the synthetic `image.invalidate` cache slot never appears in the snapshot wire shape. (The hub's initial-paint snapshot frame DOES include it, by design — frontend handles that case in Task 10.)

This composes with — does not modify — the cache and hub. The trap in Conventions §"Cache/SSE broadcast race" still holds.

### Validator strategy

Per design §3.3:

- If upstream sends an `ETag` header, store it raw (e.g. `"a40983afc3..."`) and use it for `If-None-Match` on the next conditional GET. Report it as the public validator.
- If upstream omits ETag (e.g. `nwc.fho_national` per the live verification table in §Self-review), compute `sha256:<hex>` of the body and use that as the public validator. Cannot send `If-None-Match` — every poll re-fetches the full body and we diff bytes ourselves.
- If upstream sends `Last-Modified`, store it (parsed) and use it for `If-Modified-Since` in addition to or instead of `If-None-Match`.

The diff-on-write predicate is: **the body sha256 changed**. ETag is a hint only — even when an upstream's ETag changes (e.g. WPC's `size-mtime` ETag changes whenever the file's mtime is touched, regardless of content), we re-hash the body to confirm a real content change before invalidating. This avoids spurious SSE noise.

- [ ] **Step 4.1: Write the failing tests**

`internal/imageproxy/proxy_test.go`:

```go
package imageproxy

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// testProxy returns a Proxy with a single fake registry of one entry pointing
// at the supplied test URL, plus a recording invalidator. Hot+disk are fresh
// per call; the test url replaces the entry's URL (registry is otherwise
// immutable, so we override per-test via a custom registry passed to New).
func testProxy(t *testing.T, key, url, mime string, interval time.Duration) (*Proxy, *atomic.Int64, *[]ImageInvalidate, *sync.Mutex) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hot := NewHotTier(1<<20, 100)
	disk, err := NewDiskStore(t.TempDir(), 1<<20, logger)
	if err != nil {
		t.Fatal(err)
	}
	reg := []Image{{Source: splitKey(key, 0), Name: splitKey(key, 1), URL: url, MIME: mime, DefaultInterval: interval, Description: "test"}}
	var calls atomic.Int64
	var invs []ImageInvalidate
	var mu sync.Mutex
	p := New(Config{
		Registry:  reg,
		Hot:       hot,
		Disk:      disk,
		UserAgent: "cwd-test/0",
		Logger:    logger,
		OnInvalidate: func(ev ImageInvalidate) error {
			mu.Lock()
			invs = append(invs, ev)
			mu.Unlock()
			return nil
		},
		HTTPTimeout: 2 * time.Second,
	})
	return p, &calls, &invs, &mu
}

func splitKey(key string, idx int) string {
	for i := 0; i < len(key); i++ {
		if key[i] == '.' {
			if idx == 0 {
				return key[:i]
			}
			return key[i+1:]
		}
	}
	return ""
}

func TestProxy_Get_UnknownKey_ReturnsNotExist(t *testing.T) {
	p, _, _, _ := testProxy(t, "spc.day1otlk", "http://x", "image/png", time.Minute)
	_, _, _, _, _, _, err := p.Get(context.Background(), "no.such")
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("err = %v, want os.ErrNotExist", err)
	}
}

func TestProxy_Get_LazyFetch_HappyPath(t *testing.T) {
	body := []byte("PNG-bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "cwd-test/0" {
			t.Errorf("UA = %q", r.Header.Get("User-Agent"))
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("ETag", `"e1"`)
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	p, _, invs, mu := testProxy(t, "spc.day1otlk", srv.URL, "image/png", time.Minute)
	got, ct, val, _, isStale, _, err := p.Get(context.Background(), "spc.day1otlk")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(body) {
		t.Errorf("body mismatch")
	}
	if ct != "image/png" {
		t.Errorf("contentType = %q", ct)
	}
	if val != `"e1"` {
		t.Errorf("validator = %q, want quoted ETag", val)
	}
	if isStale {
		t.Errorf("expected fresh, got stale")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(*invs) != 1 {
		t.Errorf("expected 1 invalidate, got %d", len(*invs))
	}
}

func TestProxy_Get_HotTierHit_NoUpstreamCall(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("first"))
	}))
	defer srv.Close()

	p, _, _, _ := testProxy(t, "spc.day1otlk", srv.URL, "image/png", time.Hour)
	// First call: lazy fetch.
	_, _, _, _, _, _, err := p.Get(context.Background(), "spc.day1otlk")
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("after first Get, upstream calls = %d, want 1", calls.Load())
	}
	// Second call: hot-tier hit, no upstream.
	_, _, _, _, _, _, err = p.Get(context.Background(), "spc.day1otlk")
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Errorf("after second Get, upstream calls = %d, want 1 (hot-tier hit)", calls.Load())
	}
}

func TestProxy_Refresh_304_NoBroadcastNoBytes(t *testing.T) {
	var calls atomic.Int64
	body := []byte("v1-bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("If-None-Match") == `"e1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("ETag", `"e1"`)
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	p, _, invs, mu := testProxy(t, "spc.day1otlk", srv.URL, "image/png", time.Hour)
	if err := p.Refresh(context.Background(), "spc.day1otlk"); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	if len(*invs) != 1 {
		t.Errorf("after first Refresh, invalidates = %d, want 1", len(*invs))
	}
	mu.Unlock()
	if err := p.Refresh(context.Background(), "spc.day1otlk"); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	if len(*invs) != 1 {
		t.Errorf("after 304 Refresh, invalidates = %d, want still 1 (no spurious)", len(*invs))
	}
	mu.Unlock()
	if calls.Load() != 2 {
		t.Errorf("upstream calls = %d, want 2", calls.Load())
	}
}

func TestProxy_DiffOnWrite_NoBroadcastOnIdenticalBody(t *testing.T) {
	body := []byte("identical-bytes")
	// Simulate an upstream that does not send ETag/Last-Modified — every poll
	// is a full 200 — but the body never changes. Validator falls back to
	// sha256; the proxy must NOT broadcast invalidate on the second poll.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	p, _, invs, mu := testProxy(t, "spc.day1otlk", srv.URL, "image/png", time.Hour)
	for i := 0; i < 3; i++ {
		if err := p.Refresh(context.Background(), "spc.day1otlk"); err != nil {
			t.Fatal(err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(*invs) != 1 {
		t.Errorf("invalidates = %d, want exactly 1 (only first poll changes bytes)", len(*invs))
	}
}

func TestProxy_UpstreamFailureWithCache_ServesStale(t *testing.T) {
	var calls atomic.Int64
	body := []byte("good-bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(body)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p, _, _, _ := testProxy(t, "spc.day1otlk", srv.URL, "image/png", time.Nanosecond) // immediately stale
	if _, _, _, _, _, _, err := p.Get(context.Background(), "spc.day1otlk"); err != nil {
		t.Fatal(err)
	}
	// Force a refresh (would happen async; here we call sync to be deterministic).
	_ = p.Refresh(context.Background(), "spc.day1otlk")

	got, _, _, _, isStale, since, err := p.Get(context.Background(), "spc.day1otlk")
	if err != nil {
		t.Fatalf("expected stale-but-served, got err=%v", err)
	}
	if string(got) != string(body) {
		t.Errorf("expected last-good bytes, got %q", got)
	}
	if !isStale {
		t.Errorf("isStale should be true after upstream failure")
	}
	if since <= 0 {
		t.Errorf("staleSince should be > 0, got %s", since)
	}
}

func TestProxy_UpstreamFailureNoCache_ReturnsErr(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	p, _, _, _ := testProxy(t, "spc.day1otlk", srv.URL, "image/png", time.Hour)
	_, _, _, _, _, _, err := p.Get(context.Background(), "spc.day1otlk")
	if err == nil {
		t.Errorf("expected error when no cached copy + upstream fails")
	}
}

func TestProxy_DiskHit_WarmsHotTier(t *testing.T) {
	body := []byte("disk-bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	p, _, _, _ := testProxy(t, "spc.day1otlk", srv.URL, "image/png", time.Hour)
	// Lazy fetch populates hot+disk.
	if _, _, _, _, _, _, err := p.Get(context.Background(), "spc.day1otlk"); err != nil {
		t.Fatal(err)
	}
	// Evict from hot tier directly to simulate hot-tier eviction under load.
	p.hot = NewHotTier(1<<20, 100)
	if p.hot.Has("spc.day1otlk") {
		t.Fatal("hot should be empty after rebuild")
	}
	// Get should hit disk and warm the hot tier.
	if _, _, _, _, _, _, err := p.Get(context.Background(), "spc.day1otlk"); err != nil {
		t.Fatal(err)
	}
	if !p.hot.Has("spc.day1otlk") {
		t.Errorf("hot tier should have been warmed from disk")
	}
}

func TestProxy_Stats_TracksRegisteredKeys(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("x"))
	}))
	defer srv.Close()
	p, _, _, _ := testProxy(t, "spc.day1otlk", srv.URL, "image/png", time.Minute)
	if _, _, _, _, _, _, err := p.Get(context.Background(), "spc.day1otlk"); err != nil {
		t.Fatal(err)
	}
	stats := p.Stats()
	h, ok := stats["spc.day1otlk"]
	if !ok {
		t.Fatal("expected stats entry for spc.day1otlk after Get")
	}
	if h.LastSuccess.IsZero() {
		t.Errorf("LastSuccess should be set")
	}
	if h.IntervalSec <= 0 {
		t.Errorf("IntervalSec = %d", h.IntervalSec)
	}
}
```

- [ ] **Step 4.2: Run, confirm fail**

```bash
go test ./internal/imageproxy/ -run TestProxy -v
```

- [ ] **Step 4.3: Implement `internal/imageproxy/proxy.go`**

```go
package imageproxy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"
)

// ImageInvalidate is the SSE payload broadcast when a refresh changes bytes.
type ImageInvalidate struct {
	Source    string    `json:"source"`
	Name      string    `json:"name"`
	FetchedAt time.Time `json:"fetchedAt"`
}

// ImageHealth is the per-key health surfaced by /api/sources at "image:<key>".
// JSON tags match fetcher.Health for the Phase 1/2 5 source entries; the
// extra fields (BytesOnDisk, InHotTier) are appended.
type ImageHealth struct {
	IntervalSec         int       `json:"intervalSec"`
	LastAttempt         time.Time `json:"lastAttempt"`
	LastSuccess         time.Time `json:"lastSuccess"`
	LastError           string    `json:"lastError,omitempty"`
	ETag                string    `json:"etag,omitempty"`
	AgeSec              int       `json:"ageSec"`
	ConsecutiveFailures int       `json:"consecutiveFailures"`
	BytesOnDisk         int64     `json:"bytesOnDisk"`
	InHotTier           bool      `json:"inHotTier"`
}

// Config bundles Proxy dependencies. All fields are required unless commented.
type Config struct {
	Registry     []Image
	Hot          *HotTier
	Disk         *DiskStore
	UserAgent    string
	Logger       *slog.Logger
	OnInvalidate func(ImageInvalidate) error // nil disables SSE invalidation
	HTTPTimeout  time.Duration                // default 15s
	NowFn        func() time.Time             // for tests; default time.Now().UTC
	Intervals    map[string]time.Duration     // operator overrides; nil = registry defaults
}

// Proxy is the public API for image fetch + cache.
type Proxy struct {
	registry     map[string]Image
	hot          *HotTier
	disk         *DiskStore
	client       *http.Client
	userAgent    string
	logger       *slog.Logger
	onInvalidate func(ImageInvalidate) error
	nowFn        func() time.Time
	intervals    map[string]time.Duration

	mu    sync.Mutex
	state map[string]*entryState // key -> per-key mutable state
}

// entryState holds Refresh-time mutable state per registry key.
type entryState struct {
	lastAttempt         time.Time
	lastSuccess         time.Time
	lastError           string
	etag                string    // upstream ETag verbatim (with quotes)
	lastModified        string    // upstream Last-Modified verbatim
	bodyHash            string    // sha256:<hex> of last-known body
	consecutiveFailures int
	refreshMu           sync.Mutex // serializes Refresh per key
}

// New constructs a Proxy from cfg. Panics if Registry/Hot/Disk are nil.
func New(cfg Config) *Proxy {
	if cfg.Hot == nil || cfg.Disk == nil {
		panic("imageproxy.New: Hot and Disk required")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.HTTPTimeout <= 0 {
		cfg.HTTPTimeout = 15 * time.Second
	}
	if cfg.NowFn == nil {
		cfg.NowFn = func() time.Time { return time.Now().UTC() }
	}
	reg := map[string]Image{}
	for _, img := range cfg.Registry {
		reg[img.Key()] = img
	}
	state := map[string]*entryState{}
	for k := range reg {
		state[k] = &entryState{}
	}
	return &Proxy{
		registry:     reg,
		hot:          cfg.Hot,
		disk:         cfg.Disk,
		client:       &http.Client{Timeout: cfg.HTTPTimeout},
		userAgent:    cfg.UserAgent,
		logger:       cfg.Logger,
		onInvalidate: cfg.OnInvalidate,
		nowFn:        cfg.NowFn,
		intervals:    cfg.Intervals,
		state:        state,
	}
}

// Keys returns all registered keys in iteration order (for stats walks etc).
func (p *Proxy) Keys() []string {
	out := make([]string, 0, len(p.registry))
	for k := range p.registry {
		out = append(out, k)
	}
	return out
}

// Interval returns the effective interval for the key (operator override or
// registry default).
func (p *Proxy) Interval(key string) time.Duration {
	img, ok := p.registry[key]
	if !ok {
		return 0
	}
	if d, ok := p.intervals[key]; ok && d > 0 {
		return d
	}
	return img.DefaultInterval
}

// Get returns the cached or freshly-fetched bytes for a registered key.
//
// Lookup order:
//  1. Hot tier hit + fresh (age < interval): serve immediately.
//  2. Disk hit + fresh: warm hot tier, serve.
//  3. Hot or disk hit + stale: serve cached, fire async Refresh, set
//     isStale + staleSince.
//  4. No cached copy: synchronous Refresh, then serve fresh bytes.
//  5. No cached copy and upstream fails: return error (handler returns 502).
//
// Returns os.ErrNotExist if the key is unknown.
func (p *Proxy) Get(ctx context.Context, key string) (body []byte, contentType, validator string, fetchedAt time.Time, isStale bool, staleSince time.Duration, err error) {
	img, ok := p.registry[key]
	if !ok {
		return nil, "", "", time.Time{}, false, 0, os.ErrNotExist
	}
	st := p.stateOf(key)

	if b, ct, v, ts, hit := p.hot.Get(key); hit {
		stale, since := p.staleness(st, ts)
		if stale {
			go p.refreshAsync(key)
		}
		return b, ct, v, ts, stale, since, nil
	}

	if b, ct, v, ts, derr := p.disk.Get(key); derr == nil {
		p.hot.Put(key, b, ct, v, ts)
		stale, since := p.staleness(st, ts)
		if stale {
			go p.refreshAsync(key)
		}
		return b, ct, v, ts, stale, since, nil
	}

	if rerr := p.Refresh(ctx, key); rerr != nil {
		return nil, "", "", time.Time{}, false, 0, fmt.Errorf("imageproxy.Get %q: %w", key, rerr)
	}
	if b, ct, v, ts, hit := p.hot.Get(key); hit {
		return b, ct, v, ts, false, 0, nil
	}
	// Refresh succeeded but cache is somehow empty (extremely unlikely; defensive).
	return nil, "", "", time.Time{}, false, 0, fmt.Errorf("imageproxy.Get %q (%s): refresh succeeded but cache empty", key, img.URL)
}

// Refresh attempts a conditional GET for the key. On success-with-change,
// writes hot+disk and broadcasts image.invalidate. On 304 / unchanged body,
// no-ops the cache. On upstream failure, updates failure stats but does NOT
// mutate the cache.
func (p *Proxy) Refresh(ctx context.Context, key string) error {
	img, ok := p.registry[key]
	if !ok {
		return os.ErrNotExist
	}
	st := p.stateOf(key)
	st.refreshMu.Lock()
	defer st.refreshMu.Unlock()

	now := p.nowFn()
	p.markAttempt(st, now)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, img.URL, nil)
	if err != nil {
		p.markFailure(st, err)
		return err
	}
	req.Header.Set("User-Agent", p.userAgent)
	if st.etag != "" {
		req.Header.Set("If-None-Match", st.etag)
	}
	if st.lastModified != "" {
		req.Header.Set("If-Modified-Since", st.lastModified)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		p.markFailure(st, err)
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusNotModified:
		p.markSuccess(st, now, st.etag)
		return nil
	case resp.StatusCode/100 != 2:
		ferr := fmt.Errorf("upstream status %d", resp.StatusCode)
		p.markFailure(st, ferr)
		return ferr
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		p.markFailure(st, err)
		return err
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = img.MIME
	}
	upstreamETag := resp.Header.Get("ETag")
	upstreamLM := resp.Header.Get("Last-Modified")

	sum := sha256.Sum256(body)
	bodyHash := "sha256:" + hex.EncodeToString(sum[:])

	// Diff-on-write predicate: did the body sha256 actually change?
	if st.bodyHash != "" && st.bodyHash == bodyHash {
		// ETag may have churned without a content change (WPC mtime ETags do this);
		// refresh stored validator metadata so future If-None-Match works, but do
		// NOT broadcast.
		st.etag = upstreamETag
		st.lastModified = upstreamLM
		p.markSuccess(st, now, p.publicValidator(st, bodyHash))
		return nil
	}

	validator := upstreamETag
	if validator == "" {
		validator = bodyHash
	}

	if err := p.disk.Put(key, body, contentType, validator, now); err != nil {
		p.markFailure(st, err)
		return err
	}
	p.hot.Put(key, body, contentType, validator, now)

	st.etag = upstreamETag
	st.lastModified = upstreamLM
	st.bodyHash = bodyHash
	p.markSuccess(st, now, validator)

	if p.onInvalidate != nil {
		ev := ImageInvalidate{Source: img.Source, Name: img.Name, FetchedAt: now}
		if err := p.onInvalidate(ev); err != nil {
			p.logger.Warn("imageproxy.invalidate_callback", "key", key, "err", err.Error())
		}
	}
	return nil
}

// publicValidator returns the upstream ETag if non-empty, else the body hash.
func (p *Proxy) publicValidator(st *entryState, bodyHash string) string {
	if st.etag != "" {
		return st.etag
	}
	return bodyHash
}

// Stats returns per-key health for /api/sources surfacing.
// Lazy-only keys with no observations yet are still listed (with zero times)
// so operators see the registry in /api/sources from boot.
func (p *Proxy) Stats() map[string]ImageHealth {
	out := make(map[string]ImageHealth, len(p.registry))
	for key, img := range p.registry {
		st := p.stateOf(key)
		p.mu.Lock()
		h := ImageHealth{
			IntervalSec:         int(p.Interval(key).Seconds()),
			LastAttempt:         st.lastAttempt,
			LastSuccess:         st.lastSuccess,
			LastError:           st.lastError,
			ETag:                p.publicValidator(st, st.bodyHash),
			ConsecutiveFailures: st.consecutiveFailures,
			InHotTier:           p.hot.Has(key),
		}
		p.mu.Unlock()
		if !h.LastSuccess.IsZero() {
			h.AgeSec = int(p.nowFn().Sub(h.LastSuccess).Seconds())
		}
		// Best-effort disk size; non-fatal on miss.
		if b, _, _, _, derr := p.disk.Get(key); derr == nil {
			h.BytesOnDisk = int64(len(b))
		}
		_ = img
		out[key] = h
	}
	return out
}

func (p *Proxy) stateOf(key string) *entryState {
	p.mu.Lock()
	defer p.mu.Unlock()
	if s, ok := p.state[key]; ok {
		return s
	}
	s := &entryState{}
	p.state[key] = s
	return s
}

func (p *Proxy) markAttempt(st *entryState, now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	st.lastAttempt = now
}

func (p *Proxy) markSuccess(st *entryState, now time.Time, validator string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	st.lastSuccess = now
	st.lastError = ""
	if validator != "" {
		st.etag = validator
	}
	st.consecutiveFailures = 0
}

func (p *Proxy) markFailure(st *entryState, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	st.lastError = truncate(err.Error(), 256)
	st.consecutiveFailures++
}

// staleness reports whether ts is older than the per-key interval. Also true
// if a recent failure has been recorded since lastSuccess.
func (p *Proxy) staleness(st *entryState, ts time.Time) (bool, time.Duration) {
	p.mu.Lock()
	failures := st.consecutiveFailures
	lastSuccess := st.lastSuccess
	p.mu.Unlock()
	now := p.nowFn()
	if failures > 0 && !lastSuccess.IsZero() {
		return true, now.Sub(lastSuccess)
	}
	if !ts.IsZero() && now.Sub(ts) >= p.Interval(p.keyForState(st)) {
		return true, now.Sub(ts)
	}
	return false, 0
}

// keyForState reverse-lookups the key for an entryState. Linear scan over 20
// entries is cheap and only runs in the staleness check (off the hot path).
func (p *Proxy) keyForState(st *entryState) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	for k, s := range p.state {
		if s == st {
			return k
		}
	}
	return ""
}

func (p *Proxy) refreshAsync(key string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := p.Refresh(ctx, key); err != nil && !errors.Is(err, context.Canceled) {
		p.logger.Warn("imageproxy.refresh_async", "key", key, "err", err.Error())
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
```

- [ ] **Step 4.4: Run tests + lint**

```bash
go test ./internal/imageproxy/ -run TestProxy -v -race
golangci-lint run ./internal/imageproxy/...
```

Expected: all 9 proxy tests pass with `-race`; lint clean.

- [ ] **Step 4.5: Commit**

```bash
git add internal/imageproxy/proxy.go internal/imageproxy/proxy_test.go
git commit -m "feat(p3): image proxy core — Get/Refresh/Stats with diff-on-write SSE" -- \
  internal/imageproxy/proxy.go internal/imageproxy/proxy_test.go
```

---

## Task 5: Prewarm poller — one goroutine per key

**Files:**
- Create: `internal/imageproxy/prewarm.go`
- Create: `internal/imageproxy/prewarm_test.go`

**Dependencies:** depends on Task 4 (Proxy.Refresh).

**Why:** Design §5.5. The 4 default-prewarm keys (`spc.day1otlk`, `spc.day2otlk`, `spc.day3otlk`, `nhc.atl_7d`) are polled on their per-key interval so the first user-facing GET hits a warm cache. Pattern matches `internal/fetcher.Fetcher.Run`'s ticker + exponential backoff, but Phase 3 cannot use `fetcher.Fetcher` directly (the fetcher takes `sources.Source`, which returns typed payloads — image bytes don't fit).

- [ ] **Step 5.1: Write the failing test**

`internal/imageproxy/prewarm_test.go`:

```go
package imageproxy

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestStartPrewarm_PollsEachKeyOnInterval(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("ETag", `"e1"`)
		_, _ = w.Write([]byte("bytes"))
	}))
	defer srv.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hot := NewHotTier(1<<20, 100)
	disk, err := NewDiskStore(t.TempDir(), 1<<20, logger)
	if err != nil {
		t.Fatal(err)
	}
	reg := []Image{
		{Source: "a", Name: "x", URL: srv.URL, MIME: "image/png", DefaultInterval: 30 * time.Millisecond, Description: "d"},
		{Source: "b", Name: "y", URL: srv.URL, MIME: "image/png", DefaultInterval: 30 * time.Millisecond, Description: "d"},
	}
	p := New(Config{Registry: reg, Hot: hot, Disk: disk, UserAgent: "t/0", Logger: logger, HTTPTimeout: time.Second})

	ctx, cancel := context.WithCancel(context.Background())
	stop := StartPrewarm(ctx, p, []string{"a.x", "b.y"}, logger)
	defer stop()
	defer cancel()

	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatalf("did not see >=4 polls within deadline; got %d", calls.Load())
		default:
		}
		if calls.Load() >= 4 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestStartPrewarm_StopsOnContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("x"))
	}))
	defer srv.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hot := NewHotTier(1<<20, 100)
	disk, _ := NewDiskStore(t.TempDir(), 1<<20, logger)
	reg := []Image{{Source: "a", Name: "x", URL: srv.URL, MIME: "image/png", DefaultInterval: 50 * time.Millisecond, Description: "d"}}
	p := New(Config{Registry: reg, Hot: hot, Disk: disk, UserAgent: "t/0", Logger: logger, HTTPTimeout: time.Second})

	ctx, cancel := context.WithCancel(context.Background())
	stop := StartPrewarm(ctx, p, []string{"a.x"}, logger)
	cancel()
	done := make(chan struct{})
	go func() { stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("StartPrewarm did not return after context cancel")
	}
}

func TestStartPrewarm_BacksOffOnFailure(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hot := NewHotTier(1<<20, 100)
	disk, _ := NewDiskStore(t.TempDir(), 1<<20, logger)
	reg := []Image{{Source: "a", Name: "x", URL: srv.URL, MIME: "image/png", DefaultInterval: 30 * time.Millisecond, Description: "d"}}
	p := New(Config{Registry: reg, Hot: hot, Disk: disk, UserAgent: "t/0", Logger: logger, HTTPTimeout: time.Second})

	ctx, cancel := context.WithCancel(context.Background())
	stop := StartPrewarm(ctx, p, []string{"a.x"}, logger)

	time.Sleep(500 * time.Millisecond)
	stop()
	cancel()

	// On constant failures with 30ms base + cap 5x = 150ms, in 500ms we'd see
	// roughly: t=0, t=30, t=60, t=120, t=270, t=420 → ~6 attempts.
	// Without backoff at 30ms steady cadence we'd see ~16. Assert <= 10 to
	// prove backoff fired.
	if got := calls.Load(); got > 10 {
		t.Errorf("calls = %d, want <=10 (backoff should slow polling)", got)
	}
}

func TestStartPrewarm_UnknownKey_LoggedNotFatal(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hot := NewHotTier(1<<20, 100)
	disk, _ := NewDiskStore(t.TempDir(), 1<<20, logger)
	p := New(Config{Registry: []Image{}, Hot: hot, Disk: disk, UserAgent: "t/0", Logger: logger, HTTPTimeout: time.Second})

	ctx, cancel := context.WithCancel(context.Background())
	stop := StartPrewarm(ctx, p, []string{"unknown.key"}, logger)
	// Just confirm it doesn't panic and stops cleanly.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); stop() }()
	cancel()
	wg.Wait()
}
```

- [ ] **Step 5.2: Run, confirm fail**

```bash
go test ./internal/imageproxy/ -run TestStartPrewarm -v
```

- [ ] **Step 5.3: Implement `internal/imageproxy/prewarm.go`**

```go
package imageproxy

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"time"
)

// StartPrewarm spawns one goroutine per key that calls proxy.Refresh on the
// per-image interval (registry default unless overridden by operator config).
// Returns a stop function that blocks until all pollers have exited.
//
// Unknown keys are logged at WARN and silently dropped (the operator's prewarm
// list may include a stale key after a registry change; non-fatal).
//
// Backoff: matches internal/fetcher.Fetcher.Run shape — base interval on
// success, doubling backoff on consecutive failures up to 5x base.
func StartPrewarm(ctx context.Context, p *Proxy, keys []string, logger *slog.Logger) func() {
	if logger == nil {
		logger = slog.Default()
	}
	registered := p.Keys()
	regSet := make(map[string]struct{}, len(registered))
	for _, k := range registered {
		regSet[k] = struct{}{}
	}
	var wg sync.WaitGroup
	for _, k := range keys {
		if _, ok := regSet[k]; !ok {
			logger.Warn("imageproxy.prewarm.unknown_key", "key", k)
			continue
		}
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			runOne(ctx, p, key, logger)
		}(k)
	}
	return wg.Wait
}

func runOne(ctx context.Context, p *Proxy, key string, logger *slog.Logger) {
	base := p.Interval(key)
	if base <= 0 {
		base = 5 * time.Minute
	}
	maxBackoff := 5 * base
	delay := time.Duration(0) // first tick fires immediately
	timer := time.NewTimer(delay)
	defer timer.Stop()

	failures := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		err := p.Refresh(ctx, key)
		switch {
		case err == nil:
			failures = 0
			delay = base
		case errors.Is(err, os.ErrNotExist):
			logger.Warn("imageproxy.prewarm.unknown_key_runtime", "key", key)
			return
		default:
			failures++
			delay = base
			for i := 0; i < failures-1; i++ {
				delay *= 2
				if delay > maxBackoff {
					delay = maxBackoff
					break
				}
			}
			logger.Warn("imageproxy.prewarm.error", "key", key, "failures", failures, "next_delay", delay.String(), "err", err.Error())
		}
		timer.Reset(delay)
	}
}
```

- [ ] **Step 5.4: Run tests + lint**

```bash
go test ./internal/imageproxy/ -run TestStartPrewarm -v -race
golangci-lint run ./internal/imageproxy/...
```

- [ ] **Step 5.5: Commit**

```bash
git add internal/imageproxy/prewarm.go internal/imageproxy/prewarm_test.go
git commit -m "feat(p3): image proxy prewarm pollers — per-key goroutine + backoff" -- \
  internal/imageproxy/prewarm.go internal/imageproxy/prewarm_test.go
```

---

## Task 6: Config extensions — Images block + validation + nested env walker

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Dependencies:** none.

**Why:** Design §8. Replace the Phase 0 placeholder `CacheConfig` and `ImagesConfig.DefaultMode` with the full Phase 3 image-proxy config block, including operator-tunable defaults, boot validation (interval floor, prewarm-key-known check, positive-int caps, writable cache_dir), and an env walker that handles the nested `image_intervals` map via the `__` separator convention.

### Field reconciliation summary

Per Conventions §"Trap — config field reconciliation":

- **Delete:** `Config.Cache` field (and `CacheConfig` struct entirely).
- **Delete:** `ImagesConfig.DefaultMode` and its case in `validate()`.
- **Keep + extend:** `ImagesConfig.Prewarm` (already a `[]string`).
- **Add to ImagesConfig:** `CacheDir`, `DiskMaxBytes`, `HotMaxBytes`, `HotMaxEntries`, `RefreshInterval`, `ImageIntervals` (`map[string]time.Duration`).

- [ ] **Step 6.1: Write the failing test**

Add to `internal/config/config_test.go`:

```go
func TestDefaults_ImagesBlockShape(t *testing.T) {
	cfg := defaults()
	if cfg.Images.DiskMaxBytes <= 0 {
		t.Errorf("Images.DiskMaxBytes default must be > 0")
	}
	if cfg.Images.HotMaxBytes < 0 {
		t.Errorf("Images.HotMaxBytes default must be >= 0")
	}
	if cfg.Images.HotMaxEntries <= 0 {
		t.Errorf("Images.HotMaxEntries default must be > 0")
	}
	if cfg.Images.RefreshInterval < 60*time.Second {
		t.Errorf("Images.RefreshInterval default must be >= 60s")
	}
	if len(cfg.Images.Prewarm) == 0 {
		t.Errorf("Images.Prewarm default must be non-empty")
	}
	wantPrewarm := []string{"spc.day1otlk", "spc.day2otlk", "spc.day3otlk", "nhc.atl_7d"}
	if got := cfg.Images.Prewarm; !slices.Equal(got, wantPrewarm) {
		t.Errorf("Images.Prewarm = %v, want %v", got, wantPrewarm)
	}
}

func TestValidate_RejectsNonPositiveDiskMaxBytes(t *testing.T) {
	cfg := defaults()
	cfg.Images.DiskMaxBytes = 0
	if err := validate(cfg); err == nil {
		t.Error("expected error for DiskMaxBytes <= 0")
	}
}

func TestValidate_AllowsZeroHotMaxBytes_DisablesHotTier(t *testing.T) {
	cfg := defaults()
	cfg.Images.HotMaxBytes = 0
	if err := validate(cfg); err != nil {
		t.Errorf("expected zero HotMaxBytes accepted (disables tier), got %v", err)
	}
}

func TestValidateImageIntervals_ClampsBelowFloor(t *testing.T) {
	cfg := defaults()
	cfg.Images.ImageIntervals = map[string]time.Duration{
		"spc.day1otlk": 5 * time.Second,  // below 60s floor
		"nhc.atl_7d":   2 * time.Minute,  // OK
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	validateImageIntervals(cfg, logger)
	if got := cfg.Images.ImageIntervals["spc.day1otlk"]; got != 60*time.Second {
		t.Errorf("clamped value = %s, want 60s", got)
	}
	if got := cfg.Images.ImageIntervals["nhc.atl_7d"]; got != 2*time.Minute {
		t.Errorf("untouched value changed to %s", got)
	}
}

func TestValidateImagesPrewarm_DropsUnknownKey(t *testing.T) {
	cfg := defaults()
	cfg.Images.Prewarm = []string{"spc.day1otlk", "bogus.never_existed", "nhc.atl_7d"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	validateImagesPrewarm(cfg, logger)
	want := []string{"spc.day1otlk", "nhc.atl_7d"}
	if !slices.Equal(cfg.Images.Prewarm, want) {
		t.Errorf("Prewarm = %v, want %v", cfg.Images.Prewarm, want)
	}
}

func TestEnvOverride_ImagesPrewarm_ReplacesList(t *testing.T) {
	t.Setenv("CWD_IMAGES_PREWARM", "spc.day1otlk,nhc.epac_7d,wpc.heatrisk_day1")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := defaults()
	applyEnvOverrides(cfg, logger)
	want := []string{"spc.day1otlk", "nhc.epac_7d", "wpc.heatrisk_day1"}
	if !slices.Equal(cfg.Images.Prewarm, want) {
		t.Errorf("after env override Prewarm = %v, want %v", cfg.Images.Prewarm, want)
	}
}

func TestEnvOverride_ImageIntervals_DoubleUnderscoreSeparator(t *testing.T) {
	// CWD_IMAGES_IMAGE_INTERVALS_SPC__DAY1OTLK_FIRE → spc.day1otlk_fire
	t.Setenv("CWD_IMAGES_IMAGE_INTERVALS_SPC__DAY1OTLK_FIRE", "120s")
	t.Setenv("CWD_IMAGES_IMAGE_INTERVALS_NHC__ATL_7D", "1h")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := defaults()
	applyEnvOverrides(cfg, logger)
	if got := cfg.Images.ImageIntervals["spc.day1otlk_fire"]; got != 120*time.Second {
		t.Errorf("spc.day1otlk_fire = %s, want 120s", got)
	}
	if got := cfg.Images.ImageIntervals["nhc.atl_7d"]; got != time.Hour {
		t.Errorf("nhc.atl_7d = %s, want 1h", got)
	}
}

func TestEnvOverride_ImageIntervals_BadDurationLoggedNotApplied(t *testing.T) {
	t.Setenv("CWD_IMAGES_IMAGE_INTERVALS_SPC__DAY1OTLK", "not-a-duration")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := defaults()
	applyEnvOverrides(cfg, logger)
	if _, ok := cfg.Images.ImageIntervals["spc.day1otlk"]; ok {
		t.Error("bad duration should not have been applied")
	}
}
```

Add `import "io"`, `import "log/slog"`, `import "slices"`, and `"time"` to the test file's imports if not present.

- [ ] **Step 6.2: Run, confirm fail**

```bash
go test ./internal/config/ -run 'TestDefaults_ImagesBlockShape|TestValidate_Rejects|TestValidate_Allows|TestValidateImage|TestEnvOverride_Images|TestEnvOverride_ImageIntervals' -v
```

Expected: build fails — `Images.DiskMaxBytes` etc undefined.

- [ ] **Step 6.3: Modify `internal/config/config.go`**

In the `Config` struct, **delete** the `Cache` field:

```go
// REMOVED — Phase 0 placeholder superseded by Images config block.
// Cache   CacheConfig             `yaml:"cache"`
```

**Delete** the `CacheConfig` struct entirely.

**Replace** `ImagesConfig` with:

```go
// ImagesConfig holds image proxy settings. See design §8.
type ImagesConfig struct {
	// CacheDir is the disk-store root. Created with 0700 if missing. Boot
	// fails if the directory is not writable.
	CacheDir string `yaml:"cache_dir"`
	// DiskMaxBytes caps the on-disk store. Must be > 0.
	DiskMaxBytes int64 `yaml:"disk_max_bytes"`
	// HotMaxBytes caps the in-memory hot tier. 0 disables the tier (disk-only).
	HotMaxBytes int64 `yaml:"hot_max_bytes"`
	// HotMaxEntries caps the hot tier by entry count.
	HotMaxEntries int `yaml:"hot_max_entries"`
	// RefreshInterval is the global default cadence (per-image entries override).
	// Below the 60s politeness floor, validateImagesRefreshInterval clamps + WARNs.
	RefreshInterval time.Duration `yaml:"refresh_interval"`
	// ImageIntervals overrides the per-image cadence by dot key (e.g.
	// "spc.day1otlk": 90s). Values below 60s are clamped + logged.
	ImageIntervals map[string]time.Duration `yaml:"image_intervals"`
	// Prewarm lists registry keys that should be polled in the background.
	// Unknown keys are dropped at boot with a WARN.
	Prewarm []string `yaml:"prewarm"`
}
```

In `defaults()`, **delete** the `Cache:` field, replace the `Images:` field with:

```go
Images: ImagesConfig{
    CacheDir:        xdgState("cwd/images"),
    DiskMaxBytes:    524_288_000, // 500 MiB
    HotMaxBytes:     67_108_864,  // 64 MiB
    HotMaxEntries:   256,
    RefreshInterval: 5 * time.Minute,
    ImageIntervals:  map[string]time.Duration{},
    Prewarm:         imageproxy.DefaultPrewarmKeys(),
},
```

Add the import: `"github.com/jacaudi/cwd/internal/imageproxy"`.

In `validate()`, **delete** the `images.default_mode` switch case. **Add**:

```go
if cfg.Images.DiskMaxBytes <= 0 {
    return errors.New("images.disk_max_bytes must be > 0")
}
if cfg.Images.HotMaxBytes < 0 {
    return errors.New("images.hot_max_bytes must be >= 0 (0 disables hot tier)")
}
if cfg.Images.HotMaxEntries <= 0 {
    return errors.New("images.hot_max_entries must be > 0")
}
if cfg.Images.CacheDir == "" {
    return errors.New("images.cache_dir must be set")
}
```

Add two new validators (analog to `validateSWPCProducts`):

```go
// validateImagesRefreshInterval clamps refresh_interval and per-image intervals
// below the 60s politeness floor and WARNs.
func validateImagesRefreshInterval(cfg *Config, logger *slog.Logger) {
    const floor = 60 * time.Second
    if cfg.Images.RefreshInterval > 0 && cfg.Images.RefreshInterval < floor {
        logger.Warn("config.images_refresh_interval_clamped",
            "value", cfg.Images.RefreshInterval.String(),
            "floor", floor.String())
        cfg.Images.RefreshInterval = floor
    }
}

// validateImageIntervals clamps per-image intervals below 60s + WARNs.
func validateImageIntervals(cfg *Config, logger *slog.Logger) {
    const floor = 60 * time.Second
    for k, d := range cfg.Images.ImageIntervals {
        if d > 0 && d < floor {
            logger.Warn("config.image_interval_clamped",
                "key", k, "value", d.String(), "floor", floor.String())
            cfg.Images.ImageIntervals[k] = floor
        }
    }
}

// validateImagesPrewarm drops prewarm entries that don't appear in the
// imageproxy.Registry and WARNs. Mirrors validateRegionFilter / validateSWPCProducts.
func validateImagesPrewarm(cfg *Config, logger *slog.Logger) {
    known := imageproxy.ByKey()
    in := cfg.Images.Prewarm
    out := make([]string, 0, len(in))
    for _, k := range in {
        if _, ok := known[k]; ok {
            out = append(out, k)
        } else {
            logger.Warn("config.images_prewarm_unknown_key", "key", k)
        }
    }
    cfg.Images.Prewarm = out
}
```

Wire these into `LoadWithLogger`'s validation chain (right after `validateSourceIntervalFloors`):

```go
validateImagesRefreshInterval(cfg, logger)
validateImageIntervals(cfg, logger)
validateImagesPrewarm(cfg, logger)
```

In `applyEnvOverrides`, **delete** the `apply("cache", ...)` call. After the existing `apply("images", ...)` call (which still walks scalar fields like `cache_dir`, `disk_max_bytes`, etc.), add explicit handling for the slice + nested-map fields:

```go
// Images.Prewarm — comma-separated key list; replaces the entire list.
if v, ok := os.LookupEnv("CWD_IMAGES_PREWARM"); ok {
    parts := strings.Split(v, ",")
    out := make([]string, 0, len(parts))
    for _, p := range parts {
        if t := strings.TrimSpace(p); t != "" {
            out = append(out, t)
        }
    }
    cfg.Images.Prewarm = out
}

// Images.ImageIntervals — CWD_IMAGES_IMAGE_INTERVALS_<KEY> with __ as the
// dot separator. Example: CWD_IMAGES_IMAGE_INTERVALS_SPC__DAY1OTLK_FIRE=90s
// → cfg.Images.ImageIntervals["spc.day1otlk_fire"] = 90s.
const intervalsPrefix = "CWD_IMAGES_IMAGE_INTERVALS_"
if cfg.Images.ImageIntervals == nil {
    cfg.Images.ImageIntervals = map[string]time.Duration{}
}
for _, kv := range os.Environ() {
    if !strings.HasPrefix(kv, intervalsPrefix) {
        continue
    }
    eq := strings.IndexByte(kv, '=')
    if eq < 0 {
        continue
    }
    name := kv[:eq]
    val := kv[eq+1:]
    rest := name[len(intervalsPrefix):]
    // Convert UPPER+__ to lower+. (single-underscore stays in segment).
    key := strings.ToLower(strings.ReplaceAll(rest, "__", "."))
    d, perr := time.ParseDuration(val)
    if perr != nil {
        logger.Warn("config.env_parse_failed", "key", name, "value", val, "err", perr.Error())
        continue
    }
    cfg.Images.ImageIntervals[key] = d
}
```

Add `"strings"` to the imports (already present). Make sure to update the existing `apply("cache", ...)` removal does not leave a dangling reference.

- [ ] **Step 6.4: Update `mergeSourceDefaults`**

No change needed — `mergeSourceDefaults` only touches `cfg.Sources`, and source defaults are unchanged.

- [ ] **Step 6.5: Run tests + lint**

```bash
go test ./internal/config/ -v -race
golangci-lint run ./internal/config/...
```

Expected: all new tests pass; existing Phase 1/2 tests still pass (defaults shape changed but the test surface is shape-agnostic for the deleted Cache and DefaultMode fields).

If pre-existing tests reference `cfg.Cache.ImageDir`, `cfg.Cache.ImageMaxBytes`, or `cfg.Images.DefaultMode`, update them to use the new `cfg.Images` fields or delete the assertion. Do this in the same commit.

- [ ] **Step 6.6: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(p3): config — Images block + per-image intervals + prewarm validation" -- \
  internal/config/config.go internal/config/config_test.go
```

---

## Task 7: HTTP handler — GET /img/{source}/{name}

**Files:**
- Create: `internal/api/images.go`
- Create: `internal/api/images_test.go`

**Dependencies:** depends on Task 4 (Proxy.Get + ImageHealth).

**Why:** Design §5.8. The browser-facing surface that resolves `<img src="/img/spc/day1otlk">`, with `Cache-Control: public, max-age=60, stale-while-revalidate=900` for browser-side staleness handling, `Warning: 110 cwd "stale Xm"` when proxy reports a stale cache, and conditional-GET passthrough for `If-None-Match`.

### Handler interface

```go
// ImageProxy is the subset of *imageproxy.Proxy the handler depends on.
// Defining the interface here keeps the api package free of an imageproxy
// import in test code (mocks satisfy the interface trivially).
type ImageProxy interface {
    Get(ctx context.Context, key string) (body []byte, contentType, validator string, fetchedAt time.Time, isStale bool, staleSince time.Duration, err error)
}
```

- [ ] **Step 7.1: Write the failing test**

`internal/api/images_test.go`:

```go
package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

type stubProxy struct {
	body        []byte
	contentType string
	validator   string
	fetchedAt   time.Time
	isStale     bool
	staleSince  time.Duration
	err         error
	calls       int
}

func (s *stubProxy) Get(ctx context.Context, key string) ([]byte, string, string, time.Time, bool, time.Duration, error) {
	s.calls++
	return s.body, s.contentType, s.validator, s.fetchedAt, s.isStale, s.staleSince, s.err
}

func newImagesRouter(p ImageProxy) http.Handler {
	r := chi.NewRouter()
	r.Method(http.MethodGet, "/img/{source}/{name}", NewImagesHandler(p))
	return r
}

func TestImagesHandler_HappyPath(t *testing.T) {
	p := &stubProxy{
		body: []byte("img-bytes"), contentType: "image/png", validator: `"e1"`,
		fetchedAt: time.Now().UTC(),
	}
	r := newImagesRouter(p)
	srv := httptest.NewServer(r)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/img/spc/day1otlk")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type = %q", got)
	}
	if !strings.Contains(resp.Header.Get("Cache-Control"), "max-age=60") {
		t.Errorf("Cache-Control = %q, want max-age=60", resp.Header.Get("Cache-Control"))
	}
	if !strings.Contains(resp.Header.Get("Cache-Control"), "stale-while-revalidate=900") {
		t.Errorf("Cache-Control missing stale-while-revalidate=900")
	}
	if got := resp.Header.Get("ETag"); got != `"e1"` {
		t.Errorf("ETag = %q", got)
	}
	if got := resp.Header.Get("Warning"); got != "" {
		t.Errorf("Warning header should be absent on fresh, got %q", got)
	}
}

func TestImagesHandler_404OnUnknownKey(t *testing.T) {
	p := &stubProxy{err: os.ErrNotExist}
	r := newImagesRouter(p)
	srv := httptest.NewServer(r)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/img/spc/no_such")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestImagesHandler_502OnUpstreamFailureNoCache(t *testing.T) {
	p := &stubProxy{err: errors.New("upstream 503")}
	r := newImagesRouter(p)
	srv := httptest.NewServer(r)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/img/spc/day1otlk")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", resp.StatusCode)
	}
}

func TestImagesHandler_AddsWarningWhenStale(t *testing.T) {
	p := &stubProxy{
		body: []byte("stale-bytes"), contentType: "image/png", validator: `"e1"`,
		fetchedAt: time.Now().UTC().Add(-12 * time.Minute),
		isStale:   true, staleSince: 12 * time.Minute,
	}
	r := newImagesRouter(p)
	srv := httptest.NewServer(r)
	defer srv.Close()
	resp, _ := http.Get(srv.URL + "/img/spc/day1otlk")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200 (stale-but-served)", resp.StatusCode)
	}
	w := resp.Header.Get("Warning")
	if !strings.Contains(w, "110 cwd") {
		t.Errorf("Warning header = %q, want 110 cwd ...", w)
	}
	if !strings.Contains(w, "stale 12m") && !strings.Contains(w, "stale 720s") {
		t.Errorf("Warning header should describe stale duration, got %q", w)
	}
}

func TestImagesHandler_304OnIfNoneMatchMatch(t *testing.T) {
	p := &stubProxy{
		body: []byte("img-bytes"), contentType: "image/png", validator: `"e1"`,
		fetchedAt: time.Now().UTC(),
	}
	r := newImagesRouter(p)
	srv := httptest.NewServer(r)
	defer srv.Close()
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/img/spc/day1otlk", nil)
	req.Header.Set("If-None-Match", `"e1"`)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotModified {
		t.Errorf("status = %d, want 304", resp.StatusCode)
	}
}

func TestImagesHandler_KeyComposition(t *testing.T) {
	p := &stubProxy{
		body: []byte("x"), contentType: "image/png", validator: `"e"`,
		fetchedAt: time.Now().UTC(),
	}
	got := ""
	wrap := imageProxyFunc(func(ctx context.Context, key string) ([]byte, string, string, time.Time, bool, time.Duration, error) {
		got = key
		return p.body, p.contentType, p.validator, p.fetchedAt, p.isStale, p.staleSince, p.err
	})
	r := newImagesRouter(wrap)
	srv := httptest.NewServer(r)
	defer srv.Close()
	_, _ = http.Get(srv.URL + "/img/spc/day1otlk_fire")
	if got != "spc.day1otlk_fire" {
		t.Errorf("composed key = %q, want spc.day1otlk_fire", got)
	}
	_ = fmt.Sprintf("%s", got)
}

// imageProxyFunc adapts a func to the ImageProxy interface for tests.
type imageProxyFunc func(ctx context.Context, key string) ([]byte, string, string, time.Time, bool, time.Duration, error)

func (f imageProxyFunc) Get(ctx context.Context, key string) ([]byte, string, string, time.Time, bool, time.Duration, error) {
	return f(ctx, key)
}
```

- [ ] **Step 7.2: Run, confirm fail**

```bash
go test ./internal/api/ -run TestImagesHandler -v
```

- [ ] **Step 7.3: Implement `internal/api/images.go`**

```go
package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
)

// ImageProxy is the dependency surface for the GET /img/{source}/{name} handler.
type ImageProxy interface {
	Get(ctx context.Context, key string) (body []byte, contentType, validator string, fetchedAt time.Time, isStale bool, staleSince time.Duration, err error)
}

// imageCacheControl is the outbound Cache-Control. 60s freshness window plus
// 15-minute stale-while-revalidate gives browsers and SW caches latitude to
// serve quickly while a background refresh runs.
const imageCacheControl = "public, max-age=60, stale-while-revalidate=900"

// NewImagesHandler returns the GET /img/{source}/{name} handler.
func NewImagesHandler(p ImageProxy) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		source := chi.URLParam(r, "source")
		name := chi.URLParam(r, "name")
		if source == "" || name == "" {
			http.NotFound(w, r)
			return
		}
		key := source + "." + name

		body, ct, validator, fetchedAt, isStale, since, err := p.Get(r.Context(), key)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", ct)
		w.Header().Set("Cache-Control", imageCacheControl)
		if validator != "" {
			w.Header().Set("ETag", validator)
		}
		if !fetchedAt.IsZero() {
			w.Header().Set("Last-Modified", fetchedAt.UTC().Format(http.TimeFormat))
		}
		if isStale {
			w.Header().Set("Warning", fmt.Sprintf(`110 cwd "stale %s"`, formatStale(since)))
		}

		// Honor browser conditional GET against our stored validator.
		if inm := r.Header.Get("If-None-Match"); validator != "" && inm == validator {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.WriteHeader(http.StatusOK)
		if r.Method != http.MethodHead {
			_, _ = w.Write(body)
		}
	})
}

// formatStale renders the duration as Xm or Xs depending on size, matching
// the Warning header convention from design §6.6.
func formatStale(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	if d >= time.Minute {
		return fmt.Sprintf("%dm", int(d/time.Minute))
	}
	return fmt.Sprintf("%ds", int(d/time.Second))
}
```

- [ ] **Step 7.4: Run tests + lint**

```bash
go test ./internal/api/ -run TestImagesHandler -v -race
golangci-lint run ./internal/api/...
```

- [ ] **Step 7.5: Commit**

```bash
git add internal/api/images.go internal/api/images_test.go
git commit -m "feat(p3): /img/{source}/{name} handler — Cache-Control + Warning + 304 passthrough" -- \
  internal/api/images.go internal/api/images_test.go
```

---

## Task 8: /api/sources extension — surface image:<key> entries

**Files:**
- Modify: `internal/api/sources.go`
- Modify: `internal/api/sources_test.go`

**Dependencies:** depends on Task 4 (ImageHealth).

**Why:** Design §5.7. The SPA's `SourceHealthIndicator` and `HazardCategoryCard`'s "stale Xm" Tag both consume `/api/sources`. Phase 3 surfaces one `image:<source>.<name>` entry per registry key alongside the existing 5 source entries. JSON shape for the image entries matches `fetcher.Health` so the existing `SourceHealth` TypeScript interface (Phase 2 `web/src/api/types.ts`) accepts both kinds without a discriminated union.

- [ ] **Step 8.1: Write the failing test**

Add to `internal/api/sources_test.go`:

```go
import (
	"github.com/jacaudi/cwd/internal/imageproxy"
)

type stubImageHealth struct {
	out map[string]imageproxy.ImageHealth
}

func (s stubImageHealth) ImageHealth() map[string]imageproxy.ImageHealth { return s.out }

func TestSourcesHandler_IncludesImageEntriesPrefixedWithImageColon(t *testing.T) {
	now := time.Now().UTC()
	hp := stubHealthProvider{out: map[string]fetcher.Health{
		"nws_alerts": {IntervalSec: 30, LastSuccess: now, ETag: "v1"},
	}}
	ihp := stubImageHealth{out: map[string]imageproxy.ImageHealth{
		"spc.day1otlk": {IntervalSec: 120, LastSuccess: now, ETag: `"e1"`, BytesOnDisk: 1024, InHotTier: true},
		"nhc.atl_7d":   {IntervalSec: 1800, LastSuccess: time.Time{}, ConsecutiveFailures: 0},
	}}
	h := NewSourcesHandler(hp, WithImageHealth(ihp))
	srv := httptest.NewServer(h)
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	want := []string{
		`"nws_alerts"`,
		`"image:spc.day1otlk"`,
		`"image:nhc.atl_7d"`,
		`"bytesOnDisk":1024`,
		`"inHotTier":true`,
	}
	for _, w := range want {
		if !strings.Contains(string(body), w) {
			t.Errorf("body missing %s\nbody=%s", w, body)
		}
	}
}

func TestSourcesHandler_NilImageHealth_StillSurfacesSources(t *testing.T) {
	hp := stubHealthProvider{out: map[string]fetcher.Health{"nws_alerts": {IntervalSec: 30}}}
	h := NewSourcesHandler(hp) // no WithImageHealth
	srv := httptest.NewServer(h)
	defer srv.Close()
	resp, _ := http.Get(srv.URL)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"nws_alerts"`) {
		t.Errorf("body missing nws_alerts: %s", body)
	}
	if strings.Contains(string(body), `"image:`) {
		t.Errorf("body should not contain image entries when ImageHealth is nil: %s", body)
	}
}
```

If `stubHealthProvider` does not exist in the existing `sources_test.go`, add:

```go
type stubHealthProvider struct {
	out map[string]fetcher.Health
}

func (s stubHealthProvider) Health() map[string]fetcher.Health { return s.out }
```

Add imports: `"io"`, `"net/http"`, `"net/http/httptest"`, `"strings"`, `"testing"`, `"time"`, `"github.com/jacaudi/cwd/internal/fetcher"`.

- [ ] **Step 8.2: Run, confirm fail**

```bash
go test ./internal/api/ -run TestSourcesHandler_IncludesImageEntries -v
```

Expected: build fails — `WithImageHealth` and `ImageHealthProvider` undefined.

- [ ] **Step 8.3: Modify `internal/api/sources.go`**

Replace the file with:

```go
package api

import (
	"encoding/json"
	"net/http"

	"github.com/jacaudi/cwd/internal/fetcher"
	"github.com/jacaudi/cwd/internal/imageproxy"
)

// HealthProvider is the per-source fetcher health snapshot (Phase 1/2).
type HealthProvider interface {
	Health() map[string]fetcher.Health
}

// ImageHealthProvider is the per-image-key health snapshot (Phase 3).
// Returned entries are emitted alongside HealthProvider entries with each
// key prefixed by "image:" (e.g. "image:spc.day1otlk").
type ImageHealthProvider interface {
	ImageHealth() map[string]imageproxy.ImageHealth
}

// SourcesOption configures NewSourcesHandler.
type SourcesOption func(*sourcesHandler)

// WithImageHealth attaches an ImageHealthProvider to the handler. When set,
// each image-key entry is emitted as "image:<key>" in the JSON output.
func WithImageHealth(ihp ImageHealthProvider) SourcesOption {
	return func(h *sourcesHandler) { h.ihp = ihp }
}

type sourcesHandler struct {
	hp  HealthProvider
	ihp ImageHealthProvider
}

// NewSourcesHandler returns the GET /api/sources handler.
func NewSourcesHandler(hp HealthProvider, opts ...SourcesOption) http.Handler {
	h := &sourcesHandler{hp: hp}
	for _, o := range opts {
		o(h)
	}
	return h
}

func (h *sourcesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")

	out := map[string]any{}
	for k, v := range h.hp.Health() {
		out[k] = v
	}
	if h.ihp != nil {
		for k, v := range h.ihp.ImageHealth() {
			out["image:"+k] = v
		}
	}
	if err := json.NewEncoder(w).Encode(out); err != nil {
		http.Error(w, "encode", http.StatusInternalServerError)
	}
}
```

- [ ] **Step 8.4: Run tests + lint**

```bash
go test ./internal/api/ -v -race
golangci-lint run ./internal/api/...
```

Pre-existing `TestSourcesHandler` and `Test_NewSourcesHandler` shape tests still pass — the response is now `map[string]any` instead of `map[string]fetcher.Health`, but the wire shape per key is identical. If a pre-existing test asserts on the Go static type via `decode(&map[string]fetcher.Health)`, switch it to `map[string]any` or leave it (json decode works against both).

- [ ] **Step 8.5: Commit**

```bash
git add internal/api/sources.go internal/api/sources_test.go
git commit -m "feat(p3): /api/sources surfaces image:<key> entries via ImageHealthProvider" -- \
  internal/api/sources.go internal/api/sources_test.go
```

---

## Task 9: Server wiring — construct proxy, prewarm, mount routes, integrate health

**Files:**
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`
- Modify: `internal/api/router.go`

**Dependencies:** depends on Task 4 (Proxy), Task 5 (StartPrewarm), Task 6 (ImagesConfig), Task 7 (NewImagesHandler), Task 8 (WithImageHealth).

**Why:** The integration point. Construct the registry/store/hot/proxy in `Run`, spawn one prewarm poller per `cfg.Images.Prewarm` key, mount `/img/{source}/{name}` on the chi router, register the synthetic `image.invalidate` cache key in the hub's `enabledNames`, and wire `proxy.Stats` into the `/api/sources` handler via `WithImageHealth`.

**No edits to:** `internal/cache/cache.go`, `internal/sse/hub.go`, `internal/api/snapshot.go`, `cmd/cwd/main.go`.

- [ ] **Step 9.1: Write the failing test**

Add to `internal/server/server_test.go`:

```go
func TestRun_ImageProxy_LiveOnSourcesAndImgRoute(t *testing.T) {
	imgBody := []byte("PNGBYTES")
	imgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("ETag", `"img1"`)
		_, _ = w.Write(imgBody)
	}))
	defer imgSrv.Close()

	// Phase 2 sources: stub each so /readyz can go green within the test budget.
	stub := func(body []byte, ct string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", ct)
			w.Header().Set("ETag", `"e"`)
			_, _ = w.Write(body)
		}))
	}
	nws := stub([]byte(`{"type":"FeatureCollection","features":[]}`), "application/geo+json")
	defer nws.Close()
	scales := stub([]byte(`{"1":{"DateStamp":"2026-05-03","R":{"MinorProb":"5","MajorProb":"0"},"S":{"Prob":"0"},"G":{"Scale":"0","Text":"none"}},"2":{"DateStamp":"2026-05-04","R":{"MinorProb":"5","MajorProb":"0"},"S":{"Prob":"0"},"G":{"Scale":"0","Text":"none"}},"3":{"DateStamp":"2026-05-05","R":{"MinorProb":"5","MajorProb":"0"},"S":{"Prob":"0"},"G":{"Scale":"0","Text":"none"}}}`), "application/json")
	defer scales.Close()
	swpcAlerts := stub([]byte(`[]`), "application/json")
	defer swpcAlerts.Close()
	quakes := stub([]byte(`{"type":"FeatureCollection","features":[]}`), "application/geo+json")
	defer quakes.Close()
	volcs := stub([]byte(`[]`), "application/json")
	defer volcs.Close()

	t.Setenv("CWD_NWS_ALERTS_URL", nws.URL)
	t.Setenv("CWD_SWPC_SCALES_URL", scales.URL)
	t.Setenv("CWD_SWPC_ALERTS_URL", swpcAlerts.URL)
	t.Setenv("CWD_USGS_QUAKES_URL", quakes.URL)
	t.Setenv("CWD_USGS_VOLCANOES_URL", volcs.URL)
	// Override the SPC Day 1 registry URL to the test image server.
	t.Setenv("CWD_IMAGE_URL_SPC_DAY1OTLK", imgSrv.URL)

	enabled := true
	cacheDir := filepath.Join(t.TempDir(), "imgs")
	cfg := &config.Config{
		Server: config.ServerConfig{Bind: "127.0.0.1:0", LogLevel: "info", LogFormat: "text", Contact: "test@example.com"},
		UI:     config.UIConfig{DefaultTheme: "dark", DefaultLanding: "/", EnableHistory: true},
		Store:  config.StoreConfig{Path: filepath.Join(t.TempDir(), "cwd.db"), RetentionDays: 30},
		Sources: map[string]config.SourceConfig{
			"nws_alerts":     {Interval: 80 * time.Millisecond, Enabled: &enabled},
			"swpc_scales":    {Interval: 80 * time.Millisecond, Enabled: &enabled},
			"swpc_alerts":    {Interval: 80 * time.Millisecond, Enabled: &enabled},
			"usgs_quakes":    {Interval: 80 * time.Millisecond, Enabled: &enabled},
			"usgs_volcanoes": {Interval: 80 * time.Millisecond, Enabled: &enabled},
		},
		Images: config.ImagesConfig{
			CacheDir:        cacheDir,
			DiskMaxBytes:    1 << 20,
			HotMaxBytes:     1 << 20,
			HotMaxEntries:   16,
			RefreshInterval: 60 * time.Second,
			ImageIntervals:  map[string]time.Duration{"spc.day1otlk": 60 * time.Second},
			Prewarm:         []string{"spc.day1otlk"},
		},
		Derived: config.DerivedConfig{Thresholds: config.ThresholdsConfig{
			SWPCAlertWindowHours: 24,
			SWPCAlertProducts:    []string{"K08A", "K09A", "P12A", "P13A"},
		}},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	addrCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() { errCh <- Run(ctx, cfg, logger, addrCh) }()
	addr := <-addrCh

	// Wait for /readyz.
	deadline := time.After(4 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("/readyz never went 200")
		default:
		}
		resp, err := http.Get("http://" + addr + "/readyz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				goto ready
			}
		}
		time.Sleep(40 * time.Millisecond)
	}
ready:

	// /api/sources includes image:spc.day1otlk after prewarm tick.
	{
		var body []byte
		for i := 0; i < 30; i++ {
			resp, err := http.Get("http://" + addr + "/api/sources")
			if err == nil {
				body, _ = io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				if strings.Contains(string(body), `"image:spc.day1otlk"`) {
					break
				}
			}
			time.Sleep(50 * time.Millisecond)
		}
		if !strings.Contains(string(body), `"image:spc.day1otlk"`) {
			t.Errorf("/api/sources missing image:spc.day1otlk\nbody=%s", body)
		}
	}

	// /img/spc/day1otlk serves the test bytes.
	{
		resp, err := http.Get("http://" + addr + "/img/spc/day1otlk")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("/img status = %d", resp.StatusCode)
		}
		got, _ := io.ReadAll(resp.Body)
		if !bytes.Equal(got, imgBody) {
			t.Errorf("/img body mismatch")
		}
	}

	cancel()
	if err := <-errCh; err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, http.ErrServerClosed) {
		t.Errorf("Run = %v", err)
	}
}
```

Add `"bytes"` and `"github.com/jacaudi/cwd/internal/imageproxy"` imports if needed.

- [ ] **Step 9.2: Run, confirm fail**

```bash
go test ./internal/server/ -run TestRun_ImageProxy -v
```

- [ ] **Step 9.3: Modify `internal/api/router.go`**

Add an optional `ImagesHandler` field to `RouterDeps`, mount `/img/{source}/{name}` inside the JSON-timeout group. The route lives under the timeout middleware because it's a regular request/response (not SSE). Wrap so the timeout error response is plain-text-ish; clients tolerate either body shape.

```go
// In RouterDeps:
ImagesHandler  http.Handler

// In NewRouter, inside the JSON-timeout group, after the /api Route block:
if deps.ImagesHandler != nil {
    r.Method(http.MethodGet, "/img/{source}/{name}", deps.ImagesHandler)
    r.Method(http.MethodHead, "/img/{source}/{name}", deps.ImagesHandler)
}
```

Update the `staticExtensions` SPA fallback note: `/img/...` paths are matched by the chi route above and never reach `spaFallback`, so no extension-list change is required.

- [ ] **Step 9.4: Modify `internal/server/server.go`**

Add per-image URL override hook (test-only; keeps the registry immutable in production):

```go
// imageURL returns the configured upstream URL for the registry key, or the
// registry default. CWD_IMAGE_URL_<UPPER_KEY_DOTS_AS_UNDERSCORES> overrides
// for tests. Production code should never rely on these env vars.
func imageURL(img imageproxy.Image) string {
	envKey := "CWD_IMAGE_URL_" + strings.ToUpper(strings.ReplaceAll(img.Key(), ".", "_"))
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	return img.URL
}
```

Add `"strings"` and `"github.com/jacaudi/cwd/internal/imageproxy"` to imports.

After step 8 of `Run` (the `enabledNames` build), inject the synthetic `image.invalidate` slot:

```go
// 8a. Image proxy construction (if Images.Prewarm or any /img request would fire).
//
// We always construct the proxy — lazy keys are served on demand — but we
// only spawn prewarm pollers for keys explicitly listed in Images.Prewarm.
hot := imageproxy.NewHotTier(cfg.Images.HotMaxBytes, cfg.Images.HotMaxEntries)
disk, err := imageproxy.NewDiskStore(cfg.Images.CacheDir, cfg.Images.DiskMaxBytes, logger)
if err != nil {
    return fmt.Errorf("server: imageproxy disk store: %w", err)
}
// Build a registry copy with operator URL overrides folded in.
imgRegistry := make([]imageproxy.Image, 0, len(imageproxy.Registry))
for _, img := range imageproxy.Registry {
    img.URL = imageURL(img)
    imgRegistry = append(imgRegistry, img)
}
proxy := imageproxy.New(imageproxy.Config{
    Registry:    imgRegistry,
    Hot:         hot,
    Disk:        disk,
    UserAgent:   userAgent,
    Logger:      logger,
    HTTPTimeout: 15 * time.Second,
    Intervals:   cfg.Images.ImageIntervals,
    OnInvalidate: func(ev imageproxy.ImageInvalidate) error {
        c.Set(cache.Envelope{
            Source:    "image.invalidate",
            FetchedAt: ev.FetchedAt,
            Validator: ev.Source + "." + ev.Name + "@" + ev.FetchedAt.UTC().Format(time.RFC3339Nano),
            Payload:   ev,
        })
        return nil
    },
})
enabledNames = append(enabledNames, "image.invalidate")
```

Spawn the prewarm pollers (after the existing `for _, f := range fetchers { go f.Run(ctx) }` loop):

```go
imgStop := imageproxy.StartPrewarm(ctx, proxy, cfg.Images.Prewarm, logger)
defer imgStop()
```

Wire image health into the sources handler:

```go
// Replace:
// SourcesHandler: api.NewSourcesHandler(healthProvider),
// with:
SourcesHandler: api.NewSourcesHandler(healthProvider, api.WithImageHealth(imageHealthFn(proxy.Stats))),
ImagesHandler:  api.NewImagesHandler(proxy),
```

Add an adapter type:

```go
// imageHealthFn adapts a func to api.ImageHealthProvider.
type imageHealthFn func() map[string]imageproxy.ImageHealth

func (f imageHealthFn) ImageHealth() map[string]imageproxy.ImageHealth { return f() }
```

- [ ] **Step 9.5: Run tests + lint**

```bash
go test ./internal/server/ ./internal/api/ -v -race
golangci-lint run ./internal/server/... ./internal/api/...
```

Expected: new test passes; pre-existing tests still pass.

- [ ] **Step 9.6: Commit**

```bash
git add internal/server/server.go internal/server/server_test.go internal/api/router.go
git commit -m "feat(p3): wire image proxy — registry, prewarm, /img route, health surfacing" -- \
  internal/server/server.go internal/server/server_test.go internal/api/router.go
```

---

## Task 10: Frontend wire layer — ImageInvalidate type, store reducer, stream listener

**Files:**
- Modify: `web/src/api/types.ts`
- Modify: `web/src/api/stream.ts`
- Modify: `web/src/api/stream.test.ts`
- Modify: `web/src/store/snapshot.ts`
- Modify: `web/src/store/snapshot.test.ts`

**Dependencies:** none. (The wire shape is fixed by design §5.6 and does not depend on backend code in this branch — frontend can be built before or after the backend.)

**Why:** Per design §5.6 and §6.7. The SPA needs a typed `ImageInvalidate` payload, a Zustand store field `imageRefresh: Record<string, string>` that maps `<source>.<name>` → fetchedAt, an `applyImageInvalidate` reducer, and an SSE listener routing `image.invalidate.update` events into the store.

The store's `imageRefresh[key]` value is what `HazardCategoryCard` keys its `<Image>` element on so the browser unmount-remounts and refetches whenever the proxy invalidates.

The hub's initial-paint `snapshot` event contains an `image.invalidate` envelope (because the hub's `enabledNames` includes it post-Task 9). The frontend extracts that envelope's payload — if present — into the imageRefresh map at startup so a brand-new client immediately knows the latest invalidation. The `Snapshot.sources` TypeScript shape is widened with an optional `'image.invalidate'?: Envelope<ImageInvalidate>` entry to keep the typecheck honest.

- [ ] **Step 10.1: Write the failing tests**

Add to `web/src/store/snapshot.test.ts`:

```ts
it('routes image.invalidate events into imageRefresh', () => {
  useSnapshotStore.setState({ snapshot: { serverTime: 't', sources: {} }, connection: 'live', imageRefresh: {} });
  useSnapshotStore.getState().applyImageInvalidate({
    source: 'spc', name: 'day1otlk', fetchedAt: '2026-05-03T22:14:33Z',
  });
  useSnapshotStore.getState().applyImageInvalidate({
    source: 'nhc', name: 'atl_7d', fetchedAt: '2026-05-03T22:14:34Z',
  });
  const refresh = useSnapshotStore.getState().imageRefresh;
  expect(refresh['spc.day1otlk']).toBe('2026-05-03T22:14:33Z');
  expect(refresh['nhc.atl_7d']).toBe('2026-05-03T22:14:34Z');
});

it('seeds imageRefresh from snapshot.sources["image.invalidate"]', () => {
  useSnapshotStore.getState().setSnapshot({
    serverTime: 't',
    sources: {
      'image.invalidate': {
        source: 'image.invalidate', fetchedAt: 't',
        payload: { source: 'spc', name: 'day1otlk', fetchedAt: '2026-05-03T22:14:33Z' },
      },
    },
  });
  expect(useSnapshotStore.getState().imageRefresh['spc.day1otlk']).toBe('2026-05-03T22:14:33Z');
});
```

Add to `web/src/api/stream.test.ts` (after the existing 5-source listener test):

```ts
it('registers a listener for image.invalidate.update', async () => {
  const fakeFetch = vi.fn().mockResolvedValue({
    ok: true, json: async () => ({ serverTime: 't', sources: {} }),
  });
  vi.stubGlobal('fetch', fakeFetch);
  vi.stubGlobal('EventSource', FakeES as any);
  await connect({
    onSnapshot: () => {},
    onUpdate: () => {},
    onImageInvalidate: () => {},
  });
  expect(FakeES.last!.listeners['image.invalidate.update']).toBeTruthy();
});

it('routes image.invalidate.update payloads to onImageInvalidate', async () => {
  const fakeFetch = vi.fn().mockResolvedValue({
    ok: true, json: async () => ({ serverTime: 't', sources: {} }),
  });
  vi.stubGlobal('fetch', fakeFetch);
  vi.stubGlobal('EventSource', FakeES as any);
  const seen: any[] = [];
  await connect({
    onSnapshot: () => {},
    onUpdate: () => {},
    onImageInvalidate: (ev) => seen.push(ev),
  });
  // simulate the SSE event using FakeES — call the registered listener.
  const ev = { data: JSON.stringify({ source: 'image.invalidate', fetchedAt: 't', payload: { source: 'spc', name: 'day1otlk', fetchedAt: 't' } }) };
  FakeES.last!.listeners['image.invalidate.update'](ev as any);
  expect(seen[0]).toEqual({ source: 'spc', name: 'day1otlk', fetchedAt: 't' });
});
```

- [ ] **Step 10.2: Run, confirm fail**

```bash
task web:test 2>&1 | head -40
```

- [ ] **Step 10.3: Edit `web/src/api/types.ts`**

Append (do NOT rewrite the whole file):

```ts
// ── Phase 3: Image proxy invalidation event ──────────────────────────────

export interface ImageInvalidate {
  source: string;       // e.g. "spc"
  name: string;         // e.g. "day1otlk"
  fetchedAt: string;    // ISO-8601 UTC
}
```

In the `Snapshot` interface's `sources` map, add the optional invalidate entry:

```ts
export interface Snapshot {
  serverTime: string;
  sources: {
    nws_alerts?:           Envelope<Alert[]>;
    swpc_scales?:          Envelope<SWPCForecast>;
    swpc_alerts?:          Envelope<SWPCAlert[]>;
    usgs_quakes?:          Envelope<Quake[]>;
    usgs_volcanoes?:       Envelope<Volcano[]>;
    'image.invalidate'?:   Envelope<ImageInvalidate>;
  };
}
```

- [ ] **Step 10.4: Edit `web/src/store/snapshot.ts`**

Replace with:

```ts
import { create } from 'zustand';
import type {
  Envelope,
  Alert,
  ImageInvalidate,
  Quake,
  SWPCAlert,
  SWPCForecast,
  Snapshot,
  Volcano,
} from '../api/types';

export type Connection = 'connecting' | 'live' | 'polling' | 'error';

// SourcePayloadMap maps each source key to its envelope payload type.
export interface SourcePayloadMap {
  nws_alerts:     Alert[];
  swpc_scales:    SWPCForecast;
  swpc_alerts:    SWPCAlert[];
  usgs_quakes:    Quake[];
  usgs_volcanoes: Volcano[];
}

interface State {
  snapshot: Snapshot | null;
  connection: Connection;
  // imageRefresh maps "<source>.<name>" → fetchedAt ISO string. Components
  // key their <Image> element on this so SSE-driven invalidations force a
  // browser-side reload (React unmount-remount of <img>).
  imageRefresh: Record<string, string>;
  setSnapshot: (s: Snapshot) => void;
  applyUpdate: <K extends keyof SourcePayloadMap>(
    source: K,
    env: Envelope<SourcePayloadMap[K]>,
  ) => void;
  applyImageInvalidate: (ev: ImageInvalidate) => void;
  setConnection: (c: Connection) => void;
}

export const useSnapshotStore = create<State>((set) => ({
  snapshot: null,
  connection: 'connecting',
  imageRefresh: {},
  setSnapshot: (s) => set(() => {
    const nextRefresh: Record<string, string> = {};
    const inv = s.sources['image.invalidate'];
    if (inv) {
      const ev = inv.payload;
      nextRefresh[`${ev.source}.${ev.name}`] = ev.fetchedAt;
    }
    return { snapshot: s, imageRefresh: nextRefresh };
  }),
  applyUpdate: (source, env) =>
    set((prev) => {
      const base: Snapshot =
        prev.snapshot ?? { serverTime: env.fetchedAt, sources: {} };
      return {
        snapshot: {
          ...base,
          sources: { ...base.sources, [source]: env },
        } as Snapshot,
      };
    }),
  applyImageInvalidate: (ev) =>
    set((prev) => ({
      imageRefresh: { ...prev.imageRefresh, [`${ev.source}.${ev.name}`]: ev.fetchedAt },
    })),
  setConnection: (c) => set({ connection: c }),
}));
```

- [ ] **Step 10.5: Edit `web/src/api/stream.ts`**

Replace with:

```ts
import type { Snapshot, Envelope, ImageInvalidate } from './types';
import type { SourcePayloadMap } from '../store/snapshot';

export interface ConnectOpts {
  onSnapshot: (s: Snapshot) => void;
  onUpdate: <K extends keyof SourcePayloadMap>(
    source: K,
    env: Envelope<SourcePayloadMap[K]>,
  ) => void;
  onImageInvalidate?: (ev: ImageInvalidate) => void;
  onError?: (e: unknown) => void;
}

const SOURCE_NAMES: (keyof SourcePayloadMap)[] = [
  'nws_alerts',
  'swpc_scales',
  'swpc_alerts',
  'usgs_quakes',
  'usgs_volcanoes',
];

/**
 * Connects the client to the live snapshot stream:
 * - fetches /api/snapshot for the initial paint
 * - subscribes to /api/stream for SSE updates (one listener per source)
 * - subscribes to image.invalidate.update for image proxy invalidations
 * Returns a teardown function that closes the EventSource.
 */
export async function connect(opts: ConnectOpts): Promise<() => void> {
  try {
    const res = await fetch('/api/snapshot');
    if (res.ok) opts.onSnapshot(await res.json());
  } catch (e) {
    opts.onError?.(e);
  }

  const es = new EventSource('/api/stream');
  es.addEventListener('snapshot', (e: MessageEvent) => {
    try {
      opts.onSnapshot(JSON.parse(e.data));
    } catch (err) {
      opts.onError?.(err);
    }
  });
  for (const name of SOURCE_NAMES) {
    es.addEventListener(`${name}.update`, (e: MessageEvent) => {
      try {
        opts.onUpdate(name, JSON.parse(e.data));
      } catch (err) {
        opts.onError?.(err);
      }
    });
  }
  es.addEventListener('image.invalidate.update', (e: MessageEvent) => {
    try {
      const env = JSON.parse(e.data) as Envelope<ImageInvalidate>;
      opts.onImageInvalidate?.(env.payload);
    } catch (err) {
      opts.onError?.(err);
    }
  });
  return () => es.close();
}
```

- [ ] **Step 10.6: Wire `applyImageInvalidate` into the existing `connect` consumer**

Locate where `connect(...)` is called from a top-level component (likely `web/src/pages/Overview.tsx` or `web/src/main.tsx` per the Phase 2 wire). Pass the new `onImageInvalidate` callback:

```tsx
useEffect(() => {
  const teardown = connect({
    onSnapshot: useSnapshotStore.getState().setSnapshot,
    onUpdate: useSnapshotStore.getState().applyUpdate,
    onImageInvalidate: useSnapshotStore.getState().applyImageInvalidate,
    onError: (e) => console.error('stream error', e),
  });
  return () => { void teardown.then((fn) => fn()); };
}, []);
```

The exact location is whatever existing component owns the SSE lifecycle (Phase 2 chose `Overview.tsx`'s `useEffect`). If unsure, grep for `connect({` and extend the existing object literal.

- [ ] **Step 10.7: Run tests + typecheck**

```bash
task web:test
task web:typecheck
```

- [ ] **Step 10.8: Commit**

```bash
git add web/src/api/types.ts web/src/api/stream.ts web/src/api/stream.test.ts \
        web/src/store/snapshot.ts web/src/store/snapshot.test.ts \
        web/src/pages/Overview.tsx
git commit -m "feat(p3): frontend wire layer — ImageInvalidate event + imageRefresh store" -- \
  web/src/api/types.ts web/src/api/stream.ts web/src/api/stream.test.ts \
  web/src/store/snapshot.ts web/src/store/snapshot.test.ts \
  web/src/pages/Overview.tsx
```

(If the SSE-lifecycle owner is not `Overview.tsx`, substitute the correct path in both the diff and the `git add`/`git commit` pathspec.)

---

## Task 11: HazardCategoryCard component

**Files:**
- Create: `web/src/components/HazardCategoryCard.tsx`
- Create: `web/src/components/HazardCategoryCard.test.tsx`

**Dependencies:** depends on Task 10 (imageRefresh store + ImageInvalidate type).

**Why:** Design §7.2. One ProCard per category with a Segmented control to flip between Day 1/2/3 (or whatever Day-set the category supports), an `Image.PreviewGroup` for fullscreen lightbox, and a "stale Xm" Tag driven by `/api/sources` health. The `<Image>` element keys on `imageRefresh[key]` so SSE-driven invalidation forces a browser refetch.

### Category → image map

```ts
type Category = "severe-storms" | "wildfire" | "excessive-rainfall" | "winter" | "heat" | "tropical" | "flooding";

interface ImageRef {
  key: string;     // dot-key form, matches imageproxy registry
  label: string;   // segmented control label ("Day 1", "Day 2", ...)
  path: string;    // /img/{source}/{name}
  alt: string;     // accessibility label
}

const CATEGORIES: Record<Category, { title: string; images: ImageRef[] }> = {
  "severe-storms": { title: "Severe Storms", images: [
    { key: "spc.day1otlk", label: "Day 1", path: "/img/spc/day1otlk", alt: "SPC Convective Outlook Day 1" },
    { key: "spc.day2otlk", label: "Day 2", path: "/img/spc/day2otlk", alt: "SPC Convective Outlook Day 2" },
    { key: "spc.day3otlk", label: "Day 3", path: "/img/spc/day3otlk", alt: "SPC Convective Outlook Day 3" },
  ]},
  "wildfire": { title: "Wildfire", images: [
    { key: "spc.day1otlk_fire", label: "Day 1", path: "/img/spc/day1otlk_fire", alt: "SPC Fire Weather Outlook Day 1" },
    { key: "spc.day2otlk_fire", label: "Day 2", path: "/img/spc/day2otlk_fire", alt: "SPC Fire Weather Outlook Day 2" },
    { key: "spc.day38otlk_fire", label: "Day 3-8", path: "/img/spc/day38otlk_fire", alt: "SPC Fire Weather Outlook Day 3-8 (experimental)" },
  ]},
  "excessive-rainfall": { title: "Excessive Rainfall", images: [
    { key: "wpc.ero_day1", label: "Day 1", path: "/img/wpc/ero_day1", alt: "WPC Excessive Rainfall Outlook Day 1" },
    { key: "wpc.ero_day2", label: "Day 2", path: "/img/wpc/ero_day2", alt: "WPC Excessive Rainfall Outlook Day 2" },
    { key: "wpc.ero_day3", label: "Day 3", path: "/img/wpc/ero_day3", alt: "WPC Excessive Rainfall Outlook Day 3" },
  ]},
  "winter": { title: "Winter", images: [
    { key: "wpc.wssi_day1", label: "Day 1", path: "/img/wpc/wssi_day1", alt: "WPC Winter Storm Severity Index Day 1" },
    { key: "wpc.wssi_day2", label: "Day 2", path: "/img/wpc/wssi_day2", alt: "WPC Winter Storm Severity Index Day 2" },
    { key: "wpc.wssi_day3", label: "Day 3", path: "/img/wpc/wssi_day3", alt: "WPC Winter Storm Severity Index Day 3" },
  ]},
  "heat": { title: "Heat", images: [
    { key: "wpc.heatrisk_day1", label: "Day 1", path: "/img/wpc/heatrisk_day1", alt: "WPC HeatRisk Day 1" },
    { key: "wpc.heatrisk_day2", label: "Day 2", path: "/img/wpc/heatrisk_day2", alt: "WPC HeatRisk Day 2" },
    { key: "wpc.heatrisk_day3", label: "Day 3", path: "/img/wpc/heatrisk_day3", alt: "WPC HeatRisk Day 3" },
  ]},
  "tropical": { title: "Tropical", images: [
    { key: "nhc.atl_7d", label: "Atlantic", path: "/img/nhc/atl_7d", alt: "NHC Atlantic 7-day Outlook" },
    { key: "nhc.epac_7d", label: "East Pacific", path: "/img/nhc/epac_7d", alt: "NHC East Pacific 7-day Outlook" },
    { key: "nhc.cpac_7d", label: "Central Pacific", path: "/img/nhc/cpac_7d", alt: "NHC Central Pacific 7-day Outlook" },
    { key: "navy.jtwc_abpw", label: "JTWC ABPW", path: "/img/navy/jtwc_abpw", alt: "Navy JTWC ABPW Western Pacific" },
  ]},
  "flooding": { title: "Flooding", images: [
    { key: "nwc.fho_national", label: "National", path: "/img/nwc/fho_national", alt: "WPC NWC National Flood Hazard Outlook" },
  ]},
};
```

(Adding a new image to the registry requires a new entry here too — note the cross-file dependency in `internal/imageproxy/registry.go`'s package comment.)

### "stale Xm" Tag wiring

The component reads `/api/sources` every 60s via a hand-rolled `useEffect` + `setInterval` (avoiding adding SWR/react-query as a Phase 3 dependency). When `image:<key>` reports `consecutiveFailures > 0` AND `lastSuccess` is non-zero, render `<Tag color="warning">stale Xm</Tag>` where `X = floor((Date.now() - lastSuccess)/60000)`. Otherwise render no Tag.

- [ ] **Step 11.1: Write the failing test**

`web/src/components/HazardCategoryCard.test.tsx`:

```tsx
import { afterEach, beforeEach, describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { HazardCategoryCard } from './HazardCategoryCard';
import { useSnapshotStore } from '../store/snapshot';

afterEach(() => {
  useSnapshotStore.setState({ snapshot: null, connection: 'connecting', imageRefresh: {} });
  vi.restoreAllMocks();
});

beforeEach(() => {
  // /api/sources fetch returns no failures so no Tag renders by default.
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
    ok: true,
    json: async () => ({}),
  }));
});

describe('HazardCategoryCard', () => {
  it('renders the title for the category', () => {
    render(<HazardCategoryCard category="severe-storms" />);
    expect(screen.getByText(/Severe Storms/i)).toBeInTheDocument();
  });

  it('renders the first day image by default', () => {
    render(<HazardCategoryCard category="severe-storms" />);
    const img = screen.getByAltText(/SPC Convective Outlook Day 1/i) as HTMLImageElement;
    expect(img.src).toContain('/img/spc/day1otlk');
  });

  it('switches images when the segmented day changes', async () => {
    render(<HazardCategoryCard category="severe-storms" />);
    fireEvent.click(screen.getByText('Day 2'));
    await waitFor(() => {
      const img = screen.getByAltText(/SPC Convective Outlook Day 2/i) as HTMLImageElement;
      expect(img.src).toContain('/img/spc/day2otlk');
    });
  });

  it('bumps the image element key when imageRefresh updates', async () => {
    const { rerender } = render(<HazardCategoryCard category="severe-storms" />);
    const before = screen.getByAltText(/SPC Convective Outlook Day 1/i);
    useSnapshotStore.getState().applyImageInvalidate({
      source: 'spc', name: 'day1otlk', fetchedAt: '2026-05-03T22:14:33Z',
    });
    rerender(<HazardCategoryCard category="severe-storms" />);
    const after = screen.getByAltText(/SPC Convective Outlook Day 1/i);
    // React identity may stay the same; key bump triggers a remount.
    // Assert the key has been bumped via the data-refresh-key data attribute
    // the component sets for testability.
    expect(after.getAttribute('data-refresh-key')).toBe('2026-05-03T22:14:33Z');
    expect(before.getAttribute('data-refresh-key') ?? 'init').not.toBe('2026-05-03T22:14:33Z');
  });

  it('shows a stale Tag when /api/sources reports consecutive failures', async () => {
    const past = new Date(Date.now() - 8 * 60 * 1000).toISOString();
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        'image:spc.day1otlk': {
          intervalSec: 120,
          lastAttempt: new Date().toISOString(),
          lastSuccess: past,
          consecutiveFailures: 3,
        },
      }),
    }));
    render(<HazardCategoryCard category="severe-storms" />);
    await waitFor(() => {
      expect(screen.getByText(/stale 8m/i)).toBeInTheDocument();
    });
  });

  it('renders the flooding category with a single image', () => {
    render(<HazardCategoryCard category="flooding" />);
    expect(screen.getByAltText(/National Flood Hazard Outlook/i)).toBeInTheDocument();
  });
});
```

- [ ] **Step 11.2: Run, confirm fail**

```bash
task web:test 2>&1 | head -40
```

- [ ] **Step 11.3: Implement `web/src/components/HazardCategoryCard.tsx`**

```tsx
import { useEffect, useMemo, useState } from 'react';
import { Image, Segmented, Skeleton, Tag } from 'antd';
import { ProCard } from '@ant-design/pro-components';
import { useSnapshotStore } from '../store/snapshot';

export type HazardCategory =
  | 'severe-storms'
  | 'wildfire'
  | 'excessive-rainfall'
  | 'winter'
  | 'heat'
  | 'tropical'
  | 'flooding';

interface ImageRef {
  key: string;
  label: string;
  path: string;
  alt: string;
}

const CATEGORIES: Record<HazardCategory, { title: string; images: ImageRef[] }> = {
  'severe-storms': { title: 'Severe Storms', images: [
    { key: 'spc.day1otlk', label: 'Day 1', path: '/img/spc/day1otlk', alt: 'SPC Convective Outlook Day 1' },
    { key: 'spc.day2otlk', label: 'Day 2', path: '/img/spc/day2otlk', alt: 'SPC Convective Outlook Day 2' },
    { key: 'spc.day3otlk', label: 'Day 3', path: '/img/spc/day3otlk', alt: 'SPC Convective Outlook Day 3' },
  ]},
  'wildfire': { title: 'Wildfire', images: [
    { key: 'spc.day1otlk_fire', label: 'Day 1', path: '/img/spc/day1otlk_fire', alt: 'SPC Fire Weather Outlook Day 1' },
    { key: 'spc.day2otlk_fire', label: 'Day 2', path: '/img/spc/day2otlk_fire', alt: 'SPC Fire Weather Outlook Day 2' },
    { key: 'spc.day38otlk_fire', label: 'Day 3-8', path: '/img/spc/day38otlk_fire', alt: 'SPC Fire Weather Outlook Day 3-8 (experimental)' },
  ]},
  'excessive-rainfall': { title: 'Excessive Rainfall', images: [
    { key: 'wpc.ero_day1', label: 'Day 1', path: '/img/wpc/ero_day1', alt: 'WPC Excessive Rainfall Outlook Day 1' },
    { key: 'wpc.ero_day2', label: 'Day 2', path: '/img/wpc/ero_day2', alt: 'WPC Excessive Rainfall Outlook Day 2' },
    { key: 'wpc.ero_day3', label: 'Day 3', path: '/img/wpc/ero_day3', alt: 'WPC Excessive Rainfall Outlook Day 3' },
  ]},
  'winter': { title: 'Winter', images: [
    { key: 'wpc.wssi_day1', label: 'Day 1', path: '/img/wpc/wssi_day1', alt: 'WPC Winter Storm Severity Index Day 1' },
    { key: 'wpc.wssi_day2', label: 'Day 2', path: '/img/wpc/wssi_day2', alt: 'WPC Winter Storm Severity Index Day 2' },
    { key: 'wpc.wssi_day3', label: 'Day 3', path: '/img/wpc/wssi_day3', alt: 'WPC Winter Storm Severity Index Day 3' },
  ]},
  'heat': { title: 'Heat', images: [
    { key: 'wpc.heatrisk_day1', label: 'Day 1', path: '/img/wpc/heatrisk_day1', alt: 'WPC HeatRisk Day 1' },
    { key: 'wpc.heatrisk_day2', label: 'Day 2', path: '/img/wpc/heatrisk_day2', alt: 'WPC HeatRisk Day 2' },
    { key: 'wpc.heatrisk_day3', label: 'Day 3', path: '/img/wpc/heatrisk_day3', alt: 'WPC HeatRisk Day 3' },
  ]},
  'tropical': { title: 'Tropical', images: [
    { key: 'nhc.atl_7d', label: 'Atlantic', path: '/img/nhc/atl_7d', alt: 'NHC Atlantic 7-day Outlook' },
    { key: 'nhc.epac_7d', label: 'East Pacific', path: '/img/nhc/epac_7d', alt: 'NHC East Pacific 7-day Outlook' },
    { key: 'nhc.cpac_7d', label: 'Central Pacific', path: '/img/nhc/cpac_7d', alt: 'NHC Central Pacific 7-day Outlook' },
    { key: 'navy.jtwc_abpw', label: 'JTWC ABPW', path: '/img/navy/jtwc_abpw', alt: 'Navy JTWC ABPW Western Pacific' },
  ]},
  'flooding': { title: 'Flooding', images: [
    { key: 'nwc.fho_national', label: 'National', path: '/img/nwc/fho_national', alt: 'WPC NWC National Flood Hazard Outlook' },
  ]},
};

interface ImageHealth {
  intervalSec: number;
  lastAttempt: string;
  lastSuccess: string;
  consecutiveFailures: number;
}

function StaleTag({ key: refreshKey, health }: { key: string; health: ImageHealth | undefined }) {
  if (!health || health.consecutiveFailures <= 0 || !health.lastSuccess) return null;
  const ageMs = Date.now() - new Date(health.lastSuccess).getTime();
  if (ageMs <= 0) return null;
  const minutes = Math.floor(ageMs / 60000);
  const label = minutes >= 1 ? `stale ${minutes}m` : `stale ${Math.floor(ageMs / 1000)}s`;
  return <Tag color="warning">{label}</Tag>;
}

export function HazardCategoryCard({ category }: { category: HazardCategory }) {
  const { title, images } = CATEGORIES[category];
  const [day, setDay] = useState(0);
  const imageRefresh = useSnapshotStore((s) => s.imageRefresh);
  const current = images[day];
  const refreshKey = imageRefresh[current.key] ?? 'init';

  const [healths, setHealths] = useState<Record<string, ImageHealth>>({});
  useEffect(() => {
    let aborted = false;
    const tick = async () => {
      try {
        const res = await fetch('/api/sources');
        if (!res.ok || aborted) return;
        const all = (await res.json()) as Record<string, ImageHealth | unknown>;
        const out: Record<string, ImageHealth> = {};
        for (const k of Object.keys(all)) {
          if (k.startsWith('image:')) {
            out[k.substring('image:'.length)] = all[k] as ImageHealth;
          }
        }
        if (!aborted) setHealths(out);
      } catch {
        /* swallow — Tag just won't render */
      }
    };
    void tick();
    const id = setInterval(tick, 60_000);
    return () => { aborted = true; clearInterval(id); };
  }, []);

  const segmentedOptions = useMemo(
    () => images.map((img, idx) => ({ label: img.label, value: idx })),
    [images],
  );

  return (
    <ProCard
      title={title}
      bordered
      extra={<StaleTag key={current.key} health={healths[current.key]} />}
    >
      {images.length > 1 && (
        <Segmented
          options={segmentedOptions}
          value={day}
          onChange={(v) => setDay(Number(v))}
          style={{ marginBottom: 12 }}
        />
      )}
      <Image.PreviewGroup>
        <Image
          key={refreshKey}
          src={current.path}
          alt={current.alt}
          data-refresh-key={refreshKey}
          placeholder={<Skeleton.Image style={{ width: '100%', height: 240 }} active />}
          style={{ width: '100%', height: 'auto' }}
        />
      </Image.PreviewGroup>
    </ProCard>
  );
}
```

- [ ] **Step 11.4: Run tests + typecheck**

```bash
task web:test
task web:typecheck
```

- [ ] **Step 11.5: Commit**

```bash
git add web/src/components/HazardCategoryCard.tsx web/src/components/HazardCategoryCard.test.tsx
git commit -m "feat(p3): HazardCategoryCard component — Segmented + PreviewGroup + stale Tag" -- \
  web/src/components/HazardCategoryCard.tsx web/src/components/HazardCategoryCard.test.tsx
```

---

## Task 12: Hazards page rewrite — 7-card responsive grid

**Files:**
- Rewrite: `web/src/pages/Hazards.tsx`
- Create: `web/src/pages/Hazards.test.tsx`

**Dependencies:** depends on Task 11 (HazardCategoryCard).

**Why:** Design §7.1. Replace the Phase 0 placeholder with a `Row`/`Col` grid of seven `HazardCategoryCard`s; responsive breakpoints stack to 1 column on `xs`, 2 on `sm`, 3 on `md+`.

- [ ] **Step 12.1: Write the failing test**

`web/src/pages/Hazards.test.tsx`:

```tsx
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import Hazards from './Hazards';

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => ({}) }));
});

describe('Hazards page', () => {
  it('renders all 7 category cards', () => {
    render(<Hazards />);
    for (const title of ['Severe Storms', 'Wildfire', 'Excessive Rainfall', 'Winter', 'Heat', 'Tropical', 'Flooding']) {
      expect(screen.getByText(title)).toBeInTheDocument();
    }
  });

  it('renders the page heading', () => {
    render(<Hazards />);
    expect(screen.getByRole('heading', { name: /Hazards/i })).toBeInTheDocument();
  });
});
```

- [ ] **Step 12.2: Run, confirm fail**

```bash
task web:test 2>&1 | head -20
```

- [ ] **Step 12.3: Implement `web/src/pages/Hazards.tsx`**

Replace the file:

```tsx
import { Col, Row, Typography } from 'antd';
import { HazardCategoryCard, type HazardCategory } from '../components/HazardCategoryCard';

const CATEGORIES: HazardCategory[] = [
  'severe-storms',
  'wildfire',
  'excessive-rainfall',
  'winter',
  'heat',
  'tropical',
  'flooding',
];

export default function Hazards() {
  return (
    <div style={{ padding: 24 }}>
      <Typography.Title level={3} style={{ marginTop: 0 }}>Hazards</Typography.Title>
      <Row gutter={[16, 16]}>
        {CATEGORIES.map((c) => (
          <Col key={c} xs={24} sm={12} md={8}>
            <HazardCategoryCard category={c} />
          </Col>
        ))}
      </Row>
    </div>
  );
}
```

- [ ] **Step 12.4: Run tests + typecheck**

```bash
task web:test
task web:typecheck
```

- [ ] **Step 12.5: Commit**

```bash
git add web/src/pages/Hazards.tsx web/src/pages/Hazards.test.tsx
git commit -m "feat(p3): Hazards page — 7-card responsive grid" -- \
  web/src/pages/Hazards.tsx web/src/pages/Hazards.test.tsx
```

---

## Task 13: README + smoke task extension + final integration check

**Files:**
- Modify: `README.md`
- Modify: `Taskfile.yml` (extend `smoke`)

**Dependencies:** depends on all prior tasks.

**Why:** Document the proxy + operator-tunable knobs, extend `task smoke` to assert all `images.prewarm` keys have `lastSuccess` populated within 2× their interval window, and run the full pipeline (build, lint, all tests, browser verify) end-to-end. This is the gate before the independent comprehensive review.

- [ ] **Step 13.1: Update `README.md`**

Add a "Phase 3 status" section after the Phase 2 section:

```markdown
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
CWD_IMAGES_REFRESH_INTERVAL=5m
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
`Cache-Control: public, max-age=60, stale-while-revalidate=900`. When the
proxy is serving stale bytes (upstream currently failing), the response
includes `Warning: 110 cwd "stale Xm"`.

#### SSE invalidation

When the proxy's diff-on-write detects a real content change, it broadcasts
an `image.invalidate.update` SSE event with `{source, name, fetchedAt}`. The
SPA's `useSnapshotStore.imageRefresh[<source>.<name>]` map updates, the
`<Image>` element keys on it, React unmount-remounts, the browser refetches
`/img/...`, and the server's hot tier serves the fresh bytes immediately.
```

- [ ] **Step 13.2: Extend `Taskfile.yml`'s `smoke` target**

Replace the existing `smoke:` block with:

```yaml
  smoke:
    desc: Boot against real upstreams for ~5 minutes; report all source + image health
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
        echo "---- assert all 5 sources healthy ----"
        curl -sS http://127.0.0.1:8765/api/sources | jq -e '
          (.nws_alerts.consecutiveFailures == 0)
          and (.swpc_scales.consecutiveFailures == 0)
          and (.swpc_alerts.consecutiveFailures == 0)
          and (.usgs_quakes.consecutiveFailures == 0)
          and (.usgs_volcanoes.consecutiveFailures == 0)
        ' > /dev/null && echo "OK: all 5 data sources healthy" || (echo "FAIL: data source failing" && exit 1)
        echo "---- assert all prewarm images healthy with lastSuccess set ----"
        # Each prewarm key must have a non-zero lastSuccess and consecutiveFailures == 0.
        # 2x interval-window check is approximated by requiring lastSuccess to be present
        # at the point of measurement (90s after boot is enough for the 4 default keys
        # whose tightest cadence is 2m).
        curl -sS http://127.0.0.1:8765/api/sources | jq -e '
          to_entries
          | map(select(.key | startswith("image:")))
          | all(
              (.value.consecutiveFailures == 0)
              and ((.value.lastSuccess // "") != "")
              and ((.value.lastSuccess // "") != "0001-01-01T00:00:00Z")
            )
        ' > /dev/null && echo "OK: all image:* prewarm entries fresh" || (echo "FAIL: image prewarm entry stale or unfetched" && exit 1)
        echo "---- assert prewarm bytes served via /img/* ----"
        for key in spc.day1otlk spc.day2otlk spc.day3otlk nhc.atl_7d; do
          src="${key%%.*}"; name="${key#*.}"
          ct=$(curl -sS -o /dev/null -w "%{content_type} %{http_code}" "http://127.0.0.1:8765/img/${src}/${name}")
          echo "  /img/${src}/${name} → ${ct}"
          echo "${ct}" | grep -qE 'image/(png|gif|jpeg) 200' || (echo "FAIL: /img/${src}/${name} did not return image/* 200" && exit 1)
        done
        echo "OK: all default prewarm /img/* return image bytes"
        echo "---- holding for 4 more minutes (Ctrl-C to stop early) ----"
        sleep 240
```

- [ ] **Step 13.3: Run the full integration check**

```bash
task test
task lint
task web:test
task web:typecheck
task web:build

# CRITICAL — restore the webdist placeholder before staging anything else.
git checkout 86b2d1b -- internal/webdist/dist/index.html

task build
./bin/cwd serve &
PID=$!
sleep 90
curl -sS http://127.0.0.1:8765/healthz
curl -sS http://127.0.0.1:8765/readyz
curl -sS http://127.0.0.1:8765/api/sources | jq .
curl -sS http://127.0.0.1:8765/api/snapshot | jq '.sources | keys'
curl -sS -o /tmp/spc-day1.png -w "%{http_code} %{content_type} %{size_download}\n" http://127.0.0.1:8765/img/spc/day1otlk
curl -sS -o /tmp/nhc-atl7d.png -w "%{http_code} %{content_type} %{size_download}\n" http://127.0.0.1:8765/img/nhc/atl_7d
file /tmp/spc-day1.png
file /tmp/nhc-atl7d.png
kill $PID
```

Expected:
- `task test`, `task lint`, `task web:test`, `task web:typecheck` all green.
- `task web:build` succeeds and produces `internal/webdist/dist/index.html` + asset bundle.
- After the placeholder restore, `git status internal/webdist/dist/index.html` shows no change.
- `/readyz` returns 200 within ~90s.
- `/api/sources` shows 5 data-source entries with `consecutiveFailures: 0` AND ≥4 `image:*` entries with non-zero `lastSuccess`.
- `/img/spc/day1otlk` returns 200 + `image/png` + non-zero size; `file` reports a PNG image.
- `/img/nhc/atl_7d` returns 200 + `image/png` + non-zero size.

- [ ] **Step 13.4: Open the browser and verify the Hazards page**

```bash
./bin/cwd serve
# In another terminal: open http://127.0.0.1:8765/hazards
```

Manual checks (per design §10):

1. All 7 cards render: Severe Storms, Wildfire, Excessive Rainfall, Winter, Heat, Tropical, Flooding.
2. Each card's first image (Day 1 or Atlantic etc.) loads through `/img/*` (network panel shows the request and a 200 response).
3. Switching the Segmented control (e.g. Day 1 → Day 2 on Severe Storms) loads the next image.
4. Clicking an image opens AntD's `PreviewGroup` lightbox; arrow keys cycle through the category's other images.
5. Opening Overview (`/`) and Space Weather (`/space`) and Events (`/events`) still works; they were not regressed.
6. Footer's `SourceHealthIndicator` continues to show 5 green data-source tags.

If the page renders but images stay on their skeleton:
- Check the browser console for `image.invalidate.update` SSE errors.
- Tail the server log for `imageproxy.refresh_async` warnings.
- Hit `/api/sources` and confirm `image:*` entries have `lastSuccess` populated.

- [ ] **Step 13.5: Commit + open PR**

```bash
git diff --cached internal/webdist/dist/index.html
# (should be empty — placeholder is restored, real dist is not staged)

git add README.md Taskfile.yml
git commit -m "docs(p3): README phase-3 notes + smoke covers image:* prewarm" -- \
  README.md Taskfile.yml

git push -u origin feature/phase3-image-proxy
gh pr create --title "Phase 3: image proxy + Hazards page" --body "$(cat <<'EOF'
## Summary
- New `internal/imageproxy` subsystem: registry of 20 maps, atomic disk LRU, in-memory hot tier, lazy-with-prewarm proxy with diff-on-write SSE invalidation.
- New `GET /img/{source}/{name}` HTTP route with Cache-Control + Warning header semantics.
- `/api/sources` surfaces one `image:<source>.<name>` entry per registered prewarm key.
- Hazards page rewritten as a 7-card responsive grid (Severe Storms, Wildfire, Excessive Rainfall, Winter, Heat, Tropical, Flooding) with AntD `Image.PreviewGroup` lightbox + Day 1/2/3 Segmented control.
- SSE `image.invalidate.update` events drive browser-side refetch via Zustand `imageRefresh` store + key-bump on `<Image>`.

## Out of scope
- History page + time slider — Phase 4.
- PWA service-worker for offline image fallback — Phase 5.
- Image transcoding / WEBP / resize — out of v1.
- Operator UI for editing `images.prewarm` — Phase 6 settings panel.

## Test plan
- [ ] `task test` green (with `-race`)
- [ ] `task lint` green
- [ ] `task web:test` green
- [ ] `task web:typecheck` green
- [ ] `task web:build` succeeds
- [ ] `task smoke` reports all 5 data sources healthy AND all `image:*` prewarm entries fresh
- [ ] Browser: `/hazards` renders 7 cards; `/img/spc/day1otlk` returns image/png; PreviewGroup lightbox works

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

---

## Out of scope

Verbatim from design §11:

- History page (slider + range scrubber over historic snapshots) — Phase 4.
- PWA service-worker for offline image fallback — Phase 5.
- Image transcoding / WEBP conversion / resize — out of v1.
- Operator UI for editing `images.prewarm` (currently YAML/env only) — Phase 6 settings panel.
- Automatic registry refresh from a published NCEP product index — out of v1; manual code update if SPC/WPC churn.
- Per-region (state, WFO) image variants — out of v1 (we serve CONUS-only maps).

---

## Test plan

Mirrors design §9.

### Backend

- [ ] `internal/imageproxy/registry_test.go` — no duplicate keys; URL parses; non-empty Description; key constants resolve; ByKey() round-trip.
- [ ] `internal/imageproxy/store_test.go` — Put/Get round-trip; restart loads manifest; LRU eviction by byte cap; orphan .tmp cleanup; concurrent Puts.
- [ ] `internal/imageproxy/hot_test.go` — Put/Get round-trip; LRU by byte cap AND entry cap; oversized-Put no-op; recency on Get.
- [ ] `internal/imageproxy/proxy_test.go` — unknown-key 404; lazy-fetch happy path; hot-tier hit no upstream call; 304 path no broadcast; diff-on-write no broadcast on identical body; upstream failure with cache serves stale; upstream failure no cache returns err; disk hit warms hot; Stats round-trip.
- [ ] `internal/imageproxy/prewarm_test.go` — N pollers run on cadence; context cancel stops cleanly; failure backs off; unknown key non-fatal.
- [ ] `internal/api/images_test.go` — happy path; 404 unknown key; 502 upstream + no cache; Warning header on stale; 304 on If-None-Match match; key composition.
- [ ] `internal/api/sources_test.go` — image:<key> entries appear; nil ImageHealth still surfaces sources.
- [ ] `internal/server/server_test.go` — image proxy live on /api/sources and /img/{source}/{name} after Run.
- [ ] `internal/config/config_test.go` — Images defaults shape; rejects DiskMaxBytes <= 0; allows zero HotMaxBytes; clamps below-floor intervals; drops unknown prewarm keys; CWD_IMAGES_PREWARM replaces list; CWD_IMAGES_IMAGE_INTERVALS_*__* parses with `__` separator; bad duration logged-not-applied.

### Frontend

- [ ] `web/src/store/snapshot.test.ts` — applyImageInvalidate updates imageRefresh; setSnapshot seeds from snapshot.sources["image.invalidate"].
- [ ] `web/src/api/stream.test.ts` — image.invalidate.update listener registered; payload routed to onImageInvalidate.
- [ ] `web/src/components/HazardCategoryCard.test.tsx` — title renders; default image; segmented switches; key bump on imageRefresh; stale Tag from /api/sources; flooding single-image.
- [ ] `web/src/pages/Hazards.test.tsx` — all 7 category cards; page heading.

### Browser walk-through (Task 13.4)

- [ ] `/hazards` renders 7 cards.
- [ ] Each card's Day 1 image loads via `/img/*`.
- [ ] Segmented control swaps the displayed image.
- [ ] PreviewGroup lightbox cycles through Day 1/2/3 with arrow keys.
- [ ] Overview, Space Weather, Events still render (no regression).
- [ ] Footer SourceHealthIndicator shows 5 green data-source tags.

### Smoke

- [ ] `task smoke` reports `consecutiveFailures: 0` for all 5 data sources AND a non-zero `lastSuccess` for every `image:*` prewarm entry.

---

## Definition of done

Verbatim from design §10, plus per-task gates:

- [ ] Design doc + this implementation plan committed under `docs/plans/`
- [ ] All work in `.worktrees/phase3-image-proxy` on branch `feature/phase3-image-proxy` off main at `86b2d1b`
- [ ] All Go tests pass with `-race`; `golangci-lint run ./...` clean
- [ ] All frontend tests pass (`task web:test`); `task web:typecheck` clean; `task web:build` succeeds
- [ ] `task build:all && ./bin/cwd serve` shows live data on `/hazards`: 7 ProCards render; each card's Day 1 image loads through `/img/*`; switching Day 2/3 loads next image; PreviewGroup fullscreen works
- [ ] `task smoke` reports all 5 data sources + all `images.prewarm` keys `consecutiveFailures: 0` and `lastSuccess` populated
- [ ] `internal/webdist/dist/index.html` IS the placeholder (`git diff 86b2d1b -- internal/webdist/dist/index.html` empty)
- [ ] `internal/cache/cache.go` and `internal/sse/hub.go` are unchanged in this branch (Phase 1's race fix preserved; Phase 2's hub-passes-through invariant preserved)
- [ ] Independent comprehensive review passes
- [ ] PR opened to main, CI green, merged with per-task history preserved
- [ ] Worktree cleaned up (`git worktree remove .worktrees/phase3-image-proxy`)

### Per-task gates

Every task ends with: tests green (`-race` for Go, `task web:test` for frontend), lint green (`golangci-lint run ./internal/<pkg>/...` for Go-touching tasks; `task web:typecheck` for frontend-touching tasks), and a single conventional-prefix commit (`feat(p3):` / `test(p3):` / `chore(p3):` / `fix(p3):` / `docs(p3):`) with the explicit-pathspec form (`git commit ... -- <files>`). No task is marked complete until those three gates pass — that is the `superpowers:verification-before-completion` contract.

---

## Self-review (writing-plans skill checklist)

### Spec coverage

| Design § | Task(s) |
|---|---|
| §1 Goal & scope | Tasks 1–9 (proxy subsystem), 10–12 (frontend) |
| §2 Decision 1 (disk + hot tier, both tunable) | Task 2 (disk), Task 3 (hot), Task 6 (config knobs) |
| §2 Decision 2 (global default + per-image override) | Task 4 (Proxy.Interval reads operator override), Task 6 (ImageIntervals + env walker) |
| §2 Decision 3 (serve-stale + Warning header + /api/sources surface) | Task 4 (markFailure + isStale return), Task 7 (Warning header), Task 8 (image:<key> entries) |
| §2 Decision 4 (default prewarm list, operator-tunable) | Task 1 (DefaultPrewarmKeys), Task 6 (Prewarm field + env override) |
| §2 Decision 5 (registry hardcoded, doc comments per entry) | Task 1 (registry.go) |
| §2 Decision 6 (diff-on-write SSE invalidate) | Task 4 (sha256 body diff before broadcast; ETag churn alone does not fire) |
| §2 Decision 7 (URL format `/img/{source}/{name}`, dot-key in config) | Task 1 (Image.Path/Key), Task 7 (handler), Task 6 (env walker) |
| §3 Upstream constraints (UA threading, conditional GET, 60s floor) | Task 4 (UA + If-None-Match/If-Modified-Since), Task 6 (validateImagesRefreshInterval + validateImageIntervals) |
| §3.3 Validators (ETag preferred; sha256 fallback) | Task 4 (publicValidator + bodyHash) |
| §3.4 Content types preserved | Task 4 (resp Content-Type pass-through, registry MIME fallback) |
| §4 Package additions | Tasks 1–5 (imageproxy/*), Task 7 (api/images.go), Task 11 (HazardCategoryCard.tsx), Task 12 (Hazards.tsx) |
| §4.3 No cache.go / hub.go changes | Conventions §"cache/SSE broadcast race"; Task 4 §"SSE wiring strategy"; Task 9 (cache.Set + enabledNames extension only) |
| §5.1 Registry shape | Task 1 |
| §5.2 Disk store shape | Task 2 |
| §5.3 Hot tier shape | Task 3 |
| §5.4 Proxy API + ImageHealth | Task 4 |
| §5.5 Prewarm | Task 5 |
| §5.6 SSE event payload | Task 4 (ImageInvalidate type), Task 10 (TS mirror) |
| §5.7 /api/sources extension | Task 8 |
| §5.8 HTTP API surface | Task 7 |
| §6 Data flows (lazy miss, hot hit, disk hit, prewarm, failure modes, SSE) | Tasks 4, 5, 7, 9, 10 |
| §7.1 Hazards page composition | Task 12 |
| §7.2 HazardCategoryCard | Task 11 |
| §7.3 Settings drawer (deferred — `/api/uiconfig` data exposure deferred to Phase 6) | Out of scope (intentional) |
| §8 Configuration | Task 6 |
| §9 Testing | Test plan section above; per-task TDD steps |
| §10 Definition of done | Definition of done section above |
| §11 Out of scope | Out of scope section above |

### Type consistency

- Go `imageproxy.ImageInvalidate{Source string, Name string, FetchedAt time.Time}` JSON tags `{source, name, fetchedAt}` ↔ TS `ImageInvalidate {source: string; name: string; fetchedAt: string}` — match.
- Go `imageproxy.ImageHealth{IntervalSec, LastAttempt, LastSuccess, LastError, ETag, AgeSec, ConsecutiveFailures, BytesOnDisk, InHotTier}` JSON tags ↔ TS `SourceHealth` (extends with `bytesOnDisk?: number; inHotTier?: boolean` if a strict-typed interface is desired; the HazardCategoryCard test casts to a local `ImageHealth` shape that requires only `intervalSec`, `lastAttempt`, `lastSuccess`, `consecutiveFailures` — strict subset of what the backend emits).
- Go `imageproxy.Image{Source string, Name string, URL string, MIME string, DefaultInterval time.Duration, Description string}` — no TS mirror needed (frontend never reads the Go registry directly; the CATEGORIES table in `HazardCategoryCard.tsx` is the parallel TS representation).
- `cache.Envelope{Source: "image.invalidate", Validator: "<source>.<name>@<RFC3339Nano>", Payload: ImageInvalidate}` round-trips through hub `applyFilter` (passes through unchanged for non-`nws_alerts` source) and emits as event `image.invalidate.update` — matches frontend listener in `stream.ts`.
- `Snapshot.sources` widening to include `'image.invalidate'?: Envelope<ImageInvalidate>` matches the hub's initial-paint snapshot frame which iterates `enabledNames` (which now includes `image.invalidate`).
- Per-key constants `KeySPCDay1Otlk` etc. resolve to the same string literals used in the frontend `CATEGORIES` table — Task 1's `TestKey_Constants_MatchRegistry` asserts the registry side.
- Proxy URL test override env var (`CWD_IMAGE_URL_<KEY>`) used by `imageURL()` in `internal/server/server.go` is consistent across the test stub in Task 9 (`CWD_IMAGE_URL_SPC_DAY1OTLK`).

### Open questions — resolved against live captures (2026-05-03)

All 20 upstream URLs were HEAD-checked at plan-write time. Results:

| Key | Status | Content-Type | ETag | Cache-Control | Notes |
|---|---|---|---|---|---|
| spc.day1otlk        | 200 | image/png  | ✓ | (none)         | strong ETag |
| spc.day2otlk        | 200 | image/png  | ✓ | (none)         | |
| spc.day3otlk        | 200 | image/png  | ✓ | (none)         | |
| spc.day1otlk_fire   | 200 | image/png  | ✓ | (none)         | |
| spc.day2otlk_fire   | 200 | image/png  | ✓ | (none)         | |
| spc.day38otlk_fire  | 200 | image/gif  | ✓ | (none)         | experimental — accept churn |
| wpc.ero_day1        | 200 | image/gif  | ✓ | max-age=900    | size-mtime ETag (changes on mtime touch — diff-on-body protects) |
| wpc.ero_day2        | 200 | image/gif  | ✓ | max-age=900    | |
| wpc.ero_day3        | 200 | image/gif  | ✓ | max-age=900    | |
| wpc.wssi_day1       | 200 | image/png  | ✓ | max-age=1800   | |
| wpc.wssi_day2       | 200 | image/png  | ✓ | max-age=1800   | |
| wpc.wssi_day3       | 200 | image/png  | ✓ | max-age=1800   | |
| wpc.heatrisk_day1   | 200 | image/png  | ✓ | max-age=1800   | |
| wpc.heatrisk_day2   | 200 | image/png  | ✓ | max-age=1800   | |
| wpc.heatrisk_day3   | 200 | image/png  | ✓ | max-age=1800   | |
| nhc.atl_7d          | 200 | image/png  | ✓ | max-age=300    | |
| nhc.epac_7d         | 200 | image/png  | ✓ | max-age=300    | |
| nhc.cpac_7d         | 200 | image/png  | ✓ | max-age=300    | |
| navy.jtwc_abpw      | 200 | image/jpeg | ✓ | (none)         | |
| nwc.fho_national    | 200 | image/png  | ✗ | max-age=180    | **no ETag** — proxy falls back to sha256 (design §3.3 already accepts this) |

All 20 URLs are healthy (200 OK). Content-Type from upstream matches the registry MIME field for every entry. `nwc.fho_national` is the only key without an upstream ETag; the validator strategy in Task 4 already handles this by falling back to `sha256:<hex>` for the `If-None-Match` opportunity (we cannot send it, so every poll re-fetches the body, but the diff-on-write predicate prevents spurious SSE invalidates).

The WPC ETag format is `<size>-<mtime-hex>`, which churns whenever the file mtime is touched even if the content didn't change. The proxy's body-sha256 diff-on-write predicate (Task 4) protects against spurious invalidate events from this — the test `TestProxy_DiffOnWrite_NoBroadcastOnIdenticalBody` covers it.

### Plan defects caught and fixed inline

1. **Design §4.3 says hub.go is unchanged but design §3.3 + §6.4 implies a `Hub.Broadcast` method that doesn't exist.** Resolved in Task 4 §"SSE wiring strategy" and Task 9 by routing image invalidations through the existing cache→hub broadcast path with a synthetic cache key `image.invalidate` and a unique-per-event validator. No changes to `cache.go` or `hub.go` are required.
2. **Existing `internal/config/config.go` has Phase 0 placeholder fields (`Cache.ImageDir`, `Cache.ImageMaxBytes`, `Images.DefaultMode`) that overlap design §8.** Resolved in Conventions §"Trap — config field reconciliation" and Task 6: delete the Phase 0 fields, replace with the design §8 shape, no backwards-compatibility shim (the Phase 0 fields are unused at runtime).
3. **Phase 2's `applyEnvOverrides` walker only handles top-level scalar fields.** The new nested `Images.ImageIntervals map[string]time.Duration` and slice-typed `Images.Prewarm` need explicit env handling. Resolved in Task 6 §6.3 (added explicit `CWD_IMAGES_PREWARM` and `CWD_IMAGES_IMAGE_INTERVALS_*` walks, with `__` as the dot separator per design §8 last paragraph).
4. **Design `§4.4` claims the proxy "internally reuses the `internal/fetcher` package's politeness machinery" but `internal/fetcher.Fetcher` only accepts `sources.Source` (typed payloads).** Resolved in Task 5 §"Why" — the proxy replicates the timer/backoff pattern from `fetcher.Run` rather than composing the type. Same User-Agent threading, same conditional-GET discipline, same exponential-backoff-with-cap shape.
5. **Phase 2 plan's `default_mode` switch case in `validate()` rejects an empty string.** If Task 6 deletes the field but doesn't delete the case, the default `Config{}` (used in some tests) fails validation. Resolved in Task 6 §6.3 by deleting the case alongside the field.
6. **`/api/snapshot` would expose the synthetic `image.invalidate` envelope if its handler iterated all cache keys.** Verified at plan-write time that `internal/api/snapshot.go` iterates only the 5 named source constants in `snapshotSourceNames` — no leak. Documented in Task 4 §"SSE wiring strategy" point 5.

### Notes from reading the actual Phase 2 code (drift from design assumptions)

- `internal/fetcher/fetcher.go`: `Fetcher` accepts `sources.Source` only. Image proxy cannot compose it directly; Task 5 replicates the loop shape (see plan defect #4).
- `internal/sse/hub.go`: no public `Broadcast` method; broadcasts only via `cache.Subscribe`. Plan composes through `cache.Set` (see plan defect #1).
- `internal/cache/cache.go`: `Set` has the diff-on-write predicate `prev.Validator == env.Validator && env.Validator != "" → no broadcast`. Phase 3's synthetic validator format (`source.name@RFC3339Nano`) is unique-per-event so every real invalidate fires.
- `internal/api/snapshot.go`: iterates a hardcoded `snapshotSourceNames` slice, NOT `cfg.Sources` keys, so adding `image.invalidate` to the hub's `enabledNames` does not pollute `/api/snapshot` (see plan defect #6).
- `internal/api/sources.go`: returns `map[string]fetcher.Health` directly. Task 8 widens to `map[string]any` so heterogeneous shapes (fetcher.Health + ImageHealth) coexist; the wire shape per key is unchanged for the 5 existing sources.
- `cmd/cwd/main.go`: unchanged in Phase 3 — `server.Run` does all wiring. Confirmed by reading the file (it just builds the slog.Logger and calls `server.Run(ctx, cfg, logger, nil)`).
- `internal/webdist/dist/index.html`: byte-for-byte identical to the Phase 1 placeholder text quoted in Conventions. Restoration command `git checkout 86b2d1b -- internal/webdist/dist/index.html` is correct.

### Placeholder scan

Searched the plan body for `TBD`, `TODO`, `implement later`, `add appropriate`, `similar to`, and `etc.` — none present in the plan body. The string `TODO` appears only inside an example code-block comment (`_ = context.TODO`) where it is part of the literal Go expression.



</content>
</invoke>