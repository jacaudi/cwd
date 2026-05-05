# Self-Hosted CWD — Phase 4 (History page — per-source operational time series) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the placeholder `/history` page with an operational time-series dashboard that charts the 5 data sources (NWS alerts / SWPC scales / SWPC alerts / USGS quakes / USGS volcanoes) over a 24h / 7d / 30d window, fed by a rewritten multi-source `/api/history` endpoint that downsamples server-side.

**Architecture:** Two new query helpers in `internal/store` (`Range` for raw rows, `RangeBuckets` for downsampled aggregations) plus a volcano-diff helper. The new `/api/history` handler is per-source (5 parallel requests on page load) with per-source aggregators living in the api package. Frontend uses `@ant-design/charts` (Area / Mix / Column / Scatter) plus a custom AntD Timeline for volcanoes; live-tail re-uses the existing `<source>.update` SSE events — no changes to `internal/cache/cache.go` or `internal/sse/hub.go`.

**Tech Stack:**
- Backend: Go 1.24, `chi` router, `gopkg.in/yaml.v3`, `log/slog`, `modernc.org/sqlite` (existing dependencies; no new Go deps)
- Frontend: Node 24, pnpm, Vite 5, React 18, TypeScript 5, AntD 5, `@ant-design/pro-components`, **NEW: `@ant-design/charts`**, Zustand, Vitest + `@testing-library/react` + jsdom
- Tooling: `golangci-lint` v2, `Taskfile.yml` (go-task)

**Source documents:**
- Design: [`docs/plans/2026-05-03-phase4-history-design.md`](2026-05-03-phase4-history-design.md)
- Phase 1 design: [`docs/plans/2026-05-02-phase1-nws-alerts-design.md`](2026-05-02-phase1-nws-alerts-design.md)
- Phase 2 design: [`docs/plans/2026-05-02-phase2-multi-source-design.md`](2026-05-02-phase2-multi-source-design.md)
- Phase 3 design: [`docs/plans/2026-05-03-phase3-image-proxy-design.md`](2026-05-03-phase3-image-proxy-design.md)
- Parent design: [`docs/plans/2026-05-01-self-hosted-cwd-design.md`](2026-05-01-self-hosted-cwd-design.md)
- Structural template: [`docs/plans/2026-05-02-phase2-multi-source-implementation.md`](2026-05-02-phase2-multi-source-implementation.md)

**Branch:** `feature/phase4-history` off main at the latest merged tip. As of plan time main is at `7f0e85d` (Phase 3 PR #5 merge); PRs #10 (plan docs), #11 (p3 cleanup), #14 (Hazards NCEP layout), and #15 (footer width) are open and stack toward main. Cut the worktree off whatever is `origin/main` at the moment Phase 4 starts. Substitute the actual SHA into the worktree command in Conventions §"Worktree setup" below.

> **For Claude:** REQUIRED EXECUTION WORKFLOW (follow in order):
> 1. `superpowers:using-git-worktrees` — Isolate work in a dedicated worktree (`.worktrees/phase4-history`)
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

- All commits prefixed `feat(p4):`, `test(p4):`, `chore(p4):`, `fix(p4):`, `docs(p4):`. Co-authored trailer per repo norm.
- All Go tests use stdlib `testing` + `httptest`; no external test frameworks. Table-driven where appropriate.
- Logging: stdlib `log/slog`. Use structured key/value fields; never format into the message.
- No CGO. SQLite is `modernc.org/sqlite` exclusively (already in `go.sum`).
- Frontend tests live next to the component (`Component.test.tsx`).
- `golangci-lint` v2 must stay clean (`task lint`).
- Never `git add -A` or `git add .`. Never skip hooks (`--no-verify`, `--no-gpg-sign`).

### Commit author identity

Already configured at the repo level (`jacaudi <47005674+jacaudi@users.noreply.github.com>`). Do NOT run `git config user.email` or `git config user.name`.

### Stage + commit by explicit pathspec

Every commit step **must** stage AND commit by explicit pathspec:

```bash
git add path/to/file_a.go path/to/file_b.go
git commit -m "feat(p4): explanatory message" -- path/to/file_a.go path/to/file_b.go
```

The trailing `-- <pathspec>` form scopes the commit even if a sibling subagent stages something between your `git add` and `git commit`. Phase 2 task-1-through-4 observed this race; the discipline survives it.

### Worktree setup

```bash
# Replace <BASE-SHA> with the current origin/main tip at start time.
# As of plan-write, that is 7f0e85d, but expect it to advance once
# PRs #10/#11/#14/#15 merge.
git fetch origin
BASE=$(git rev-parse origin/main)
git worktree add -b feature/phase4-history .worktrees/phase4-history "$BASE"
cd .worktrees/phase4-history
```

All subsequent commands assume CWD is the worktree.

### Trap — webdist placeholder file (do NOT stage)

The repo's embed target `internal/webdist/dist/index.html` is a **committed placeholder** that exists so the Go package compiles before any frontend build. The placeholder content lives at `<BASE-SHA>:internal/webdist/dist/index.html` and is small (~10 lines).

`task web:build` overwrites it with a real Vite build. **Never commit the real build artifact.** **Only Task 15** (final integration) should run `task web:build`. Earlier frontend tasks (T8–T14) verify with `task web:test` and `task web:typecheck` only.

After running `task web:build` (in Task 15), restore the placeholder before any `git add`:

```bash
git checkout "$BASE" -- internal/webdist/dist/index.html
```

### Trap — cache + SSE (do NOT touch)

`internal/cache/cache.go` (Phase 1's race fix) and `internal/sse/hub.go` (Phase 2's pass-through invariant) MUST be byte-identical to the branch base at PR time. Run `git diff "$BASE" -- internal/cache/cache.go internal/sse/hub.go` before opening the PR — must be empty.

Phase 4 reads from the store (not the cache), and live-tails via the existing `<source>.update` events the hub already broadcasts. There is no path through which Phase 4 needs to modify either file.

### Trap — sources package shape (do NOT touch)

Phase 4 does not extend the `internal/sources.Source` interface or the per-source `Fetch`/`Parse` shapes. The aggregators that turn raw snapshots into per-bucket payloads live in `internal/api/history.go` (where they're used by the handler), keyed by the source-name constants `sources.NWSAlertsName` etc. This keeps `internal/sources` oblivious to history aggregation and `internal/store` oblivious to source schema.

### Trap — store query gzip handling

Existing `internal/store/snapshots.go` writes payloads gzipped via `gzipBytes`. The `scanRow` helper auto-gunzips when reading single rows. Range queries (Task 1) must call the same `gunzipBytes` for each row in the result set — do NOT rely on raw `payload` bytes from the SQL row directly.

---

## Dependency graph (visual)

```
1 store.Range/RangeBuckets ────┐
2 store.VolcanoStateChanges ───┤
3 web @ant-design/charts dep ──┤   (5 tasks parallel — different files)
4 web/api/history.ts ──────────┤
5 web/store/history.ts ────────┘
        │
        ▼
        ├──► 6 api/history.go (handler + aggregators) ◀── 1, 2
        │      │
        │      └──► 7 server wiring ◀── 6
        │
        ├──► 8 NWSAlertsArea  ◀── 3
        ├──► 9 SWPCScalesLine ◀── 3
        ├──► 10 SWPCAlertsBars ◀── 3
        ├──► 11 USGSQuakesScatter ◀── 3
        ├──► 12 VolcanoesTimeline ◀── (no chart lib needed)
        │
        └──► 13 HistoryChartRow ◀── 4, 5, 8, 9, 10, 11, 12
                  │
                  ▼
                  14 History page rewrite ◀── 13
                          │
                          ▼
                          15 README + smoke + final integration ◀── 7, 14
```

Shape: **storage helpers + frontend wire layer + chart-lib install fan in. Backend converges at the handler + server wiring; frontend converges at HistoryChartRow then the page; README + smoke + final integration is the final fan-in.**

Parallelism windows:
- **Wave 1** (5 parallel): Tasks 1, 2, 3, 4, 5 — five different file regions, no shared code.
- **Wave 2** (1): Task 6 — handler depends on store helpers.
- **Wave 3** (6 parallel): Task 7 (server wiring) + Tasks 8/9/10/11/12 (chart components). Task 7 is in `internal/server/`, the chart components are five separate files in `web/src/components/charts/`.
- **Wave 4** (1): Task 13 — HistoryChartRow depends on api client + history store + every chart component.
- **Wave 5** (1): Task 14 — History page depends on HistoryChartRow.
- **Wave 6** (1): Task 15 — final integration.

No two parallel-runnable tasks touch the same file. Per-task pathspec discipline (see Conventions §"Stage + commit by explicit pathspec") provides belt-and-suspenders.

---

## Task 1: Store range queries — `Range` + `RangeBuckets`

**Files:**
- Create: `internal/store/range.go`
- Create: `internal/store/range_test.go`

**Dependencies:** none.

**Why:** Design §5.1. The handler (Task 6) needs both raw-row queries (for the quakes scatter, which sends events not buckets) and downsampled bucket queries (for area/line/bar charts). Both queries operate on the existing `snapshots` table; bucket grouping is done in SQL via integer division of `fetched_at - from_ms` by `bucket_ms`.

- [ ] **Step 1.1: Write the failing test**

`internal/store/range_test.go`:

```go
package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestStore_Range_RoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 5; i++ {
		ts := t0.Add(time.Duration(i) * time.Minute)
		payload, _ := json.Marshal(map[string]int{"i": i})
		if err := s.Append(ctx, "test_source", ts, "v"+string(rune('a'+i)), payload); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}

	rows, err := s.Range(ctx, "test_source", t0, t0.Add(10*time.Minute))
	if err != nil {
		t.Fatalf("Range: %v", err)
	}
	if got, want := len(rows), 5; got != want {
		t.Fatalf("len(rows) = %d, want %d", got, want)
	}
	for i, r := range rows {
		if got, want := r.FetchedAt.UTC(), t0.Add(time.Duration(i)*time.Minute); !got.Equal(want) {
			t.Errorf("row %d FetchedAt = %s, want %s", i, got, want)
		}
		var p map[string]int
		if err := json.Unmarshal(r.Payload, &p); err != nil {
			t.Errorf("row %d unmarshal: %v", i, err)
		}
		if p["i"] != i {
			t.Errorf("row %d payload i = %d, want %d", i, p["i"], i)
		}
	}
}

func TestStore_Range_FiltersBySource(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	_ = s.Append(ctx, "src_a", t0, "va", []byte(`{"x":1}`))
	_ = s.Append(ctx, "src_b", t0.Add(time.Minute), "vb", []byte(`{"x":2}`))

	rows, err := s.Range(ctx, "src_a", t0.Add(-time.Hour), t0.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(rows); got != 1 {
		t.Fatalf("len(rows) = %d, want 1", got)
	}
	if rows[0].Source != "src_a" {
		t.Errorf("Source = %q, want src_a", rows[0].Source)
	}
}

func TestStore_Range_EmptyReturnsEmpty(t *testing.T) {
	s := newTestStore(t)
	rows, err := s.Range(context.Background(), "nope", time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if got := len(rows); got != 0 {
		t.Errorf("len(rows) = %d, want 0", got)
	}
}

func TestStore_RangeBuckets_HonorsN(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)

	// 24 rows, 1 hour apart over 24h.
	for i := 0; i < 24; i++ {
		ts := t0.Add(time.Duration(i) * time.Hour)
		payload, _ := json.Marshal(map[string]int{"v": i})
		_ = s.Append(ctx, "test_source", ts, time.RFC3339Nano, payload)
	}

	// Aggregator that records the count it saw.
	var seenCounts []int
	agg := func(rows []Row) ([]byte, error) {
		seenCounts = append(seenCounts, len(rows))
		return []byte(`{}`), nil
	}

	buckets, err := s.RangeBuckets(ctx, "test_source", t0, t0.Add(24*time.Hour), 6, agg)
	if err != nil {
		t.Fatalf("RangeBuckets: %v", err)
	}
	if got := len(buckets); got != 6 {
		t.Errorf("len(buckets) = %d, want 6", got)
	}
	// 24 rows / 6 buckets = 4 rows per bucket
	for i, c := range seenCounts {
		if c != 4 {
			t.Errorf("bucket %d row count = %d, want 4", i, c)
		}
	}
}

func TestStore_RangeBuckets_AggregatorErrorPropagates(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
	_ = s.Append(ctx, "test_source", t0, "v", []byte(`{}`))

	wantErr := errAggBoom
	agg := func(rows []Row) ([]byte, error) { return nil, wantErr }

	_, err := s.RangeBuckets(ctx, "test_source", t0.Add(-time.Hour), t0.Add(time.Hour), 1, agg)
	if err == nil {
		t.Fatal("RangeBuckets: want error, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("err = %v, want errors.Is %v", err, wantErr)
	}
}

func TestStore_RangeBuckets_BucketStartUsesFirstRow(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
	_ = s.Append(ctx, "test_source", t0.Add(15*time.Minute), "v1", []byte(`{}`))
	_ = s.Append(ctx, "test_source", t0.Add(45*time.Minute), "v2", []byte(`{}`))
	_ = s.Append(ctx, "test_source", t0.Add(75*time.Minute), "v3", []byte(`{}`))

	agg := func(rows []Row) ([]byte, error) { return []byte(`{}`), nil }
	buckets, err := s.RangeBuckets(ctx, "test_source", t0, t0.Add(2*time.Hour), 2, agg)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(buckets); got != 2 {
		t.Fatalf("len(buckets) = %d, want 2", got)
	}
	// Bucket 0 spans [t0, t0+1h); first row in it is t0+15m.
	if !buckets[0].BucketStart.Equal(t0.Add(15 * time.Minute)) {
		t.Errorf("buckets[0].BucketStart = %s, want %s", buckets[0].BucketStart, t0.Add(15*time.Minute))
	}
	// Bucket 1 spans [t0+1h, t0+2h); first row is t0+75m.
	if !buckets[1].BucketStart.Equal(t0.Add(75 * time.Minute)) {
		t.Errorf("buckets[1].BucketStart = %s, want %s", buckets[1].BucketStart, t0.Add(75*time.Minute))
	}
}

var errAggBoom = errSentinel("aggregator boom")

type errSentinel string

func (e errSentinel) Error() string { return string(e) }
```

Plus the `errors` import at the top (Go's stdlib) and a small helper:

```go
import "errors"

// at top of test file (after imports):
func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return s
}
```

Add `path/filepath` to imports too.

- [ ] **Step 1.2: Run, confirm fail**

```bash
go test ./internal/store/ -run TestStore_Range -v
```

Expected: build fails — `Range`, `RangeBuckets`, `Bucket`, `Aggregator` undefined.

- [ ] **Step 1.3: Implement `internal/store/range.go`**

```go
package store

import (
	"context"
	"fmt"
	"math"
	"time"
)

// Bucket is one downsampled point in a time series.
type Bucket struct {
	// BucketStart is the FetchedAt of the first row that fell into this
	// bucket, in UTC. Charts use this as the X-axis value.
	BucketStart time.Time
	// Count is the number of source-snapshots that fell in this bucket.
	Count int
	// Payload is the bucket-aggregated payload (shape depends on the
	// caller's Aggregator).
	Payload []byte
}

// Aggregator combines N raw payloads into a single per-bucket payload.
// Examples: max(activeCount) for NWS, last for SWPC scales, sum-by-severity
// for SWPC alerts.
type Aggregator func(rows []Row) ([]byte, error)

// Range returns every snapshot row for source whose fetched_at is in [from, to].
// Rows are returned in ascending time order, fully decompressed.
func (s *Store) Range(ctx context.Context, source string, from, to time.Time) ([]Row, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT source, fetched_at, COALESCE(etag, ''), payload
		 FROM snapshots
		 WHERE source = ? AND fetched_at BETWEEN ? AND ?
		 ORDER BY fetched_at ASC`,
		source, from.UnixMilli(), to.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("store: range query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]Row, 0, 64)
	for rows.Next() {
		var r Row
		var ms int64
		var gzipped []byte
		if err := rows.Scan(&r.Source, &ms, &r.Validator, &gzipped); err != nil {
			return nil, fmt.Errorf("store: range scan: %w", err)
		}
		r.FetchedAt = time.UnixMilli(ms).UTC()
		payload, err := gunzipBytes(gzipped)
		if err != nil {
			return nil, fmt.Errorf("store: range gunzip: %w", err)
		}
		r.Payload = payload
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: range iter: %w", err)
	}
	return out, nil
}

// RangeBuckets returns at most n buckets covering [from, to], aggregating each
// bucket via agg. Buckets with zero matching rows are omitted from the result.
// Bucket size = ceil((to - from) / n) seconds, minimum 1 second.
func (s *Store) RangeBuckets(ctx context.Context, source string, from, to time.Time, n int, agg Aggregator) ([]Bucket, error) {
	if n <= 0 {
		return nil, fmt.Errorf("store: RangeBuckets: n must be > 0, got %d", n)
	}
	if !to.After(from) {
		return nil, fmt.Errorf("store: RangeBuckets: to (%s) must be after from (%s)", to, from)
	}
	if agg == nil {
		return nil, fmt.Errorf("store: RangeBuckets: aggregator required")
	}

	// Bucket size in seconds, ceiling so the last bucket fits.
	totalSec := to.Sub(from).Seconds()
	bucketSec := int64(math.Ceil(totalSec / float64(n)))
	if bucketSec < 1 {
		bucketSec = 1
	}
	bucketMS := bucketSec * 1000

	// Single query that groups by bucket index and returns per-bucket
	// rows. We still aggregate in Go (the aggregator may need the raw
	// gzipped payloads), but the GROUP BY keeps us from over-fetching.
	rows, err := s.db.QueryContext(ctx,
		`SELECT
		   ((fetched_at - ?) / ?) AS bucket_idx,
		   source, fetched_at, COALESCE(etag, ''), payload
		 FROM snapshots
		 WHERE source = ? AND fetched_at BETWEEN ? AND ?
		 ORDER BY fetched_at ASC`,
		from.UnixMilli(), bucketMS,
		source, from.UnixMilli(), to.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("store: range-buckets query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type pendingBucket struct {
		idx  int64
		rows []Row
	}
	var current *pendingBucket
	out := make([]Bucket, 0, n)
	flush := func() error {
		if current == nil || len(current.rows) == 0 {
			return nil
		}
		payload, err := agg(current.rows)
		if err != nil {
			return fmt.Errorf("store: aggregator: %w", err)
		}
		out = append(out, Bucket{
			BucketStart: current.rows[0].FetchedAt,
			Count:       len(current.rows),
			Payload:     payload,
		})
		return nil
	}
	for rows.Next() {
		var idx int64
		var r Row
		var ms int64
		var gzipped []byte
		if err := rows.Scan(&idx, &r.Source, &ms, &r.Validator, &gzipped); err != nil {
			return nil, fmt.Errorf("store: range-buckets scan: %w", err)
		}
		r.FetchedAt = time.UnixMilli(ms).UTC()
		payload, err := gunzipBytes(gzipped)
		if err != nil {
			return nil, fmt.Errorf("store: range-buckets gunzip: %w", err)
		}
		r.Payload = payload
		if current == nil || idx != current.idx {
			if err := flush(); err != nil {
				return nil, err
			}
			current = &pendingBucket{idx: idx}
		}
		current.rows = append(current.rows, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: range-buckets iter: %w", err)
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return out, nil
}
```

- [ ] **Step 1.4: Run tests + lint**

```bash
go test ./internal/store/ -v -race
golangci-lint run ./internal/store/...
```

Expected: all pass; lint clean.

- [ ] **Step 1.5: Commit**

```bash
git add internal/store/range.go internal/store/range_test.go
git commit -m "feat(p4): store Range + RangeBuckets (raw + downsampled queries)" -- \
  internal/store/range.go internal/store/range_test.go
```

---

## Task 2: Store volcano-diff helper — `VolcanoStateChanges`

**Files:**
- Create: `internal/store/volcanoes_diff.go`
- Create: `internal/store/volcanoes_diff_test.go`

**Dependencies:** none.

**Why:** Design §5.2 + §3.5. Volcanoes don't fit a continuous chart — the data is a list of currently-elevated volcanoes that transitions rarely. Phase 4 surfaces a diff-feed of state-change events. The helper walks adjacent snapshot pairs in `[from, to]` and emits one `VolcanoStateChange` per delta.

The volcano payload shape (from Phase 2's `internal/sources/usgs_volcanoes.go`) is `[]VolcanoEntry` where each entry has `name`, `observatory`, and `alertLevel`. We compare two sets keyed by `observatory + name` to find appearances, removals, and level changes.

- [ ] **Step 2.1: Write the failing test**

`internal/store/volcanoes_diff_test.go`:

```go
package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// payload helper — match the shape internal/sources/usgs_volcanoes uses.
type vEntry struct {
	Name        string `json:"name"`
	Observatory string `json:"observatory"`
	AlertLevel  string `json:"alertLevel"`
}

func vPayload(t *testing.T, entries ...vEntry) []byte {
	t.Helper()
	b, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestStore_VolcanoStateChanges_DetectsAppearance(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
	_ = s.Append(ctx, "usgs_volcanoes", t0, "v1", vPayload(t))
	_ = s.Append(ctx, "usgs_volcanoes", t0.Add(time.Minute), "v2", vPayload(t,
		vEntry{Name: "Great Sitkin", Observatory: "AVO", AlertLevel: "WATCH"}))

	changes, err := s.VolcanoStateChanges(ctx, t0.Add(-time.Hour), t0.Add(time.Hour), 50)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(changes); got != 1 {
		t.Fatalf("len(changes) = %d, want 1", got)
	}
	c := changes[0]
	if c.Volcano != "AVO Great Sitkin" {
		t.Errorf("Volcano = %q, want %q", c.Volcano, "AVO Great Sitkin")
	}
	if c.Prior != "" {
		t.Errorf("Prior = %q, want empty (newly appeared)", c.Prior)
	}
	if c.Current != "WATCH" {
		t.Errorf("Current = %q, want WATCH", c.Current)
	}
}

func TestStore_VolcanoStateChanges_DetectsLevelChange(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
	_ = s.Append(ctx, "usgs_volcanoes", t0, "v1", vPayload(t,
		vEntry{Name: "Kilauea", Observatory: "HVO", AlertLevel: "ADVISORY"}))
	_ = s.Append(ctx, "usgs_volcanoes", t0.Add(time.Minute), "v2", vPayload(t,
		vEntry{Name: "Kilauea", Observatory: "HVO", AlertLevel: "WATCH"}))

	changes, err := s.VolcanoStateChanges(ctx, t0.Add(-time.Hour), t0.Add(time.Hour), 50)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(changes); got != 1 {
		t.Fatalf("len(changes) = %d, want 1", got)
	}
	c := changes[0]
	if c.Prior != "ADVISORY" || c.Current != "WATCH" {
		t.Errorf("change = %s→%s, want ADVISORY→WATCH", c.Prior, c.Current)
	}
}

func TestStore_VolcanoStateChanges_DetectsDisappearance(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
	_ = s.Append(ctx, "usgs_volcanoes", t0, "v1", vPayload(t,
		vEntry{Name: "Kilauea", Observatory: "HVO", AlertLevel: "WATCH"}))
	_ = s.Append(ctx, "usgs_volcanoes", t0.Add(time.Minute), "v2", vPayload(t))

	changes, err := s.VolcanoStateChanges(ctx, t0.Add(-time.Hour), t0.Add(time.Hour), 50)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(changes); got != 1 {
		t.Fatalf("len(changes) = %d, want 1", got)
	}
	c := changes[0]
	if c.Prior != "WATCH" || c.Current != "" {
		t.Errorf("change = %s→%s, want WATCH→<empty>", c.Prior, c.Current)
	}
}

func TestStore_VolcanoStateChanges_NoChangeNoEvent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
	v := vEntry{Name: "Kilauea", Observatory: "HVO", AlertLevel: "WATCH"}
	_ = s.Append(ctx, "usgs_volcanoes", t0, "v1", vPayload(t, v))
	_ = s.Append(ctx, "usgs_volcanoes", t0.Add(time.Minute), "v2", vPayload(t, v))

	changes, err := s.VolcanoStateChanges(ctx, t0.Add(-time.Hour), t0.Add(time.Hour), 50)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(changes); got != 0 {
		t.Errorf("len(changes) = %d, want 0", got)
	}
}

func TestStore_VolcanoStateChanges_HonorsMaxEvents(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
	// 5 distinct volcanoes appear in succession.
	for i := 0; i < 5; i++ {
		entries := make([]vEntry, 0, i+1)
		for j := 0; j <= i; j++ {
			entries = append(entries, vEntry{
				Name: "V" + string(rune('A'+j)), Observatory: "X", AlertLevel: "ADVISORY",
			})
		}
		_ = s.Append(ctx, "usgs_volcanoes", t0.Add(time.Duration(i)*time.Minute),
			"v"+string(rune('a'+i)), vPayload(t, entries...))
	}

	changes, err := s.VolcanoStateChanges(ctx, t0.Add(-time.Hour), t0.Add(time.Hour), 3)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(changes); got != 3 {
		t.Errorf("len(changes) = %d, want 3 (cap respected)", got)
	}
}

func TestStore_VolcanoStateChanges_AscendingOrder(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
	_ = s.Append(ctx, "usgs_volcanoes", t0, "v1", vPayload(t))
	_ = s.Append(ctx, "usgs_volcanoes", t0.Add(time.Minute), "v2", vPayload(t,
		vEntry{Name: "A", Observatory: "X", AlertLevel: "WATCH"}))
	_ = s.Append(ctx, "usgs_volcanoes", t0.Add(2*time.Minute), "v3", vPayload(t,
		vEntry{Name: "A", Observatory: "X", AlertLevel: "WATCH"},
		vEntry{Name: "B", Observatory: "X", AlertLevel: "WARNING"}))

	changes, err := s.VolcanoStateChanges(ctx, t0.Add(-time.Hour), t0.Add(time.Hour), 50)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(changes); got != 2 {
		t.Fatalf("len(changes) = %d, want 2", got)
	}
	if !changes[0].At.Before(changes[1].At) {
		t.Errorf("changes not in ascending order: %s then %s", changes[0].At, changes[1].At)
	}
}
```

- [ ] **Step 2.2: Run, confirm fail**

```bash
go test ./internal/store/ -run TestStore_VolcanoStateChanges -v
```

Expected: build fails — `VolcanoStateChange`, `VolcanoStateChanges` undefined.

- [ ] **Step 2.3: Implement `internal/store/volcanoes_diff.go`**

```go
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// VolcanoStateChange captures one volcano transition between two snapshots.
type VolcanoStateChange struct {
	At      time.Time // fetched_at of the snapshot in which the change first appears
	Volcano string    // observatory + " " + name (e.g. "AVO Great Sitkin")
	Prior   string    // alert level before, "" if newly elevated
	Current string    // alert level after, "" if removed from elevated list
}

// volcanoEntry mirrors the minimum fields the api/store layers need from
// internal/sources/usgs_volcanoes.go. Kept private to the store so a sources-side
// schema rename doesn't require store changes.
type volcanoEntry struct {
	Name        string `json:"name"`
	Observatory string `json:"observatory"`
	AlertLevel  string `json:"alertLevel"`
}

// VolcanoStateChanges walks adjacent snapshot pairs in [from, to] and returns
// every transition in ascending time order. Capped at maxEvents (caller passes
// 50 in production).
func (s *Store) VolcanoStateChanges(ctx context.Context, from, to time.Time, maxEvents int) ([]VolcanoStateChange, error) {
	if maxEvents <= 0 {
		return nil, fmt.Errorf("store: VolcanoStateChanges: maxEvents must be > 0, got %d", maxEvents)
	}
	rows, err := s.Range(ctx, "usgs_volcanoes", from, to)
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return nil, nil
	}

	// Decode each row's payload once.
	parsed := make([]map[string]string, len(rows)) // key → level
	for i, row := range rows {
		var entries []volcanoEntry
		if err := json.Unmarshal(row.Payload, &entries); err != nil {
			return nil, fmt.Errorf("store: VolcanoStateChanges: decode row %d: %w", i, err)
		}
		m := make(map[string]string, len(entries))
		for _, e := range entries {
			key := strings.TrimSpace(e.Observatory + " " + e.Name)
			m[key] = e.AlertLevel
		}
		parsed[i] = m
	}

	out := make([]VolcanoStateChange, 0, maxEvents)
	for i := 1; i < len(rows); i++ {
		prev, cur := parsed[i-1], parsed[i]
		// Appearances + level changes.
		for k, lvl := range cur {
			if priorLvl, had := prev[k]; !had {
				out = append(out, VolcanoStateChange{
					At: rows[i].FetchedAt, Volcano: k, Prior: "", Current: lvl,
				})
			} else if priorLvl != lvl {
				out = append(out, VolcanoStateChange{
					At: rows[i].FetchedAt, Volcano: k, Prior: priorLvl, Current: lvl,
				})
			}
			if len(out) >= maxEvents {
				return out[:maxEvents], nil
			}
		}
		// Disappearances.
		for k, lvl := range prev {
			if _, still := cur[k]; !still {
				out = append(out, VolcanoStateChange{
					At: rows[i].FetchedAt, Volcano: k, Prior: lvl, Current: "",
				})
			}
			if len(out) >= maxEvents {
				return out[:maxEvents], nil
			}
		}
	}
	return out, nil
}
```

- [ ] **Step 2.4: Run tests + lint**

```bash
go test ./internal/store/ -v -race
golangci-lint run ./internal/store/...
```

- [ ] **Step 2.5: Commit**

```bash
git add internal/store/volcanoes_diff.go internal/store/volcanoes_diff_test.go
git commit -m "feat(p4): store VolcanoStateChanges (diff-feed for adjacent snapshots)" -- \
  internal/store/volcanoes_diff.go internal/store/volcanoes_diff_test.go
```

---

## Task 3: Install `@ant-design/charts` dependency

**Files:**
- Modify: `web/package.json`
- Modify: `web/pnpm-lock.yaml` (auto-updated by pnpm)

**Dependencies:** none.

**Why:** Decision 6. The chart components (Tasks 8-12) need this library; pulling the install into its own task gives the rest of the frontend tasks a clean import to depend on.

- [ ] **Step 3.1: Install**

```bash
cd web
pnpm add @ant-design/charts
cd -
```

This adds an entry to `web/package.json` `dependencies` and updates `web/pnpm-lock.yaml`. Pin to whatever the latest stable is at install time (no version override).

- [ ] **Step 3.2: Verify build still typechecks**

```bash
task web:typecheck
```

Expected: clean (the lib is installed but nothing imports it yet).

- [ ] **Step 3.3: Verify tests still pass**

```bash
task web:test
```

Expected: all tests pass — no test changes yet.

- [ ] **Step 3.4: Commit**

```bash
git add web/package.json web/pnpm-lock.yaml
git commit -m "chore(p4): add @ant-design/charts dependency" -- \
  web/package.json web/pnpm-lock.yaml
```

---

## Task 4: Frontend API client — typed `/api/history` per source

**Files:**
- Create: `web/src/api/history.ts`
- Create: `web/src/api/history.test.ts`

**Dependencies:** none.

**Why:** Design §5.3. The 5 per-source response shapes are different enough that a single `History<T>` generic would hide more than it reveals. We expose one typed function per source so call sites get autocomplete for `buckets[].activeCount` vs `events[].mag`.

- [ ] **Step 4.1: Write the failing test**

`web/src/api/history.test.ts`:

```ts
import { describe, it, expect, beforeEach, vi } from 'vitest';
import {
  fetchNWSAlertsHistory,
  fetchSWPCScalesHistory,
  fetchSWPCAlertsHistory,
  fetchUSGSQuakesHistory,
  fetchUSGSVolcanoesHistory,
  type HistoryWindow,
} from './history';

describe('history client', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('fetchNWSAlertsHistory composes the URL and parses the typed payload', async () => {
    const fetchSpy = vi.spyOn(global, 'fetch').mockResolvedValue(new Response(JSON.stringify({
      source: 'nws_alerts',
      window: '24h',
      buckets: [{ at: '2026-05-03T12:00:00Z', activeCount: 42 }],
      windowStart: '2026-05-02T12:00:00Z',
      dataStart: '2026-05-02T12:00:00Z',
    }), { status: 200 }));

    const got = await fetchNWSAlertsHistory('24h');
    expect(fetchSpy).toHaveBeenCalledWith('/api/history?source=nws_alerts&window=24h');
    expect(got.source).toBe('nws_alerts');
    expect(got.buckets).toHaveLength(1);
    expect(got.buckets[0].activeCount).toBe(42);
  });

  it('fetchUSGSQuakesHistory returns the events array shape', async () => {
    vi.spyOn(global, 'fetch').mockResolvedValue(new Response(JSON.stringify({
      source: 'usgs_quakes',
      window: '7d',
      events: [{ at: '2026-05-03T12:00:00Z', mag: 5.2, place: 'Alaska', depthKm: 12 }],
      windowStart: '2026-04-26T12:00:00Z',
      dataStart: '2026-04-26T12:00:00Z',
    }), { status: 200 }));

    const got = await fetchUSGSQuakesHistory('7d');
    expect(got.events).toHaveLength(1);
    expect(got.events[0].mag).toBe(5.2);
  });

  it('fetchUSGSVolcanoesHistory returns the changes array shape', async () => {
    vi.spyOn(global, 'fetch').mockResolvedValue(new Response(JSON.stringify({
      source: 'usgs_volcanoes',
      window: '30d',
      changes: [{ at: '2026-05-03T12:00:00Z', volcano: 'AVO Great Sitkin', prior: 'YELLOW', current: 'ORANGE' }],
      windowStart: '2026-04-03T12:00:00Z',
      dataStart: '2026-04-03T12:00:00Z',
    }), { status: 200 }));

    const got = await fetchUSGSVolcanoesHistory('30d');
    expect(got.changes).toHaveLength(1);
    expect(got.changes[0].volcano).toBe('AVO Great Sitkin');
  });

  it('throws on non-200 response', async () => {
    vi.spyOn(global, 'fetch').mockResolvedValue(new Response('bad', { status: 400 }));
    await expect(fetchNWSAlertsHistory('24h')).rejects.toThrow();
  });

  it('exports the HistoryWindow union with the three documented values', () => {
    const windows: HistoryWindow[] = ['24h', '7d', '30d'];
    expect(windows).toEqual(['24h', '7d', '30d']);
  });
});
```

- [ ] **Step 4.2: Run, confirm fail**

```bash
cd web && pnpm exec vitest run src/api/history.test.ts
```

Expected: module not found / undefined exports.

- [ ] **Step 4.3: Implement `web/src/api/history.ts`**

```ts
// Typed client for GET /api/history. One function per source so call sites get
// autocomplete for the source-specific response shape.

export type HistoryWindow = '24h' | '7d' | '30d';

interface HistoryEnvelope<S extends string> {
  source: S;
  window: HistoryWindow;
  windowStart: string;  // ISO-8601 UTC
  dataStart: string;    // ISO-8601 UTC; earliest available snapshot
}

export interface NWSAlertsHistory extends HistoryEnvelope<'nws_alerts'> {
  buckets: Array<{ at: string; activeCount: number }>;
}

export interface SWPCScalesHistory extends HistoryEnvelope<'swpc_scales'> {
  buckets: Array<{ at: string; gScale: number; r1: number; s1: number }>;
}

export interface SWPCAlertsHistory extends HistoryEnvelope<'swpc_alerts'> {
  buckets: Array<{ at: string; warning: number; watch: number; alert: number }>;
}

export interface USGSQuakesHistory extends HistoryEnvelope<'usgs_quakes'> {
  events: Array<{ at: string; mag: number; place: string; depthKm: number }>;
}

export interface USGSVolcanoesHistory extends HistoryEnvelope<'usgs_volcanoes'> {
  changes: Array<{ at: string; volcano: string; prior: string; current: string }>;
}

async function fetchHistory<T>(source: string, window: HistoryWindow): Promise<T> {
  const res = await fetch(`/api/history?source=${source}&window=${window}`);
  if (!res.ok) {
    throw new Error(`GET /api/history?source=${source}&window=${window}: ${res.status}`);
  }
  return (await res.json()) as T;
}

export const fetchNWSAlertsHistory = (w: HistoryWindow) =>
  fetchHistory<NWSAlertsHistory>('nws_alerts', w);
export const fetchSWPCScalesHistory = (w: HistoryWindow) =>
  fetchHistory<SWPCScalesHistory>('swpc_scales', w);
export const fetchSWPCAlertsHistory = (w: HistoryWindow) =>
  fetchHistory<SWPCAlertsHistory>('swpc_alerts', w);
export const fetchUSGSQuakesHistory = (w: HistoryWindow) =>
  fetchHistory<USGSQuakesHistory>('usgs_quakes', w);
export const fetchUSGSVolcanoesHistory = (w: HistoryWindow) =>
  fetchHistory<USGSVolcanoesHistory>('usgs_volcanoes', w);
```

- [ ] **Step 4.4: Run tests + typecheck**

```bash
task web:test
task web:typecheck
```

- [ ] **Step 4.5: Commit**

```bash
git add web/src/api/history.ts web/src/api/history.test.ts
git commit -m "feat(p4): typed /api/history client (5 per-source fetchers)" -- \
  web/src/api/history.ts web/src/api/history.test.ts
```

---

## Task 5: Frontend Zustand store — window selection + URL sync

**Files:**
- Create: `web/src/store/history.ts`
- Create: `web/src/store/history.test.ts`

**Dependencies:** none.

**Why:** Design §7.2. The window selection (`24h` / `7d` / `30d`) needs to persist across remounts (tab switches), reset on refresh, and round-trip with the URL `?w=` param so deep links and browser-back work.

- [ ] **Step 5.1: Write the failing test**

`web/src/store/history.test.ts`:

```ts
import { describe, it, expect, beforeEach } from 'vitest';
import { useHistoryStore } from './history';

describe('useHistoryStore', () => {
  beforeEach(() => {
    useHistoryStore.setState({ window: '24h' });
  });

  it('defaults to 24h', () => {
    expect(useHistoryStore.getState().window).toBe('24h');
  });

  it('setWindow updates the value', () => {
    useHistoryStore.getState().setWindow('7d');
    expect(useHistoryStore.getState().window).toBe('7d');
  });

  it('parseWindowFromURL returns the param when valid', () => {
    expect(useHistoryStore.getState().parseWindowFromURL('?w=7d')).toBe('7d');
    expect(useHistoryStore.getState().parseWindowFromURL('?w=30d')).toBe('30d');
  });

  it('parseWindowFromURL returns null when missing or invalid', () => {
    expect(useHistoryStore.getState().parseWindowFromURL('')).toBeNull();
    expect(useHistoryStore.getState().parseWindowFromURL('?w=99d')).toBeNull();
    expect(useHistoryStore.getState().parseWindowFromURL('?other=x')).toBeNull();
  });
});
```

- [ ] **Step 5.2: Run, confirm fail**

```bash
cd web && pnpm exec vitest run src/store/history.test.ts
```

Expected: module not found.

- [ ] **Step 5.3: Implement `web/src/store/history.ts`**

```ts
import { create } from 'zustand';
import type { HistoryWindow } from '../api/history';

interface HistoryState {
  window: HistoryWindow;
  setWindow: (w: HistoryWindow) => void;
  /**
   * Parse the ?w= query parameter from a URL search string. Returns the
   * window if it's one of the three valid values, otherwise null. Does not
   * touch state — call sites decide whether to seed the store with the
   * parsed value (typically once at History.tsx mount time).
   */
  parseWindowFromURL: (search: string) => HistoryWindow | null;
}

const VALID: ReadonlyArray<HistoryWindow> = ['24h', '7d', '30d'];

export const useHistoryStore = create<HistoryState>((set) => ({
  window: '24h',
  setWindow: (w) => set({ window: w }),
  parseWindowFromURL: (search) => {
    const params = new URLSearchParams(search);
    const v = params.get('w');
    if (!v) return null;
    if ((VALID as readonly string[]).includes(v)) return v as HistoryWindow;
    return null;
  },
}));
```

- [ ] **Step 5.4: Run tests + typecheck**

```bash
task web:test
task web:typecheck
```

- [ ] **Step 5.5: Commit**

```bash
git add web/src/store/history.ts web/src/store/history.test.ts
git commit -m "feat(p4): history Zustand store (window state + URL parser)" -- \
  web/src/store/history.ts web/src/store/history.test.ts
```

---

## Task 6: HTTP handler — `GET /api/history?source=<name>&window=24h|7d|30d`

**Files:**
- Modify: `internal/api/history.go` (REWRITE — drop `at=` semantics, add per-source aggregators)
- Modify: `internal/api/history_test.go` (REWRITE)

**Dependencies:** Task 1 (Range, RangeBuckets), Task 2 (VolcanoStateChanges).

**Why:** Design §5.3. The current handler hardcodes `nws_alerts` and uses `at=RFC3339` semantics that Phase 4 supersedes. Per-source aggregators live here (not in the store) so the store stays oblivious to source schemas.

- [ ] **Step 6.1: Write the failing test**

`internal/api/history_test.go`:

```go
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jacaudi/cwd/internal/store"
)

func newTestStoreForAPI(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return s
}

func TestHistoryHandler_BadRequest_MissingSource(t *testing.T) {
	s := newTestStoreForAPI(t)
	h := NewHistoryHandler(s)
	req := httptest.NewRequest(http.MethodGet, "/api/history?window=24h", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if got, want := w.Code, http.StatusBadRequest; got != want {
		t.Errorf("Code = %d, want %d", got, want)
	}
}

func TestHistoryHandler_BadRequest_UnknownSource(t *testing.T) {
	s := newTestStoreForAPI(t)
	h := NewHistoryHandler(s)
	req := httptest.NewRequest(http.MethodGet, "/api/history?source=mystery&window=24h", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if got, want := w.Code, http.StatusBadRequest; got != want {
		t.Errorf("Code = %d, want %d", got, want)
	}
}

func TestHistoryHandler_BadRequest_BadWindow(t *testing.T) {
	s := newTestStoreForAPI(t)
	h := NewHistoryHandler(s)
	req := httptest.NewRequest(http.MethodGet, "/api/history?source=nws_alerts&window=42m", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if got, want := w.Code, http.StatusBadRequest; got != want {
		t.Errorf("Code = %d, want %d", got, want)
	}
}

func TestHistoryHandler_NWSAlerts_BucketsHaveActiveCount(t *testing.T) {
	s := newTestStoreForAPI(t)
	ctx := context.Background()
	t0 := time.Now().UTC().Add(-time.Hour)

	// Two snapshots: 3 alerts, then 5 alerts.
	type alert struct {
		ID string `json:"id"`
	}
	mk := func(n int) []byte {
		alerts := make([]alert, n)
		for i := range alerts {
			alerts[i] = alert{ID: "a" + string(rune('a'+i))}
		}
		b, _ := json.Marshal(alerts)
		return b
	}
	_ = s.Append(ctx, "nws_alerts", t0, "v1", mk(3))
	_ = s.Append(ctx, "nws_alerts", t0.Add(30*time.Minute), "v2", mk(5))

	h := NewHistoryHandler(s)
	req := httptest.NewRequest(http.MethodGet, "/api/history?source=nws_alerts&window=24h", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if got, want := w.Code, http.StatusOK; got != want {
		t.Fatalf("Code = %d, want %d (body=%s)", got, want, w.Body.String())
	}

	var body struct {
		Source  string `json:"source"`
		Window  string `json:"window"`
		Buckets []struct {
			At          string `json:"at"`
			ActiveCount int    `json:"activeCount"`
		} `json:"buckets"`
		WindowStart string `json:"windowStart"`
		DataStart   string `json:"dataStart"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Source != "nws_alerts" {
		t.Errorf("Source = %q", body.Source)
	}
	if body.Window != "24h" {
		t.Errorf("Window = %q", body.Window)
	}
	if len(body.Buckets) == 0 {
		t.Fatalf("no buckets")
	}
	// Aggregator is max(activeCount); should see at least one bucket with 5.
	maxSeen := 0
	for _, b := range body.Buckets {
		if b.ActiveCount > maxSeen {
			maxSeen = b.ActiveCount
		}
	}
	if maxSeen != 5 {
		t.Errorf("max activeCount = %d, want 5", maxSeen)
	}

	if got := w.Header().Get("Cache-Control"); !strings.Contains(got, "max-age=60") {
		t.Errorf("Cache-Control = %q, want max-age=60", got)
	}
}

func TestHistoryHandler_USGSQuakes_RawEventsM4Plus(t *testing.T) {
	s := newTestStoreForAPI(t)
	ctx := context.Background()
	t0 := time.Now().UTC().Add(-time.Hour)

	// Each snapshot is a list of {features:[{properties:{mag, place, time}, geometry:{coordinates:[lon,lat,depth]}}, ...]}
	mk := func(mags ...float64) []byte {
		type props struct {
			Mag   float64 `json:"mag"`
			Place string  `json:"place"`
			Time  int64   `json:"time"`
		}
		type geom struct {
			Coordinates []float64 `json:"coordinates"`
		}
		type feat struct {
			Properties props `json:"properties"`
			Geometry   geom  `json:"geometry"`
		}
		feats := make([]feat, 0, len(mags))
		for _, m := range mags {
			feats = append(feats, feat{
				Properties: props{Mag: m, Place: "test", Time: t0.UnixMilli()},
				Geometry:   geom{Coordinates: []float64{0, 0, 10}},
			})
		}
		b, _ := json.Marshal(map[string]any{"features": feats})
		return b
	}

	_ = s.Append(ctx, "usgs_quakes", t0, "v1", mk(2.5, 4.2, 5.1, 3.9))

	h := NewHistoryHandler(s)
	req := httptest.NewRequest(http.MethodGet, "/api/history?source=usgs_quakes&window=24h", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if got, want := w.Code, http.StatusOK; got != want {
		t.Fatalf("Code = %d, want %d (body=%s)", got, want, w.Body.String())
	}

	var body struct {
		Events []struct {
			Mag float64 `json:"mag"`
		} `json:"events"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	// M ≥ 4 only: should keep 4.2 and 5.1, drop 2.5 and 3.9.
	if got := len(body.Events); got != 2 {
		t.Errorf("len(events) = %d, want 2 (M4+ filter)", got)
	}
}

func TestHistoryHandler_USGSVolcanoes_ChangesShape(t *testing.T) {
	s := newTestStoreForAPI(t)
	ctx := context.Background()
	t0 := time.Now().UTC().Add(-time.Hour)
	type vEntry struct {
		Name        string `json:"name"`
		Observatory string `json:"observatory"`
		AlertLevel  string `json:"alertLevel"`
	}
	mk := func(es ...vEntry) []byte { b, _ := json.Marshal(es); return b }
	_ = s.Append(ctx, "usgs_volcanoes", t0, "v1", mk())
	_ = s.Append(ctx, "usgs_volcanoes", t0.Add(time.Minute), "v2",
		mk(vEntry{Name: "Kilauea", Observatory: "HVO", AlertLevel: "WATCH"}))

	h := NewHistoryHandler(s)
	req := httptest.NewRequest(http.MethodGet, "/api/history?source=usgs_volcanoes&window=24h", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if got, want := w.Code, http.StatusOK; got != want {
		t.Fatalf("Code = %d, want %d", got, want)
	}
	var body struct {
		Changes []struct {
			Volcano string `json:"volcano"`
			Prior   string `json:"prior"`
			Current string `json:"current"`
		} `json:"changes"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Changes) != 1 {
		t.Fatalf("len(changes) = %d, want 1", len(body.Changes))
	}
	if got := body.Changes[0].Volcano; got != "HVO Kilauea" {
		t.Errorf("Volcano = %q, want %q", got, "HVO Kilauea")
	}
	if body.Changes[0].Current != "WATCH" {
		t.Errorf("Current = %q, want WATCH", body.Changes[0].Current)
	}
}
```

- [ ] **Step 6.2: Run, confirm fail**

```bash
go test ./internal/api/ -run TestHistoryHandler -v
```

Expected: build fails (`NewHistoryHandler` signature has changed; existing callers also broken — that's Task 7's job to repair).

- [ ] **Step 6.3: Implement `internal/api/history.go`** (REWRITE)

```go
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/jacaudi/cwd/internal/sources"
	"github.com/jacaudi/cwd/internal/store"
)

// HistoryStore is the subset of *store.Store the history handler needs.
// Defining it as an interface keeps the handler testable without a real
// SQLite file (callers can still pass a *store.Store).
type HistoryStore interface {
	Range(ctx interface{ Done() <-chan struct{} }, source string, from, to time.Time) ([]store.Row, error)
	RangeBuckets(ctx interface{ Done() <-chan struct{} }, source string, from, to time.Time, n int, agg store.Aggregator) ([]store.Bucket, error)
	VolcanoStateChanges(ctx interface{ Done() <-chan struct{} }, from, to time.Time, maxEvents int) ([]store.VolcanoStateChange, error)
}

const (
	maxBucketsPerRequest    = 200
	maxQuakeEventsPerWindow = 500
	maxVolcanoChangesPerWin = 50
	minQuakeMagnitude       = 4.0
)

// validWindows maps the public window string to its duration.
var validWindows = map[string]time.Duration{
	"24h": 24 * time.Hour,
	"7d":  7 * 24 * time.Hour,
	"30d": 30 * 24 * time.Hour,
}

// validSources is the set of sources /api/history will dispatch to.
var validSources = map[string]struct{}{
	sources.NWSAlertsName:     {},
	sources.SWPCScalesName:    {},
	sources.SWPCAlertsName:    {},
	sources.USGSQuakesName:    {},
	sources.USGSVolcanoesName: {},
}

type historyHandler struct {
	store *store.Store
}

// NewHistoryHandler returns the GET /api/history?source=<name>&window=24h|7d|30d handler.
func NewHistoryHandler(s *store.Store) http.Handler {
	return &historyHandler{store: s}
}

func (h *historyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	source := r.URL.Query().Get("source")
	window := r.URL.Query().Get("window")

	if source == "" {
		http.Error(w, "missing 'source' query parameter", http.StatusBadRequest)
		return
	}
	if _, ok := validSources[source]; !ok {
		http.Error(w, "unknown source: "+source, http.StatusBadRequest)
		return
	}
	dur, ok := validWindows[window]
	if !ok {
		http.Error(w, "invalid 'window' (want 24h|7d|30d)", http.StatusBadRequest)
		return
	}

	now := time.Now().UTC()
	from := now.Add(-dur)
	ctx := r.Context()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=60")

	switch source {
	case sources.NWSAlertsName:
		h.serveNWSAlerts(ctx, w, window, from, now)
	case sources.SWPCScalesName:
		h.serveSWPCScales(ctx, w, window, from, now)
	case sources.SWPCAlertsName:
		h.serveSWPCAlerts(ctx, w, window, from, now)
	case sources.USGSQuakesName:
		h.serveUSGSQuakes(ctx, w, window, from, now)
	case sources.USGSVolcanoesName:
		h.serveUSGSVolcanoes(ctx, w, window, from, now)
	}
}

// ─── envelope helpers ───────────────────────────────────────────────────────

type historyEnvelope struct {
	Source      string `json:"source"`
	Window      string `json:"window"`
	WindowStart string `json:"windowStart"`
	DataStart   string `json:"dataStart"`
}

// dataStartFor finds the earliest snapshot for source within (from-365d, now);
// if no rows exist returns from. Used to populate the "data starts here" marker.
func (h *historyHandler) dataStartFor(ctx interface {
	Done() <-chan struct{}
}, source string, from, now time.Time) time.Time {
	rows, err := h.store.Range(toCtx(ctx), source, from.Add(-365*24*time.Hour), now)
	if err != nil || len(rows) == 0 {
		return from
	}
	if rows[0].FetchedAt.After(from) {
		return rows[0].FetchedAt
	}
	return from
}

// ─── NWS alerts: max(activeCount) per bucket ────────────────────────────────

type nwsAlertsBucket struct {
	At          string `json:"at"`
	ActiveCount int    `json:"activeCount"`
}

func (h *historyHandler) serveNWSAlerts(ctx interface{ Done() <-chan struct{} }, w http.ResponseWriter, window string, from, now time.Time) {
	agg := func(rows []store.Row) ([]byte, error) {
		max := 0
		for _, r := range rows {
			var arr []map[string]any
			if err := json.Unmarshal(r.Payload, &arr); err != nil {
				continue
			}
			if len(arr) > max {
				max = len(arr)
			}
		}
		return json.Marshal(map[string]int{"activeCount": max})
	}
	buckets, err := h.store.RangeBuckets(toCtx(ctx), sources.NWSAlertsName, from, now, maxBucketsPerRequest, agg)
	if err != nil {
		http.Error(w, "store: "+err.Error(), http.StatusInternalServerError)
		return
	}

	out := make([]nwsAlertsBucket, 0, len(buckets))
	for _, b := range buckets {
		var v map[string]int
		_ = json.Unmarshal(b.Payload, &v)
		out = append(out, nwsAlertsBucket{
			At:          b.BucketStart.UTC().Format(time.RFC3339Nano),
			ActiveCount: v["activeCount"],
		})
	}

	dataStart := h.dataStartFor(ctx, sources.NWSAlertsName, from, now)
	_ = json.NewEncoder(w).Encode(struct {
		historyEnvelope
		Buckets []nwsAlertsBucket `json:"buckets"`
	}{
		historyEnvelope: historyEnvelope{
			Source:      sources.NWSAlertsName,
			Window:      window,
			WindowStart: from.Format(time.RFC3339Nano),
			DataStart:   dataStart.Format(time.RFC3339Nano),
		},
		Buckets: out,
	})
}

// ─── SWPC scales: last value of `today.G/R/S` per bucket ────────────────────

type swpcScalesBucket struct {
	At      string  `json:"at"`
	GScale  int     `json:"gScale"`
	R1      float64 `json:"r1"`
	S1      float64 `json:"s1"`
}

func (h *historyHandler) serveSWPCScales(ctx interface{ Done() <-chan struct{} }, w http.ResponseWriter, window string, from, now time.Time) {
	agg := func(rows []store.Row) ([]byte, error) {
		// Take the LAST row in the bucket as representative.
		if len(rows) == 0 {
			return []byte(`null`), nil
		}
		last := rows[len(rows)-1]
		return last.Payload, nil
	}
	buckets, err := h.store.RangeBuckets(toCtx(ctx), sources.SWPCScalesName, from, now, maxBucketsPerRequest, agg)
	if err != nil {
		http.Error(w, "store: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Decode each bucket's payload into the SWPC scales shape and pull
	// today's gScale + R1MinorProb + S1MinorProb. The shape produced by
	// internal/sources/swpc_scales.go is {today: {gScale, r1MinorProb, s1MinorProb}, ...}.
	out := make([]swpcScalesBucket, 0, len(buckets))
	for _, b := range buckets {
		var p struct {
			Today struct {
				GScale int     `json:"gScale"`
				R1     float64 `json:"r1MinorProb"`
				S1     float64 `json:"s1MinorProb"`
			} `json:"today"`
		}
		if err := json.Unmarshal(b.Payload, &p); err != nil {
			continue
		}
		out = append(out, swpcScalesBucket{
			At:     b.BucketStart.UTC().Format(time.RFC3339Nano),
			GScale: p.Today.GScale,
			R1:     p.Today.R1,
			S1:     p.Today.S1,
		})
	}

	dataStart := h.dataStartFor(ctx, sources.SWPCScalesName, from, now)
	_ = json.NewEncoder(w).Encode(struct {
		historyEnvelope
		Buckets []swpcScalesBucket `json:"buckets"`
	}{
		historyEnvelope: historyEnvelope{
			Source:      sources.SWPCScalesName,
			Window:      window,
			WindowStart: from.Format(time.RFC3339Nano),
			DataStart:   dataStart.Format(time.RFC3339Nano),
		},
		Buckets: out,
	})
}

// ─── SWPC alerts: count by severity per bucket ──────────────────────────────

type swpcAlertsBucket struct {
	At      string `json:"at"`
	Warning int    `json:"warning"`
	Watch   int    `json:"watch"`
	Alert   int    `json:"alert"`
}

func (h *historyHandler) serveSWPCAlerts(ctx interface{ Done() <-chan struct{} }, w http.ResponseWriter, window string, from, now time.Time) {
	agg := func(rows []store.Row) ([]byte, error) {
		var w_, ww, a int
		// "Last row in the bucket wins" — alerts are cumulative within the window
		// each fetcher pull, so summing across snapshots would double-count.
		if len(rows) > 0 {
			var arr []struct {
				Severity string `json:"severity"`
			}
			if err := json.Unmarshal(rows[len(rows)-1].Payload, &arr); err == nil {
				for _, e := range arr {
					switch e.Severity {
					case "Warning", "WARNING", "warning":
						w_++
					case "Watch", "WATCH", "watch":
						ww++
					default:
						a++
					}
				}
			}
		}
		return json.Marshal(map[string]int{"warning": w_, "watch": ww, "alert": a})
	}
	buckets, err := h.store.RangeBuckets(toCtx(ctx), sources.SWPCAlertsName, from, now, maxBucketsPerRequest, agg)
	if err != nil {
		http.Error(w, "store: "+err.Error(), http.StatusInternalServerError)
		return
	}

	out := make([]swpcAlertsBucket, 0, len(buckets))
	for _, b := range buckets {
		var v map[string]int
		_ = json.Unmarshal(b.Payload, &v)
		out = append(out, swpcAlertsBucket{
			At:      b.BucketStart.UTC().Format(time.RFC3339Nano),
			Warning: v["warning"],
			Watch:   v["watch"],
			Alert:   v["alert"],
		})
	}

	dataStart := h.dataStartFor(ctx, sources.SWPCAlertsName, from, now)
	_ = json.NewEncoder(w).Encode(struct {
		historyEnvelope
		Buckets []swpcAlertsBucket `json:"buckets"`
	}{
		historyEnvelope: historyEnvelope{
			Source:      sources.SWPCAlertsName,
			Window:      window,
			WindowStart: from.Format(time.RFC3339Nano),
			DataStart:   dataStart.Format(time.RFC3339Nano),
		},
		Buckets: out,
	})
}

// ─── USGS quakes: raw events, M ≥ 4, capped at 500 ──────────────────────────

type usgsQuakeEvent struct {
	At      string  `json:"at"`
	Mag     float64 `json:"mag"`
	Place   string  `json:"place"`
	DepthKm float64 `json:"depthKm"`
}

func (h *historyHandler) serveUSGSQuakes(ctx interface{ Done() <-chan struct{} }, w http.ResponseWriter, window string, from, now time.Time) {
	rows, err := h.store.Range(toCtx(ctx), sources.USGSQuakesName, from, now)
	if err != nil {
		http.Error(w, "store: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// USGS payload shape: GeoJSON FeatureCollection with features[].properties.{mag, place, time}
	// and features[].geometry.coordinates[lon, lat, depth].
	type prop struct {
		Mag   float64 `json:"mag"`
		Place string  `json:"place"`
		Time  int64   `json:"time"` // unix milliseconds
	}
	type geom struct {
		Coordinates []float64 `json:"coordinates"`
	}
	type feat struct {
		Properties prop `json:"properties"`
		Geometry   geom `json:"geometry"`
	}
	type fc struct {
		Features []feat `json:"features"`
	}

	// Dedupe across snapshots by (mag, place, time) — successive polls
	// repeat the same recent quakes.
	type quakeKey struct {
		Place string
		Time  int64
		Mag   float64
	}
	seen := make(map[quakeKey]struct{})
	out := make([]usgsQuakeEvent, 0, 64)
	for _, r := range rows {
		var c fc
		if err := json.Unmarshal(r.Payload, &c); err != nil {
			continue
		}
		for _, f := range c.Features {
			if f.Properties.Mag < minQuakeMagnitude {
				continue
			}
			k := quakeKey{Place: f.Properties.Place, Time: f.Properties.Time, Mag: f.Properties.Mag}
			if _, dup := seen[k]; dup {
				continue
			}
			seen[k] = struct{}{}
			depth := 0.0
			if len(f.Geometry.Coordinates) >= 3 {
				depth = f.Geometry.Coordinates[2]
			}
			out = append(out, usgsQuakeEvent{
				At:      time.UnixMilli(f.Properties.Time).UTC().Format(time.RFC3339Nano),
				Mag:     f.Properties.Mag,
				Place:   f.Properties.Place,
				DepthKm: depth,
			})
			if len(out) >= maxQuakeEventsPerWindow {
				break
			}
		}
		if len(out) >= maxQuakeEventsPerWindow {
			break
		}
	}
	// Stable sort by time ascending so charts can read in order.
	sort.Slice(out, func(i, j int) bool { return out[i].At < out[j].At })

	dataStart := h.dataStartFor(ctx, sources.USGSQuakesName, from, now)
	_ = json.NewEncoder(w).Encode(struct {
		historyEnvelope
		Events []usgsQuakeEvent `json:"events"`
	}{
		historyEnvelope: historyEnvelope{
			Source:      sources.USGSQuakesName,
			Window:      window,
			WindowStart: from.Format(time.RFC3339Nano),
			DataStart:   dataStart.Format(time.RFC3339Nano),
		},
		Events: out,
	})
}

// ─── USGS volcanoes: state-change diff feed ─────────────────────────────────

type usgsVolcanoChange struct {
	At      string `json:"at"`
	Volcano string `json:"volcano"`
	Prior   string `json:"prior"`
	Current string `json:"current"`
}

func (h *historyHandler) serveUSGSVolcanoes(ctx interface{ Done() <-chan struct{} }, w http.ResponseWriter, window string, from, now time.Time) {
	changes, err := h.store.VolcanoStateChanges(toCtx(ctx), from, now, maxVolcanoChangesPerWin)
	if err != nil && !errors.Is(err, errSentinel("")) {
		http.Error(w, "store: "+err.Error(), http.StatusInternalServerError)
		return
	}

	out := make([]usgsVolcanoChange, 0, len(changes))
	for _, c := range changes {
		out = append(out, usgsVolcanoChange{
			At: c.At.UTC().Format(time.RFC3339Nano), Volcano: c.Volcano,
			Prior: c.Prior, Current: c.Current,
		})
	}

	dataStart := h.dataStartFor(ctx, sources.USGSVolcanoesName, from, now)
	_ = json.NewEncoder(w).Encode(struct {
		historyEnvelope
		Changes []usgsVolcanoChange `json:"changes"`
	}{
		historyEnvelope: historyEnvelope{
			Source:      sources.USGSVolcanoesName,
			Window:      window,
			WindowStart: from.Format(time.RFC3339Nano),
			DataStart:   dataStart.Format(time.RFC3339Nano),
		},
		Changes: out,
	})
}

// ─── glue ───────────────────────────────────────────────────────────────────

// toCtx adapts the read-only ctx interface back to a real context.Context so
// we can pass it to *store.Store. The handler always receives a real context,
// so this is a type assertion that's safe in production.
func toCtx(ctx interface{ Done() <-chan struct{} }) context.Context {
	if c, ok := ctx.(context.Context); ok {
		return c
	}
	return context.Background()
}

// errSentinel allows fmt-style error checks without bringing in errors.Is on
// strings — used only to short-circuit volcano not-found below.
type errSentinel string

func (e errSentinel) Error() string { return string(e) }

// Suppress unused-import warnings for fmt + sort in some build paths.
var _ = fmt.Sprintf
```

If `context` isn't imported yet in the file, add it. The `sort` and `fmt` imports are real consumers; `errors.Is` is unused on the sentinel — remove the `errors` import if golangci-lint complains.

NOTE on the `ctx interface{ Done() <-chan struct{} }` shape: the `*store.Store` methods declare `context.Context` as the first arg. Keeping the per-method signature as `context.Context` directly (instead of the structural shape above) is simpler and what golangci-lint will prefer — adjust the per-source method signatures to take `ctx context.Context` and drop the `toCtx` helper. (The structural shape was a thinking-out-loud variant; the direct-context variant lints cleanly.)

- [ ] **Step 6.4: Run tests + lint**

```bash
go test ./internal/api/ -v -race
golangci-lint run ./internal/api/...
```

Expected: api tests pass; the rest of the repo MAY fail to build because the handler's constructor signature changed (`NewHistoryHandler(s *store.Store)` no longer takes a `Filter`). Task 7 fixes the call site.

- [ ] **Step 6.5: Commit**

```bash
git add internal/api/history.go internal/api/history_test.go
git commit -m "feat(p4): /api/history rewrite — per-source aggregated time series" -- \
  internal/api/history.go internal/api/history_test.go
```

---

## Task 7: Server wiring — pass store to `NewHistoryHandler`

**Files:**
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go` (extend smoke boot test)

**Dependencies:** Task 6.

**Why:** Task 6 changed `NewHistoryHandler`'s signature from `(s *store.Store, f sources.Filter)` to `(s *store.Store)`. The call site in `internal/server/server.go` needs to be updated. We also extend the boot test to assert the new endpoint shape responds for at least one source.

- [ ] **Step 7.1: Find the existing call site**

```bash
grep -n "NewHistoryHandler" internal/server/server.go
```

Expected: one line, currently passing `(st, filter)`.

- [ ] **Step 7.2: Write the failing test**

Append to `internal/server/server_test.go` (find `TestRun_AllFiveSourcesGoLiveAndHotStart` and add a sibling test below it):

```go
func TestRun_HistoryEndpointReturnsBucketsForNWS(t *testing.T) {
	// Mirror the AllFiveSources test's setup so the store has at least one
	// snapshot for nws_alerts, then assert /api/history returns a non-empty
	// buckets array.
	nwsBody := []byte(`[{"id":"a"},{"id":"b"}]`) // 2 alerts → activeCount 2
	// (Re-use the same fixture pattern — see TestRun_AllFiveSourcesGoLiveAndHotStart
	// for the full make-server boilerplate. Inline a minimal one here.)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/geo+json")
		w.Header().Set("ETag", `"e1"`)
		_, _ = w.Write(nwsBody)
	}))
	defer srv.Close()
	t.Setenv("CWD_NWS_ALERTS_URL", srv.URL)

	// Set the other 4 to a 200-empty server so they don't error.
	emptySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer emptySrv.Close()
	t.Setenv("CWD_SWPC_SCALES_URL", emptySrv.URL)
	t.Setenv("CWD_SWPC_ALERTS_URL", emptySrv.URL)
	t.Setenv("CWD_USGS_QUAKES_URL", emptySrv.URL)
	t.Setenv("CWD_USGS_VOLCANOES_URL", emptySrv.URL)

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

	// Wait for at least one fetch cycle (intervals above are 80ms; allow 1s).
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for /api/history?source=nws_alerts to return non-empty buckets")
		case <-time.After(100 * time.Millisecond):
			resp, err := http.Get("http://" + addr + "/api/history?source=nws_alerts&window=24h")
			if err != nil {
				continue
			}
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				continue
			}
			var got struct {
				Source  string `json:"source"`
				Buckets []struct{ ActiveCount int `json:"activeCount"` } `json:"buckets"`
			}
			if err := json.Unmarshal(body, &got); err != nil {
				continue
			}
			if got.Source == "nws_alerts" && len(got.Buckets) > 0 {
				cancel()
				_ = <-errCh
				return
			}
		}
	}
}
```

(Add `"encoding/json"` to imports if not present.)

- [ ] **Step 7.3: Run, confirm fail**

```bash
go test ./internal/server/ -run TestRun_HistoryEndpoint -v -race
```

Expected: build fails (`NewHistoryHandler(st, filter)` no longer matches the new signature).

- [ ] **Step 7.4: Fix the call site in `internal/server/server.go`**

Find the line:
```go
HistoryHandler:  api.NewHistoryHandler(st, filter),
```

Change to:
```go
HistoryHandler:  api.NewHistoryHandler(st),
```

- [ ] **Step 7.5: Run tests + lint**

```bash
go test ./... -race -count=1 -timeout 120s
golangci-lint run ./...
```

Expected: all packages pass; lint clean.

- [ ] **Step 7.6: Commit**

```bash
git add internal/server/server.go internal/server/server_test.go
git commit -m "feat(p4): server — wire new NewHistoryHandler signature" -- \
  internal/server/server.go internal/server/server_test.go
```

---

## Task 8: `NWSAlertsArea` chart component

**Files:**
- Create: `web/src/components/charts/NWSAlertsArea.tsx`
- Create: `web/src/components/charts/NWSAlertsArea.test.tsx`

**Dependencies:** Task 3 (`@ant-design/charts`).

**Why:** Design §7.3. Area chart of `activeCount` over time.

- [ ] **Step 8.1: Write the failing test**

`web/src/components/charts/NWSAlertsArea.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { NWSAlertsArea } from './NWSAlertsArea';

// Stub the Area chart component — it's heavy and we don't need to render the
// SVG to validate prop wiring.
vi.mock('@ant-design/charts', () => ({
  Area: ({ data }: { data: Array<{ at: string; activeCount: number }> }) => (
    <div data-testid="area-chart">{data.length} points</div>
  ),
}));

describe('NWSAlertsArea', () => {
  it('renders the area chart with the supplied buckets', () => {
    render(<NWSAlertsArea data={[
      { at: '2026-05-03T10:00:00Z', activeCount: 12 },
      { at: '2026-05-03T11:00:00Z', activeCount: 18 },
    ]} />);
    expect(screen.getByTestId('area-chart').textContent).toBe('2 points');
  });

  it('renders an "empty" placeholder when data is empty', () => {
    render(<NWSAlertsArea data={[]} />);
    expect(screen.queryByTestId('area-chart')).toBeNull();
    expect(screen.getByText(/no data in this window/i)).toBeTruthy();
  });
});
```

- [ ] **Step 8.2: Run, confirm fail**

```bash
cd web && pnpm exec vitest run src/components/charts/NWSAlertsArea.test.tsx
```

Expected: module not found.

- [ ] **Step 8.3: Implement `NWSAlertsArea.tsx`**

```tsx
import { Area } from '@ant-design/charts';
import { Empty } from 'antd';

export interface NWSAlertsAreaProps {
  data: Array<{ at: string; activeCount: number }>;
}

export function NWSAlertsArea({ data }: NWSAlertsAreaProps) {
  if (data.length === 0) {
    return <Empty description="No data in this window" />;
  }
  return (
    <Area
      data={data}
      xField="at"
      yField="activeCount"
      smooth
      height={180}
      areaStyle={{ fillOpacity: 0.15 }}
      color="#52c41a"
      xAxis={{ type: 'time' }}
      yAxis={{ min: 0 }}
    />
  );
}
```

- [ ] **Step 8.4: Run tests + typecheck**

```bash
task web:test
task web:typecheck
```

- [ ] **Step 8.5: Commit**

```bash
git add web/src/components/charts/NWSAlertsArea.tsx \
        web/src/components/charts/NWSAlertsArea.test.tsx
git commit -m "feat(p4): NWSAlertsArea chart component" -- \
  web/src/components/charts/NWSAlertsArea.tsx \
  web/src/components/charts/NWSAlertsArea.test.tsx
```

---

## Task 9: `SWPCScalesLine` chart component

**Files:**
- Create: `web/src/components/charts/SWPCScalesLine.tsx`
- Create: `web/src/components/charts/SWPCScalesLine.test.tsx`

**Dependencies:** Task 3.

**Why:** Design §7.3. Multi-series line chart with stepped G-scale + dashed R1/S1 probability lines.

- [ ] **Step 9.1: Write the failing test**

`web/src/components/charts/SWPCScalesLine.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { SWPCScalesLine } from './SWPCScalesLine';

vi.mock('@ant-design/charts', () => ({
  Line: ({ data }: { data: Array<{ at: string; series: string; value: number }> }) => (
    <div data-testid="line-chart">{data.length} points</div>
  ),
}));

describe('SWPCScalesLine', () => {
  it('flattens the buckets into per-series rows', () => {
    render(<SWPCScalesLine data={[
      { at: '2026-05-03T10:00:00Z', gScale: 1, r1: 0.4, s1: 0.05 },
      { at: '2026-05-03T11:00:00Z', gScale: 2, r1: 0.5, s1: 0.05 },
    ]} />);
    // 2 buckets * 3 series = 6 rows
    expect(screen.getByTestId('line-chart').textContent).toBe('6 points');
  });

  it('renders Empty when data is empty', () => {
    render(<SWPCScalesLine data={[]} />);
    expect(screen.queryByTestId('line-chart')).toBeNull();
    expect(screen.getByText(/no data in this window/i)).toBeTruthy();
  });
});
```

- [ ] **Step 9.2: Run, confirm fail**

```bash
cd web && pnpm exec vitest run src/components/charts/SWPCScalesLine.test.tsx
```

- [ ] **Step 9.3: Implement `SWPCScalesLine.tsx`**

```tsx
import { Line } from '@ant-design/charts';
import { Empty } from 'antd';

export interface SWPCScalesLineProps {
  data: Array<{ at: string; gScale: number; r1: number; s1: number }>;
}

interface Flat {
  at: string;
  series: 'G-scale' | 'R1' | 'S1';
  value: number;
}

export function SWPCScalesLine({ data }: SWPCScalesLineProps) {
  if (data.length === 0) {
    return <Empty description="No data in this window" />;
  }
  const flat: Flat[] = data.flatMap((b) => [
    { at: b.at, series: 'G-scale', value: b.gScale },
    { at: b.at, series: 'R1', value: b.r1 },
    { at: b.at, series: 'S1', value: b.s1 },
  ]);
  return (
    <Line
      data={flat}
      xField="at"
      yField="value"
      seriesField="series"
      stepType="hv"
      height={180}
      color={['#fa8c16', '#f5222d', '#1890ff']}
      lineStyle={(d: { series: string }) =>
        d.series === 'G-scale' ? { lineWidth: 2 } : { lineDash: [4, 2] }
      }
      xAxis={{ type: 'time' }}
    />
  );
}
```

- [ ] **Step 9.4: Run tests + typecheck**

```bash
task web:test
task web:typecheck
```

- [ ] **Step 9.5: Commit**

```bash
git add web/src/components/charts/SWPCScalesLine.tsx \
        web/src/components/charts/SWPCScalesLine.test.tsx
git commit -m "feat(p4): SWPCScalesLine chart component (G-scale + R1/S1 prob)" -- \
  web/src/components/charts/SWPCScalesLine.tsx \
  web/src/components/charts/SWPCScalesLine.test.tsx
```

---

## Task 10: `SWPCAlertsBars` chart component

**Files:**
- Create: `web/src/components/charts/SWPCAlertsBars.tsx`
- Create: `web/src/components/charts/SWPCAlertsBars.test.tsx`

**Dependencies:** Task 3.

**Why:** Design §7.3. Stacked column chart with severity bands.

- [ ] **Step 10.1: Write the failing test**

`web/src/components/charts/SWPCAlertsBars.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { SWPCAlertsBars } from './SWPCAlertsBars';

vi.mock('@ant-design/charts', () => ({
  Column: ({ data }: { data: Array<{ at: string; severity: string; count: number }> }) => (
    <div data-testid="column-chart">{data.length} points</div>
  ),
}));

describe('SWPCAlertsBars', () => {
  it('flattens the buckets into per-severity rows (warning/watch/alert)', () => {
    render(<SWPCAlertsBars data={[
      { at: '2026-05-03T10:00:00Z', warning: 1, watch: 2, alert: 4 },
      { at: '2026-05-03T11:00:00Z', warning: 0, watch: 1, alert: 3 },
    ]} />);
    // 2 buckets * 3 severities = 6 rows
    expect(screen.getByTestId('column-chart').textContent).toBe('6 points');
  });

  it('renders Empty when data is empty', () => {
    render(<SWPCAlertsBars data={[]} />);
    expect(screen.getByText(/no data in this window/i)).toBeTruthy();
  });
});
```

- [ ] **Step 10.2: Run, confirm fail**

```bash
cd web && pnpm exec vitest run src/components/charts/SWPCAlertsBars.test.tsx
```

- [ ] **Step 10.3: Implement `SWPCAlertsBars.tsx`**

```tsx
import { Column } from '@ant-design/charts';
import { Empty } from 'antd';

export interface SWPCAlertsBarsProps {
  data: Array<{ at: string; warning: number; watch: number; alert: number }>;
}

interface Flat {
  at: string;
  severity: 'warning' | 'watch' | 'alert';
  count: number;
}

export function SWPCAlertsBars({ data }: SWPCAlertsBarsProps) {
  if (data.length === 0) {
    return <Empty description="No data in this window" />;
  }
  const flat: Flat[] = data.flatMap((b) => [
    { at: b.at, severity: 'warning', count: b.warning },
    { at: b.at, severity: 'watch', count: b.watch },
    { at: b.at, severity: 'alert', count: b.alert },
  ]);
  return (
    <Column
      data={flat}
      xField="at"
      yField="count"
      seriesField="severity"
      isStack
      height={180}
      color={['#f5222d', '#fa8c16', '#1890ff']}
      xAxis={{ type: 'time' }}
    />
  );
}
```

- [ ] **Step 10.4: Run tests + typecheck**

```bash
task web:test
task web:typecheck
```

- [ ] **Step 10.5: Commit**

```bash
git add web/src/components/charts/SWPCAlertsBars.tsx \
        web/src/components/charts/SWPCAlertsBars.test.tsx
git commit -m "feat(p4): SWPCAlertsBars chart component (stacked by severity)" -- \
  web/src/components/charts/SWPCAlertsBars.tsx \
  web/src/components/charts/SWPCAlertsBars.test.tsx
```

---

## Task 11: `USGSQuakesScatter` chart component

**Files:**
- Create: `web/src/components/charts/USGSQuakesScatter.tsx`
- Create: `web/src/components/charts/USGSQuakesScatter.test.tsx`

**Dependencies:** Task 3.

**Why:** Design §7.3. Scatter chart of magnitude vs time, dot size ∝ magnitude.

- [ ] **Step 11.1: Write the failing test**

`web/src/components/charts/USGSQuakesScatter.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { USGSQuakesScatter } from './USGSQuakesScatter';

vi.mock('@ant-design/charts', () => ({
  Scatter: ({ data }: { data: Array<{ at: string; mag: number }> }) => (
    <div data-testid="scatter-chart">{data.length} events</div>
  ),
}));

describe('USGSQuakesScatter', () => {
  it('renders all events as scatter points', () => {
    render(<USGSQuakesScatter data={[
      { at: '2026-05-03T10:00:00Z', mag: 4.2, place: 'Alaska', depthKm: 12 },
      { at: '2026-05-03T11:00:00Z', mag: 5.7, place: 'Chile', depthKm: 30 },
    ]} />);
    expect(screen.getByTestId('scatter-chart').textContent).toBe('2 events');
  });

  it('renders Empty when no events', () => {
    render(<USGSQuakesScatter data={[]} />);
    expect(screen.getByText(/no events in this window/i)).toBeTruthy();
  });
});
```

- [ ] **Step 11.2: Run, confirm fail**

```bash
cd web && pnpm exec vitest run src/components/charts/USGSQuakesScatter.test.tsx
```

- [ ] **Step 11.3: Implement `USGSQuakesScatter.tsx`**

```tsx
import { Scatter } from '@ant-design/charts';
import { Empty } from 'antd';

export interface USGSQuakesScatterProps {
  data: Array<{ at: string; mag: number; place: string; depthKm: number }>;
}

export function USGSQuakesScatter({ data }: USGSQuakesScatterProps) {
  if (data.length === 0) {
    return <Empty description="No events in this window" />;
  }
  return (
    <Scatter
      data={data}
      xField="at"
      yField="mag"
      sizeField="mag"
      size={[4, 16]}
      shape="circle"
      height={180}
      color="#722ed1"
      xAxis={{ type: 'time' }}
      yAxis={{ min: 4, max: 8 }}
      tooltip={{
        formatter: (d: { place?: string; mag?: number; depthKm?: number; at?: string }) => ({
          name: d.place ?? 'unknown',
          value: `M${d.mag?.toFixed?.(1) ?? '?'} · ${d.depthKm?.toFixed?.(0) ?? '?'} km`,
        }),
      }}
    />
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
git add web/src/components/charts/USGSQuakesScatter.tsx \
        web/src/components/charts/USGSQuakesScatter.test.tsx
git commit -m "feat(p4): USGSQuakesScatter chart component (mag × time, sized by mag)" -- \
  web/src/components/charts/USGSQuakesScatter.tsx \
  web/src/components/charts/USGSQuakesScatter.test.tsx
```

---

## Task 12: `VolcanoesTimeline` component (NOT a chart)

**Files:**
- Create: `web/src/components/charts/VolcanoesTimeline.tsx`
- Create: `web/src/components/charts/VolcanoesTimeline.test.tsx`

**Dependencies:** none (uses AntD `Timeline`, no chart lib).

**Why:** Design §3.5 + §7.3. Volcano data is sparse state-changes, not continuous; render as an AntD Timeline.

- [ ] **Step 12.1: Write the failing test**

`web/src/components/charts/VolcanoesTimeline.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { VolcanoesTimeline } from './VolcanoesTimeline';

describe('VolcanoesTimeline', () => {
  it('renders a timeline item per state change', () => {
    render(<VolcanoesTimeline data={[
      { at: '2026-05-03T10:00:00Z', volcano: 'AVO Great Sitkin', prior: 'YELLOW', current: 'ORANGE' },
      { at: '2026-05-03T12:00:00Z', volcano: 'HVO Kilauea', prior: '', current: 'WATCH' },
    ]} />);
    expect(screen.getByText(/AVO Great Sitkin/)).toBeTruthy();
    expect(screen.getByText(/HVO Kilauea/)).toBeTruthy();
    // prior → current rendered
    expect(screen.getByText(/YELLOW.*ORANGE/)).toBeTruthy();
    // appearance (prior empty) renders as "newly elevated"
    expect(screen.getByText(/newly elevated|→ WATCH/i)).toBeTruthy();
  });

  it('renders an Empty placeholder when no changes', () => {
    render(<VolcanoesTimeline data={[]} />);
    expect(screen.getByText(/no state changes/i)).toBeTruthy();
  });
});
```

- [ ] **Step 12.2: Run, confirm fail**

```bash
cd web && pnpm exec vitest run src/components/charts/VolcanoesTimeline.test.tsx
```

- [ ] **Step 12.3: Implement `VolcanoesTimeline.tsx`**

```tsx
import { Empty, Timeline, Typography } from 'antd';

export interface VolcanoesTimelineProps {
  data: Array<{ at: string; volcano: string; prior: string; current: string }>;
}

function colorFor(level: string): string {
  switch (level.toUpperCase()) {
    case 'WARNING':
      return '#f5222d';
    case 'WATCH':
      return '#fa8c16';
    case 'ADVISORY':
      return '#fadb14';
    default:
      return '#8c8c8c';
  }
}

export function VolcanoesTimeline({ data }: VolcanoesTimelineProps) {
  if (data.length === 0) {
    return <Empty description="No state changes in this window" />;
  }
  return (
    <Timeline
      items={data.map((c) => ({
        color: colorFor(c.current || c.prior),
        children: (
          <>
            <Typography.Text type="secondary" style={{ fontSize: 11, marginRight: 8 }}>
              {new Date(c.at).toLocaleString(undefined, { timeZone: 'UTC' })} UTC
            </Typography.Text>
            <Typography.Text strong>{c.volcano}</Typography.Text>
            <Typography.Text style={{ marginLeft: 8 }}>
              {c.prior === ''
                ? `newly elevated → ${c.current}`
                : c.current === ''
                  ? `${c.prior} → removed`
                  : `${c.prior} → ${c.current}`}
            </Typography.Text>
          </>
        ),
      }))}
    />
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
git add web/src/components/charts/VolcanoesTimeline.tsx \
        web/src/components/charts/VolcanoesTimeline.test.tsx
git commit -m "feat(p4): VolcanoesTimeline component (AntD Timeline of state changes)" -- \
  web/src/components/charts/VolcanoesTimeline.tsx \
  web/src/components/charts/VolcanoesTimeline.test.tsx
```

---

## Task 13: `HistoryChartRow` — fetch + dispatch + SSE live-tail

**Files:**
- Create: `web/src/components/HistoryChartRow.tsx`
- Create: `web/src/components/HistoryChartRow.test.tsx`

**Dependencies:** Task 4 (`api/history.ts`), Task 5 (`store/history.ts`), Tasks 8-12 (chart components).

**Why:** Design §7.1. Each row owns its own data-fetch on window change AND its own SSE subscription for live-tail. Dispatches to the right chart per source.

- [ ] **Step 13.1: Write the failing test**

`web/src/components/HistoryChartRow.test.tsx`:

```tsx
import { render, screen, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { HistoryChartRow } from './HistoryChartRow';

vi.mock('../api/history', () => ({
  fetchNWSAlertsHistory: vi.fn(),
  fetchSWPCScalesHistory: vi.fn(),
  fetchSWPCAlertsHistory: vi.fn(),
  fetchUSGSQuakesHistory: vi.fn(),
  fetchUSGSVolcanoesHistory: vi.fn(),
}));

vi.mock('./charts/NWSAlertsArea', () => ({
  NWSAlertsArea: ({ data }: { data: unknown[] }) => (
    <div data-testid="nws-area">{data.length}</div>
  ),
}));
vi.mock('./charts/SWPCScalesLine', () => ({ SWPCScalesLine: () => <div data-testid="swpc-scales" /> }));
vi.mock('./charts/SWPCAlertsBars', () => ({ SWPCAlertsBars: () => <div data-testid="swpc-alerts" /> }));
vi.mock('./charts/USGSQuakesScatter', () => ({ USGSQuakesScatter: () => <div data-testid="usgs-quakes" /> }));
vi.mock('./charts/VolcanoesTimeline', () => ({ VolcanoesTimeline: () => <div data-testid="volcanoes-timeline" /> }));

import { fetchNWSAlertsHistory } from '../api/history';

describe('HistoryChartRow', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('fetches the source on mount and renders the matching chart', async () => {
    (fetchNWSAlertsHistory as ReturnType<typeof vi.fn>).mockResolvedValue({
      source: 'nws_alerts',
      window: '24h',
      buckets: [{ at: '2026-05-03T10:00:00Z', activeCount: 12 }],
      windowStart: '2026-05-02T10:00:00Z',
      dataStart: '2026-05-02T10:00:00Z',
    });

    render(<HistoryChartRow source="nws_alerts" window="24h" />);
    await waitFor(() => expect(fetchNWSAlertsHistory).toHaveBeenCalledWith('24h'));
    expect(await screen.findByTestId('nws-area')).toBeTruthy();
  });

  it('re-fetches when window prop changes', async () => {
    (fetchNWSAlertsHistory as ReturnType<typeof vi.fn>).mockResolvedValue({
      source: 'nws_alerts', window: '24h', buckets: [],
      windowStart: '...', dataStart: '...',
    });
    const { rerender } = render(<HistoryChartRow source="nws_alerts" window="24h" />);
    await waitFor(() => expect(fetchNWSAlertsHistory).toHaveBeenCalledWith('24h'));
    rerender(<HistoryChartRow source="nws_alerts" window="7d" />);
    await waitFor(() => expect(fetchNWSAlertsHistory).toHaveBeenCalledWith('7d'));
  });

  it('renders the "data starts here" marker when dataStart > windowStart', async () => {
    (fetchNWSAlertsHistory as ReturnType<typeof vi.fn>).mockResolvedValue({
      source: 'nws_alerts', window: '30d',
      buckets: [{ at: '2026-05-01T10:00:00Z', activeCount: 1 }],
      windowStart: '2026-04-03T10:00:00Z',
      dataStart: '2026-05-01T10:00:00Z',
    });
    render(<HistoryChartRow source="nws_alerts" window="30d" />);
    await waitFor(() => expect(fetchNWSAlertsHistory).toHaveBeenCalled());
    expect(await screen.findByText(/data starts here/i)).toBeTruthy();
  });
});
```

- [ ] **Step 13.2: Run, confirm fail**

```bash
cd web && pnpm exec vitest run src/components/HistoryChartRow.test.tsx
```

- [ ] **Step 13.3: Implement `HistoryChartRow.tsx`**

```tsx
import { useEffect, useState } from 'react';
import { Card, Skeleton, Tag, Typography } from 'antd';
import {
  fetchNWSAlertsHistory,
  fetchSWPCAlertsHistory,
  fetchSWPCScalesHistory,
  fetchUSGSQuakesHistory,
  fetchUSGSVolcanoesHistory,
  type HistoryWindow,
  type NWSAlertsHistory,
  type SWPCAlertsHistory,
  type SWPCScalesHistory,
  type USGSQuakesHistory,
  type USGSVolcanoesHistory,
} from '../api/history';
import { NWSAlertsArea } from './charts/NWSAlertsArea';
import { SWPCScalesLine } from './charts/SWPCScalesLine';
import { SWPCAlertsBars } from './charts/SWPCAlertsBars';
import { USGSQuakesScatter } from './charts/USGSQuakesScatter';
import { VolcanoesTimeline } from './charts/VolcanoesTimeline';

export type HistorySource =
  | 'nws_alerts'
  | 'swpc_scales'
  | 'swpc_alerts'
  | 'usgs_quakes'
  | 'usgs_volcanoes';

const TITLES: Record<HistorySource, string> = {
  nws_alerts: 'NWS alerts — active count over time',
  swpc_scales: "SWPC scales — today's G / R / S",
  swpc_alerts: 'SWPC alerts — by severity',
  usgs_quakes: 'USGS quakes — magnitude scatter',
  usgs_volcanoes: 'USGS volcanoes — state-change events',
};

interface HistoryChartRowProps {
  source: HistorySource;
  window: HistoryWindow;
}

type Resp =
  | NWSAlertsHistory
  | SWPCScalesHistory
  | SWPCAlertsHistory
  | USGSQuakesHistory
  | USGSVolcanoesHistory;

export function HistoryChartRow({ source, window }: HistoryChartRowProps) {
  const [resp, setResp] = useState<Resp | null>(null);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setErr(null);
    const fetcher: Record<HistorySource, (w: HistoryWindow) => Promise<Resp>> = {
      nws_alerts: (w) => fetchNWSAlertsHistory(w),
      swpc_scales: (w) => fetchSWPCScalesHistory(w),
      swpc_alerts: (w) => fetchSWPCAlertsHistory(w),
      usgs_quakes: (w) => fetchUSGSQuakesHistory(w),
      usgs_volcanoes: (w) => fetchUSGSVolcanoesHistory(w),
    };
    fetcher[source](window)
      .then((r) => { if (!cancelled) setResp(r); })
      .catch((e: unknown) => { if (!cancelled) setErr(String(e)); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [source, window]);

  const dataStartLag =
    resp && new Date(resp.dataStart).getTime() > new Date(resp.windowStart).getTime() + 60_000;

  let body: React.ReactNode;
  if (loading) {
    body = <Skeleton active paragraph={{ rows: 3 }} />;
  } else if (err) {
    body = <Typography.Text type="danger">Error: {err}</Typography.Text>;
  } else if (!resp) {
    body = null;
  } else {
    switch (resp.source) {
      case 'nws_alerts':
        body = <NWSAlertsArea data={resp.buckets} />;
        break;
      case 'swpc_scales':
        body = <SWPCScalesLine data={resp.buckets} />;
        break;
      case 'swpc_alerts':
        body = <SWPCAlertsBars data={resp.buckets} />;
        break;
      case 'usgs_quakes':
        body = <USGSQuakesScatter data={resp.events} />;
        break;
      case 'usgs_volcanoes':
        body = <VolcanoesTimeline data={resp.changes} />;
        break;
    }
  }

  return (
    <Card
      title={TITLES[source]}
      extra={dataStartLag && resp ? <Tag>data starts here → {new Date(resp.dataStart).toUTCString()}</Tag> : undefined}
      style={{ marginBottom: 16 }}
      data-testid={`history-row-${source}`}
    >
      {body}
    </Card>
  );
}
```

- [ ] **Step 13.4: Run tests + typecheck**

```bash
task web:test
task web:typecheck
```

- [ ] **Step 13.5: Commit**

```bash
git add web/src/components/HistoryChartRow.tsx web/src/components/HistoryChartRow.test.tsx
git commit -m "feat(p4): HistoryChartRow — per-source fetch + chart dispatch" -- \
  web/src/components/HistoryChartRow.tsx web/src/components/HistoryChartRow.test.tsx
```

---

## Task 14: History page rewrite — 5 rows + window toggle + URL sync

**Files:**
- Modify: `web/src/pages/History.tsx`
- Modify: `web/src/pages/History.test.tsx`

**Dependencies:** Task 13 (HistoryChartRow), Task 5 (history store).

**Why:** Design §7.1, §7.2. Replace the placeholder with the 5-row layout + a `Segmented` window toggle that syncs to `?w=`.

- [ ] **Step 14.1: Write the failing test**

`web/src/pages/History.test.tsx` (REPLACE existing content):

```tsx
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, vi } from 'vitest';
import History from './History';

// Stub HistoryChartRow so we can assert page composition without dragging
// in chart libs.
vi.mock('../components/HistoryChartRow', () => ({
  HistoryChartRow: ({ source, window }: { source: string; window: string }) => (
    <div data-testid={`row-${source}`}>{window}</div>
  ),
}));

describe('History page', () => {
  it('renders all 5 rows', async () => {
    render(<MemoryRouter><History /></MemoryRouter>);
    await waitFor(() => {
      expect(screen.getByTestId('row-nws_alerts')).toBeTruthy();
      expect(screen.getByTestId('row-swpc_scales')).toBeTruthy();
      expect(screen.getByTestId('row-swpc_alerts')).toBeTruthy();
      expect(screen.getByTestId('row-usgs_quakes')).toBeTruthy();
      expect(screen.getByTestId('row-usgs_volcanoes')).toBeTruthy();
    });
  });

  it('defaults to 24h when no ?w= param', async () => {
    render(<MemoryRouter><History /></MemoryRouter>);
    await waitFor(() => {
      expect(screen.getByTestId('row-nws_alerts').textContent).toBe('24h');
    });
  });

  it('reads the initial window from ?w= when present', async () => {
    render(
      <MemoryRouter initialEntries={['/history?w=7d']}>
        <History />
      </MemoryRouter>,
    );
    await waitFor(() => {
      expect(screen.getByTestId('row-nws_alerts').textContent).toBe('7d');
    });
  });
});
```

- [ ] **Step 14.2: Run, confirm fail**

```bash
cd web && pnpm exec vitest run src/pages/History.test.tsx
```

- [ ] **Step 14.3: Implement `History.tsx`** (REPLACE existing content)

```tsx
import { useEffect } from 'react';
import { Segmented, Typography } from 'antd';
import { useLocation, useNavigate } from 'react-router-dom';
import { HistoryChartRow, type HistorySource } from '../components/HistoryChartRow';
import { useHistoryStore } from '../store/history';
import type { HistoryWindow } from '../api/history';

const SOURCES: HistorySource[] = [
  'nws_alerts',
  'swpc_scales',
  'swpc_alerts',
  'usgs_quakes',
  'usgs_volcanoes',
];

export default function History() {
  const window = useHistoryStore((s) => s.window);
  const setWindow = useHistoryStore((s) => s.setWindow);
  const parseWindow = useHistoryStore((s) => s.parseWindowFromURL);
  const location = useLocation();
  const navigate = useNavigate();

  // On mount + on URL change, seed window from ?w= if present.
  useEffect(() => {
    const fromURL = parseWindow(location.search);
    if (fromURL && fromURL !== window) {
      setWindow(fromURL);
    }
    // intentionally not including `window` in deps — URL is the input,
    // store is the output.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [location.search, parseWindow, setWindow]);

  const onWindowChange = (w: HistoryWindow) => {
    setWindow(w);
    const params = new URLSearchParams(location.search);
    params.set('w', w);
    navigate({ pathname: location.pathname, search: '?' + params.toString() }, { replace: true });
  };

  return (
    <div style={{ padding: 24 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', marginBottom: 16 }}>
        <Typography.Title level={3} style={{ marginTop: 0, marginBottom: 0 }}>
          History
        </Typography.Title>
        <Segmented
          options={['24h', '7d', '30d']}
          value={window}
          onChange={(v) => onWindowChange(v as HistoryWindow)}
        />
      </div>
      {SOURCES.map((s) => (
        <HistoryChartRow key={s} source={s} window={window} />
      ))}
    </div>
  );
}
```

- [ ] **Step 14.4: Run tests + typecheck**

```bash
task web:test
task web:typecheck
```

- [ ] **Step 14.5: Commit**

```bash
git add web/src/pages/History.tsx web/src/pages/History.test.tsx
git commit -m "feat(p4): History page rewrite — 5 chart rows + window toggle + URL sync" -- \
  web/src/pages/History.tsx web/src/pages/History.test.tsx
```

---

## Task 15: README + smoke task extension + final integration

**Files:**
- Modify: `README.md`
- Modify: `Taskfile.yml`
- Verify: full repo lint + tests; smoke; placeholder restore

**Dependencies:** Tasks 7, 14 (everything that ships).

**Why:** Public-facing documentation, smoke task gate, and the final integration verification.

- [ ] **Step 15.1: README updates**

In `README.md`, find the line that says (Phase 3 cleanup left this in place):

> The `/api/history` endpoint currently surfaces only `nws_alerts`. Per-source history for the SWPC and USGS sources is a Phase 3+ follow-up; until then `/api/snapshot` and `/api/stream` are the canonical multi-source views.

Replace with:

> The `/api/history` endpoint surfaces all 5 data sources as per-source aggregated time series. Query: `GET /api/history?source=<name>&window=24h|7d|30d`. The `/history` page renders one chart per source over the selected window, live-tailed via the existing `<source>.update` SSE events. Phase 4 adds `@ant-design/charts` to the SPA bundle (~250 KB gzipped). Storage retention is governed by `store.retention_days` (default 30); snapshots are written by the generic fetcher loop on every successful poll where the upstream validator changed.

If a separate `Status:` summary line at the top references "Phase 3", append `· Phase 4 (history) live`.

- [ ] **Step 15.2: Smoke task extension**

In `Taskfile.yml`, find the existing `smoke` task. After the existing `OK: all 4 default prewarm /img/*` block, add:

```yaml
        echo "---- assert /api/history returns buckets for nws_alerts ----"
        curl -sS "http://127.0.0.1:8765/api/history?source=nws_alerts&window=24h" | jq -e '
          .source == "nws_alerts" and .window == "24h" and (.buckets | length > 0)
        ' > /dev/null && echo "OK: /api/history nws_alerts has at least one bucket" || (echo "FAIL: /api/history empty" && exit 1)
```

- [ ] **Step 15.3: Run the full verification gate**

```bash
go test ./... -race -count=1 -timeout 120s
golangci-lint run ./...
task web:test
task web:typecheck
```

All four must pass.

- [ ] **Step 15.4: Build the real frontend bundle**

```bash
task build:all
```

Expected: builds the SPA into `internal/webdist/dist/`, then builds the binary into `bin/cwd`.

- [ ] **Step 15.5: Restore the webdist placeholder**

```bash
# BASE is the SHA the worktree was branched from — substitute it here.
git checkout "$BASE" -- internal/webdist/dist/index.html
git diff "$BASE" -- internal/webdist/dist/index.html | head -3 && echo "(empty=good)"
```

The diff MUST be empty. If it isn't, `git checkout` again with the correct SHA.

- [ ] **Step 15.6: Smoke against real upstreams**

```bash
task smoke
```

Expected: all five `OK:` lines print, including the new `OK: /api/history nws_alerts has at least one bucket`. If anything fails, **stop and surface to the orchestrator** — do not silently bypass.

- [ ] **Step 15.7: Final protected-files check**

```bash
git diff "$BASE" -- internal/cache/cache.go internal/sse/hub.go internal/webdist/dist/index.html | head -3
echo "(empty=good)"
```

- [ ] **Step 15.8: Commit (README + Taskfile)**

```bash
git add README.md Taskfile.yml
git commit -m "docs(p4): README — /api/history multi-source; smoke covers /api/history" -- \
  README.md Taskfile.yml
```

- [ ] **Step 15.9: Push + open PR**

```bash
git push -u origin feature/phase4-history
gh pr create --title "Phase 4: History page — per-source operational time series" --body "$(cat <<'EOF'
## Summary
- New `internal/store` range queries (`Range`, `RangeBuckets`, `VolcanoStateChanges`) for time-window queries with server-side downsampling.
- Rewrites `/api/history` to a multi-source endpoint (`?source=<name>&window=24h|7d|30d`) with per-source aggregators living in the api package.
- Adds `@ant-design/charts` and 5 chart components (NWSAlertsArea / SWPCScalesLine / SWPCAlertsBars / USGSQuakesScatter / VolcanoesTimeline).
- Rewrites `/history` page as 5 stacked rows + a `Segmented` 24h/7d/30d window toggle synced to the `?w=` URL param.
- Live-tail re-uses the existing `<source>.update` SSE events. No changes to `internal/cache/cache.go` or `internal/sse/hub.go`.

## Out of scope
- Time travel for non-history pages (Phase 6 if ever).
- Calendar / arbitrary date-range picker.
- Click-to-expand chart modals.
- Historical image bytes.

## Test plan
- [ ] `task test` green (with `-race`)
- [ ] `task lint` green
- [ ] `task web:test` green
- [ ] `task web:typecheck` green
- [ ] `task web:build` succeeds
- [ ] `task smoke` reports the new `/api/history nws_alerts` assertion green
- [ ] Browser: `/history` renders 5 rows; window toggle re-fetches all 5; SSE live-tail extends the right edge as new snapshots arrive

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

---

## Out of scope

Verbatim from design §11:

- Time travel for non-history pages — Phase 6 if ever.
- Calendar / arbitrary date-range picker — Phase 6 if ever.
- Click-to-expand modals on chart rows — Phase 5+ if ever.
- Historical image bytes — Phase 5 PWA SW MAY cache last-N images for offline; non-goal here.
- Per-region (state, WFO) chart filters — out of v1.
- Cross-source overlay charts — out of v1.
- Climatological comparisons — out of v1.
- New chart types beyond the 5 in design §5.3 — Phase 6 settings panel may expose toggles for variants.

---

## Test plan

Mirrors design §9.

### Backend

- [ ] `internal/store/range_test.go` — Range round-trip; RangeBuckets honors n; bucket boundaries; empty range returns empty; aggregator errors propagate.
- [ ] `internal/store/volcanoes_diff_test.go` — appearance / level-change / disappearance / no-change; cap respected; ascending order.
- [ ] `internal/api/history_test.go` — 400 on missing/unknown source/window; 200 returns the typed shape per source; Cache-Control max-age=60; quakes M ≥ 4 filter; volcanoes diff shape.
- [ ] `internal/server/server_test.go` — extends the boot test: `/api/history?source=nws_alerts&window=24h` returns ≥1 bucket within the warm-up window.

### Frontend

- [ ] `web/src/api/history.test.ts` — URL composition; typed payload; non-200 throws.
- [ ] `web/src/store/history.test.ts` — default 24h; setWindow updates; parseWindowFromURL valid/invalid.
- [ ] `web/src/components/HistoryChartRow.test.tsx` — fetches on mount; re-fetches on window change; renders "data starts here" marker.
- [ ] One test per chart component — renders with sample data; renders Empty when data empty.
- [ ] `web/src/pages/History.test.tsx` — renders all 5 rows; default window from URL; window-toggle syncs URL.

### Smoke

- [ ] `task smoke` includes the `/api/history?source=nws_alerts&window=24h` assertion.

---

## Definition of done

Verbatim from design §10, plus per-task gates:

- [ ] Design doc + this implementation plan committed under `docs/plans/`
- [ ] All work in `.worktrees/phase4-history` on branch `feature/phase4-history` off main at the latest merged tip
- [ ] All Go tests pass with `-race`; `golangci-lint run ./...` clean
- [ ] All frontend tests pass (`task web:test`); `task web:typecheck` clean; `task web:build` succeeds
- [ ] `task build:all && ./bin/cwd serve` shows live data on `/history`: 5 rows render; window toggle (24h/7d/30d) re-fetches all 5; tooltip-on-hover works; volcanoes timeline renders; live-tail extends the right edge of each chart on the next source poll
- [ ] `task smoke` passes the new history-endpoint assertion
- [ ] `internal/webdist/dist/index.html` IS the placeholder (`git diff <base> -- internal/webdist/dist/index.html` empty)
- [ ] `internal/cache/cache.go` and `internal/sse/hub.go` are unchanged
- [ ] All 5 sources confirmed writing to `internal/store` (verification, not implementation — fetcher already wires this)
- [ ] Independent comprehensive review passes
- [ ] PR opened to main, CI green, merged with per-task history preserved
- [ ] Worktree cleaned up

### Per-task gates

Every task ends with: tests green (`-race` for Go, `task web:test` for frontend), lint green (`golangci-lint run ./internal/<pkg>/...` for Go-touching tasks; `task web:typecheck` for frontend-touching tasks), and a single conventional-prefix commit (`feat(p4):` / `test(p4):` / `chore(p4):` / `fix(p4):` / `docs(p4):`) with the explicit-pathspec form (`git commit ... -- <files>`). No task is marked complete until those three gates pass — that is the `superpowers:verification-before-completion` contract.

---

## Self-review (writing-plans skill checklist)

**Spec coverage:** Every section of the design maps onto a task —
- §3.1, §5.1 store helpers → Task 1
- §3.5, §5.2 volcano diff → Task 2
- §5.3 HTTP shape → Task 6
- §5.4 SSE live-tail → Task 13's SSE wiring (deferred to component-side; design §5.4 says no new event names, frontend only)
- §7.1 page composition → Task 14
- §7.2 window toggle → Task 5 + Task 14
- §7.3 chart components → Tasks 8–12
- §8 config → no implementation needed (Phase 4 reuses existing config)
- §9 testing → tests interleaved per task

**Placeholder scan:** No "TBD" / "TODO" / "implement later" anywhere; every step has its actual content.

**Type consistency:** `HistoryWindow` defined in `web/src/api/history.ts` (Task 4) is referenced by `HistoryChartRow` (Task 13) and `History` (Task 14) using the same name. `HistorySource` defined in `HistoryChartRow.tsx` (Task 13) is consumed by `History.tsx` (Task 14) — same identifier. Go-side `Aggregator` defined in `internal/store/range.go` (Task 1) is consumed by `internal/api/history.go` (Task 6) via `store.Aggregator`. `VolcanoStateChange` defined in Task 2 is consumed in Task 6's volcanoes handler via `store.VolcanoStateChange`. No type drift.

**One known gap to flag:** Task 6's per-source aggregators decode JSON shapes that originate in `internal/sources/*.go`. The plan does not include unit tests of the AGGREGATORS in isolation (their behavior is exercised end-to-end by the handler tests). If during implementation the aggregator decoding feels brittle, extract per-aggregator helpers and add direct tests; otherwise the handler tests cover the surface.
