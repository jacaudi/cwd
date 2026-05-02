# Self-Hosted CWD — Phase 2 (Remaining four sources) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire `swpc_scales`, `swpc_alerts`, `usgs_quakes`, and `usgs_volcanoes` end-to-end using the Phase 1 contracts (`Source` / `Fetcher` / `Cache` / `Store` / `Hub` / `Snapshot`), light up the `/space` and `/events` pages, and migrate the Phase 1 `TsunamiPanel` from Overview to Events.

**Architecture:** Same shape as Phase 1 — one fetcher goroutine per enabled source doing diff-on-write to the typed cache + SQLite ring + SSE hub. Phase 2 adds 4 new `sources.Source` implementations, extends the snapshot handler to surface 4 new typed payloads, and adds 4 new frontend components consuming the live Zustand store. No interface changes anywhere.

**Tech Stack:**
- Backend: Go 1.24, `chi` router, `gopkg.in/yaml.v3`, `log/slog` stdlib, `modernc.org/sqlite`
- Frontend: Node 24, pnpm, Vite 5, React 18, TypeScript 5, AntD 5, `@ant-design/pro-components`, Zustand, Vitest + `@testing-library/react` + jsdom, `dayjs` (already a Phase 1 transitive dep via AntD)
- Tooling: `golangci-lint` v2, GitHub Actions via `jacaudi/github-actions` reusable workflows, Renovate, `Taskfile.yml` (go-task)

**Source documents:**
- Design: [`docs/plans/2026-05-02-phase2-multi-source-design.md`](2026-05-02-phase2-multi-source-design.md)
- Phase 1 design: [`docs/plans/2026-05-02-phase1-nws-alerts-design.md`](2026-05-02-phase1-nws-alerts-design.md)
- Phase 1 implementation (structural template): [`docs/plans/2026-05-02-phase1-nws-alerts-implementation.md`](2026-05-02-phase1-nws-alerts-implementation.md)
- Parent design: [`docs/plans/2026-05-01-self-hosted-cwd-design.md`](2026-05-01-self-hosted-cwd-design.md)
- Recon: [`docs/recon/2026-05-01-ncep-cwd-status-recon.md`](../recon/2026-05-01-ncep-cwd-status-recon.md) §5.1, §5.2, §5.4, §5.5, §8

**Branch:** `feature/phase2-multi-source` off main at the Phase 1 merge commit `0668675`. All work happens inside the worktree `.worktrees/phase2-multi-source`.

> **For Claude:** REQUIRED EXECUTION WORKFLOW (follow in order):
> 1. `superpowers:using-git-worktrees` — Isolate work in a dedicated worktree (`.worktrees/phase2-multi-source`)
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

- All commits prefixed `feat(p2):`, `test(p2):`, `chore(p2):`, `fix(p2):`, `docs(p2):`. Co-authored trailer per repo norm.
- All Go tests use stdlib `testing` + `httptest`; no external test frameworks. Table-driven where appropriate.
- All upstream HTTP must include `User-Agent: cwd-self-host/<version> (<contact>)`. Phase 1 already threads this via `buildUserAgent` in `internal/server/server.go`; new sources just inherit the string passed to their constructor.
- Logging: stdlib `log/slog`. Use structured key/value fields; never format into the message.
- No CGO. SQLite is `modernc.org/sqlite` exclusively.
- Frontend tests live next to the component (`Component.test.tsx`).
- Stage files **by name only**. Never `git add -A` or `git add .`.
- Never skip hooks (`--no-verify`, `--no-gpg-sign`).
- `golangci-lint` v2 must stay clean (`task lint`).

### Commit author identity

The branch and PR author identity is already configured at the repo level:

```
Author: jacaudi <47005674+jacaudi@users.noreply.github.com>
```

Do not run `git config user.email` or `git config user.name`. The privacy-block address (`47005674+jacaudi@…`) is required — using `adam.caudill@proton.me` as the author email will be rejected by GitHub's privacy block on push.

### Trap — webdist placeholder file (do NOT stage)

The repo's embed target `internal/webdist/dist/index.html` is a **committed placeholder** that exists so the Go package compiles before any frontend build. The file at the Phase 1 merge commit `0668675` is:

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

`task web:build` overwrites it with a real Vite build. **Never commit the real build artifact** — it bloats history and makes diffs unreviewable. After running `task web:build` (e.g. during the final integration check in Task 21), restore the placeholder before any `git add`:

```bash
git checkout 0668675 -- internal/webdist/dist/index.html
```

The branch point SHA `0668675` is the Phase 1 merge commit; that is the source of truth for the placeholder content during this branch's lifetime.

### Trap — cache/SSE broadcast race (do NOT touch)

Phase 1 fixed a real race in the cache→SSE broadcast path: `cache.Set` originally read the subscriber list outside `c.mu`, allowing a subscriber that was being closed concurrently to receive on a closed channel. The fixed pattern in `internal/cache/cache.go` snapshots the subscriber slice **inside** the lock, then iterates outside the lock, and each subscriber's `send` is guarded by `s.mu` against `closeOnce`. **Phase 2 does not change cache or hub broadcast logic.** If a Phase 2 task seems to require touching `cache.Set` / `Subscribe` / `Hub.Serve` / the ping ticker, stop and reconsider — every change in Phase 2 should compose with the existing pattern, not modify it.

### Per-source env override pattern

Phase 1 Task 14 wired the env walker (`applyEnvOverrides` in `internal/config/config.go`) to walk **all** keys of `cfg.Sources` and apply `CWD_SOURCES_<UPPER_NAME>_INTERVAL` and `CWD_SOURCES_<UPPER_NAME>_ENABLED`. Phase 2 adds zero env-walker code — the new source names slot in automatically the moment they appear in `cfg.Sources` (which they already do at `0668675` via `defaults()` in `internal/config/config.go`). For example:

```bash
CWD_SOURCES_SWPC_SCALES_INTERVAL=120s
CWD_SOURCES_SWPC_ALERTS_ENABLED=false
CWD_SOURCES_USGS_QUAKES_INTERVAL=30s
CWD_SOURCES_USGS_VOLCANOES_ENABLED=false
```

### Worktree setup

```bash
git worktree add -b feature/phase2-multi-source .worktrees/phase2-multi-source 0668675
cd .worktrees/phase2-multi-source
```

All subsequent commands assume CWD is the worktree.

---

## Dependency graph (visual)

```
1 swpc_scales fixture ─┐
2 swpc_alerts fixture ─┤
3 usgs_quakes fixtures ┤
4 usgs_volcanoes fix.  ─┤
                       ▼
              5 swpc_scales source ◀── 1
              6 swpc_alerts source ◀── 2
              7 usgs_quakes source ◀── 3
              8 usgs_volcanoes src ◀── 4
                       │
                       ▼
              9 hotStartDecode helper (independent of 5-8 in code, but tested against types from each)
                       │
                       ▼
             10 snapshot ext ◀── 5,6,7,8
             11 boot validation ext (independent — extends Phase 1 config layer)
             12 server wiring ◀── 5,6,7,8,9,10,11
                       │
                       ▼
             13 frontend wire layer (types+store+stream) ◀── (consumes shapes from 5-8 + 10)
                       │
       ┌───────────────┼───────────────┬────────────────┐
       ▼               ▼               ▼                ▼
14 SWPCForecast  15 SWPCAlerts  16 EarthquakeList  17 VolcanoList
   ◀── 13           ◀── 13         ◀── 13              ◀── 13
       │               │               │                │
       └──┬────────────┘               └─────┬──────────┘
          ▼                                  ▼
   18 SpaceWeather page              19 Events page
       ◀── 14,15                        ◀── 16,17 (also consumes Phase 1 TsunamiPanel)
                       │
                       ▼
             20 Overview update (drop TsunamiPanel, redirect Tsunami badge → /events#tsunami)
                       │
                       ▼
             21 README + smoke + final integration ◀── all
```

Shape: **fixtures fan in to per-source parsers, fan in to backend integration (hot-start + snapshot + server), fan in to frontend wire layer, fan in to four components, fan in to two pages, fan in to Overview, fan in to README/smoke.**

---

## Task 1: Capture SWPC noaa-scales test fixture

**Files:**
- Create: `internal/sources/testdata/swpc_scales_typical.json`
- Modify: `internal/sources/testdata/README.md`

**Dependencies:** none.

- [ ] **Step 1.1: Capture a live SWPC noaa-scales fixture**

```bash
curl -sS \
  -H 'User-Agent: cwd-self-host-fixture-capture/0.0 (adam.caudill@proton.me)' \
  -H 'Accept: application/json' \
  'https://services.swpc.noaa.gov/products/noaa-scales.json' \
  > internal/sources/testdata/swpc_scales_typical.json
```

The body is always populated (this endpoint always returns a 4-element object keyed `"0".."3"` with current + 3-day forecast). If the capture happens during a quiet period, all probabilities will be low — that's fine; tests don't depend on specific magnitudes.

- [ ] **Step 1.2: Sanity-check the fixture shape**

```bash
jq 'keys' internal/sources/testdata/swpc_scales_typical.json
# Expect: ["-1","0","1","2","3"] OR ["0","1","2","3"]  — accept either.
jq '."1"' internal/sources/testdata/swpc_scales_typical.json
# Expect: object with DateStamp, R, S, G subkeys.
```

If keys are missing, retry the capture. SWPC occasionally serves a stale/partial body during their internal cache rotation; a single retry usually resolves it.

- [ ] **Step 1.3: Document the fixture in the testdata README**

Append to `internal/sources/testdata/README.md`:

```markdown

## SWPC fixtures

| File | Captured | Notes |
|---|---|---|
| `swpc_scales_typical.json` | 2026-05-02 | Live capture of noaa-scales.json; keys "0".."3" populated. Recapture if SWPC reshapes the upstream contract. |
```

- [ ] **Step 1.4: Commit**

```bash
git add internal/sources/testdata/swpc_scales_typical.json \
        internal/sources/testdata/README.md
git commit -m "test(p2): capture SWPC noaa-scales fixture"
```

---

## Task 2: Capture SWPC alerts test fixture

**Files:**
- Create: `internal/sources/testdata/swpc_alerts_typical.json`
- Modify: `internal/sources/testdata/README.md`

**Dependencies:** none.

- [ ] **Step 2.1: Capture a live SWPC alerts fixture**

```bash
curl -sS \
  -H 'User-Agent: cwd-self-host-fixture-capture/0.0 (adam.caudill@proton.me)' \
  -H 'Accept: application/json' \
  'https://services.swpc.noaa.gov/products/alerts.json' \
  > internal/sources/testdata/swpc_alerts_typical.json
```

The endpoint returns a JSON array of `{product_id, issue_datetime, message}` objects spanning roughly the last 30 days. Capture is acceptable any time; the parser tests inject a fixed `time.Now()` to evaluate the 24h window deterministically against whatever capture date is on hand.

- [ ] **Step 2.2: Sanity-check**

```bash
jq 'length' internal/sources/testdata/swpc_alerts_typical.json
# Expect: > 0 (typical: 50-150 over 30 days).
jq '.[0]' internal/sources/testdata/swpc_alerts_typical.json
# Expect: object with product_id, issue_datetime, message keys at minimum.
jq '[.[] | .product_id] | unique' internal/sources/testdata/swpc_alerts_typical.json
# Expect: a mix of K-series, P-series, WARK*, SUM*, etc. codes.
```

If the capture has zero K/P/WARK products, run again later — the typical body always carries a healthy mix.

- [ ] **Step 2.3: Document the fixture**

Append to `internal/sources/testdata/README.md` under the SWPC section:

```markdown
| `swpc_alerts_typical.json` | 2026-05-02 | Live capture of alerts.json; ~30-day rolling window with mixed K/P/WARK/SUM products. |
```

- [ ] **Step 2.4: Commit**

```bash
git add internal/sources/testdata/swpc_alerts_typical.json \
        internal/sources/testdata/README.md
git commit -m "test(p2): capture SWPC alerts fixture"
```

---

## Task 3: Capture USGS earthquake test fixtures

**Files:**
- Create: `internal/sources/testdata/usgs_quakes_typical.geojson`
- Create: `internal/sources/testdata/usgs_quakes_empty.geojson`
- Modify: `internal/sources/testdata/README.md`

**Dependencies:** none.

- [ ] **Step 3.1: Capture a live USGS significant_day fixture**

```bash
curl -sS \
  -H 'User-Agent: cwd-self-host-fixture-capture/0.0 (adam.caudill@proton.me)' \
  -H 'Accept: application/geo+json' \
  'https://earthquake.usgs.gov/earthquakes/feed/v1.0/summary/significant_day.geojson' \
  > internal/sources/testdata/usgs_quakes_typical.geojson
```

The endpoint commonly returns 0–3 features in a quiet 24h window. If the capture has 0 features (`jq '.features | length'` → 0), continue to Step 3.2 anyway — the empty case is the second fixture's job and the typical capture's geometry/header still locks behavior. If you want a richer typical fixture, retry on a busier day.

- [ ] **Step 3.2: Hand-craft an empty-features fixture**

```bash
cat > internal/sources/testdata/usgs_quakes_empty.geojson <<'EOF'
{"type":"FeatureCollection","metadata":{"generated":1714651200000,"url":"https://earthquake.usgs.gov/earthquakes/feed/v1.0/summary/significant_day.geojson","title":"USGS Significant Earthquakes, Past Day","status":200,"api":"1.10.3","count":0},"features":[],"bbox":[]}
EOF
```

- [ ] **Step 3.3: Sanity-check**

```bash
jq '.type' internal/sources/testdata/usgs_quakes_typical.geojson
# Expect: "FeatureCollection"
jq '.features | length' internal/sources/testdata/usgs_quakes_empty.geojson
# Expect: 0
```

- [ ] **Step 3.4: Document the fixtures**

Append to `internal/sources/testdata/README.md`:

```markdown

## USGS fixtures

| File | Captured | Notes |
|---|---|---|
| `usgs_quakes_typical.geojson` | 2026-05-02 | Live capture of significant_day.geojson. Feature count varies 0–20 depending on day. |
| `usgs_quakes_empty.geojson` | 2026-05-02 | Hand-crafted, zero features. Locks the empty-day path. |
```

- [ ] **Step 3.5: Commit**

```bash
git add internal/sources/testdata/usgs_quakes_typical.geojson \
        internal/sources/testdata/usgs_quakes_empty.geojson \
        internal/sources/testdata/README.md
git commit -m "test(p2): capture USGS earthquake fixtures (typical + empty)"
```

---

## Task 4: Capture USGS volcano test fixture

**Files:**
- Create: `internal/sources/testdata/usgs_volcanoes_typical.json`
- Modify: `internal/sources/testdata/README.md`

**Dependencies:** none.

- [ ] **Step 4.1: Capture a live USGS getElevatedVolcanoes fixture**

```bash
curl -sS \
  -H 'User-Agent: cwd-self-host-fixture-capture/0.0 (adam.caudill@proton.me)' \
  -H 'Accept: application/json' \
  'https://volcanoes.usgs.gov/hans-public/api/volcano/getElevatedVolcanoes' \
  > internal/sources/testdata/usgs_volcanoes_typical.json
```

The endpoint returns a JSON array of volcanoes the USGS HANS system currently lists at any non-NORMAL alert level (typically 5–15 volcanoes, mostly Alaska). If the capture has zero entries (rare but possible during a globally quiet period), retry over the next 24 hours; in the meantime, parser tests tolerate empty arrays.

- [ ] **Step 4.2: Sanity-check**

```bash
jq 'type' internal/sources/testdata/usgs_volcanoes_typical.json
# Expect: "array"
jq 'length' internal/sources/testdata/usgs_volcanoes_typical.json
# Expect: > 0 (commonly 5-15)
jq '.[0] | keys' internal/sources/testdata/usgs_volcanoes_typical.json
# Expect: a key set including (at minimum) volcano name, alert/color level, latitude, longitude, region.
```

- [ ] **Step 4.3: Document the fixture**

Append to `internal/sources/testdata/README.md` (under the USGS section):

```markdown
| `usgs_volcanoes_typical.json` | 2026-05-02 | Live capture of getElevatedVolcanoes; non-NORMAL volcanoes only by upstream design. |
```

- [ ] **Step 4.4: Commit**

```bash
git add internal/sources/testdata/usgs_volcanoes_typical.json \
        internal/sources/testdata/README.md
git commit -m "test(p2): capture USGS volcano fixture"
```

---


## Task 5: `swpc_scales` source — types, parser, `Source` impl, tests

**Files:**
- Create: `internal/sources/swpc_scales.go`
- Create: `internal/sources/swpc_scales_test.go`

**Dependencies:** — depends on: Task 1 (SWPC scales fixture).

**Why:** First of four new sources. Implements the `internal/sources.Source` interface verbatim (no interface change). Reads `services.swpc.noaa.gov/products/noaa-scales.json`, takes only forecast indices `"1"`, `"2"`, `"3"` (today is intentionally skipped — see design §6.1), derives the worst G-scale from the per-day `G` block, and produces a `SWPCForecast` payload. Validator is a sha256 over the raw body (SWPC noaa-scales sends `Cache-Control: max-age=60` but no usable ETag — see recon §8).

- [ ] **Step 5.1: Write the failing tests**

Create `internal/sources/swpc_scales_test.go`:

```go
package sources

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSWPCScales_ParseFixture(t *testing.T) {
	body := loadFixture(t, "swpc_scales_typical.json")
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	got, err := ParseSWPCScales(body, logger)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got.Days) != 3 {
		t.Fatalf("want 3 forecast days, got %d", len(got.Days))
	}
	for i, d := range got.Days {
		if d.Date == "" {
			t.Errorf("day %d missing Date", i)
		}
		if d.R1 < 0 || d.R1 > 100 {
			t.Errorf("day %d R1 out of range: %d", i, d.R1)
		}
		if d.R3 < 0 || d.R3 > 100 {
			t.Errorf("day %d R3 out of range: %d", i, d.R3)
		}
		if d.S1 < 0 || d.S1 > 100 {
			t.Errorf("day %d S1 out of range: %d", i, d.S1)
		}
	}
}

func TestSWPCScales_DerivesWorstGScale(t *testing.T) {
	body := []byte(`{
		"0":{"DateStamp":"2026-05-02","R":{"MinorProb":"5","MajorProb":"1"},"S":{"Prob":"1"},"G":{"Scale":"0","Text":"none"}},
		"1":{"DateStamp":"2026-05-03","R":{"MinorProb":"10","MajorProb":"1"},"S":{"Prob":"1"},"G":{"Scale":"2","Text":"G2 expected"}},
		"2":{"DateStamp":"2026-05-04","R":{"MinorProb":"20","MajorProb":"5"},"S":{"Prob":"1"},"G":{"Scale":"4","Text":"G4 watch"}},
		"3":{"DateStamp":"2026-05-05","R":{"MinorProb":"15","MajorProb":"3"},"S":{"Prob":"1"},"G":{"Scale":"5","Text":"G5 extreme"}}
	}`)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	got, err := ParseSWPCScales(body, logger)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.Days[0].G != "G2" {
		t.Errorf("day 1 G = %q, want G2", got.Days[0].G)
	}
	if got.Days[1].G != "G4" {
		t.Errorf("day 2 G = %q, want G4", got.Days[1].G)
	}
	if got.Days[2].G != "G5" {
		t.Errorf("day 3 G = %q, want G5", got.Days[2].G)
	}
	if got.Days[0].GText != "G2 expected" {
		t.Errorf("day 1 GText = %q", got.Days[0].GText)
	}
}

func TestSWPCScales_GScaleZeroIsEmptyString(t *testing.T) {
	body := []byte(`{
		"1":{"DateStamp":"2026-05-03","R":{"MinorProb":"5","MajorProb":"0"},"S":{"Prob":"0"},"G":{"Scale":"0","Text":"none"}},
		"2":{"DateStamp":"2026-05-04","R":{"MinorProb":"5","MajorProb":"0"},"S":{"Prob":"0"},"G":{"Scale":"0","Text":"none"}},
		"3":{"DateStamp":"2026-05-05","R":{"MinorProb":"5","MajorProb":"0"},"S":{"Prob":"0"},"G":{"Scale":"0","Text":"none"}}
	}`)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	got, err := ParseSWPCScales(body, logger)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for i, d := range got.Days {
		if d.G != "" {
			t.Errorf("day %d: want G=\"\" when scale is 0, got %q", i, d.G)
		}
	}
}

func TestSWPCScales_ParseMalformedJSON(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	if _, err := ParseSWPCScales([]byte("not json"), logger); err == nil {
		t.Fatalf("want error, got nil")
	}
}

func TestSWPCScales_MissingDayIsZeroValued(t *testing.T) {
	// SWPC has been observed to drop a day under maintenance; defensive parse
	// should not panic — missing day stays zero-valued.
	body := []byte(`{
		"1":{"DateStamp":"2026-05-03","R":{"MinorProb":"5","MajorProb":"0"},"S":{"Prob":"0"},"G":{"Scale":"0","Text":"none"}},
		"3":{"DateStamp":"2026-05-05","R":{"MinorProb":"15","MajorProb":"3"},"S":{"Prob":"1"},"G":{"Scale":"2","Text":"G2"}}
	}`)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	got, err := ParseSWPCScales(body, logger)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.Days[0].Date != "2026-05-03" {
		t.Errorf("day 1 date wrong: %q", got.Days[0].Date)
	}
	if got.Days[1].Date != "" {
		t.Errorf("day 2 should be zero-valued, got Date=%q", got.Days[1].Date)
	}
	if got.Days[2].Date != "2026-05-05" {
		t.Errorf("day 3 date wrong: %q", got.Days[2].Date)
	}
}

func TestSWPCScales_FetchHTTP(t *testing.T) {
	body := loadFixture(t, "swpc_scales_typical.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); !strings.Contains(got, "cwd-self-host") {
			t.Errorf("missing/bad UA: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	src := NewSWPCScales(srv.URL, "cwd-self-host/test (test@example.com)", slog.New(slog.NewJSONHandler(io.Discard, nil)))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	res, err := src.Fetch(ctx)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if !strings.HasPrefix(res.Validator, "sha256:") {
		t.Errorf("validator should be sha256-prefixed: %q", res.Validator)
	}
	fc, ok := res.Payload.(SWPCForecast)
	if !ok {
		t.Fatalf("payload type %T not SWPCForecast", res.Payload)
	}
	if len(fc.Days) != 3 {
		t.Errorf("want 3 days, got %d", len(fc.Days))
	}
	res2, _ := src.Fetch(ctx)
	if res.Validator != res2.Validator {
		t.Errorf("validator not deterministic: %q vs %q", res.Validator, res2.Validator)
	}
}

func TestSWPCScales_NameAndInterval(t *testing.T) {
	src := NewSWPCScales("http://x", "ua", slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if src.Name() != "swpc_scales" {
		t.Errorf("Name() = %q, want swpc_scales", src.Name())
	}
	if src.Interval() != 60*time.Second {
		t.Errorf("default Interval() = %v, want 60s", src.Interval())
	}
	src.SetInterval(120 * time.Second)
	if src.Interval() != 120*time.Second {
		t.Errorf("SetInterval did not stick: %v", src.Interval())
	}
}
```

- [ ] **Step 5.2: Run, confirm fail**

```bash
go test ./internal/sources/ -run SWPCScales -v
# Expect: tests fail because ParseSWPCScales / SWPCForecast / NewSWPCScales not defined.
```

- [ ] **Step 5.3: Implement `internal/sources/swpc_scales.go`**

```go
package sources

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// SWPCScalesName is the canonical source name for the SWPC NOAA-Scales 3-day forecast.
const SWPCScalesName = "swpc_scales"

// GScale is a geomagnetic G-scale label ("G1".."G5"); zero scale returns "".
type GScale string

// SWPCDay is a single day of the SWPC NOAA scales forecast.
type SWPCDay struct {
	Date  string `json:"date"`            // "YYYY-MM-DD" UTC, copied from upstream DateStamp
	R1    int    `json:"r1"`              // % chance R1+ event (R MinorProb)
	R3    int    `json:"r3"`              // % chance R3+ event (R MajorProb)
	S1    int    `json:"s1"`              // % chance S1+ event (S Prob)
	G     GScale `json:"g,omitempty"`     // worst predicted G-scale (G1..G5, or "" when scale=0)
	GText string `json:"gText,omitempty"` // free-text note (G.Text)
}

// SWPCForecast is the parsed wire shape of /products/noaa-scales.json,
// covering the 3-day forecast (indices "1","2","3" upstream). The
// "current" (index "0") and "yesterday" (index "-1") slots are deliberately
// skipped in v1 — Phase 6 is when they get rendered.
type SWPCForecast struct {
	Days [3]SWPCDay `json:"days"`
}

// rawSWPC is the upstream JSON shape: a JSON object keyed by day-offset
// (string ints "-1".."3"). All sub-values are strings (SWPC quirk).
type rawSWPC map[string]rawSWPCDay

type rawSWPCDay struct {
	DateStamp string     `json:"DateStamp"`
	R         rawSWPCR   `json:"R"`
	S         rawSWPCS   `json:"S"`
	G         rawSWPCG   `json:"G"`
}

type rawSWPCR struct {
	MinorProb string `json:"MinorProb"`
	MajorProb string `json:"MajorProb"`
}

type rawSWPCS struct {
	Prob string `json:"Prob"`
}

type rawSWPCG struct {
	Scale string `json:"Scale"`
	Text  string `json:"Text"`
}

func atoiOrZero(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

func gScaleFrom(s string) GScale {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return ""
	}
	if n > 5 {
		n = 5
	}
	return GScale("G" + strconv.Itoa(n))
}

// ParseSWPCScales decodes the noaa-scales.json body into a SWPCForecast covering
// indices "1","2","3" (the 3-day forecast). Missing day keys produce zero-valued
// SWPCDay slots; malformed JSON returns an error. Logger is reserved for future
// drift-canary use; nil is tolerated.
func ParseSWPCScales(body []byte, logger *slog.Logger) (SWPCForecast, error) {
	if logger == nil {
		logger = slog.Default()
	}
	var raw rawSWPC
	if err := json.Unmarshal(body, &raw); err != nil {
		return SWPCForecast{}, fmt.Errorf("swpc_scales: unmarshal: %w", err)
	}
	var out SWPCForecast
	for i, key := range []string{"1", "2", "3"} {
		d, ok := raw[key]
		if !ok {
			continue // missing day → zero-valued slot
		}
		out.Days[i] = SWPCDay{
			Date:  d.DateStamp,
			R1:    atoiOrZero(d.R.MinorProb),
			R3:    atoiOrZero(d.R.MajorProb),
			S1:    atoiOrZero(d.S.Prob),
			G:     gScaleFrom(d.G.Scale),
			GText: d.G.Text,
		}
	}
	return out, nil
}

// SWPCScales implements Source for the SWPC NOAA-scales 3-day forecast.
type SWPCScales struct {
	url       string
	userAgent string
	interval  time.Duration
	client    *http.Client
	logger    *slog.Logger
}

// NewSWPCScales constructs a SWPCScales source.
func NewSWPCScales(url, userAgent string, logger *slog.Logger) *SWPCScales {
	return &SWPCScales{
		url:       url,
		userAgent: userAgent,
		interval:  60 * time.Second,
		client:    &http.Client{Timeout: 15 * time.Second},
		logger:    logger,
	}
}

// Name returns the source identifier.
func (s *SWPCScales) Name() string { return SWPCScalesName }

// Interval returns the polling interval.
func (s *SWPCScales) Interval() time.Duration { return s.interval }

// SetInterval is the boot-time setter used by the config env walker. Not safe
// for concurrent use once the fetcher is running.
func (s *SWPCScales) SetInterval(d time.Duration) { s.interval = d }

// Fetch retrieves the current SWPC NOAA-scales forecast and returns a SWPCForecast
// payload + sha256 content hash validator. SWPC's noaa-scales endpoint sends only
// Cache-Control: max-age=60 (no usable ETag), so content-hash is the right
// validator strategy here.
func (s *SWPCScales) Fetch(ctx context.Context) (FetchResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return FetchResult{}, fmt.Errorf("swpc_scales: build request: %w", err)
	}
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return FetchResult{}, fmt.Errorf("swpc_scales: do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return FetchResult{}, fmt.Errorf("swpc_scales: upstream status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return FetchResult{}, fmt.Errorf("swpc_scales: read body: %w", err)
	}
	fc, err := ParseSWPCScales(body, s.logger)
	if err != nil {
		return FetchResult{}, err
	}
	sum := sha256.Sum256(body)
	return FetchResult{
		Payload:   fc,
		Validator: "sha256:" + hex.EncodeToString(sum[:]),
	}, nil
}
```

- [ ] **Step 5.4: Run tests, lint, commit**

```bash
go test ./internal/sources/ -run SWPCScales -v -race
golangci-lint run ./internal/sources/...
git add internal/sources/swpc_scales.go internal/sources/swpc_scales_test.go
git commit -m "feat(p2): swpc_scales Source (parser + Fetch + tests)"
```

---
## Task 6: `swpc_alerts` source — types, code namespace comment, parser, `Source` impl, tests

**Files:**
- Create: `internal/sources/swpc_alerts.go`
- Create: `internal/sources/swpc_alerts_test.go`

**Dependencies:** — depends on: Task 2 (SWPC alerts fixture).

**Why:** Reads `services.swpc.noaa.gov/products/alerts.json`, applies the configured allowlist (`derived.thresholds.swpc_alert_products`, default `[K08A, K09A, P12A, P13A]`) AND a time-window filter (`derived.thresholds.swpc_alert_window_hours`, default 24h), resolves a human label via the embedded code table, truncates `message` to 512 bytes, and produces a slice of `SWPCAlert`. The product-code-namespace comment block from design §6.2 is committed verbatim into this file.

The constructor takes the allowlist, window duration, and an injectable `now func() time.Time` (defaults to `time.Now`) so the parser can be unit-tested deterministically against the captured fixture regardless of capture date.

- [ ] **Step 6.1: Write the failing tests**

Create `internal/sources/swpc_alerts_test.go`:

```go
package sources

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSWPCAlerts_AppliesAllowlistAndWindow(t *testing.T) {
	body := []byte(`[
		{"product_id":"K08A","issue_datetime":"2026-05-02 10:00:00.000","message":"Geomagnetic K=8 reached. Detail follows..."},
		{"product_id":"K05A","issue_datetime":"2026-05-02 09:00:00.000","message":"K=5 (G1)."},
		{"product_id":"P12A","issue_datetime":"2026-05-01 23:00:00.000","message":"Solar proton event observed."},
		{"product_id":"P13A","issue_datetime":"2026-04-28 12:00:00.000","message":"Older proton event — outside 24h window."},
		{"product_id":"WARK04W","issue_datetime":"2026-05-02 08:00:00.000","message":"Geomagnetic K>=4 expected."}
	]`)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	now := func() time.Time { return time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC) }
	allow := []string{"K08A", "K09A", "P12A", "P13A"}
	got, err := ParseSWPCAlerts(body, allow, 24*time.Hour, now, logger)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Want: K08A (in allow + within 24h) and P12A (in allow + within 24h)
	// Drop: K05A (not in allow), P13A (outside 24h), WARK04W (not in allow)
	if len(got) != 2 {
		t.Fatalf("want 2, got %d: %+v", len(got), got)
	}
	codes := []string{got[0].Code, got[1].Code}
	if !contains2(codes, "K08A") || !contains2(codes, "P12A") {
		t.Errorf("want K08A and P12A, got %v", codes)
	}
}

func TestSWPCAlerts_ResolvesSeriesAndDescription(t *testing.T) {
	body := []byte(`[
		{"product_id":"K08A","issue_datetime":"2026-05-02 10:00:00.000","message":"detail"},
		{"product_id":"P12A","issue_datetime":"2026-05-02 10:00:00.000","message":"detail"}
	]`)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	now := func() time.Time { return time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC) }
	got, err := ParseSWPCAlerts(body, []string{"K08A", "P12A"}, 24*time.Hour, now, logger)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, a := range got {
		if a.Series == "" {
			t.Errorf("%s missing Series", a.Code)
		}
		if a.Description == "" {
			t.Errorf("%s missing Description", a.Code)
		}
	}
}

func TestSWPCAlerts_TruncatesMessageTo512(t *testing.T) {
	long := strings.Repeat("x", 1024)
	body := []byte(`[{"product_id":"K08A","issue_datetime":"2026-05-02 10:00:00.000","message":"` + long + `"}]`)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	now := func() time.Time { return time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC) }
	got, err := ParseSWPCAlerts(body, []string{"K08A"}, 24*time.Hour, now, logger)
	if err != nil || len(got) != 1 {
		t.Fatalf("parse: %v, len=%d", err, len(got))
	}
	if len(got[0].Message) != 512 {
		t.Errorf("Message len = %d, want 512", len(got[0].Message))
	}
}

func TestSWPCAlerts_UnknownCodePassesThroughWithEmptyDescription(t *testing.T) {
	body := []byte(`[{"product_id":"ZZZ99X","issue_datetime":"2026-05-02 10:00:00.000","message":"..."}]`)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	now := func() time.Time { return time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC) }
	got, err := ParseSWPCAlerts(body, []string{"ZZZ99X"}, 24*time.Hour, now, logger)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
	if got[0].Description != "" {
		t.Errorf("unknown code should have empty description, got %q", got[0].Description)
	}
}

func TestSWPCAlerts_FixtureRoundTrip(t *testing.T) {
	body := loadFixture(t, "swpc_alerts_typical.json")
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	// Use a far-future "now" so the window filter doesn't drop everything;
	// then a wide window. The point is to exercise the parser against real
	// upstream shapes, not to assert specific counts.
	now := func() time.Time { return time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC) }
	allow := []string{"K05A", "K06A", "K07A", "K08A", "K09A", "P10A", "P11A", "P12A", "P13A", "WARK04W", "WARK05W", "WARK06W", "WARK07W", "WARK08W", "WARK09W"}
	got, err := ParseSWPCAlerts(body, allow, 365*24*time.Hour, now, logger)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, a := range got {
		if a.Code == "" {
			t.Errorf("alert missing Code: %+v", a)
		}
		if a.Issued.IsZero() {
			t.Errorf("alert %s missing Issued", a.Code)
		}
	}
}

func TestSWPCAlerts_ParseMalformedJSON(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	now := func() time.Time { return time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC) }
	if _, err := ParseSWPCAlerts([]byte("not json"), []string{"K08A"}, 24*time.Hour, now, logger); err == nil {
		t.Fatalf("want error, got nil")
	}
}

func TestSWPCAlerts_FetchHTTP(t *testing.T) {
	body := []byte(`[{"product_id":"K08A","issue_datetime":"2026-05-02 10:00:00.000","message":"x"}]`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	src := NewSWPCAlerts(srv.URL, "ua", []string{"K08A"}, 24*time.Hour, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	src.now = func() time.Time { return time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC) }
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	res, err := src.Fetch(ctx)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if !strings.HasPrefix(res.Validator, "sha256:") {
		t.Errorf("validator: %q", res.Validator)
	}
	alerts, ok := res.Payload.([]SWPCAlert)
	if !ok {
		t.Fatalf("payload type %T", res.Payload)
	}
	if len(alerts) != 1 {
		t.Errorf("want 1 alert, got %d", len(alerts))
	}
	if src.Name() != "swpc_alerts" {
		t.Errorf("Name() = %q", src.Name())
	}
	_ = io.Discard // keep import
}

func contains2(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
```

- [ ] **Step 6.2: Run, confirm fail**

```bash
go test ./internal/sources/ -run SWPCAlerts -v
# Expect: tests fail (types not defined yet).
```

- [ ] **Step 6.3: Implement `internal/sources/swpc_alerts.go`**

The product-code namespace comment block (per design §6.2) goes verbatim above `swpcCodeTable`.

```go
package sources

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// SWPCAlertsName is the canonical source name for the SWPC alerts feed.
const SWPCAlertsName = "swpc_alerts"

// swpcMessageMaxBytes truncates upstream message bodies to keep wire payloads small.
const swpcMessageMaxBytes = 512

// swpcIssueDatetimeLayout is the upstream timestamp shape: "2026-05-02 10:00:00.000".
const swpcIssueDatetimeLayout = "2006-01-02 15:04:05.000"

// SWPCAlert is a parsed SWPC alert (post-allowlist, post-window-filter, post-truncate).
type SWPCAlert struct {
	Code        string    `json:"code"`              // upstream product_id, e.g. "K08A", "P12A", "WARK04W"
	Series      string    `json:"series"`            // resolved label family, e.g. "K-Index", "Proton-Event"
	Description string    `json:"description"`       // resolved short human label; "" for unknown codes
	Issued      time.Time `json:"issued"`            // upstream issue_datetime parsed as UTC
	Message     string    `json:"message"`           // first 512 bytes of upstream message
	URL         string    `json:"url,omitempty"`     // reserved for future per-product permalinks
}

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
var swpcCodeTable = map[string]struct{ Series, Description string }{
	"K04A":    {"K-Index", "K-index 4 (active)"},
	"K05A":    {"K-Index", "K-index 5 = G1 minor"},
	"K06A":    {"K-Index", "K-index 6 = G2 moderate"},
	"K07A":    {"K-Index", "K-index 7 = G3 strong"},
	"K08A":    {"K-Index", "K-index 8 = G4 severe"},
	"K09A":    {"K-Index", "K-index 9 = G5 extreme"},
	"WARK04W": {"Geomagnetic-Watch", "K≥4 expected"},
	"WARK05W": {"Geomagnetic-Watch", "K≥5 expected"},
	"WARK06W": {"Geomagnetic-Watch", "K≥6 expected"},
	"WARK07W": {"Geomagnetic-Watch", "K≥7 expected"},
	"WARK08W": {"Geomagnetic-Watch", "K≥8 expected"},
	"WARK09W": {"Geomagnetic-Watch", "K≥9 expected"},
	"WATA50W": {"Geomagnetic-Watch", "A≥50 watch"},
	"P10A":    {"Proton-Event", "≥10 pfu (S1)"},
	"P11A":    {"Proton-Event", "≥100 pfu (S2)"},
	"P12A":    {"Proton-Event", "≥1,000 pfu"},
	"P13A":    {"Proton-Event", "≥10,000 pfu (S3)"},
	"P14A":    {"Proton-Event", "≥100,000 pfu (S4)"},
	"P15A":    {"Proton-Event", "≥1,000,000 pfu (S5)"},
	"RWAR":    {"Radio-Blackout", "Radio blackout warning"},
	"X1XW":    {"X-Ray-Flare", "X-ray flux ≥ X1 (R3)"},
	"X10XW":   {"X-Ray-Flare", "X-ray flux ≥ X10 (R5)"},
	"SUM01R":  {"Radio-Sweep", "Type II radio sweep"},
	"SUM02R":  {"Radio-Sweep", "Type IV radio sweep"},
	"SUM01D":  {"Discussion", "Daily 3-day forecast discussion"},
}

type rawSWPCAlert struct {
	ProductID     string `json:"product_id"`
	IssueDatetime string `json:"issue_datetime"`
	Message       string `json:"message"`
}

// ParseSWPCAlerts decodes alerts.json, drops entries whose product_id is not in
// allow, drops entries older than window relative to now(), resolves the
// series+description from swpcCodeTable, and truncates message to 512 bytes.
// Bad timestamps cause the entry to be dropped (defensive).
func ParseSWPCAlerts(body []byte, allow []string, window time.Duration, now func() time.Time, logger *slog.Logger) ([]SWPCAlert, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if now == nil {
		now = time.Now
	}
	allowSet := make(map[string]struct{}, len(allow))
	for _, c := range allow {
		allowSet[c] = struct{}{}
	}
	var raw []rawSWPCAlert
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("swpc_alerts: unmarshal: %w", err)
	}
	cutoff := now().Add(-window)
	out := make([]SWPCAlert, 0, len(raw))
	for _, r := range raw {
		if _, ok := allowSet[r.ProductID]; !ok {
			continue
		}
		issued, err := time.Parse(swpcIssueDatetimeLayout, r.IssueDatetime)
		if err != nil {
			logger.Debug("swpc_alerts.bad_timestamp", "product_id", r.ProductID, "value", r.IssueDatetime, "err", err.Error())
			continue
		}
		issued = issued.UTC()
		if issued.Before(cutoff) {
			continue
		}
		msg := r.Message
		if len(msg) > swpcMessageMaxBytes {
			msg = msg[:swpcMessageMaxBytes]
		}
		entry := SWPCAlert{
			Code:    r.ProductID,
			Issued:  issued,
			Message: msg,
		}
		if meta, ok := swpcCodeTable[r.ProductID]; ok {
			entry.Series = meta.Series
			entry.Description = meta.Description
		}
		out = append(out, entry)
	}
	return out, nil
}

// SWPCAlerts implements Source for the SWPC alerts feed.
type SWPCAlerts struct {
	url       string
	userAgent string
	allow     []string
	window    time.Duration
	now       func() time.Time
	interval  time.Duration
	client    *http.Client
	logger    *slog.Logger
}

// NewSWPCAlerts constructs a SWPCAlerts source. allow is the configured
// product-id allowlist (derived.thresholds.swpc_alert_products); window is
// the configured age cutoff (derived.thresholds.swpc_alert_window_hours).
func NewSWPCAlerts(url, userAgent string, allow []string, window time.Duration, logger *slog.Logger) *SWPCAlerts {
	return &SWPCAlerts{
		url:       url,
		userAgent: userAgent,
		allow:     append([]string(nil), allow...),
		window:    window,
		now:       time.Now,
		interval:  60 * time.Second,
		client:    &http.Client{Timeout: 15 * time.Second},
		logger:    logger,
	}
}

// Name returns the source identifier.
func (s *SWPCAlerts) Name() string { return SWPCAlertsName }

// Interval returns the polling interval.
func (s *SWPCAlerts) Interval() time.Duration { return s.interval }

// SetInterval is the boot-time setter used by the config env walker.
func (s *SWPCAlerts) SetInterval(d time.Duration) { s.interval = d }

// Fetch retrieves the SWPC alerts feed and returns a filtered slice + sha256 validator.
// Like noaa-scales, the alerts.json endpoint sends Cache-Control: max-age=60 with no
// usable ETag — content-hash is the right validator strategy.
func (s *SWPCAlerts) Fetch(ctx context.Context) (FetchResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return FetchResult{}, fmt.Errorf("swpc_alerts: build request: %w", err)
	}
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return FetchResult{}, fmt.Errorf("swpc_alerts: do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return FetchResult{}, fmt.Errorf("swpc_alerts: upstream status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return FetchResult{}, fmt.Errorf("swpc_alerts: read body: %w", err)
	}
	alerts, err := ParseSWPCAlerts(body, s.allow, s.window, s.now, s.logger)
	if err != nil {
		return FetchResult{}, err
	}
	sum := sha256.Sum256(body)
	return FetchResult{
		Payload:   alerts,
		Validator: "sha256:" + hex.EncodeToString(sum[:]),
	}, nil
}
```

- [ ] **Step 6.4: Run tests, lint, commit**

```bash
go test ./internal/sources/ -run SWPCAlerts -v -race
golangci-lint run ./internal/sources/...
git add internal/sources/swpc_alerts.go internal/sources/swpc_alerts_test.go
git commit -m "feat(p2): swpc_alerts Source (allowlist + 24h window + code namespace)"
```

---
## Task 7: `usgs_quakes` source — types, alternative-feeds comment, parser, `Source` impl with ETag passthrough, tests

**Files:**
- Create: `internal/sources/usgs_quakes.go`
- Create: `internal/sources/usgs_quakes_test.go`

**Dependencies:** — depends on: Task 3 (USGS quake fixtures).

**Why:** Reads `earthquake.usgs.gov/.../significant_day.geojson`, decodes GeoJSON `FeatureCollection`, normalizes to `[]Quake` sorted by magnitude descending, **and** uses upstream ETag for change detection (`If-None-Match` on subsequent fetches → on 304 returns the prior validator with the cached payload — let the fetcher's diff-on-write handle it). The alternative-feeds comment block from design §6.3 is committed verbatim.

- [ ] **Step 7.1: Write the failing tests**

Create `internal/sources/usgs_quakes_test.go`:

```go
package sources

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestUSGSQuakes_ParseEmpty(t *testing.T) {
	body := loadFixture(t, "usgs_quakes_empty.geojson")
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	got, err := ParseUSGSQuakes(body, logger)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want 0, got %d", len(got))
	}
}

func TestUSGSQuakes_ParseFixture(t *testing.T) {
	body := loadFixture(t, "usgs_quakes_typical.geojson")
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	got, err := ParseUSGSQuakes(body, logger)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, q := range got {
		if q.ID == "" {
			t.Errorf("quake missing ID: %+v", q)
		}
		if q.Time.IsZero() {
			t.Errorf("quake %s missing Time", q.ID)
		}
	}
}

func TestUSGSQuakes_SortsByMagnitudeDescending(t *testing.T) {
	body := []byte(`{"type":"FeatureCollection","features":[
		{"id":"a","properties":{"mag":4.5,"place":"X","time":1714651200000,"updated":1714651200000,"tsunami":0,"alert":null,"url":"https://x"},"geometry":{"type":"Point","coordinates":[10,20,5]}},
		{"id":"b","properties":{"mag":7.0,"place":"Y","time":1714651300000,"updated":1714651300000,"tsunami":1,"alert":"yellow","url":"https://y"},"geometry":{"type":"Point","coordinates":[30,40,15]}},
		{"id":"c","properties":{"mag":5.5,"place":"Z","time":1714651400000,"updated":1714651400000,"tsunami":0,"alert":null,"url":"https://z"},"geometry":{"type":"Point","coordinates":[50,60,25]}}
	]}`)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	got, err := ParseUSGSQuakes(body, logger)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3, got %d", len(got))
	}
	if !sort.SliceIsSorted(got, func(i, j int) bool { return got[i].Magnitude > got[j].Magnitude }) {
		t.Errorf("not sorted desc by magnitude: %+v", got)
	}
	if got[0].ID != "b" {
		t.Errorf("largest first should be b, got %s", got[0].ID)
	}
	if !got[0].Tsunami {
		t.Errorf("b should carry tsunami=true")
	}
	if got[0].Alert != "yellow" {
		t.Errorf("b alert: %q", got[0].Alert)
	}
}

func TestUSGSQuakes_FetchPassesThroughETag(t *testing.T) {
	body := []byte(`{"type":"FeatureCollection","features":[]}`)
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Header.Get("If-None-Match") == `"abc"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"abc"`)
		w.Header().Set("Content-Type", "application/geo+json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	src := NewUSGSQuakes(srv.URL, "ua", slog.New(slog.NewJSONHandler(io.Discard, nil)))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	res1, err := src.Fetch(ctx)
	if err != nil {
		t.Fatalf("fetch1: %v", err)
	}
	if res1.Validator != `"abc"` {
		t.Errorf("first validator = %q, want \"abc\"", res1.Validator)
	}

	res2, err := src.Fetch(ctx)
	if err != nil {
		t.Fatalf("fetch2: %v", err)
	}
	if res2.Validator != `"abc"` {
		t.Errorf("second validator = %q, want \"abc\"", res2.Validator)
	}
	if hits != 2 {
		t.Errorf("upstream hits = %d, want 2", hits)
	}
	if _, ok := res2.Payload.([]Quake); !ok {
		t.Errorf("304 payload type %T, want []Quake", res2.Payload)
	}

	if src.Name() != "usgs_quakes" {
		t.Errorf("Name() = %q", src.Name())
	}
	if src.Interval() != 60*time.Second {
		t.Errorf("default Interval = %v", src.Interval())
	}
	_ = strings.Contains
}

func TestUSGSQuakes_ParseMalformedJSON(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	if _, err := ParseUSGSQuakes([]byte("nope"), logger); err == nil {
		t.Fatalf("want error")
	}
}
```

- [ ] **Step 7.2: Run, confirm fail**

```bash
go test ./internal/sources/ -run USGSQuakes -v
```

- [ ] **Step 7.3: Implement `internal/sources/usgs_quakes.go`**

```go
package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"sync"
	"time"
)

// USGSQuakesName is the canonical source name for the USGS earthquake feed.
const USGSQuakesName = "usgs_quakes"

// Quake is a normalized USGS earthquake event.
type Quake struct {
	ID        string    `json:"id"`
	Magnitude float64   `json:"magnitude"`
	Place     string    `json:"place"`
	Time      time.Time `json:"time"`
	UpdatedAt time.Time `json:"updatedAt"`
	Lat       float64   `json:"lat"`
	Lon       float64   `json:"lon"`
	DepthKm   float64   `json:"depthKm"`
	Tsunami   bool      `json:"tsunami"`
	Alert     string    `json:"alert,omitempty"`
	URL       string    `json:"url,omitempty"`
}

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
type rawQuakeFC struct {
	Features []rawQuakeFeature `json:"features"`
}

type rawQuakeFeature struct {
	ID         string        `json:"id"`
	Properties rawQuakeProps `json:"properties"`
	Geometry   rawPointGeom  `json:"geometry"`
}

type rawQuakeProps struct {
	Mag     float64 `json:"mag"`
	Place   string  `json:"place"`
	Time    int64   `json:"time"`
	Updated int64   `json:"updated"`
	Tsunami int     `json:"tsunami"`
	Alert   *string `json:"alert"`
	URL     string  `json:"url"`
}

type rawPointGeom struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"`
}

// ParseUSGSQuakes decodes a GeoJSON FeatureCollection of significant_day.geojson
// shape and returns a slice sorted by magnitude descending.
func ParseUSGSQuakes(body []byte, logger *slog.Logger) ([]Quake, error) {
	if logger == nil {
		logger = slog.Default()
	}
	var fc rawQuakeFC
	if err := json.Unmarshal(body, &fc); err != nil {
		return nil, fmt.Errorf("usgs_quakes: unmarshal: %w", err)
	}
	out := make([]Quake, 0, len(fc.Features))
	for _, f := range fc.Features {
		var lat, lon, depth float64
		if len(f.Geometry.Coordinates) >= 3 {
			lon = f.Geometry.Coordinates[0]
			lat = f.Geometry.Coordinates[1]
			depth = f.Geometry.Coordinates[2]
		}
		alert := ""
		if f.Properties.Alert != nil {
			alert = *f.Properties.Alert
		}
		out = append(out, Quake{
			ID:        f.ID,
			Magnitude: f.Properties.Mag,
			Place:     f.Properties.Place,
			Time:      time.UnixMilli(f.Properties.Time).UTC(),
			UpdatedAt: time.UnixMilli(f.Properties.Updated).UTC(),
			Lat:       lat,
			Lon:       lon,
			DepthKm:   depth,
			Tsunami:   f.Properties.Tsunami != 0,
			Alert:     alert,
			URL:       f.Properties.URL,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Magnitude > out[j].Magnitude })
	return out, nil
}

// USGSQuakes implements Source for USGS significant_day.geojson, with
// upstream-ETag passthrough so 304 responses keep the cached payload.
type USGSQuakes struct {
	url       string
	userAgent string
	interval  time.Duration
	client    *http.Client
	logger    *slog.Logger

	mu         sync.Mutex
	lastETag   string
	lastQuakes []Quake
}

// NewUSGSQuakes constructs a USGSQuakes source.
func NewUSGSQuakes(url, userAgent string, logger *slog.Logger) *USGSQuakes {
	return &USGSQuakes{
		url:       url,
		userAgent: userAgent,
		interval:  60 * time.Second,
		client:    &http.Client{Timeout: 15 * time.Second},
		logger:    logger,
	}
}

// Name returns the source identifier.
func (q *USGSQuakes) Name() string { return USGSQuakesName }

// Interval returns the polling interval.
func (q *USGSQuakes) Interval() time.Duration { return q.interval }

// SetInterval is the boot-time setter for the config env walker.
func (q *USGSQuakes) SetInterval(d time.Duration) { q.interval = d }

// Fetch retrieves the USGS feed with If-None-Match passthrough. On 304 it
// returns the prior payload + the prior ETag (the fetcher's diff-on-write
// then sees identical Validator and skips broadcast).
func (q *USGSQuakes) Fetch(ctx context.Context) (FetchResult, error) {
	q.mu.Lock()
	priorETag := q.lastETag
	priorPayload := append([]Quake(nil), q.lastQuakes...)
	q.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, q.url, nil)
	if err != nil {
		return FetchResult{}, fmt.Errorf("usgs_quakes: build request: %w", err)
	}
	req.Header.Set("User-Agent", q.userAgent)
	req.Header.Set("Accept", "application/geo+json")
	if priorETag != "" {
		req.Header.Set("If-None-Match", priorETag)
	}
	resp, err := q.client.Do(req)
	if err != nil {
		return FetchResult{}, fmt.Errorf("usgs_quakes: do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotModified {
		return FetchResult{Payload: priorPayload, Validator: priorETag}, nil
	}
	if resp.StatusCode/100 != 2 {
		return FetchResult{}, fmt.Errorf("usgs_quakes: upstream status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return FetchResult{}, fmt.Errorf("usgs_quakes: read body: %w", err)
	}
	quakes, err := ParseUSGSQuakes(body, q.logger)
	if err != nil {
		return FetchResult{}, err
	}
	etag := resp.Header.Get("ETag")
	q.mu.Lock()
	q.lastETag = etag
	q.lastQuakes = append([]Quake(nil), quakes...)
	q.mu.Unlock()
	return FetchResult{Payload: quakes, Validator: etag}, nil
}
```

- [ ] **Step 7.4: Run tests, lint, commit**

```bash
go test ./internal/sources/ -run USGSQuakes -v -race
golangci-lint run ./internal/sources/...
git add internal/sources/usgs_quakes.go internal/sources/usgs_quakes_test.go
git commit -m "feat(p2): usgs_quakes Source (GeoJSON parse + ETag passthrough)"
```

---

## Task 8: `usgs_volcanoes` source — types, RSS-fallback comment, parser with NORMAL filter, `Source` impl with ETag, tests

**Files:**
- Create: `internal/sources/usgs_volcanoes.go`
- Create: `internal/sources/usgs_volcanoes_test.go`

**Dependencies:** — depends on: Task 4 (USGS volcano fixture).

**Why:** Reads `volcanoes.usgs.gov/hans-public/api/volcano/getElevatedVolcanoes` (JSON), drops entries whose alert level is `NORMAL` (the upstream commonly returns only non-NORMAL but the parser is defensive), sorts by region then name for stable display, and uses upstream ETag passthrough (5-minute cadence per design §3). The RSS-fallback comment block from design §6.4 is committed verbatim.

- [ ] **Step 8.1: Write the failing tests**

```go
package sources

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"
)

func TestUSGSVolcanoes_ParseFixture(t *testing.T) {
	body := loadFixture(t, "usgs_volcanoes_typical.json")
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	got, err := ParseUSGSVolcanoes(body, logger)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, v := range got {
		if v.Name == "" {
			t.Errorf("missing Name: %+v", v)
		}
		if v.Alert == "NORMAL" {
			t.Errorf("NORMAL should be filtered: %+v", v)
		}
	}
}

func TestUSGSVolcanoes_DropsNORMAL(t *testing.T) {
	body := []byte(`[
		{"volcanoCd":"a1","volcanoName":"A","obsAbbr":"AVO","region":"Alaska","latitude":60,"longitude":-150,"alertLevel":"NORMAL","colorCode":"GREEN","summary":"quiet","updateDate":"2026-05-02T10:00:00Z","url":"https://a"},
		{"volcanoCd":"a2","volcanoName":"B","obsAbbr":"AVO","region":"Alaska","latitude":61,"longitude":-152,"alertLevel":"WATCH","colorCode":"ORANGE","summary":"unrest","updateDate":"2026-05-02T10:00:00Z","url":"https://b"},
		{"volcanoCd":"a3","volcanoName":"C","obsAbbr":"CalVO","region":"CalVO","latitude":40,"longitude":-122,"alertLevel":"ADVISORY","colorCode":"YELLOW","summary":"elevated","updateDate":"2026-05-02T10:00:00Z","url":"https://c"}
	]`)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	got, err := ParseUSGSVolcanoes(body, logger)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 (NORMAL dropped), got %d", len(got))
	}
	for _, v := range got {
		if v.Alert == "NORMAL" {
			t.Errorf("NORMAL leaked: %+v", v)
		}
	}
}

func TestUSGSVolcanoes_SortsByRegionThenName(t *testing.T) {
	body := []byte(`[
		{"volcanoCd":"3","volcanoName":"Zeta","region":"Alaska","alertLevel":"WATCH","colorCode":"ORANGE","updateDate":"2026-05-02T10:00:00Z"},
		{"volcanoCd":"1","volcanoName":"Beta","region":"Alaska","alertLevel":"WATCH","colorCode":"ORANGE","updateDate":"2026-05-02T10:00:00Z"},
		{"volcanoCd":"2","volcanoName":"Alpha","region":"CalVO","alertLevel":"WATCH","colorCode":"ORANGE","updateDate":"2026-05-02T10:00:00Z"}
	]`)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	got, err := ParseUSGSVolcanoes(body, logger)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !sort.SliceIsSorted(got, func(i, j int) bool {
		if got[i].Region != got[j].Region {
			return got[i].Region < got[j].Region
		}
		return got[i].Name < got[j].Name
	}) {
		t.Errorf("not sorted by region/name: %+v", got)
	}
	if got[0].Name != "Beta" || got[1].Name != "Zeta" || got[2].Name != "Alpha" {
		t.Errorf("order: %+v", got)
	}
}

func TestUSGSVolcanoes_FetchETagPassthrough(t *testing.T) {
	body := []byte(`[]`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `W/"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `W/"v1"`)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	src := NewUSGSVolcanoes(srv.URL, "ua", slog.New(slog.NewJSONHandler(io.Discard, nil)))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	res1, err := src.Fetch(ctx)
	if err != nil {
		t.Fatalf("fetch1: %v", err)
	}
	if res1.Validator != `W/"v1"` {
		t.Errorf("first validator: %q", res1.Validator)
	}
	res2, err := src.Fetch(ctx)
	if err != nil {
		t.Fatalf("fetch2: %v", err)
	}
	if res2.Validator != `W/"v1"` {
		t.Errorf("second validator: %q", res2.Validator)
	}
	if src.Name() != "usgs_volcanoes" {
		t.Errorf("Name() = %q", src.Name())
	}
	if src.Interval() != 5*time.Minute {
		t.Errorf("default Interval = %v, want 5m", src.Interval())
	}
	_ = io.Discard
}

func TestUSGSVolcanoes_ParseMalformedJSON(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	if _, err := ParseUSGSVolcanoes([]byte("nope"), logger); err == nil {
		t.Fatalf("want error")
	}
}
```

- [ ] **Step 8.2: Run, confirm fail**

```bash
go test ./internal/sources/ -run USGSVolcanoes -v
```

- [ ] **Step 8.3: Implement `internal/sources/usgs_volcanoes.go`**

```go
package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"sync"
	"time"
)

// USGSVolcanoesName is the canonical source name for the USGS volcano feed.
const USGSVolcanoesName = "usgs_volcanoes"

// AlertLevel is the USGS volcanic alert level.
type AlertLevel string

// ColorCode is the USGS aviation color code.
type ColorCode string

// Volcano alert levels (NORMAL is filtered out before storage).
const (
	AlertNORMAL   AlertLevel = "NORMAL"
	AlertADVISORY AlertLevel = "ADVISORY"
	AlertWATCH    AlertLevel = "WATCH"
	AlertWARNING  AlertLevel = "WARNING"
)

// Volcano is a normalized non-NORMAL USGS volcano entry.
type Volcano struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Region    string     `json:"region"`
	Lat       float64    `json:"lat"`
	Lon       float64    `json:"lon"`
	Alert     AlertLevel `json:"alert"`
	Color     ColorCode  `json:"color"`
	UpdatedAt time.Time  `json:"updatedAt"`
	Synopsis  string     `json:"synopsis,omitempty"`
	URL       string     `json:"url,omitempty"`
}

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
type rawVolcano struct {
	VolcanoCd   string  `json:"volcanoCd"`
	VolcanoName string  `json:"volcanoName"`
	Region      string  `json:"region"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	AlertLevel  string  `json:"alertLevel"`
	ColorCode   string  `json:"colorCode"`
	Summary     string  `json:"summary"`
	UpdateDate  string  `json:"updateDate"`
	URL         string  `json:"url"`
}

// ParseUSGSVolcanoes decodes the getElevatedVolcanoes JSON array, drops
// NORMAL-level entries, and sorts by region then name. Bad timestamps fall
// back to zero time (entry not dropped — name+alert are the load-bearing
// fields).
func ParseUSGSVolcanoes(body []byte, logger *slog.Logger) ([]Volcano, error) {
	if logger == nil {
		logger = slog.Default()
	}
	var raw []rawVolcano
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("usgs_volcanoes: unmarshal: %w", err)
	}
	out := make([]Volcano, 0, len(raw))
	for _, r := range raw {
		alert := AlertLevel(r.AlertLevel)
		if alert == AlertNORMAL || alert == "" {
			continue
		}
		ts, err := time.Parse(time.RFC3339, r.UpdateDate)
		if err != nil {
			logger.Debug("usgs_volcanoes.bad_timestamp", "volcano", r.VolcanoName, "value", r.UpdateDate)
			ts = time.Time{}
		}
		out = append(out, Volcano{
			ID:        r.VolcanoCd,
			Name:      r.VolcanoName,
			Region:    r.Region,
			Lat:       r.Latitude,
			Lon:       r.Longitude,
			Alert:     alert,
			Color:     ColorCode(r.ColorCode),
			UpdatedAt: ts.UTC(),
			Synopsis:  r.Summary,
			URL:       r.URL,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Region != out[j].Region {
			return out[i].Region < out[j].Region
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// USGSVolcanoes implements Source for the USGS getElevatedVolcanoes endpoint.
type USGSVolcanoes struct {
	url       string
	userAgent string
	interval  time.Duration
	client    *http.Client
	logger    *slog.Logger

	mu        sync.Mutex
	lastETag  string
	lastVolcs []Volcano
}

// NewUSGSVolcanoes constructs a USGSVolcanoes source.
func NewUSGSVolcanoes(url, userAgent string, logger *slog.Logger) *USGSVolcanoes {
	return &USGSVolcanoes{
		url:       url,
		userAgent: userAgent,
		interval:  5 * time.Minute,
		client:    &http.Client{Timeout: 15 * time.Second},
		logger:    logger,
	}
}

// Name returns the source identifier.
func (v *USGSVolcanoes) Name() string { return USGSVolcanoesName }

// Interval returns the polling interval.
func (v *USGSVolcanoes) Interval() time.Duration { return v.interval }

// SetInterval is the boot-time setter for the config env walker.
func (v *USGSVolcanoes) SetInterval(d time.Duration) { v.interval = d }

// Fetch retrieves the USGS volcano feed with If-None-Match passthrough.
func (v *USGSVolcanoes) Fetch(ctx context.Context) (FetchResult, error) {
	v.mu.Lock()
	priorETag := v.lastETag
	priorPayload := append([]Volcano(nil), v.lastVolcs...)
	v.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.url, nil)
	if err != nil {
		return FetchResult{}, fmt.Errorf("usgs_volcanoes: build request: %w", err)
	}
	req.Header.Set("User-Agent", v.userAgent)
	req.Header.Set("Accept", "application/json")
	if priorETag != "" {
		req.Header.Set("If-None-Match", priorETag)
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return FetchResult{}, fmt.Errorf("usgs_volcanoes: do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotModified {
		return FetchResult{Payload: priorPayload, Validator: priorETag}, nil
	}
	if resp.StatusCode/100 != 2 {
		return FetchResult{}, fmt.Errorf("usgs_volcanoes: upstream status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return FetchResult{}, fmt.Errorf("usgs_volcanoes: read body: %w", err)
	}
	volcs, err := ParseUSGSVolcanoes(body, v.logger)
	if err != nil {
		return FetchResult{}, err
	}
	etag := resp.Header.Get("ETag")
	v.mu.Lock()
	v.lastETag = etag
	v.lastVolcs = append([]Volcano(nil), volcs...)
	v.mu.Unlock()
	return FetchResult{Payload: volcs, Validator: etag}, nil
}
```

- [ ] **Step 8.4: Run tests, lint, commit**

```bash
go test ./internal/sources/ -run USGSVolcanoes -v -race
golangci-lint run ./internal/sources/...
git add internal/sources/usgs_volcanoes.go internal/sources/usgs_volcanoes_test.go
git commit -m "feat(p2): usgs_volcanoes Source (NORMAL filter + ETag passthrough)"
```

---

## Task 9: Hot-start decode helper `hotStartDecode`

**Files:**
- Create: `internal/server/hotstart.go`
- Create: `internal/server/hotstart_test.go`

**Dependencies:** — depends on: Task 5 (SWPCForecast), Task 6 (SWPCAlert), Task 7 (Quake), Task 8 (Volcano).

**Why:** The Phase 1 `server.Run` hot-start loop unmarshals into `[]sources.Alert` directly. Phase 2 has 5 source-payload shapes. Centralize the per-name JSON-decode in one helper that returns `any` so `server.Run` stays a single loop. Per design §6.6.

- [ ] **Step 9.1: Write the failing test**

Create `internal/server/hotstart_test.go`:

```go
package server

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jacaudi/cwd/internal/sources"
)

func TestHotStartDecode_KnownSources(t *testing.T) {
	cases := []struct {
		name    string
		payload any
		check   func(t *testing.T, got any)
	}{
		{
			name:    "nws_alerts",
			payload: []sources.Alert{{ID: "a1", Category: sources.CatTornado}},
			check: func(t *testing.T, got any) {
				as, ok := got.([]sources.Alert)
				if !ok {
					t.Fatalf("type %T", got)
				}
				if len(as) != 1 || as[0].ID != "a1" {
					t.Errorf("got %+v", as)
				}
			},
		},
		{
			name:    "swpc_scales",
			payload: sources.SWPCForecast{Days: [3]sources.SWPCDay{{Date: "2026-05-03", R1: 5}, {}, {}}},
			check: func(t *testing.T, got any) {
				fc, ok := got.(sources.SWPCForecast)
				if !ok {
					t.Fatalf("type %T", got)
				}
				if fc.Days[0].Date != "2026-05-03" {
					t.Errorf("Days[0].Date = %q", fc.Days[0].Date)
				}
			},
		},
		{
			name:    "swpc_alerts",
			payload: []sources.SWPCAlert{{Code: "K08A", Issued: time.Now().UTC()}},
			check: func(t *testing.T, got any) {
				as, ok := got.([]sources.SWPCAlert)
				if !ok {
					t.Fatalf("type %T", got)
				}
				if len(as) != 1 || as[0].Code != "K08A" {
					t.Errorf("got %+v", as)
				}
			},
		},
		{
			name:    "usgs_quakes",
			payload: []sources.Quake{{ID: "q1", Magnitude: 5.5}},
			check: func(t *testing.T, got any) {
				qs, ok := got.([]sources.Quake)
				if !ok {
					t.Fatalf("type %T", got)
				}
				if len(qs) != 1 || qs[0].ID != "q1" {
					t.Errorf("got %+v", qs)
				}
			},
		},
		{
			name:    "usgs_volcanoes",
			payload: []sources.Volcano{{ID: "v1", Name: "Test", Alert: sources.AlertWATCH}},
			check: func(t *testing.T, got any) {
				vs, ok := got.([]sources.Volcano)
				if !ok {
					t.Fatalf("type %T", got)
				}
				if len(vs) != 1 || vs[0].Name != "Test" {
					t.Errorf("got %+v", vs)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(tc.payload)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			got, err := hotStartDecode(tc.name, raw)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			tc.check(t, got)
		})
	}
}

func TestHotStartDecode_UnknownSource(t *testing.T) {
	if _, err := hotStartDecode("bogus", []byte(`{}`)); err == nil {
		t.Fatalf("want error for unknown source")
	}
}

func TestHotStartDecode_MalformedJSON(t *testing.T) {
	if _, err := hotStartDecode("nws_alerts", []byte("not json")); err == nil {
		t.Fatalf("want error for malformed JSON")
	}
}
```

- [ ] **Step 9.2: Run, confirm fail**

```bash
go test ./internal/server/ -run HotStartDecode -v
```

- [ ] **Step 9.3: Implement `internal/server/hotstart.go`**

```go
package server

import (
	"encoding/json"
	"fmt"

	"github.com/jacaudi/cwd/internal/sources"
)

// hotStartDecode decodes a stored payload row into the typed Go shape that
// matches the source's wire contract. Returns an error for unknown sources
// so server boot fails loud rather than silently storing the wrong type
// in the cache.
func hotStartDecode(name string, raw []byte) (any, error) {
	switch name {
	case sources.NWSAlertsName:
		var p []sources.Alert
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("hot-start %s: %w", name, err)
		}
		return p, nil
	case sources.SWPCScalesName:
		var p sources.SWPCForecast
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("hot-start %s: %w", name, err)
		}
		return p, nil
	case sources.SWPCAlertsName:
		var p []sources.SWPCAlert
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("hot-start %s: %w", name, err)
		}
		return p, nil
	case sources.USGSQuakesName:
		var p []sources.Quake
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("hot-start %s: %w", name, err)
		}
		return p, nil
	case sources.USGSVolcanoesName:
		var p []sources.Volcano
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("hot-start %s: %w", name, err)
		}
		return p, nil
	default:
		return nil, fmt.Errorf("hot-start: unknown source %q", name)
	}
}
```

- [ ] **Step 9.4: Run tests, commit**

```bash
go test ./internal/server/ -run HotStartDecode -v -race
golangci-lint run ./internal/server/...
git add internal/server/hotstart.go internal/server/hotstart_test.go
git commit -m "feat(p2): hotStartDecode helper for typed payload restore"
```

---

## Task 10: `/api/snapshot` extension — present-only typed payloads for all 5 sources

**Files:**
- Modify: `internal/api/snapshot.go`
- Modify: `internal/api/snapshot_test.go`

**Dependencies:** — depends on: Task 5 (SWPCForecast), Task 6 (SWPCAlert), Task 7 (Quake), Task 8 (Volcano).

**Why:** The Phase 1 snapshot handler hardcodes a single `cache.Get(NWSAlertsName)` lookup with the region filter applied. Phase 2 needs to surface 4 more typed payloads. Region filter still only applies to `nws_alerts` (per design §9 decision 9). Absent sources are omitted via map-key absence (NOT null fields).

- [ ] **Step 10.1: Extend the test**

Add to `internal/api/snapshot_test.go`:

```go
func TestSnapshot_FiveSourcesAllPresent(t *testing.T) {
	c := cache.New()
	now := time.Now().UTC()
	c.Set(cache.Envelope{Source: "nws_alerts", FetchedAt: now, Validator: "v1", Payload: []sources.Alert{{ID: "a1", Category: sources.CatTornado}}})
	c.Set(cache.Envelope{Source: "swpc_scales", FetchedAt: now, Validator: "sha256:abc", Payload: sources.SWPCForecast{Days: [3]sources.SWPCDay{{Date: "2026-05-03", R1: 5}, {}, {}}}})
	c.Set(cache.Envelope{Source: "swpc_alerts", FetchedAt: now, Validator: "sha256:def", Payload: []sources.SWPCAlert{{Code: "K08A", Issued: now}}})
	c.Set(cache.Envelope{Source: "usgs_quakes", FetchedAt: now, Validator: `"e1"`, Payload: []sources.Quake{{ID: "q1", Magnitude: 6.0}}})
	c.Set(cache.Envelope{Source: "usgs_volcanoes", FetchedAt: now, Validator: `W/"v2"`, Payload: []sources.Volcano{{ID: "vol1", Name: "Test", Alert: sources.AlertWATCH}}})

	h := NewSnapshotHandler(c, sources.NewFilter(nil, nil))
	req := httptest.NewRequest(http.MethodGet, "/api/snapshot", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	src, _ := got["sources"].(map[string]any)
	for _, want := range []string{"nws_alerts", "swpc_scales", "swpc_alerts", "usgs_quakes", "usgs_volcanoes"} {
		if _, ok := src[want]; !ok {
			t.Errorf("missing %q in snapshot.sources: %+v", want, src)
		}
	}
}

func TestSnapshot_AbsentSourceOmittedNotNull(t *testing.T) {
	c := cache.New()
	c.Set(cache.Envelope{Source: "swpc_scales", FetchedAt: time.Now(), Validator: "v", Payload: sources.SWPCForecast{}})
	h := NewSnapshotHandler(c, sources.NewFilter(nil, nil))
	req := httptest.NewRequest(http.MethodGet, "/api/snapshot", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if contains(body, "nws_alerts") {
		t.Errorf("expected nws_alerts omitted (not null), body: %s", body)
	}
	if contains(body, "null") && contains(body, "usgs_quakes") {
		t.Errorf("expected usgs_quakes omitted entirely, body: %s", body)
	}
}

func TestSnapshot_RegionFilterAppliesOnlyToNWSAlerts(t *testing.T) {
	c := cache.New()
	now := time.Now()
	c.Set(cache.Envelope{Source: "nws_alerts", FetchedAt: now, Validator: "v", Payload: []sources.Alert{
		{ID: "in", Category: sources.CatTornado, UGCs: []string{"VAC059"}},
		{ID: "out", Category: sources.CatTornado, UGCs: []string{"CAZ505"}},
	}})
	c.Set(cache.Envelope{Source: "usgs_quakes", FetchedAt: now, Validator: "v", Payload: []sources.Quake{{ID: "q-anywhere", Magnitude: 5.0}}})

	h := NewSnapshotHandler(c, sources.NewFilter([]string{"VAC059"}, nil))
	req := httptest.NewRequest(http.MethodGet, "/api/snapshot", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if !contains(body, `"in"`) {
		t.Errorf("kept alert missing")
	}
	if contains(body, `"out"`) {
		t.Errorf("filtered alert leaked")
	}
	// usgs_quakes is NOT region-filtered — it's global.
	if !contains(body, "q-anywhere") {
		t.Errorf("quake should pass through region filter unchanged")
	}
}
```

- [ ] **Step 10.2: Run, confirm fail**

```bash
go test ./internal/api/ -run TestSnapshot -v
```

- [ ] **Step 10.3: Rewrite `internal/api/snapshot.go`**

```go
package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jacaudi/cwd/internal/cache"
	"github.com/jacaudi/cwd/internal/sources"
)

// Envelope is the wire wrapper for a single source's payload. Mirrors design §5.
type Envelope struct {
	Source    string    `json:"source"`
	FetchedAt time.Time `json:"fetchedAt"`
	ETag      string    `json:"etag,omitempty"`
	Payload   any       `json:"payload"`
}

// SnapshotResponse is the wire shape of /api/snapshot. Sources is a free-form
// map keyed by source name; only present sources appear (absence == omitted).
type SnapshotResponse struct {
	ServerTime time.Time           `json:"serverTime"`
	Sources    map[string]Envelope `json:"sources"`
}

type snapshotHandler struct {
	cache  *cache.Cache
	filter sources.Filter
}

// NewSnapshotHandler returns the GET /api/snapshot handler.
func NewSnapshotHandler(c *cache.Cache, f sources.Filter) http.Handler {
	return &snapshotHandler{cache: c, filter: f}
}

// snapshotSourceNames is the canonical wire-order for the snapshot map.
// Iteration order doesn't change correctness (map encoding is unordered) but
// the slice keeps the 5-source contract close to the handler so adding a
// 6th source in a future phase only touches one place here.
var snapshotSourceNames = []string{
	sources.NWSAlertsName,
	sources.SWPCScalesName,
	sources.SWPCAlertsName,
	sources.USGSQuakesName,
	sources.USGSVolcanoesName,
}

func (h *snapshotHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	resp := SnapshotResponse{
		ServerTime: time.Now().UTC(),
		Sources:    map[string]Envelope{},
	}
	for _, name := range snapshotSourceNames {
		env, ok := h.cache.Get(name)
		if !ok {
			continue
		}
		payload := env.Payload
		// Region filter applies only to nws_alerts (design §9 decision 9 —
		// space-weather and global quake/volcano feeds aren't UGC/WFO-coded).
		if name == sources.NWSAlertsName {
			if alerts, ok := env.Payload.([]sources.Alert); ok {
				payload = h.filter.Apply(alerts)
			}
		}
		resp.Sources[name] = Envelope{
			Source:    env.Source,
			FetchedAt: env.FetchedAt,
			ETag:      env.Validator,
			Payload:   payload,
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "encode", http.StatusInternalServerError)
	}
}
```

- [ ] **Step 10.4: Run tests, commit**

```bash
go test ./internal/api/ -run TestSnapshot -v -race
golangci-lint run ./internal/api/...
git add internal/api/snapshot.go internal/api/snapshot_test.go
git commit -m "feat(p2): /api/snapshot surfaces all 5 typed source payloads"
```

---

## Task 11: Boot-time config validation extensions (SWPC product regex + window clamp + per-source interval floor)

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Dependencies:** none. Independent of Tasks 5–10.

**Why:** Design §8 calls for three validation extensions on top of Phase 1's `validateRegionFilter`:
1. Drop entries in `swpc_alert_products` that don't match `^[A-Z]{3,8}[0-9A-Z]?$` and WARN.
2. Clamp `swpc_alert_window_hours <= 0` to 24 and WARN.
3. WARN (don't fail) when any enabled source has `interval < 10s`.

- [ ] **Step 11.1: Write the failing tests**

Add to `internal/config/config_test.go`:

```go
func TestValidateSWPCProducts_DropsMalformedAndWARNs(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	cfg := &Config{
		Server: ServerConfig{Bind: "127.0.0.1:0", LogLevel: "info", LogFormat: "json"},
		UI:     UIConfig{DefaultTheme: "dark", DefaultLanding: "/", EnableHistory: true},
		Store:  StoreConfig{Path: "/tmp/x.db", RetentionDays: 30},
		Images: ImagesConfig{DefaultMode: "lazy"},
		Derived: DerivedConfig{Thresholds: ThresholdsConfig{
			SWPCAlertWindowHours: 24,
			SWPCAlertProducts:    []string{"K08A", "lowercase", "K05A;DROP", "P12A", "WARK04W"},
		}},
	}
	validateSWPCProducts(cfg, logger)
	got := cfg.Derived.Thresholds.SWPCAlertProducts
	want := []string{"K08A", "P12A", "WARK04W"}
	if !sameStrings(got, want) {
		t.Errorf("kept = %v, want %v", got, want)
	}
	if !bytes.Contains(buf.Bytes(), []byte("config.swpc_alert_product_invalid")) {
		t.Errorf("expected WARN, got: %s", buf.String())
	}
}

func TestValidateSWPCWindow_ClampsAndWARNs(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	cfg := &Config{Derived: DerivedConfig{Thresholds: ThresholdsConfig{SWPCAlertWindowHours: -3}}}
	validateSWPCWindow(cfg, logger)
	if cfg.Derived.Thresholds.SWPCAlertWindowHours != 24 {
		t.Errorf("clamp failed: got %d", cfg.Derived.Thresholds.SWPCAlertWindowHours)
	}
	if !bytes.Contains(buf.Bytes(), []byte("config.swpc_alert_window_clamped")) {
		t.Errorf("expected WARN, got: %s", buf.String())
	}
}

func TestValidateSourceIntervalFloors_WARNUnder10s(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	enabled := true
	cfg := &Config{Sources: map[string]SourceConfig{
		"nws_alerts":  {Interval: 5 * time.Second, Enabled: &enabled},
		"swpc_scales": {Interval: 60 * time.Second, Enabled: &enabled},
	}}
	validateSourceIntervalFloors(cfg, logger)
	if !bytes.Contains(buf.Bytes(), []byte("config.source_interval_too_low")) {
		t.Errorf("expected WARN for 5s interval, got: %s", buf.String())
	}
	if bytes.Count(buf.Bytes(), []byte("config.source_interval_too_low")) != 1 {
		t.Errorf("WARN should fire once (only the 5s source), got: %s", buf.String())
	}
	// Values must NOT be clamped — operator override stands.
	if cfg.Sources["nws_alerts"].Interval != 5*time.Second {
		t.Errorf("interval mutated: %v", cfg.Sources["nws_alerts"].Interval)
	}
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
```

- [ ] **Step 11.2: Run, confirm fail**

```bash
go test ./internal/config/ -run 'TestValidate(SWPC|Source)' -v
```

- [ ] **Step 11.3: Extend `internal/config/config.go`**

Add these three helpers (place them next to `validateRegionFilter`):

```go
var swpcProductRE = regexp.MustCompile(`^[A-Z]{3,8}[0-9A-Z]?$`)

// validateSWPCProducts drops entries that don't match the SWPC product code
// shape and WARNs. Mirrors validateRegionFilter's pattern.
func validateSWPCProducts(cfg *Config, logger *slog.Logger) {
	in := cfg.Derived.Thresholds.SWPCAlertProducts
	out := make([]string, 0, len(in))
	for _, p := range in {
		if swpcProductRE.MatchString(p) {
			out = append(out, p)
		} else {
			logger.Warn("config.swpc_alert_product_invalid", "value", p, "pattern", swpcProductRE.String())
		}
	}
	cfg.Derived.Thresholds.SWPCAlertProducts = out
}

// validateSWPCWindow clamps non-positive window-hours to 24 and WARNs.
func validateSWPCWindow(cfg *Config, logger *slog.Logger) {
	if cfg.Derived.Thresholds.SWPCAlertWindowHours <= 0 {
		logger.Warn("config.swpc_alert_window_clamped",
			"value", cfg.Derived.Thresholds.SWPCAlertWindowHours,
			"clamped_to", 24)
		cfg.Derived.Thresholds.SWPCAlertWindowHours = 24
	}
}

// validateSourceIntervalFloors WARNs (does not modify) for any enabled source
// whose interval is under 10s — protects upstreams from over-polling without
// overriding an operator's explicit choice.
func validateSourceIntervalFloors(cfg *Config, logger *slog.Logger) {
	const floor = 10 * time.Second
	for name, src := range cfg.Sources {
		if !src.IsEnabled() {
			continue
		}
		if src.Interval > 0 && src.Interval < floor {
			logger.Warn("config.source_interval_too_low",
				"source", name,
				"interval", src.Interval.String(),
				"floor", floor.String())
		}
	}
}
```

Then wire them into `LoadWithLogger` next to `validateRegionFilter` (just before `validate(cfg)`):

```go
	validateRegionFilter(cfg, logger)
	validateSWPCProducts(cfg, logger)
	validateSWPCWindow(cfg, logger)
	validateSourceIntervalFloors(cfg, logger)
```

- [ ] **Step 11.4: Run tests, commit**

```bash
go test ./internal/config/ -v -race
golangci-lint run ./internal/config/...
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(p2): boot-time validation for swpc_alert_products + window + interval floors"
```

---
## Task 12: Server wiring — boot 4 more sources, integrate `hotStartDecode`, expose on hub + healthProvider

**Files:**
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`

**Dependencies:** — depends on: Task 5 (SWPCScales), Task 6 (SWPCAlerts), Task 7 (USGSQuakes), Task 8 (USGSVolcanoes), Task 9 (hotStartDecode), Task 10 (snapshot), Task 11 (boot validation).

**Why:** Phase 1's `server.Run` constructs only `nws_alerts` and hot-starts only `[]sources.Alert`. Phase 2 adds 4 more constructors with their own URLs, swaps the hot-start `json.Unmarshal` for `hotStartDecode`, and lets `enabledNames` grow naturally from `srcs` keys. **No interface changes anywhere** — fetcher, cache, hub, healthProvider all consume `sources.Source` verbatim.

- [ ] **Step 12.1: Extend the test**

Add to `internal/server/server_test.go`:

```go
func TestRun_AllFiveSourcesGoLiveAndHotStart(t *testing.T) {
	nwsBody := []byte(`{"type":"FeatureCollection","features":[]}`)
	swpcScalesBody := []byte(`{"1":{"DateStamp":"2026-05-03","R":{"MinorProb":"5","MajorProb":"0"},"S":{"Prob":"0"},"G":{"Scale":"0","Text":"none"}},"2":{"DateStamp":"2026-05-04","R":{"MinorProb":"5","MajorProb":"0"},"S":{"Prob":"0"},"G":{"Scale":"0","Text":"none"}},"3":{"DateStamp":"2026-05-05","R":{"MinorProb":"5","MajorProb":"0"},"S":{"Prob":"0"},"G":{"Scale":"0","Text":"none"}}}`)
	swpcAlertsBody := []byte(`[]`)
	quakesBody := []byte(`{"type":"FeatureCollection","features":[]}`)
	volcsBody := []byte(`[]`)

	makeServer := func(body []byte, contentType string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", contentType)
			w.Header().Set("ETag", `"e1"`)
			_, _ = w.Write(body)
		}))
	}
	nwsSrv := makeServer(nwsBody, "application/geo+json")
	defer nwsSrv.Close()
	scalesSrv := makeServer(swpcScalesBody, "application/json")
	defer scalesSrv.Close()
	alertsSrv := makeServer(swpcAlertsBody, "application/json")
	defer alertsSrv.Close()
	quakesSrv := makeServer(quakesBody, "application/geo+json")
	defer quakesSrv.Close()
	volcsSrv := makeServer(volcsBody, "application/json")
	defer volcsSrv.Close()

	t.Setenv("CWD_NWS_ALERTS_URL", nwsSrv.URL)
	t.Setenv("CWD_SWPC_SCALES_URL", scalesSrv.URL)
	t.Setenv("CWD_SWPC_ALERTS_URL", alertsSrv.URL)
	t.Setenv("CWD_USGS_QUAKES_URL", quakesSrv.URL)
	t.Setenv("CWD_USGS_VOLCANOES_URL", volcsSrv.URL)

	enabled := true
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
		Derived: config.DerivedConfig{Thresholds: config.ThresholdsConfig{
			SWPCAlertWindowHours: 24,
			SWPCAlertProducts:    []string{"K08A", "K09A", "P12A", "P13A"},
		}},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	addrCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() { errCh <- Run(ctx, cfg, logger, addrCh) }()
	addr := <-addrCh

	deadline := time.After(3 * time.Second)
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
				goto done
			}
		}
		time.Sleep(40 * time.Millisecond)
	}
done:
	resp, err := http.Get("http://" + addr + "/api/sources")
	if err != nil {
		t.Fatalf("GET /api/sources: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	for _, name := range []string{"nws_alerts", "swpc_scales", "swpc_alerts", "usgs_quakes", "usgs_volcanoes"} {
		if !strings.Contains(string(body), name) {
			t.Errorf("/api/sources missing %s: %s", name, body)
		}
	}
	cancel()
	if err := <-errCh; err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, http.ErrServerClosed) {
		t.Errorf("Run error = %v", err)
	}
}
```

(Add `"strings"` to the imports if not present.)

- [ ] **Step 12.2: Run, confirm fail**

```bash
go test ./internal/server/ -run TestRun_AllFiveSources -v
```

- [ ] **Step 12.3: Extend `internal/server/server.go`**

Add four new URL var-functions next to `nwsAlertsURL` (each follows the same `CWD_*_URL` env-override-for-tests pattern):

```go
var swpcScalesURL = func() string {
	if v := os.Getenv("CWD_SWPC_SCALES_URL"); v != "" {
		return v
	}
	return "https://services.swpc.noaa.gov/products/noaa-scales.json"
}

var swpcAlertsURL = func() string {
	if v := os.Getenv("CWD_SWPC_ALERTS_URL"); v != "" {
		return v
	}
	return "https://services.swpc.noaa.gov/products/alerts.json"
}

var usgsQuakesURL = func() string {
	if v := os.Getenv("CWD_USGS_QUAKES_URL"); v != "" {
		return v
	}
	return "https://earthquake.usgs.gov/earthquakes/feed/v1.0/summary/significant_day.geojson"
}

var usgsVolcanoesURL = func() string {
	if v := os.Getenv("CWD_USGS_VOLCANOES_URL"); v != "" {
		return v
	}
	return "https://volcanoes.usgs.gov/hans-public/api/volcano/getElevatedVolcanoes"
}
```

In step 4 of `Run` (the source map build), extend with all four constructors immediately after the `nws_alerts` block:

```go
	if scfg, ok := cfg.Sources["swpc_scales"]; ok && scfg.IsEnabled() {
		s := sources.NewSWPCScales(swpcScalesURL(), userAgent, logger)
		if scfg.Interval > 0 {
			s.SetInterval(scfg.Interval)
		}
		srcs[sources.SWPCScalesName] = s
	}
	if scfg, ok := cfg.Sources["swpc_alerts"]; ok && scfg.IsEnabled() {
		window := time.Duration(cfg.Derived.Thresholds.SWPCAlertWindowHours) * time.Hour
		s := sources.NewSWPCAlerts(swpcAlertsURL(), userAgent, cfg.Derived.Thresholds.SWPCAlertProducts, window, logger)
		if scfg.Interval > 0 {
			s.SetInterval(scfg.Interval)
		}
		srcs[sources.SWPCAlertsName] = s
	}
	if scfg, ok := cfg.Sources["usgs_quakes"]; ok && scfg.IsEnabled() {
		s := sources.NewUSGSQuakes(usgsQuakesURL(), userAgent, logger)
		if scfg.Interval > 0 {
			s.SetInterval(scfg.Interval)
		}
		srcs[sources.USGSQuakesName] = s
	}
	if scfg, ok := cfg.Sources["usgs_volcanoes"]; ok && scfg.IsEnabled() {
		s := sources.NewUSGSVolcanoes(usgsVolcanoesURL(), userAgent, logger)
		if scfg.Interval > 0 {
			s.SetInterval(scfg.Interval)
		}
		srcs[sources.USGSVolcanoesName] = s
	}
```

Replace the Phase 1 hot-start block with a call to the helper:

```go
	// 5. Hot-start cache from store for each enabled source.
	for name := range srcs {
		row, ok, err := st.Latest(ctx, name)
		if err != nil {
			logger.Warn("server.hotstart", "source", name, "err", err.Error())
			continue
		}
		if !ok {
			continue
		}
		payload, err := hotStartDecode(name, row.Payload)
		if err != nil {
			logger.Warn("server.hotstart_decode", "source", name, "err", err.Error())
			continue
		}
		c.Set(cache.Envelope{Source: name, FetchedAt: row.FetchedAt, Validator: row.Validator, Payload: payload})
	}
```

Everything else (`fetchers` loop, `enabledNames`, `hub`, `ready`, `healthProvider`, router wiring) is unchanged — those already loop over `srcs` and `fetchers` and need zero edits.

- [ ] **Step 12.4: Remove the now-unused `encoding/json` import if no other call sites remain**

```bash
goimports -w internal/server/server.go || gofmt -w internal/server/server.go
```

- [ ] **Step 12.5: Run tests, lint, commit**

```bash
go test ./internal/server/ -v -race
golangci-lint run ./internal/server/...
git add internal/server/server.go internal/server/server_test.go
git commit -m "feat(p2): wire 4 more sources + hotStartDecode integration"
```

---

## Task 13: Frontend wire layer — types, store, stream client extension

**Files:**
- Modify: `web/src/api/types.ts`
- Modify: `web/src/api/stream.ts`
- Modify: `web/src/api/stream.test.ts`
- Modify: `web/src/store/snapshot.ts`
- Modify: `web/src/store/snapshot.test.ts`

**Dependencies:** — depends on: Task 5 (SWPCForecast wire shape), Task 6 (SWPCAlert), Task 7 (Quake), Task 8 (Volcano), Task 10 (snapshot wire shape).

**Why:** The Phase 1 frontend types/store/stream layer hardcodes `nws_alerts` as the single payload shape. Phase 2 adds 4 typed payloads, generalizes the store's `applyUpdate` to dispatch on source name, and extends the SSE stream client to register listeners for all 5 `<source>.update` event types.

- [ ] **Step 13.1: Replace `web/src/api/types.ts`**

```ts
export type ThemeMode = "dark" | "light" | "auto";

export interface UIConfig {
  defaultTheme: ThemeMode;
  defaultLanding: string;
  enableHistory: boolean;
}

export interface VersionInfo {
  version: string;
  commit: string;
  date: string;
  go: string;
}

// ── Phase 1: NWS Alerts wire types ────────────────────────────────────────

export type Severity = 'Extreme' | 'Severe' | 'Moderate' | 'Minor' | 'Unknown';

export type Category =
  | 'Tornado' | 'SevereThunderstorm' | 'FlashFlood' | 'Tropical'
  | 'HighWind' | 'RedFlag' | 'Winter' | 'ExtremeHeat' | 'ExtremeCold'
  | 'Tsunami' | 'Unknown';

export interface Alert {
  id: string;
  event: string;
  awips: string;
  headline: string;
  severity: Severity;
  sent: string;
  effective: string;
  onset?: string;
  expires: string;
  ends?: string;
  areas: string[];
  ugcs?: string[];
  sames?: string[];
  wfo?: string;
  vtecEtn?: string;
  url?: string;
  category: Category;
}

// ── Phase 2: SWPC + USGS wire types ───────────────────────────────────────

export type GScale = 'G1' | 'G2' | 'G3' | 'G4' | 'G5';

export interface SWPCDay {
  date: string;
  r1: number;
  r3: number;
  s1: number;
  g?: GScale;
  gText?: string;
}

export interface SWPCForecast {
  days: [SWPCDay, SWPCDay, SWPCDay];
}

export interface SWPCAlert {
  code: string;
  series: string;
  description: string;
  issued: string;
  message: string;
  url?: string;
}

export interface Quake {
  id: string;
  magnitude: number;
  place: string;
  time: string;
  updatedAt: string;
  lat: number;
  lon: number;
  depthKm: number;
  tsunami: boolean;
  alert?: string;
  url?: string;
}

export type AlertLevel = 'NORMAL' | 'ADVISORY' | 'WATCH' | 'WARNING';
export type ColorCode  = 'GREEN'  | 'YELLOW'   | 'ORANGE' | 'RED';

export interface Volcano {
  id: string;
  name: string;
  region: string;
  lat: number;
  lon: number;
  alert: AlertLevel;
  color: ColorCode;
  updatedAt: string;
  synopsis?: string;
  url?: string;
}

export interface Envelope<T> {
  source: string;
  fetchedAt: string;
  etag?: string;
  payload: T;
}

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

export interface SourceHealth {
  intervalSec: number;
  lastAttempt: string;
  lastSuccess: string;
  lastError?: string;
  etag?: string;
  ageSec: number;
  consecutiveFailures: number;
}
```

- [ ] **Step 13.2: Rewrite `web/src/store/snapshot.ts`**

```ts
import { create } from 'zustand';
import type {
  Envelope,
  Alert,
  Quake,
  SWPCAlert,
  SWPCForecast,
  Snapshot,
  Volcano,
} from '../api/types';

export type Connection = 'connecting' | 'live' | 'polling' | 'error';

// SourcePayloadMap maps each source key to its envelope payload type. The
// stream client + store both narrow against this so a misrouted update
// (e.g. quake payload routed to nws_alerts) is a TS compile error.
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
  setSnapshot: (s: Snapshot) => void;
  applyUpdate: <K extends keyof SourcePayloadMap>(source: K, env: Envelope<SourcePayloadMap[K]>) => void;
  setConnection: (c: Connection) => void;
}

export const useSnapshotStore = create<State>((set) => ({
  snapshot: null,
  connection: 'connecting',
  setSnapshot: (s) => set({ snapshot: s }),
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
  setConnection: (c) => set({ connection: c }),
}));
```

- [ ] **Step 13.3: Rewrite `web/src/api/stream.ts`**

```ts
import type { Snapshot, Envelope } from './types';
import type { SourcePayloadMap } from '../store/snapshot';

export interface ConnectOpts {
  onSnapshot: (s: Snapshot) => void;
  onUpdate: <K extends keyof SourcePayloadMap>(source: K, env: Envelope<SourcePayloadMap[K]>) => void;
  onError?: (e: unknown) => void;
}

const SOURCE_NAMES: (keyof SourcePayloadMap)[] = [
  'nws_alerts',
  'swpc_scales',
  'swpc_alerts',
  'usgs_quakes',
  'usgs_volcanoes',
];

export async function connect(opts: ConnectOpts): Promise<() => void> {
  try {
    const res = await fetch('/api/snapshot');
    if (res.ok) opts.onSnapshot(await res.json());
  } catch (e) {
    opts.onError?.(e);
  }

  const es = new EventSource('/api/stream');
  es.addEventListener('snapshot', (e: MessageEvent) => {
    try { opts.onSnapshot(JSON.parse(e.data)); } catch (err) { opts.onError?.(err); }
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
  return () => es.close();
}
```

- [ ] **Step 13.4: Extend the existing tests**

In `web/src/store/snapshot.test.ts`, add:

```ts
it('routes updates by source key', () => {
  useSnapshotStore.getState().setSnapshot({ serverTime: 't', sources: {} });
  useSnapshotStore.getState().applyUpdate('swpc_scales', {
    source: 'swpc_scales', fetchedAt: 't',
    payload: {
      days: [
        { date: '2026-05-03', r1: 5, r3: 0, s1: 0 },
        { date: '2026-05-04', r1: 5, r3: 0, s1: 0 },
        { date: '2026-05-05', r1: 5, r3: 0, s1: 0 },
      ],
    },
  });
  useSnapshotStore.getState().applyUpdate('usgs_quakes', {
    source: 'usgs_quakes', fetchedAt: 't',
    payload: [{ id: 'q1', magnitude: 6.0, place: 'X', time: 't', updatedAt: 't', lat: 0, lon: 0, depthKm: 5, tsunami: false }],
  });
  const s = useSnapshotStore.getState().snapshot!;
  expect(s.sources.swpc_scales?.payload.days[0].date).toBe('2026-05-03');
  expect(s.sources.usgs_quakes?.payload[0].id).toBe('q1');
});
```

In `web/src/api/stream.test.ts`, add (the existing FakeES exposes a `listeners` map):

```ts
it('registers a listener for every source.update event', async () => {
  const fakeFetch = vi.fn().mockResolvedValue({
    ok: true, json: async () => ({ serverTime: 't', sources: {} }),
  });
  vi.stubGlobal('fetch', fakeFetch);
  vi.stubGlobal('EventSource', FakeES as any);

  await connect({ onSnapshot: () => {}, onUpdate: () => {} });
  for (const name of ['nws_alerts', 'swpc_scales', 'swpc_alerts', 'usgs_quakes', 'usgs_volcanoes']) {
    expect(FakeES.last!.listeners[`${name}.update`]).toBeTruthy();
  }
});
```

- [ ] **Step 13.5: Run tests, typecheck, commit**

```bash
task web:test
task web:typecheck
git add web/src/api/types.ts web/src/api/stream.ts web/src/api/stream.test.ts \
        web/src/store/snapshot.ts web/src/store/snapshot.test.ts
git commit -m "feat(p2): frontend types + store + stream client cover all 5 sources"
```

---

## Task 14: Frontend `SWPCForecast` component

**Files:**
- Create: `web/src/components/SWPCForecast.tsx`
- Create: `web/src/components/SWPCForecast.test.tsx`

**Dependencies:** — depends on: Task 13 (frontend wire layer).

**Why:** Block 1 of the SpaceWeather page (design §7.1). Renders the 3-day forecast as three `ProCard.StatisticCard`s side-by-side, each showing R1/R3/S1 percentages with severity-colored progress bars and a G-scale tag.

- [ ] **Step 14.1: Write the failing test**

`web/src/components/SWPCForecast.test.tsx`:

```tsx
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { SWPCForecast } from './SWPCForecast';
import type { SWPCForecast as SWPCForecastT } from '../api/types';

const fc: SWPCForecastT = {
  days: [
    { date: '2026-05-03', r1: 10, r3: 1,  s1: 1, g: 'G2', gText: 'G2 expected' },
    { date: '2026-05-04', r1: 30, r3: 5,  s1: 5, g: 'G4', gText: 'G4 watch' },
    { date: '2026-05-05', r1: 60, r3: 25, s1: 80, g: 'G5', gText: 'G5 extreme' },
  ],
};

describe('SWPCForecast', () => {
  it('renders three day cards with date subtitles', () => {
    render(<SWPCForecast forecast={fc} fetchedAt="2026-05-02T12:00:00Z" />);
    expect(screen.getByText(/Tomorrow/)).toBeInTheDocument();
    expect(screen.getByText(/\+2 days/)).toBeInTheDocument();
    expect(screen.getByText(/\+3 days/)).toBeInTheDocument();
    expect(screen.getByText('2026-05-03')).toBeInTheDocument();
    expect(screen.getByText('2026-05-05')).toBeInTheDocument();
  });

  it('renders R1/R3/S1 percentages', () => {
    render(<SWPCForecast forecast={fc} fetchedAt="2026-05-02T12:00:00Z" />);
    expect(screen.getAllByText(/R1/i).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/R3/i).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/S1/i).length).toBeGreaterThan(0);
  });

  it('renders G-scale chips with color', () => {
    render(<SWPCForecast forecast={fc} fetchedAt="2026-05-02T12:00:00Z" />);
    expect(screen.getByText('G2')).toBeInTheDocument();
    expect(screen.getByText('G4')).toBeInTheDocument();
    expect(screen.getByText('G5')).toBeInTheDocument();
  });

  it('renders a muted "G-scale: none" label when g is absent', () => {
    const empty: SWPCForecastT = {
      days: [
        { date: '2026-05-03', r1: 5, r3: 0, s1: 0 },
        { date: '2026-05-04', r1: 5, r3: 0, s1: 0 },
        { date: '2026-05-05', r1: 5, r3: 0, s1: 0 },
      ],
    };
    render(<SWPCForecast forecast={empty} fetchedAt="2026-05-02T12:00:00Z" />);
    expect(screen.getAllByText(/G-scale: none/i).length).toBe(3);
  });

  it('renders skeleton state when forecast is null', () => {
    render(<SWPCForecast forecast={null} fetchedAt={null} />);
    expect(screen.getAllByLabelText(/forecast-skeleton/i).length).toBe(3);
  });
});
```

- [ ] **Step 14.2: Implement `web/src/components/SWPCForecast.tsx`**

```tsx
import { Skeleton, Space, Statistic, Tag, Tooltip, Typography } from 'antd';
import { ProCard } from '@ant-design/pro-components';
import type { GScale, SWPCDay, SWPCForecast as SWPCForecastT } from '../api/types';

interface Props {
  forecast: SWPCForecastT | null;
  fetchedAt: string | null;
}

const DAY_LABELS = ['Tomorrow', '+2 days', '+3 days'];

const G_COLOR: Record<GScale, string> = {
  G1: 'green',
  G2: 'gold',
  G3: 'orange',
  G4: 'red',
  G5: 'magenta',
};

function severityColor(pct: number): string {
  if (pct >= 75) return '#ff4d4f';
  if (pct >= 50) return '#fa8c16';
  if (pct >= 25) return '#faad14';
  return '#52c41a';
}

function ProbRow({ label, value }: { label: string; value: number }) {
  return (
    <div style={{ marginBottom: 4 }}>
      <Space size="small">
        <Typography.Text strong>{label}</Typography.Text>
        <Statistic
          value={value}
          suffix="%"
          valueStyle={{ fontSize: 14, color: severityColor(value) }}
        />
      </Space>
    </div>
  );
}

function GChip({ day }: { day: SWPCDay }) {
  if (!day.g) {
    return (
      <Typography.Text type="secondary">G-scale: none</Typography.Text>
    );
  }
  return <Tag color={G_COLOR[day.g]}>{day.g}</Tag>;
}

export function SWPCForecast({ forecast, fetchedAt }: Props) {
  if (!forecast) {
    return (
      <ProCard.Group direction="row">
        {DAY_LABELS.map((label) => (
          <ProCard key={label} title={label} bordered>
            <Skeleton active paragraph={{ rows: 3 }} aria-label="forecast-skeleton" />
          </ProCard>
        ))}
      </ProCard.Group>
    );
  }
  const fetchedNote = fetchedAt
    ? `as of ${new Date(fetchedAt).toUTCString()}`
    : '';
  return (
    <ProCard.Group direction="row">
      {DAY_LABELS.map((label, i) => {
        const day = forecast.days[i];
        const tooltipTitle = `${day.gText ?? ''} ${fetchedNote}`.trim();
        return (
          <Tooltip key={label} title={tooltipTitle || undefined}>
            <ProCard
              title={
                <Space direction="vertical" size={0}>
                  <span>{label}</span>
                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                    {day.date}
                  </Typography.Text>
                </Space>
              }
              bordered
              extra={<GChip day={day} />}
            >
              <ProbRow label="R1" value={day.r1} />
              <ProbRow label="R3" value={day.r3} />
              <ProbRow label="S1" value={day.s1} />
            </ProCard>
          </Tooltip>
        );
      })}
    </ProCard.Group>
  );
}
```

- [ ] **Step 14.3: Run tests, commit**

```bash
task web:test
task web:typecheck
git add web/src/components/SWPCForecast.tsx web/src/components/SWPCForecast.test.tsx
git commit -m "feat(p2): SWPCForecast component (3-day cards with G chips)"
```

---

## Task 15: Frontend `SWPCAlerts` component

**Files:**
- Create: `web/src/components/SWPCAlerts.tsx`
- Create: `web/src/components/SWPCAlerts.test.tsx`

**Dependencies:** — depends on: Task 13 (frontend wire layer).

**Why:** Block 2 of the SpaceWeather page (design §7.1). Renders the active 24h SWPC alerts as an AntD `List`, with code as a colored Tag (color from per-series palette), series + description as the meta title, relative-issued time, and the truncated message body.

- [ ] **Step 15.1: Write the failing test**

`web/src/components/SWPCAlerts.test.tsx`:

```tsx
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { SWPCAlerts } from './SWPCAlerts';
import type { SWPCAlert } from '../api/types';

function mk(over: Partial<SWPCAlert>): SWPCAlert {
  return {
    code: 'K08A',
    series: 'K-Index',
    description: 'K-index 8 = G4 severe',
    issued: '2026-05-02T10:00:00Z',
    message: 'Geomagnetic K=8 reached. Detail follows...',
    ...over,
  };
}

describe('SWPCAlerts', () => {
  it('renders Empty state when payload is empty', () => {
    render(<SWPCAlerts alerts={[]} />);
    expect(screen.getByText(/No active SWPC alerts/i)).toBeInTheDocument();
  });

  it('renders one row per alert with code, series, description', () => {
    render(<SWPCAlerts alerts={[mk({}), mk({ code: 'P12A', series: 'Proton-Event', description: '≥1,000 pfu', message: 'proton event' })]} />);
    expect(screen.getByText('K08A')).toBeInTheDocument();
    expect(screen.getByText('P12A')).toBeInTheDocument();
    expect(screen.getByText(/K-Index/)).toBeInTheDocument();
    expect(screen.getByText(/Proton-Event/)).toBeInTheDocument();
  });

  it('renders code Tag with series-derived color (data-series attribute)', () => {
    render(<SWPCAlerts alerts={[mk({})]} />);
    const tag = screen.getByText('K08A');
    expect(tag.getAttribute('data-series')).toBe('K-Index');
  });

  it('renders truncated message body', () => {
    const long = 'x'.repeat(400);
    render(<SWPCAlerts alerts={[mk({ message: long })]} />);
    // The component truncates display to ~120 chars; substring of x's must be present.
    expect(screen.getByText(/x{50,}/)).toBeInTheDocument();
  });
});
```

- [ ] **Step 15.2: Implement `web/src/components/SWPCAlerts.tsx`**

```tsx
import { Empty, List, Space, Tag, Typography } from 'antd';
import type { SWPCAlert } from '../api/types';

interface Props {
  alerts: SWPCAlert[];
}

const SERIES_COLOR: Record<string, string> = {
  'K-Index':            'magenta',
  'Geomagnetic-Watch':  'volcano',
  'Proton-Event':       'gold',
  'Radio-Blackout':     'red',
  'X-Ray-Flare':        'orange',
  'Radio-Sweep':        'cyan',
  'Discussion':         'default',
};

function relative(iso: string): string {
  const diffMs = Date.now() - new Date(iso).getTime();
  const mins = Math.round(diffMs / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

function truncate(s: string, max = 120): string {
  if (s.length <= max) return s;
  return s.slice(0, max - 1) + '…';
}

export function SWPCAlerts({ alerts }: Props) {
  if (alerts.length === 0) {
    return <Empty description="No active SWPC alerts in window" />;
  }
  return (
    <List
      dataSource={alerts}
      renderItem={(a) => (
        <List.Item>
          <List.Item.Meta
            avatar={
              <Tag
                color={SERIES_COLOR[a.series] ?? 'default'}
                data-series={a.series}
              >
                {a.code}
              </Tag>
            }
            title={
              <Space size="small">
                <Typography.Text strong>{a.series}</Typography.Text>
                <Typography.Text type="secondary">{a.description}</Typography.Text>
                <Typography.Text type="secondary">· {relative(a.issued)}</Typography.Text>
              </Space>
            }
            description={truncate(a.message)}
          />
        </List.Item>
      )}
    />
  );
}
```

- [ ] **Step 15.3: Run tests, commit**

```bash
task web:test
task web:typecheck
git add web/src/components/SWPCAlerts.tsx web/src/components/SWPCAlerts.test.tsx
git commit -m "feat(p2): SWPCAlerts component (per-series tag color + relative time)"
```

---
## Task 16: Frontend `EarthquakeList` component

**Files:**
- Create: `web/src/components/EarthquakeList.tsx`
- Create: `web/src/components/EarthquakeList.test.tsx`

**Dependencies:** — depends on: Task 13 (frontend wire layer).

**Why:** Block 2 of the Events page (design §7.2). Renders significant earthquakes as an AntD `List` with magnitude-colored Tags, place title, depth/time/tsunami description, and an external link. **Returns `null` when payload is empty so the page hides the block.**

- [ ] **Step 16.1: Write the failing test**

`web/src/components/EarthquakeList.test.tsx`:

```tsx
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { EarthquakeList } from './EarthquakeList';
import type { Quake } from '../api/types';

function mk(over: Partial<Quake>): Quake {
  return {
    id: 'usqx',
    magnitude: 5.5,
    place: '100km W of Town',
    time: '2026-05-02T10:00:00Z',
    updatedAt: '2026-05-02T10:05:00Z',
    lat: 40.0,
    lon: -120.0,
    depthKm: 12.0,
    tsunami: false,
    url: 'https://earthquake.usgs.gov/usqx',
    ...over,
  };
}

describe('EarthquakeList', () => {
  it('returns null when payload is empty', () => {
    const { container } = render(<EarthquakeList quakes={[]} />);
    expect(container.firstChild).toBeNull();
  });

  it('renders a row per quake with place + magnitude + depth', () => {
    render(<EarthquakeList quakes={[mk({}), mk({ id: 'usqy', magnitude: 7.0, place: 'Ocean' })]} />);
    expect(screen.getByText('100km W of Town')).toBeInTheDocument();
    expect(screen.getByText('Ocean')).toBeInTheDocument();
    expect(screen.getAllByText(/12 km/).length).toBeGreaterThan(0);
  });

  it('uses tier-based magnitude color via data-mag-tier', () => {
    render(<EarthquakeList quakes={[
      mk({ id: '1', magnitude: 4.5 }),
      mk({ id: '2', magnitude: 5.5 }),
      mk({ id: '3', magnitude: 6.5 }),
      mk({ id: '4', magnitude: 7.5 }),
    ]} />);
    const tags = screen.getAllByTestId('quake-mag-tag');
    expect(tags[0].getAttribute('data-mag-tier')).toBe('lt5');
    expect(tags[1].getAttribute('data-mag-tier')).toBe('5to6');
    expect(tags[2].getAttribute('data-mag-tier')).toBe('6to7');
    expect(tags[3].getAttribute('data-mag-tier')).toBe('ge7');
  });

  it('renders tsunami icon when flag is true', () => {
    render(<EarthquakeList quakes={[mk({ tsunami: true })]} />);
    expect(screen.getByLabelText('tsunami-flagged')).toBeInTheDocument();
  });

  it('renders PAGER alert tag when present', () => {
    render(<EarthquakeList quakes={[mk({ alert: 'yellow' })]} />);
    expect(screen.getByText(/PAGER: yellow/i)).toBeInTheDocument();
  });

  it('renders external link to USGS page', () => {
    render(<EarthquakeList quakes={[mk({ url: 'https://earthquake.usgs.gov/usqx' })]} />);
    const link = screen.getByRole('link') as HTMLAnchorElement;
    expect(link.href).toContain('usqx');
    expect(link.target).toBe('_blank');
    expect(link.rel).toContain('noopener');
  });
});
```

- [ ] **Step 16.2: Implement `web/src/components/EarthquakeList.tsx`**

```tsx
import { List, Space, Tag, Typography } from 'antd';
import { ExportOutlined } from '@ant-design/icons';
import type { Quake } from '../api/types';

interface Props {
  quakes: Quake[];
}

type MagTier = 'lt5' | '5to6' | '6to7' | 'ge7';

function magTier(mag: number): MagTier {
  if (mag >= 7.0) return 'ge7';
  if (mag >= 6.0) return '6to7';
  if (mag >= 5.0) return '5to6';
  return 'lt5';
}

const TIER_COLOR: Record<MagTier, string> = {
  lt5:  'blue',
  '5to6': 'orange',
  '6to7': 'red',
  ge7:  'magenta',
};

function relative(iso: string): string {
  const diffMs = Date.now() - new Date(iso).getTime();
  const mins = Math.round(diffMs / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

export function EarthquakeList({ quakes }: Props) {
  if (quakes.length === 0) return null;
  return (
    <List
      header={<Typography.Title level={5} style={{ margin: 0 }}>Significant earthquakes</Typography.Title>}
      dataSource={quakes}
      renderItem={(q) => {
        const tier = magTier(q.magnitude);
        return (
          <List.Item
            actions={q.url ? [
              <a key="link" href={q.url} target="_blank" rel="noopener noreferrer">
                <ExportOutlined /> USGS
              </a>,
            ] : []}
          >
            <List.Item.Meta
              avatar={
                <Tag
                  color={TIER_COLOR[tier]}
                  data-mag-tier={tier}
                  data-testid="quake-mag-tag"
                >
                  M{q.magnitude.toFixed(1)}
                </Tag>
              }
              title={q.place}
              description={
                <Space size="small" wrap>
                  <Typography.Text type="secondary">{Math.round(q.depthKm)} km depth</Typography.Text>
                  <Typography.Text type="secondary">· {relative(q.time)}</Typography.Text>
                  {q.tsunami && <span aria-label="tsunami-flagged">🌊</span>}
                  {q.alert && <Tag color="purple">PAGER: {q.alert}</Tag>}
                </Space>
              }
            />
          </List.Item>
        );
      }}
    />
  );
}
```

- [ ] **Step 16.3: Run tests, commit**

```bash
task web:test
task web:typecheck
git add web/src/components/EarthquakeList.tsx web/src/components/EarthquakeList.test.tsx
git commit -m "feat(p2): EarthquakeList component (mag-tier color + tsunami icon + PAGER tag)"
```

---

## Task 17: Frontend `VolcanoList` component

**Files:**
- Create: `web/src/components/VolcanoList.tsx`
- Create: `web/src/components/VolcanoList.test.tsx`

**Dependencies:** — depends on: Task 13 (frontend wire layer).

**Why:** Block 3 of the Events page (design §7.2). Renders elevated-alert volcanoes as an AntD `List` with name title; alert-level Tag (`ADVISORY` yellow, `WATCH` orange, `WARNING` red); color-code Tag; truncated synopsis; external link. **Returns `null` when empty.**

- [ ] **Step 17.1: Write the failing test**

`web/src/components/VolcanoList.test.tsx`:

```tsx
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { VolcanoList } from './VolcanoList';
import type { Volcano } from '../api/types';

function mk(over: Partial<Volcano>): Volcano {
  return {
    id: 'avo-redoubt',
    name: 'Redoubt',
    region: 'Alaska',
    lat: 60.4853,
    lon: -152.7438,
    alert: 'WATCH',
    color: 'ORANGE',
    updatedAt: '2026-05-02T10:00:00Z',
    synopsis: 'Elevated unrest with periodic seismicity.',
    url: 'https://volcanoes.usgs.gov/volcano/avo-redoubt',
    ...over,
  };
}

describe('VolcanoList', () => {
  it('returns null when payload is empty', () => {
    const { container } = render(<VolcanoList volcanoes={[]} />);
    expect(container.firstChild).toBeNull();
  });

  it('renders a row per volcano with name + region', () => {
    render(<VolcanoList volcanoes={[mk({}), mk({ id: 'b', name: 'Shasta', region: 'CalVO' })]} />);
    expect(screen.getByText('Redoubt')).toBeInTheDocument();
    expect(screen.getByText('Shasta')).toBeInTheDocument();
    expect(screen.getByText(/Alaska/)).toBeInTheDocument();
    expect(screen.getByText(/CalVO/)).toBeInTheDocument();
  });

  it('renders alert-level tag color via data-alert', () => {
    render(<VolcanoList volcanoes={[
      mk({ id: 'a', alert: 'ADVISORY' }),
      mk({ id: 'b', name: 'B', alert: 'WATCH' }),
      mk({ id: 'c', name: 'C', alert: 'WARNING' }),
    ]} />);
    const tags = screen.getAllByTestId('volcano-alert-tag');
    expect(tags[0].getAttribute('data-alert')).toBe('ADVISORY');
    expect(tags[1].getAttribute('data-alert')).toBe('WATCH');
    expect(tags[2].getAttribute('data-alert')).toBe('WARNING');
  });

  it('renders the color-code chip', () => {
    render(<VolcanoList volcanoes={[mk({ color: 'ORANGE' })]} />);
    expect(screen.getByText('ORANGE')).toBeInTheDocument();
  });

  it('renders external link', () => {
    render(<VolcanoList volcanoes={[mk({ url: 'https://volcanoes.usgs.gov/volcano/avo-redoubt' })]} />);
    const link = screen.getByRole('link') as HTMLAnchorElement;
    expect(link.href).toContain('avo-redoubt');
    expect(link.target).toBe('_blank');
    expect(link.rel).toContain('noopener');
  });
});
```

- [ ] **Step 17.2: Implement `web/src/components/VolcanoList.tsx`**

```tsx
import { List, Space, Tag, Typography } from 'antd';
import { ExportOutlined } from '@ant-design/icons';
import type { AlertLevel, ColorCode, Volcano } from '../api/types';

interface Props {
  volcanoes: Volcano[];
}

const ALERT_COLOR: Record<AlertLevel, string> = {
  NORMAL:   'green',
  ADVISORY: 'gold',
  WATCH:    'orange',
  WARNING:  'red',
};

const COLOR_CHIP: Record<ColorCode, string> = {
  GREEN:  'green',
  YELLOW: 'gold',
  ORANGE: 'orange',
  RED:    'red',
};

function truncate(s: string, max = 150): string {
  if (s.length <= max) return s;
  return s.slice(0, max - 1) + '…';
}

function relative(iso: string): string {
  if (!iso) return '';
  const diffMs = Date.now() - new Date(iso).getTime();
  const mins = Math.round(diffMs / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

export function VolcanoList({ volcanoes }: Props) {
  if (volcanoes.length === 0) return null;
  return (
    <List
      header={<Typography.Title level={5} style={{ margin: 0 }}>Volcanoes at elevated alert</Typography.Title>}
      dataSource={volcanoes}
      renderItem={(v) => (
        <List.Item
          actions={v.url ? [
            <a key="link" href={v.url} target="_blank" rel="noopener noreferrer">
              <ExportOutlined /> USGS
            </a>,
          ] : []}
        >
          <List.Item.Meta
            title={v.name}
            description={
              <Space size="small" wrap>
                <Typography.Text type="secondary">{v.region}</Typography.Text>
                <Tag
                  color={ALERT_COLOR[v.alert]}
                  data-alert={v.alert}
                  data-testid="volcano-alert-tag"
                >
                  {v.alert}
                </Tag>
                <Tag color={COLOR_CHIP[v.color]}>{v.color}</Tag>
                {v.synopsis && (
                  <Typography.Text type="secondary">{truncate(v.synopsis)}</Typography.Text>
                )}
                {v.updatedAt && (
                  <Typography.Text type="secondary">· {relative(v.updatedAt)}</Typography.Text>
                )}
              </Space>
            }
          />
        </List.Item>
      )}
    />
  );
}
```

- [ ] **Step 17.3: Run tests, commit**

```bash
task web:test
task web:typecheck
git add web/src/components/VolcanoList.tsx web/src/components/VolcanoList.test.tsx
git commit -m "feat(p2): VolcanoList component (alert-level + color-code chips)"
```

---

## Task 18: SpaceWeather page rewrite

**Files:**
- Modify: `web/src/pages/SpaceWeather.tsx` (replace `<Empty>` with the two new blocks)
- Create: `web/src/pages/SpaceWeather.test.tsx`

**Dependencies:** — depends on: Task 13 (frontend wire layer), Task 14 (SWPCForecast), Task 15 (SWPCAlerts).

**Why:** Phase 0 left this page as `<Empty>`. Phase 2 turns it into two stacked blocks: 3-day forecast cards on top, active alerts list below. Page subscribes to the live snapshot store; SSE updates flow in via the existing `connect` machinery wired from `Overview.tsx` (which keeps the shared subscription alive across page navigations because Zustand state is global).

- [ ] **Step 18.1: Write the failing tests**

`web/src/pages/SpaceWeather.test.tsx`:

```tsx
import { afterEach, describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import SpaceWeather from './SpaceWeather';
import { useSnapshotStore } from '../store/snapshot';

afterEach(() => {
  useSnapshotStore.setState({ snapshot: null, connection: 'connecting' });
  vi.restoreAllMocks();
});

describe('SpaceWeather page', () => {
  it('renders skeleton forecast cards when snapshot is null', () => {
    render(<SpaceWeather />);
    expect(screen.getAllByLabelText(/forecast-skeleton/i).length).toBe(3);
  });

  it('renders forecast cards + alerts list when snapshot is populated', () => {
    useSnapshotStore.setState({
      snapshot: {
        serverTime: 't',
        sources: {
          swpc_scales: {
            source: 'swpc_scales', fetchedAt: '2026-05-02T12:00:00Z',
            payload: {
              days: [
                { date: '2026-05-03', r1: 5,  r3: 1, s1: 1 },
                { date: '2026-05-04', r1: 30, r3: 5, s1: 5, g: 'G4', gText: 'G4 watch' },
                { date: '2026-05-05', r1: 60, r3: 25, s1: 80, g: 'G5' },
              ],
            },
          },
          swpc_alerts: {
            source: 'swpc_alerts', fetchedAt: '2026-05-02T12:00:00Z',
            payload: [
              { code: 'K08A', series: 'K-Index', description: 'K=8 = G4', issued: '2026-05-02T11:00:00Z', message: 'event observed.' },
            ],
          },
        },
      },
      connection: 'live',
    });
    render(<SpaceWeather />);
    expect(screen.getByText('2026-05-03')).toBeInTheDocument();
    expect(screen.getByText('K08A')).toBeInTheDocument();
    expect(screen.getByText(/K-Index/)).toBeInTheDocument();
  });

  it('shows Empty state for the alerts block when alerts are empty', () => {
    useSnapshotStore.setState({
      snapshot: {
        serverTime: 't',
        sources: {
          swpc_scales: {
            source: 'swpc_scales', fetchedAt: '2026-05-02T12:00:00Z',
            payload: {
              days: [
                { date: '2026-05-03', r1: 5, r3: 0, s1: 0 },
                { date: '2026-05-04', r1: 5, r3: 0, s1: 0 },
                { date: '2026-05-05', r1: 5, r3: 0, s1: 0 },
              ],
            },
          },
          swpc_alerts: {
            source: 'swpc_alerts', fetchedAt: '2026-05-02T12:00:00Z',
            payload: [],
          },
        },
      },
      connection: 'live',
    });
    render(<SpaceWeather />);
    expect(screen.getByText(/No active SWPC alerts/i)).toBeInTheDocument();
  });
});
```

- [ ] **Step 18.2: Implement `web/src/pages/SpaceWeather.tsx`**

```tsx
import { Card, Space, Typography } from 'antd';
import { SWPCAlerts } from '../components/SWPCAlerts';
import { SWPCForecast } from '../components/SWPCForecast';
import { useSnapshotStore } from '../store/snapshot';

export default function SpaceWeather() {
  const snapshot = useSnapshotStore((s) => s.snapshot);
  const scales = snapshot?.sources.swpc_scales;
  const alerts = snapshot?.sources.swpc_alerts;

  return (
    <Space direction="vertical" size="middle" style={{ width: '100%', padding: 24 }}>
      <Typography.Title level={3} style={{ margin: 0 }}>Space Weather</Typography.Title>
      <Card title="3-day NOAA scales forecast" extra={scales?.fetchedAt ? (
        <Typography.Text type="secondary">as of {new Date(scales.fetchedAt).toUTCString()}</Typography.Text>
      ) : null}>
        <SWPCForecast forecast={scales?.payload ?? null} fetchedAt={scales?.fetchedAt ?? null} />
      </Card>
      <Card title="Active SWPC alerts" extra={alerts?.fetchedAt ? (
        <Typography.Text type="secondary">as of {new Date(alerts.fetchedAt).toUTCString()}</Typography.Text>
      ) : null}>
        <SWPCAlerts alerts={alerts?.payload ?? []} />
      </Card>
    </Space>
  );
}
```

- [ ] **Step 18.3: Run tests, commit**

```bash
task web:test
task web:typecheck
git add web/src/pages/SpaceWeather.tsx web/src/pages/SpaceWeather.test.tsx
git commit -m "feat(p2): SpaceWeather page (forecast cards + alerts list)"
```

---

## Task 19: Events page rewrite

**Files:**
- Modify: `web/src/pages/Events.tsx` (replace `<Empty>` with three stacked blocks)
- Create: `web/src/pages/Events.test.tsx`

**Dependencies:** — depends on: Task 13 (wire layer), Task 16 (EarthquakeList), Task 17 (VolcanoList). The Phase 1 `TsunamiPanel` is reused unchanged.

**Why:** Phase 0 left this page as `<Empty>`. Phase 2 turns it into three stacked blocks (top to bottom): tsunami panel (anchored at `id="tsunami"` for the deep-link from Overview's badge), significant earthquakes, elevated volcanoes. Each block self-hides when its source has zero items (per design §7.2).

- [ ] **Step 19.1: Write the failing tests**

`web/src/pages/Events.test.tsx`:

```tsx
import { afterEach, describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import Events from './Events';
import { useSnapshotStore } from '../store/snapshot';

afterEach(() => {
  useSnapshotStore.setState({ snapshot: null, connection: 'connecting' });
  vi.restoreAllMocks();
});

describe('Events page', () => {
  it('renders the page title even with no data', () => {
    render(<Events />);
    expect(screen.getByRole('heading', { name: /Events/i })).toBeInTheDocument();
  });

  it('exposes the tsunami anchor id for deep-link from Overview', () => {
    const { container } = render(<Events />);
    const anchor = container.querySelector('#tsunami');
    expect(anchor).not.toBeNull();
  });

  it('renders all three blocks when snapshot is fully populated', () => {
    useSnapshotStore.setState({
      snapshot: {
        serverTime: 't',
        sources: {
          nws_alerts: {
            source: 'nws_alerts', fetchedAt: 't',
            payload: [{
              id: 'tsu1', event: 'Tsunami Warning', awips: 'TSUWCA',
              headline: 'Tsunami Warning issued', severity: 'Extreme',
              sent: '2026-05-02T11:00:00Z', effective: '2026-05-02T11:00:00Z',
              expires: '2026-05-02T17:00:00Z', areas: ['Coastal CA'],
              category: 'Tsunami', wfo: 'NTW',
            }],
          },
          usgs_quakes: {
            source: 'usgs_quakes', fetchedAt: 't',
            payload: [{ id: 'q1', magnitude: 6.5, place: 'Ocean', time: '2026-05-02T10:00:00Z', updatedAt: 't', lat: 0, lon: 0, depthKm: 12, tsunami: false }],
          },
          usgs_volcanoes: {
            source: 'usgs_volcanoes', fetchedAt: 't',
            payload: [{ id: 'v1', name: 'Redoubt', region: 'Alaska', lat: 60, lon: -152, alert: 'WATCH', color: 'ORANGE', updatedAt: '2026-05-02T10:00:00Z' }],
          },
        },
      },
      connection: 'live',
    });
    render(<Events />);
    expect(screen.getByText(/Tsunami Warning issued/)).toBeInTheDocument();
    expect(screen.getByText('Ocean')).toBeInTheDocument();
    expect(screen.getByText('Redoubt')).toBeInTheDocument();
  });

  it('hides quake + volcano blocks (returns null) when those sources are empty', () => {
    useSnapshotStore.setState({
      snapshot: {
        serverTime: 't',
        sources: {
          usgs_quakes:    { source: 'usgs_quakes',    fetchedAt: 't', payload: [] },
          usgs_volcanoes: { source: 'usgs_volcanoes', fetchedAt: 't', payload: [] },
        },
      },
      connection: 'live',
    });
    render(<Events />);
    expect(screen.queryByText(/Significant earthquakes/i)).toBeNull();
    expect(screen.queryByText(/Volcanoes at elevated alert/i)).toBeNull();
  });
});
```

- [ ] **Step 19.2: Implement `web/src/pages/Events.tsx`**

```tsx
import { Space, Typography } from 'antd';
import { EarthquakeList } from '../components/EarthquakeList';
import { TsunamiPanel } from '../components/TsunamiPanel';
import { VolcanoList } from '../components/VolcanoList';
import { useSnapshotStore } from '../store/snapshot';

export default function Events() {
  const snapshot = useSnapshotStore((s) => s.snapshot);
  const alerts = snapshot?.sources.nws_alerts?.payload ?? [];
  const quakes = snapshot?.sources.usgs_quakes?.payload ?? [];
  const volcanoes = snapshot?.sources.usgs_volcanoes?.payload ?? [];

  return (
    <Space direction="vertical" size="middle" style={{ width: '100%', padding: 24 }}>
      <Typography.Title level={3} style={{ margin: 0 }}>Events</Typography.Title>
      <div id="tsunami">
        <TsunamiPanel alerts={alerts} />
      </div>
      <EarthquakeList quakes={quakes} />
      <VolcanoList volcanoes={volcanoes} />
    </Space>
  );
}
```

- [ ] **Step 19.3: Run tests, commit**

```bash
task web:test
task web:typecheck
git add web/src/pages/Events.tsx web/src/pages/Events.test.tsx
git commit -m "feat(p2): Events page (tsunami + earthquakes + volcanoes)"
```

---
## Task 20: Overview update — drop TsunamiPanel, redirect Tsunami badge to `/events#tsunami`

**Files:**
- Modify: `web/src/pages/Overview.tsx`
- Modify: `web/src/pages/Overview.test.tsx`

**Dependencies:** — depends on: Task 19 (Events page renders the tsunami anchor at `id="tsunami"`).

**Why:** Per design §7.3 and decision 8, the TsunamiPanel migrates from Overview to Events. The AlertBadgeBar's Tsunami-badge `onClick` now navigates to `/events#tsunami` via `useNavigate`. This is a minimal Overview change — no other Overview behavior changes.

- [ ] **Step 20.1: Replace `web/src/pages/Overview.test.tsx`**

```tsx
import { afterEach, describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import Overview from './Overview';
import { useSnapshotStore } from '../store/snapshot';

describe('Overview page', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    useSnapshotStore.setState({ snapshot: null, connection: 'connecting' });
  });

  function renderWithRouter() {
    return render(
      <MemoryRouter initialEntries={['/']}>
        <Routes>
          <Route path="/" element={<Overview />} />
          <Route path="/events" element={<div data-testid="events-page" />} />
        </Routes>
      </MemoryRouter>,
    );
  }

  it('renders badge bar after initial snapshot', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        serverTime: 't',
        sources: { nws_alerts: { source: 'nws_alerts', fetchedAt: 't', payload: [] } },
      }),
    }));
    class FakeES {
      addEventListener() {}
      close() {}
      onerror: any = null;
    }
    vi.stubGlobal('EventSource', FakeES as any);

    renderWithRouter();
    await waitFor(() => expect(screen.getByLabelText('Tornado')).toBeInTheDocument());
  });

  it('navigates to /events#tsunami when Tsunami badge is clicked with non-zero count', async () => {
    useSnapshotStore.setState({
      snapshot: {
        serverTime: 't',
        sources: {
          nws_alerts: {
            source: 'nws_alerts', fetchedAt: 't',
            payload: [{
              id: 'tsu1', event: 'Tsunami Warning', awips: 'TSUWCA',
              headline: 'Tsunami', severity: 'Extreme',
              sent: 't', effective: 't', expires: 't', areas: [], category: 'Tsunami',
            }],
          },
        },
      },
      connection: 'live',
    });
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => ({ serverTime: 't', sources: {} }) }));
    class FakeES { addEventListener() {} close() {} onerror: any = null; }
    vi.stubGlobal('EventSource', FakeES as any);

    renderWithRouter();
    const tag = await screen.findByLabelText('Tsunami');
    fireEvent.click(tag);
    await waitFor(() => expect(screen.getByTestId('events-page')).toBeInTheDocument());
  });
});
```

- [ ] **Step 20.2: Rewrite `web/src/pages/Overview.tsx`**

```tsx
import { useEffect } from 'react';
import { Card, Space, Typography } from 'antd';
import { useNavigate } from 'react-router-dom';
import { AlertBadgeBar } from '../components/AlertBadgeBar';
import { useSnapshotStore } from '../store/snapshot';
import { connect } from '../api/stream';

export default function Overview() {
  const navigate = useNavigate();
  const snapshot = useSnapshotStore((s) => s.snapshot);
  const setSnapshot = useSnapshotStore((s) => s.setSnapshot);
  const applyUpdate = useSnapshotStore((s) => s.applyUpdate);
  const setConnection = useSnapshotStore((s) => s.setConnection);

  useEffect(() => {
    let cancel: (() => void) | undefined;
    void connect({
      onSnapshot: (s) => {
        setSnapshot(s);
        setConnection('live');
      },
      onUpdate: (name, env) => applyUpdate(name, env),
      onError: () => setConnection('error'),
    }).then((c) => {
      cancel = c;
    });
    return () => cancel?.();
  }, [setSnapshot, applyUpdate, setConnection]);

  const alerts = snapshot?.sources.nws_alerts?.payload ?? [];
  const fetchedAt = snapshot?.sources.nws_alerts?.fetchedAt;

  return (
    <Space direction="vertical" size="middle" style={{ width: '100%', padding: 24 }}>
      <Typography.Title level={3} style={{ margin: 0 }}>Overview</Typography.Title>
      <Card
        title="Active Alerts"
        extra={fetchedAt ? (
          <Typography.Text type="secondary">
            as of {new Date(fetchedAt).toUTCString()}
          </Typography.Text>
        ) : null}
      >
        <AlertBadgeBar
          alerts={alerts}
          onTsunamiClick={() => navigate('/events#tsunami')}
        />
      </Card>
    </Space>
  );
}
```

- [ ] **Step 20.3: Run tests, commit**

```bash
task web:test
task web:typecheck
git add web/src/pages/Overview.tsx web/src/pages/Overview.test.tsx
git commit -m "feat(p2): Overview drops TsunamiPanel; Tsunami badge → /events#tsunami"
```

---

## Task 21: README + smoke task extension + final integration check

**Files:**
- Modify: `README.md`
- Modify: `Taskfile.yml` (extend `smoke` to cover all 5 sources)

**Dependencies:** — depends on: all prior tasks.

**Why:** Final-mile work: document the Phase 2 status and per-source env knobs, extend the smoke target to assert all 5 sources are healthy, and run the whole pipeline (build, lint, test, browser verify).

- [ ] **Step 21.1: Update `README.md`**

Add a "Phase 2 status" section (after the Phase 1 section) describing:

- What's live now: `nws_alerts`, `swpc_scales`, `swpc_alerts`, `usgs_quakes`, `usgs_volcanoes` end-to-end. SpaceWeather page (3-day forecast cards + alerts list), Events page (tsunami + earthquakes + volcanoes), Overview's Tsunami-badge deep-links to `/events#tsunami`.
- Per-source env vars (full table):
  ```
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
- Per-source cadence table:
  | Source | Default interval | Validator | Notes |
  |---|---|---|---|
  | `nws_alerts` | 30s | sha256 content hash | api.weather.gov politeness |
  | `swpc_scales` | 60s | sha256 content hash | matches upstream max-age=60 |
  | `swpc_alerts` | 60s | sha256 content hash | 24h window + product allowlist |
  | `usgs_quakes` | 60s | upstream ETag | If-None-Match passthrough |
  | `usgs_volcanoes` | 5m | upstream ETag | NORMAL filtered server-side |
- Note that the HazSimp category map expansion (Tornado Watch, Flood Warning, etc.) is tracked in [issue #3](https://github.com/jacaudi/cwd/issues/3) for a future PR.

- [ ] **Step 21.2: Extend `Taskfile.yml`'s `smoke` target**

Replace the existing `smoke:` target with:

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
        echo "---- assert all 5 sources healthy ----"
        curl -sS http://127.0.0.1:8765/api/sources | jq -e '
          (.nws_alerts.consecutiveFailures == 0)
          and (.swpc_scales.consecutiveFailures == 0)
          and (.swpc_alerts.consecutiveFailures == 0)
          and (.usgs_quakes.consecutiveFailures == 0)
          and (.usgs_volcanoes.consecutiveFailures == 0)
        ' > /dev/null && echo "OK: all 5 sources healthy" || (echo "FAIL: some source failing" && exit 1)
        echo "---- holding for 4 more minutes (Ctrl-C to stop early) ----"
        sleep 240
```

- [ ] **Step 21.3: Run the full integration check**

```bash
task test
task lint
task web:test
task web:typecheck
task web:build

# CRITICAL — restore the webdist placeholder before staging anything else.
# task web:build overwrites internal/webdist/dist/index.html with the real
# Vite output, which must NEVER be committed.
git checkout 0668675 -- internal/webdist/dist/index.html

task build
./bin/cwd serve &
PID=$!
sleep 90
curl -sS http://127.0.0.1:8765/healthz
curl -sS http://127.0.0.1:8765/readyz
curl -sS http://127.0.0.1:8765/api/sources | jq .
curl -sS http://127.0.0.1:8765/api/snapshot | jq '.sources | keys'
kill $PID
```

Expected:
- `task test`, `task lint`, `task web:test`, `task web:typecheck` all green.
- `task web:build` succeeds and produces `internal/webdist/dist/index.html` + asset bundle.
- After the placeholder restore, `git status internal/webdist/dist/index.html` shows no change.
- `/readyz` returns 200 within ~90s.
- `/api/sources` shows 5 entries, all with `consecutiveFailures: 0`.
- `/api/snapshot` `.sources | keys` includes `["nws_alerts","swpc_alerts","swpc_scales","usgs_quakes","usgs_volcanoes"]` (or a subset based on what's currently active — `swpc_scales` and `usgs_volcanoes` always have data; the others may be empty in quiet conditions).

- [ ] **Step 21.4: Open the browser and verify the pages**

```bash
./bin/cwd serve
# In another terminal: open http://127.0.0.1:8765/
```

Manual checks (per design §11):
1. **Overview** — Alert badge bar populates. Click the **Tsunami** badge: navigates to `/events#tsunami`.
2. **Space Weather** (`/space`) — 3-day forecast cards render with the upcoming dates; G-scale chips render where applicable; alerts list shows recent codes (or `<Empty>` if none in the 24h window).
3. **Events** (`/events`) — tsunami panel renders only when active TSU* alerts exist; significant earthquakes list renders only when `usgs_quakes.payload` is non-empty; volcanoes list renders only when `usgs_volcanoes.payload` is non-empty.
4. **Footer** — `SourceHealthIndicator` shows 5 green tags (one per source).

If anything renders but stays stuck on its skeleton/loading state, check the browser console for SSE errors and tail the server logs for `server.hotstart_decode` warnings.

- [ ] **Step 21.5: Commit + open PR**

```bash
git diff --cached internal/webdist/dist/index.html
# (should be empty — placeholder is restored, real dist is not staged)

git add README.md Taskfile.yml
git commit -m "docs(p2): README phase-2 notes + 5-source smoke task"

git push -u origin feature/phase2-multi-source
gh pr create --title "Phase 2: SWPC + USGS sources end-to-end" --body "$(cat <<'EOF'
## Summary
- Adds `swpc_scales`, `swpc_alerts`, `usgs_quakes`, `usgs_volcanoes` end-to-end on the Phase 1 pipeline.
- Lights up `/space` (3-day forecast + alerts) and `/events` (tsunami + earthquakes + volcanoes).
- Migrates the TsunamiPanel from Overview to Events (Tsunami badge now deep-links to `/events#tsunami`).

## Out of scope
- HazSimp category map expansion — tracked in #3.
- Status banner, 3-day Outlook timeline, per-source teasers on Overview — Phase 6.
- Image proxy + Hazards page — Phase 3.

## Test plan
- [ ] `task test` green
- [ ] `task lint` green
- [ ] `task web:test` green
- [ ] `task web:typecheck` green
- [ ] `task web:build` succeeds
- [ ] `task smoke` reports all 5 sources `consecutiveFailures: 0`
- [ ] Browser: `/`, `/space`, `/events` render live data; footer shows 5 green source tags

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

---

## Definition of done

Verbatim from design §11, plus per-task gates:

- [ ] Design doc + this implementation plan committed under `docs/plans/`
- [ ] All work in `.worktrees/phase2-multi-source` on branch `feature/phase2-multi-source` off main at `0668675`
- [ ] All Go tests pass with `-race`; `golangci-lint run ./...` clean
- [ ] All frontend tests pass (`task web:test`); `task web:typecheck` clean; `task web:build` succeeds
- [ ] `task build:all && ./bin/cwd serve` shows live data on `/space` (3-day forecast cards + alerts list) and `/events` (tsunami panel + significant earthquakes list + elevated volcanoes list); footer shows 5 green source tags
- [ ] `task smoke` reports all 5 sources `consecutiveFailures: 0`
- [ ] `internal/webdist/dist/index.html` is the placeholder, NOT the real Vite output (verify with `git diff 0668675 -- internal/webdist/dist/index.html` → empty)
- [ ] Independent comprehensive review passes (the worktree+subagent execution rule's final review step)
- [ ] PR opened to main, CI green, merged with per-task history preserved
- [ ] Worktree cleaned up (`git worktree remove .worktrees/phase2-multi-source`)
- [ ] Issue #3 referenced in PR description as the tracked follow-up for category map expansion

### Per-task gates

Every task ends with: tests green (`-race` for Go, `task web:test` for frontend), lint green (`golangci-lint run ./internal/<pkg>/...` for Go-touching tasks; `task web:typecheck` for frontend-touching tasks), and a single conventional-prefix commit (`feat(p2):` / `test(p2):` / `chore(p2):` / `fix(p2):` / `docs(p2):`). No task is marked complete until those three gates pass — that is the `superpowers:verification-before-completion` contract.

---



## Self-review (writing-plans skill checklist)

### Spec coverage

| Design § | Task(s) |
|---|---|
| §1 Goal & scope | Tasks 5–8 (sources), 18–19 (pages), 20 (Overview migrate) |
| §2 Decisions log (1: single PR; 2: Overview unchanged; 3: 3-day forecast; 4: window+allowlist; 5: significant_day; 6: getElevatedVolcanoes; 7: HazSimp unchanged; 8: tsunami → /events; 9: NWS-only filter) | All decisions are settled in the design — this plan does not relitigate them; verifications are spread across Tasks 5–10, 18–20 |
| §3 Upstream constraints (validators, cadences, UA) | Task 5 (sha256), Task 6 (sha256), Task 7 (ETag), Task 8 (ETag); all sources reuse Phase 1 UA threading via the `userAgent` ctor argument |
| §4.1 Package additions (8 new Go files + 6 new TSX files) | Tasks 5–8, 13–17 |
| §4.2 Source contract continuity | Tasks 5–8 (each implements the `Source` interface verbatim) |
| §5 Wire types (SWPCDay/SWPCForecast, SWPCAlert, Quake, Volcano + AlertLevel + ColorCode) | Task 5 (SWPCForecast), Task 6 (SWPCAlert), Task 7 (Quake), Task 8 (Volcano + enums), Task 10 (snapshot Envelope), Task 13 (TS mirror) |
| §6.1 SWPC scales data flow | Task 5 |
| §6.2 SWPC alerts data flow + product code namespace comment | Task 6 |
| §6.3 USGS quakes data flow + alternative-feeds comment | Task 7 |
| §6.4 USGS volcanoes data flow + RSS-fallback comment + NORMAL filter | Task 8 |
| §6.5 SSE hub change (no structural change; enabledNames grows) | Task 12 (no hub edits — confirmed in step 12.3) |
| §6.6 Hot-start type assertion via helper | Task 9 (helper) + Task 12 (integration) |
| §7.1 SpaceWeather page (forecast block + alerts block) | Task 14 + Task 15 + Task 18 |
| §7.2 Events page (tsunami + quakes + volcanoes; self-hide on empty) | Task 16 + Task 17 + Task 19 |
| §7.3 Overview change (drop TsunamiPanel; navigate to /events#tsunami) | Task 20 |
| §7.4 SourceHealthIndicator unchanged (5 tags) | No code change required; confirmed by Task 12 boot test asserting all 5 names appear in `/api/sources` |
| §8 Configuration (no new keys; boot validation extensions; per-source env override pattern) | Task 11 (validation) + Conventions section (env override pattern documented; no env-walker code change needed) |
| §9 Testing (backend per-source + snapshot + server boot; frontend per-component + per-page; smoke) | Tasks 5–8 (per-source), Task 10 (snapshot), Task 12 (server boot), Tasks 14–17 (component), Tasks 18–19 (page), Task 21 (smoke) |
| §10 `/api/sources` shape (5 keys) | Task 12 boot test asserts all 5 keys; no handler change needed |
| §11 Definition of done | Definition of done section above |
| §12 Out of scope | Documented in design; no tasks (intentionally) |
| §13 Risks (SWPC drift, USGS shape, polling load, NORMAL downgrade silent, SSE bandwidth) | Acknowledged in Task 6 (unknown code passes through with empty desc — non-canary), Task 8 (NORMAL silent drop — design accepts), Task 12 (boot test exercises 5-source contention path) |

### Placeholder scan

Searched the plan body for `TBD`, `TODO`, `implement later`, `add appropriate`, `similar to`, and `etc.` — none present in the plan body. (The string `TODO` appears only inside example code-block comments where it is part of the reference content.)

### Type consistency

- Go: `SWPCForecast.Days [3]SWPCDay`; TS: `SWPCForecast.days: [SWPCDay, SWPCDay, SWPCDay]` — tuple length matches.
- Go `SWPCAlert{Code, Series, Description, Issued, Message, URL}` ↔ TS `SWPCAlert{code, series, description, issued, message, url?}` — JSON tags match field-for-field.
- Go `Quake{ID, Magnitude, Place, Time, UpdatedAt, Lat, Lon, DepthKm, Tsunami, Alert?, URL?}` ↔ TS `Quake{id, magnitude, place, time, updatedAt, lat, lon, depthKm, tsunami, alert?, url?}` — JSON tags match.
- Go `Volcano{ID, Name, Region, Lat, Lon, Alert, Color, UpdatedAt, Synopsis?, URL?}` ↔ TS `Volcano{id, name, region, lat, lon, alert, color, updatedAt, synopsis?, url?}` — JSON tags match.
- Go `AlertLevel` enum members `NORMAL/ADVISORY/WATCH/WARNING` ↔ TS `AlertLevel` union — match. Same for `ColorCode` `GREEN/YELLOW/ORANGE/RED`.
- Hot-start helper signatures match the names exported by each source file (`SWPCScalesName`, `SWPCAlertsName`, `USGSQuakesName`, `USGSVolcanoesName`, `NWSAlertsName`).
- `SourcePayloadMap` (Task 13) names match `Snapshot.sources` keys exactly.

### Open questions for the executor (gaps the design did NOT cover)

1. **SWPC `issue_datetime` timezone (design §6.2):** The design specifies the field is parsed but doesn't explicitly state the upstream tz. The plan assumes UTC (treats `2026-05-02 10:00:00.000` as a UTC wall-clock via `time.Parse(layout, ...)`, which produces a UTC `time.Time`). If SWPC documents this as Eastern or some other zone, the parser will need a `time.LoadLocation` step. **Action for executor: confirm against the captured fixture's `issue_datetime` values vs. published SWPC timing during Step 6.1 fixture sanity-check; flag if they look offset from real-time SWPC alerts feed cadence.**
2. **USGS `getElevatedVolcanoes` field name casing (design §6.4):** The design names the wire fields generically (`alert`, `color`, `synopsis`, `region`) but the upstream uses `alertLevel` / `colorCode` / `summary` / `volcanoName` / etc. The plan picks the upstream names for the `rawVolcano` struct and maps them into the documented `Volcano` shape. **Action for executor: verify against the live capture in Step 4.2 — if any field is missing from the upstream JSON, log a Debug-level message and tolerate zero values rather than failing the parse.**
3. **`usgs_volcanoes` URL field (design §6.4):** The design lists `URL` on the wire shape but doesn't specify where it comes from in the upstream. The plan reads it from `r.URL`; if the upstream uses a different key (e.g. `volcanoUrl`), the executor should add a json tag mapping rather than synthesize a URL. Do not invent a `https://volcanoes.usgs.gov/...` template — leave the field empty if the upstream omits it.

These three are the only items in the design that require the executor to make a concrete choice based on the live upstream shape during fixture capture (Tasks 1–4). Everything else is fully specified.

### Notes from reading the actual Phase 1 code (drift from design assumptions)

- The Phase 1 `cache.Cache` uses `*Cache` and `Envelope` lives in package `cache`, while the design's wire-shape sketch shows `Envelope[T]` as a generic in `internal/api/snapshot.go`. In practice the existing `internal/api/snapshot.go` defines a non-generic `Envelope` struct with `Payload any`, and that is what Task 10 extends — the design's `SourcesMap` struct with `*Envelope[T]` fields is conceptual, not literal. The plan honors the actual Go shape.
- `internal/sse/hub.go applyFilter` already returns the env unchanged for any `name != "nws_alerts"`, so the Phase 2 §6.5 statement "no code change to the hub itself" is verified by reading the file. Task 12 confirms this with no edits to `hub.go`.
- `cmd/cwd/main.go` is unchanged in Phase 2 — `server.Run` does all wiring. Confirmed by reading the file (it just builds the slog.Logger and calls `server.Run(ctx, cfg, logger, nil)`).
- The Phase 1 fetcher's `Health` struct is already source-agnostic and the `healthFn` adapter in `server.go` already loops over `fetchers` — so `/api/sources` automatically surfaces all 5 entries the moment Task 12 adds the constructors. No `internal/api/sources.go` edits are needed.


