# Phase 4 — History page polish + per-event-type counters — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close out the visual + behavioral gaps in the Phase 4 v1 History page (dark-mode chart theme, headline-stat per row, SSE live-tail re-fetch, X-axis time formatting, page-width stretch, footer dev-tag gating, build-version tag) and add a per-event-type counter strip (Tornado / Severe Thunderstorm / Flash Flood) above the NWS area chart.

**Architecture:** Extend the existing `/api/history` NWS aggregator to emit per-event-type bucket counts; add one new frontend component (`NWSEventCountsStrip`); patch the four existing chart components for dark theme + time formatting; rewire `HistoryChartRow` to compose the new strip, expose a headline-stat tag, and re-fetch on SSE `<source>.update` events; replace the obsolete "Phase 0 — skeleton" header tag with the build version; gate footer `image:*` health tags behind `import.meta.env.DEV`.

**Tech Stack:** Same as Phase 4 v1 — Go 1.24 backend, React 18 + TS 5 + AntD 5 + `@ant-design/charts` 2.6.7 + Zustand frontend, Vitest + Testing Library + jsdom, Taskfile.

**Source documents:**
- Approved design: [`docs/plans/2026-05-05-phase4-history-polish-design.md`](2026-05-05-phase4-history-polish-design.md)
- Phase 4 v1 implementation (structural template): [`docs/plans/2026-05-03-phase4-history-implementation.md`](2026-05-03-phase4-history-implementation.md)
- Phase 4 design (load-bearing for shapes): [`docs/plans/2026-05-03-phase4-history-design.md`](2026-05-03-phase4-history-design.md)

**Branch:** continues on `feature/phase4-history`. **BASE for diffs:** `665e40711738d00c593b16d014fbb29a665b624b`. The 16 commits already on the branch (the docs(p4) plan commit + 14 feat/chore commits + the WIP `docs(p4): README ...` from Task 15.1–15.2 that has not yet been committed) are the baseline. This implementation plan layers on top.

> **For Claude:** REQUIRED EXECUTION WORKFLOW (follow in order):
> 1. `superpowers:using-git-worktrees` — Already in `.worktrees/phase4-history`; reuse it.
> 2. `superpowers:subagent-driven-development` — Dispatch a fresh subagent per task.
> 3. `superpowers:test-driven-development` — All subagents use TDD.
> 4. `superpowers:verification-before-completion` — Verify all tests pass per task.
> 5. `superpowers:requesting-code-review` — Per-task review (built-in).
> 6. After all tasks: independent comprehensive code review on the diff vs `665e40711738d00c593b16d014fbb29a665b624b`.
> 7. `superpowers:finishing-a-development-branch` — Push + open PR (or extend the existing draft PR).
>
> Skills carry their own model and effort settings. Do not override them.

---

## Conventions

- All commits prefixed `feat(p4):`, `test(p4):`, `chore(p4):`, `fix(p4):`, `docs(p4):` (this is a continuation of Phase 4; we keep the same prefix family rather than introducing `(p4-polish)`).
- All Go tests use stdlib `testing` + `httptest`.
- Logging: stdlib `log/slog`. Structured key/value fields.
- No CGO. SQLite is `modernc.org/sqlite`.
- Frontend tests live next to the component (`Component.test.tsx`).
- `golangci-lint` v2 must stay clean (`task lint`).
- Never `git add -A` / `git add .`. Never skip hooks (`--no-verify`, `--no-gpg-sign`).

### Commit author identity

Already configured at the repo level (`jacaudi <47005674+jacaudi@users.noreply.github.com>`). Do NOT run `git config`.

### Stage + commit by explicit pathspec

```bash
git add path/to/file_a.go path/to/file_b.go
git commit -m "feat(p4): explanatory message" -- path/to/file_a.go path/to/file_b.go
```

The trailing `-- <pathspec>` form scopes the commit even if a sibling subagent stages something between your `git add` and `git commit`.

### Worktree

Already created in Phase 4 v1 — `.worktrees/phase4-history`, branch `feature/phase4-history`. All commands assume CWD is the worktree.

### Trap — webdist placeholder file (do NOT stage)

`internal/webdist/dist/index.html` is a committed placeholder. `task web:build` overwrites it with a real Vite build. Only Task 9 (final integration) may run `task web:build`. Earlier tasks verify with `task web:test` and `task web:typecheck` only. After running `task web:build`, restore via:

```bash
git checkout 665e40711738d00c593b16d014fbb29a665b624b -- internal/webdist/dist/index.html
```

### Trap — cache + SSE (do NOT touch)

`internal/cache/cache.go` and `internal/sse/hub.go` MUST be byte-identical to BASE. Run before opening the PR:

```bash
git diff 665e40711738d00c593b16d014fbb29a665b624b -- internal/cache/cache.go internal/sse/hub.go
```

Output must be empty. Phase 4 polish reads from the snapshot store (which receives SSE updates via the existing pipeline) — no SSE/cache infrastructure changes.

### Trap — sources package shape (do NOT touch)

`internal/sources/*.go` is unchanged for this round. The new event-type counting logic lives in `internal/api/history.go` (where the existing aggregators are), keyed off the upstream NWS payload's `event` string field — no source-side schema change.

### Trap — protected files contract (full list)

Before each commit AND before the PR, verify these four paths are byte-identical to BASE:
- `internal/cache/cache.go`
- `internal/sse/hub.go`
- `internal/webdist/dist/index.html`
- `internal/sources/*.go` (every file)

```bash
git diff 665e40711738d00c593b16d014fbb29a665b624b -- \
  internal/cache/cache.go internal/sse/hub.go internal/webdist/dist/index.html \
  'internal/sources/*.go' | head -3
```

Empty = good.

---

## Dependency graph

```
1 server: extend NWS aggregator (eventCounts) ────┐
2 client: extend NWSAlertsHistory type ───────────┤
3 client: useIsDark hook ─────────────────────────┤
4 client: drop "Phase 0 — skeleton" + version tag ┤  (5 tasks parallel — different file regions)
5 client: gate image:* footer tags ────────────────┘
        │
        ▼
        ├──► 6 chart components: dark theme + time format ◀── 3
        │
        ├──► 7 NWSEventCountsStrip component ◀── 2, 3
        │
        ▼
        8 HistoryChartRow rewire (headline + SSE re-fetch + strip + max-width) ◀── 2, 6, 7
        │
        ▼
        9 verify + Playwright tooltip smoke + final commit/PR ◀── all
```

**Parallelism windows:**
- **Wave 1** (5 parallel): Tasks 1, 2, 3, 4, 5 — five different file regions (server / client api types / client hook / App.tsx / SourceHealthIndicator).
- **Wave 2** (2 parallel): Task 6 (chart components polish, 4 files in `web/src/components/charts/{NWS,SWPC*,USGS}*.tsx`) + Task 7 (new file `web/src/components/charts/NWSEventCountsStrip.tsx`).
- **Wave 3** (1): Task 8 — HistoryChartRow integration. Touches `web/src/components/HistoryChartRow.tsx{,.test.tsx}`.
- **Wave 4** (1): Task 9 — verify, Playwright tooltip smoke, restore webdist placeholder, commit, PR.

No two parallel-runnable tasks touch the same file. Per-task pathspec discipline provides belt-and-suspenders.

---

## Task 1: Server — extend NWS aggregator with `eventCounts`

**Files:**
- Modify: `internal/api/history.go` (around the `serveNWSAlerts` function + add a package-level `nwsEventCounters` map)
- Modify: `internal/api/history_test.go` (add new test)

**Dependencies:** none.

**Why:** Design Task D, server side. The history endpoint must emit per-bucket counts for Tornado / Severe Thunderstorm / Flash Flood alongside the existing `activeCount`. The frontend strip (Task 7) reads these directly.

- [ ] **Step 1.1: Write the failing test**

Append to `internal/api/history_test.go` (after the existing `TestHistoryHandler_NWSAlerts_BucketsHaveActiveCount`):

```go
func TestHistoryHandler_NWSAlerts_PerEventTypeCounts(t *testing.T) {
	s := newTestStoreForAPI(t)
	ctx := context.Background()
	t0 := time.Now().UTC().Add(-time.Hour)

	// Snapshot mixing event types — 1 tornado, 2 severe-tstorm, 1 flash flood, 1 unrelated.
	type alert struct {
		ID    string `json:"id"`
		Event string `json:"event"`
	}
	mk := func(events ...string) []byte {
		alerts := make([]alert, 0, len(events))
		for i, e := range events {
			alerts = append(alerts, alert{ID: "a" + string(rune('a'+i)), Event: e})
		}
		b, _ := json.Marshal(alerts)
		return b
	}
	_ = s.Append(ctx, "nws_alerts", t0, "v1", mk(
		"Tornado Warning",
		"Severe Thunderstorm Warning",
		"Severe Thunderstorm Watch",
		"Flash Flood Warning",
		"Winter Storm Watch",
	))

	h := NewHistoryHandler(s)
	req := httptest.NewRequest(http.MethodGet, "/api/history?source=nws_alerts&window=24h", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if got, want := w.Code, http.StatusOK; got != want {
		t.Fatalf("Code = %d, want %d (body=%s)", got, want, w.Body.String())
	}

	var body struct {
		Buckets []struct {
			ActiveCount int `json:"activeCount"`
			EventCounts struct {
				Tornado      int `json:"tornado"`
				SevereTstorm int `json:"severeTstorm"`
				FlashFlood   int `json:"flashFlood"`
			} `json:"eventCounts"`
		} `json:"buckets"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Buckets) == 0 {
		t.Fatal("no buckets")
	}
	// Find the bucket with the activity (max(activeCount) is 5).
	var seen *struct {
		ActiveCount int `json:"activeCount"`
		EventCounts struct {
			Tornado      int `json:"tornado"`
			SevereTstorm int `json:"severeTstorm"`
			FlashFlood   int `json:"flashFlood"`
		} `json:"eventCounts"`
	}
	for i := range body.Buckets {
		if body.Buckets[i].ActiveCount == 5 {
			seen = &body.Buckets[i]
			break
		}
	}
	if seen == nil {
		t.Fatalf("no bucket with activeCount=5; buckets=%+v", body.Buckets)
	}
	if seen.EventCounts.Tornado != 1 {
		t.Errorf("tornado = %d, want 1", seen.EventCounts.Tornado)
	}
	if seen.EventCounts.SevereTstorm != 2 {
		t.Errorf("severeTstorm = %d, want 2", seen.EventCounts.SevereTstorm)
	}
	if seen.EventCounts.FlashFlood != 1 {
		t.Errorf("flashFlood = %d, want 1", seen.EventCounts.FlashFlood)
	}
}
```

- [ ] **Step 1.2: Run, confirm fail**

```bash
go test ./internal/api/ -run TestHistoryHandler_NWSAlerts_PerEventTypeCounts -v
```

Expected: FAIL — `eventCounts` field absent / counts all zero.

- [ ] **Step 1.3: Modify the NWS aggregator in `internal/api/history.go`**

Add at file scope (e.g. just below the existing `validSources` map):

```go
// nwsEventCounters defines which NWS upstream "event" strings count toward
// each per-event-type category surfaced on the History page. Names are matched
// case-sensitive against the upstream NWS feed's `event` field. Unmatched
// events are simply not counted (they still count toward activeCount).
var nwsEventCounters = map[string][]string{
	"tornado":      {"Tornado Warning", "Tornado Watch", "Tornado Emergency"},
	"severeTstorm": {"Severe Thunderstorm Warning", "Severe Thunderstorm Watch"},
	"flashFlood":   {"Flash Flood Warning", "Flash Flood Watch", "Flash Flood Emergency"},
}
```

Replace the existing `nwsAlertsBucket` struct with:

```go
type nwsAlertsBucket struct {
	At          string `json:"at"`
	ActiveCount int    `json:"activeCount"`
	EventCounts struct {
		Tornado      int `json:"tornado"`
		SevereTstorm int `json:"severeTstorm"`
		FlashFlood   int `json:"flashFlood"`
	} `json:"eventCounts"`
}
```

Replace the existing `serveNWSAlerts` body with:

```go
func (h *historyHandler) serveNWSAlerts(ctx context.Context, w http.ResponseWriter, window string, from, now time.Time) {
	agg := func(rows []store.Row) ([]byte, error) {
		maxCount := 0
		maxByCategory := map[string]int{"tornado": 0, "severeTstorm": 0, "flashFlood": 0}
		for _, r := range rows {
			var arr []map[string]any
			if err := json.Unmarshal(r.Payload, &arr); err != nil {
				continue
			}
			if len(arr) > maxCount {
				maxCount = len(arr)
			}
			rowByCat := map[string]int{"tornado": 0, "severeTstorm": 0, "flashFlood": 0}
			for _, alert := range arr {
				event, _ := alert["event"].(string)
				for cat, names := range nwsEventCounters {
					for _, name := range names {
						if name == event {
							rowByCat[cat]++
							break
						}
					}
				}
			}
			for cat, count := range rowByCat {
				if count > maxByCategory[cat] {
					maxByCategory[cat] = count
				}
			}
		}
		return json.Marshal(map[string]any{
			"activeCount": maxCount,
			"eventCounts": maxByCategory,
		})
	}
	buckets, err := h.store.RangeBuckets(ctx, sources.NWSAlertsName, from, now, maxBucketsPerRequest, agg)
	if err != nil {
		http.Error(w, "store: "+err.Error(), http.StatusInternalServerError)
		return
	}

	out := make([]nwsAlertsBucket, 0, len(buckets))
	for _, b := range buckets {
		var v struct {
			ActiveCount int            `json:"activeCount"`
			EventCounts map[string]int `json:"eventCounts"`
		}
		_ = json.Unmarshal(b.Payload, &v)
		bucket := nwsAlertsBucket{
			At:          b.BucketStart.UTC().Format(time.RFC3339Nano),
			ActiveCount: v.ActiveCount,
		}
		bucket.EventCounts.Tornado = v.EventCounts["tornado"]
		bucket.EventCounts.SevereTstorm = v.EventCounts["severeTstorm"]
		bucket.EventCounts.FlashFlood = v.EventCounts["flashFlood"]
		out = append(out, bucket)
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
```

- [ ] **Step 1.4: Run tests + lint**

```bash
go test ./internal/api/ -v -race
golangci-lint run ./internal/api/...
```

Expected: all api tests pass (the existing `TestHistoryHandler_NWSAlerts_BucketsHaveActiveCount` still passes because the new struct includes the same `activeCount` JSON field), the new test passes, lint clean.

- [ ] **Step 1.5: Commit**

```bash
git add internal/api/history.go internal/api/history_test.go
git commit -m "feat(p4): NWS aggregator emits per-event-type bucket counts" -- \
  internal/api/history.go internal/api/history_test.go
```

---

## Task 2: Client — extend `NWSAlertsHistory` type with `eventCounts`

**Files:**
- Modify: `web/src/api/history.ts`
- Modify: `web/src/api/history.test.ts`

**Dependencies:** none (TypeScript types are decoupled from the Go server's actual response shape).

**Why:** The frontend needs the new field on the typed envelope so HistoryChartRow + NWSEventCountsStrip can read it.

- [ ] **Step 2.1: Write the failing test**

Append to `web/src/api/history.test.ts` (inside the existing `describe('history client', ...)` block):

```ts
  it('NWSAlertsHistory bucket includes eventCounts shape', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify({
      source: 'nws_alerts',
      window: '24h',
      buckets: [{
        at: '2026-05-05T12:00:00Z',
        activeCount: 42,
        eventCounts: { tornado: 1, severeTstorm: 2, flashFlood: 0 },
      }],
      windowStart: '2026-05-04T12:00:00Z',
      dataStart: '2026-05-04T12:00:00Z',
    }), { status: 200 }));

    const got = await fetchNWSAlertsHistory('24h');
    expect(got.buckets[0].eventCounts.tornado).toBe(1);
    expect(got.buckets[0].eventCounts.severeTstorm).toBe(2);
    expect(got.buckets[0].eventCounts.flashFlood).toBe(0);
  });
```

- [ ] **Step 2.2: Run, confirm fail**

```bash
cd web && pnpm exec vitest run src/api/history.test.ts
```

Expected: typecheck/runtime fail — `eventCounts` not a property on the bucket type.

- [ ] **Step 2.3: Modify `web/src/api/history.ts`**

Replace the `NWSAlertsHistory` interface with:

```ts
export interface NWSAlertsHistory extends HistoryEnvelope<'nws_alerts'> {
  buckets: Array<{
    at: string;
    activeCount: number;
    eventCounts: {
      tornado: number;
      severeTstorm: number;
      flashFlood: number;
    };
  }>;
}
```

- [ ] **Step 2.4: Run tests + typecheck**

```bash
task web:test
task web:typecheck
```

Expected: green / clean. Existing tests that mock NWS responses without `eventCounts` will need updating; if any fail, add the missing field to the fixture in those tests' setup. (Most tests mock at the function-call level via `vi.mock('../api/history', ...)`, not the response body, so this should be a non-issue.)

- [ ] **Step 2.5: Commit**

```bash
git add web/src/api/history.ts web/src/api/history.test.ts
git commit -m "feat(p4): NWSAlertsHistory.buckets[].eventCounts type" -- \
  web/src/api/history.ts web/src/api/history.test.ts
```

---

## Task 3: Client — `useIsDark()` hook

**Files:**
- Create: `web/src/theme.ts` already exists (from the existing codebase). We **add** an exported hook to it. Locate the existing file with `cat web/src/theme.ts` first.
- Modify: `web/src/theme.ts` — append `useIsDark`
- Create: `web/src/theme.test.ts` (or extend the existing one if present)

**Dependencies:** none.

**Why:** Charts (Task 6) and possibly other components need to know whether AntD is currently in dark mode so they can pass `theme="academy"` to G2. The existing `themeMode: 'light' | 'dark' | 'system'` is in App.tsx state but not exposed beyond the `ConfigProvider`. The cleanest fix: a hook that reads the resolved (post-`'system'`-resolution) value via AntD's `theme.useToken()`.

- [ ] **Step 3.1: Inspect existing `web/src/theme.ts`**

```bash
cat web/src/theme.ts
```

Note the existing exports (`buildThemeConfig`, `persistTheme`, `loadStoredTheme` — confirm names). Don't break them.

- [ ] **Step 3.2: Write the failing test**

Create `web/src/theme.test.ts` (or append if it exists):

```ts
import { describe, it, expect } from 'vitest';
import { renderHook } from '@testing-library/react';
import { ConfigProvider, theme as antTheme } from 'antd';
import type { ReactNode } from 'react';
import { useIsDark } from './theme';

function wrapWith(algorithm: typeof antTheme.darkAlgorithm | typeof antTheme.defaultAlgorithm) {
  return ({ children }: { children: ReactNode }) => (
    <ConfigProvider theme={{ algorithm }}>{children}</ConfigProvider>
  );
}

describe('useIsDark', () => {
  it('returns true under darkAlgorithm', () => {
    const { result } = renderHook(() => useIsDark(), {
      wrapper: wrapWith(antTheme.darkAlgorithm),
    });
    expect(result.current).toBe(true);
  });

  it('returns false under defaultAlgorithm', () => {
    const { result } = renderHook(() => useIsDark(), {
      wrapper: wrapWith(antTheme.defaultAlgorithm),
    });
    expect(result.current).toBe(false);
  });
});
```

- [ ] **Step 3.3: Run, confirm fail**

```bash
cd web && pnpm exec vitest run src/theme.test.ts
```

Expected: import fail — `useIsDark` undefined.

- [ ] **Step 3.4: Implement `useIsDark` in `web/src/theme.ts`**

Append to the existing file:

```ts
import { theme as antTheme } from 'antd';

/**
 * useIsDark returns true when the surrounding AntD ConfigProvider is using
 * the dark algorithm. Use this in chart components to pick a matching G2
 * theme — without it, AntD Charts default to light text on a dark card and
 * render axis labels invisibly.
 */
export function useIsDark(): boolean {
  const { token } = antTheme.useToken();
  // colorBgBase is "#ffffff" in light, "#000000" in dark (per AntD 5 tokens).
  // Read the first hex pair as luminance — a value < 128 means dark theme.
  const hex = token.colorBgBase.replace('#', '');
  if (hex.length < 2) return false;
  const r = parseInt(hex.slice(0, 2), 16);
  return r < 128;
}
```

(If the existing `web/src/theme.ts` already imports `theme as antTheme` from antd, reuse that import — don't double-import.)

- [ ] **Step 3.5: Run tests + typecheck**

```bash
task web:test
task web:typecheck
```

Expected: clean / green.

- [ ] **Step 3.6: Commit**

```bash
git add web/src/theme.ts web/src/theme.test.ts
git commit -m "feat(p4): useIsDark hook for chart theme detection" -- \
  web/src/theme.ts web/src/theme.test.ts
```

---

## Task 4: Client — replace "Phase 0 — skeleton" tag with build-version tag

**Files:**
- Modify: `web/src/App.tsx` (line ~127)
- Modify: `web/src/App.test.tsx`

**Dependencies:** none.

**Why:** Design decision 6 (resolved). The header tag is obsolete. Replace with a tag showing the running build version (the SPA already fetches `/api/version` into `serverVersion` state on mount).

- [ ] **Step 4.1: Write the failing test**

In `web/src/App.test.tsx`, locate or add a test that asserts the deprecated text is gone and the version tag is rendered. Append:

```tsx
import { vi, describe, it, expect } from 'vitest';
import { render, waitFor, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import App from './App';

vi.mock('./api/client', () => ({
  api: {
    uiconfig: vi.fn().mockResolvedValue({ defaultTheme: 'dark', defaultLanding: '/', enableHistory: true }),
    version: vi.fn().mockResolvedValue({ version: 'v0.4.1', commit: 'abc123', date: '2026-05-05' }),
  },
}));
vi.mock('./api/stream', () => ({
  connect: vi.fn().mockResolvedValue(() => undefined),
}));

describe('App header tag', () => {
  it('renders the build-version tag once /api/version resolves', async () => {
    render(<MemoryRouter><App initialThemeMode="dark" /></MemoryRouter>);
    await waitFor(() => {
      expect(screen.getByText(/v0\.4\.1/)).toBeTruthy();
    });
  });

  it('does not render the deprecated "Phase 0 — skeleton" tag', () => {
    render(<MemoryRouter><App initialThemeMode="dark" /></MemoryRouter>);
    expect(screen.queryByText(/Phase 0 — skeleton/)).toBeNull();
  });
});
```

(If `App.test.tsx` already exists with different tests, integrate this into it — don't replace.)

- [ ] **Step 4.2: Run, confirm fail**

```bash
cd web && pnpm exec vitest run src/App.test.tsx
```

Expected: at least one test fails — either the version tag isn't rendered yet, or the "Phase 0 — skeleton" string is still present.

- [ ] **Step 4.3: Modify `web/src/App.tsx`**

Replace the `rightContentRender` `<Tag>` element. Find:

```tsx
            <Tag color="default">Phase 0 — skeleton</Tag>
```

Replace with:

```tsx
            {serverVersion?.version ? (
              <Tag color="default">cwd {serverVersion.version}</Tag>
            ) : null}
```

- [ ] **Step 4.4: Run tests + typecheck**

```bash
task web:test
task web:typecheck
```

Expected: green / clean.

- [ ] **Step 4.5: Commit**

```bash
git add web/src/App.tsx web/src/App.test.tsx
git commit -m "feat(p4): replace Phase 0 skeleton tag with build version" -- \
  web/src/App.tsx web/src/App.test.tsx
```

---

## Task 5: Client — gate `image:*` footer health tags behind `import.meta.env.DEV`

**Files:**
- Modify: `web/src/components/SourceHealthIndicator.tsx`
- Modify: `web/src/components/SourceHealthIndicator.test.tsx`

**Dependencies:** none.

**Why:** Design decision 7 (resolved). The footer source-health tags include 20+ `image:*` prewarm entries that are dev-time noise and clutter the production view.

- [ ] **Step 5.1: Read the existing component**

```bash
cat web/src/components/SourceHealthIndicator.tsx
cat web/src/components/SourceHealthIndicator.test.tsx
```

(Header context: it polls `/api/sources` every 15s and renders one `<Tag>` per key. We add a filter that excludes `image:*` keys when `!import.meta.env.DEV`.)

- [ ] **Step 5.2: Write the failing test**

Append to `web/src/components/SourceHealthIndicator.test.tsx`:

```tsx
import { vi, describe, it, expect, beforeEach } from 'vitest';
import { render, waitFor, screen } from '@testing-library/react';
import { SourceHealthIndicator } from './SourceHealthIndicator';

const sample = {
  nws_alerts: { consecutiveFailures: 0, ageSec: 5, intervalSec: 60, lastSuccess: '2026-05-05T12:00:00Z' },
  'image:spc.day1otlk': { consecutiveFailures: 0, ageSec: 5, intervalSec: 60, lastSuccess: '2026-05-05T12:00:00Z' },
};

describe('SourceHealthIndicator dev-vs-prod', () => {
  beforeEach(() => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify(sample), { status: 200 }));
  });

  it('hides image:* tags when import.meta.env.DEV is false', async () => {
    vi.stubEnv('DEV', false as unknown as string);
    render(<SourceHealthIndicator pollMs={1_000_000} />);
    await waitFor(() => expect(screen.getByText('nws_alerts')).toBeTruthy());
    expect(screen.queryByText('image:spc.day1otlk')).toBeNull();
    vi.unstubAllEnvs();
  });

  it('shows image:* tags when import.meta.env.DEV is true', async () => {
    vi.stubEnv('DEV', true as unknown as string);
    render(<SourceHealthIndicator pollMs={1_000_000} />);
    await waitFor(() => expect(screen.getByText('nws_alerts')).toBeTruthy());
    await waitFor(() => expect(screen.getByText('image:spc.day1otlk')).toBeTruthy());
    vi.unstubAllEnvs();
  });
});
```

- [ ] **Step 5.3: Run, confirm fail**

```bash
cd web && pnpm exec vitest run src/components/SourceHealthIndicator.test.tsx
```

Expected: the prod-mode test fails — the `image:*` tag is rendered regardless of env.

- [ ] **Step 5.4: Modify `web/src/components/SourceHealthIndicator.tsx`**

Replace the inner `Object.entries(data).map(...)` with a filtered-then-mapped version:

```tsx
  const isDev = import.meta.env.DEV;
  return (
    <Space size="small">
      {Object.entries(data)
        .filter(([name]) => isDev || !name.startsWith('image:'))
        .map(([name, h]) => {
          const color =
            h.consecutiveFailures > 0
              ? 'red'
              : h.ageSec > 2 * h.intervalSec
                ? 'gold'
                : 'green';
          const tip = `${h.lastSuccess || 'never'} (age: ${h.ageSec}s, failures: ${h.consecutiveFailures})`;
          return (
            <Tooltip key={name} title={tip}>
              <Tag color={color}>{name}</Tag>
            </Tooltip>
          );
        })}
    </Space>
  );
```

- [ ] **Step 5.5: Run tests + typecheck**

```bash
task web:test
task web:typecheck
```

Expected: clean / green.

- [ ] **Step 5.6: Commit**

```bash
git add web/src/components/SourceHealthIndicator.tsx web/src/components/SourceHealthIndicator.test.tsx
git commit -m "feat(p4): hide image:* health tags outside dev builds" -- \
  web/src/components/SourceHealthIndicator.tsx web/src/components/SourceHealthIndicator.test.tsx
```

---

## Task 6: Client — chart components dark theme + X-axis time formatting

**Files:**
- Modify: `web/src/components/charts/NWSAlertsArea.tsx`
- Modify: `web/src/components/charts/SWPCScalesLine.tsx`
- Modify: `web/src/components/charts/SWPCAlertsBars.tsx`
- Modify: `web/src/components/charts/USGSQuakesScatter.tsx`
- Modify: each component's `.test.tsx` (assert theme + label-formatter wiring)

**Dependencies:** Task 3 (`useIsDark`).

**Why:** Combined Tasks A + G from the design — same files. Charts must (a) pass `theme="academy"` when in dark mode so axis/legend text is visible, and (b) format the X-axis time ticks compactly per the active window (24h → `HH:mm`, 7d → `MM-dd HH:mm`, 30d → `MM-dd`).

The `window` prop is passed in from `HistoryChartRow` (Task 8 will wire it). For this task we add the prop to each chart component and update the tests; HistoryChartRow's wiring lands in Task 8.

- [ ] **Step 6.1: Add a shared formatter helper**

Create `web/src/components/charts/timeFormat.ts`:

```ts
import type { HistoryWindow } from '../../api/history';

/** dayjs-style mask per window. G2 v2's axis.x.labelFormatter uses these. */
export function timeMaskForWindow(window: HistoryWindow): string {
  switch (window) {
    case '24h':
      return 'HH:mm';
    case '7d':
      return 'MM-DD HH:mm';
    case '30d':
      return 'MM-DD';
  }
}

/** Returns a labelFormatter callback compatible with @ant-design/charts v2 axis spec. */
export function timeAxisFormatter(window: HistoryWindow): (d: Date | string | number) => string {
  const mask = timeMaskForWindow(window);
  return (d) => {
    const date = d instanceof Date ? d : new Date(d);
    const yyyy = date.getFullYear();
    const mm = String(date.getMonth() + 1).padStart(2, '0');
    const dd = String(date.getDate()).padStart(2, '0');
    const hh = String(date.getHours()).padStart(2, '0');
    const mi = String(date.getMinutes()).padStart(2, '0');
    return mask
      .replace('YYYY', String(yyyy))
      .replace('MM', mm)
      .replace('DD', dd)
      .replace('HH', hh)
      .replace('mm', mi);
  };
}
```

And `web/src/components/charts/timeFormat.test.ts`:

```ts
import { describe, it, expect } from 'vitest';
import { timeMaskForWindow, timeAxisFormatter } from './timeFormat';

describe('timeFormat', () => {
  it('returns HH:mm for 24h', () => {
    expect(timeMaskForWindow('24h')).toBe('HH:mm');
  });
  it('returns MM-DD HH:mm for 7d', () => {
    expect(timeMaskForWindow('7d')).toBe('MM-DD HH:mm');
  });
  it('returns MM-DD for 30d', () => {
    expect(timeMaskForWindow('30d')).toBe('MM-DD');
  });
  it('formatter formats a Date in the local zone per the mask', () => {
    const f = timeAxisFormatter('24h');
    const d = new Date(2026, 4, 5, 14, 7, 25); // local; month 4 = May
    expect(f(d)).toBe('14:07');
  });
});
```

- [ ] **Step 6.2: Run, confirm fail**

```bash
cd web && pnpm exec vitest run src/components/charts/timeFormat.test.ts
```

Expected: import resolution fail (file doesn't exist before commit) — actually since we're writing both files in the same step, run after writing them: should PASS. (TDD here is "write helper test alongside helper" — one commit.)

- [ ] **Step 6.3: Update `NWSAlertsArea.tsx`**

Replace the entire file with:

```tsx
import { Area } from '@ant-design/charts';
import { Empty } from 'antd';
import { useIsDark } from '../../theme';
import type { HistoryWindow } from '../../api/history';
import { timeAxisFormatter } from './timeFormat';

export interface NWSAlertsAreaProps {
  data: Array<{ at: string; activeCount: number }>;
  window: HistoryWindow;
}

export function NWSAlertsArea({ data, window }: NWSAlertsAreaProps) {
  const isDark = useIsDark();
  if (data.length === 0) {
    return <Empty description="No data in this window" />;
  }
  const points = data.map((d) => ({ at: new Date(d.at), activeCount: d.activeCount }));
  return (
    <Area
      data={points}
      xField="at"
      yField="activeCount"
      shapeField="smooth"
      height={180}
      style={{ fillOpacity: 0.15, fill: '#52c41a', stroke: '#52c41a' }}
      scale={{ y: { domainMin: 0 } }}
      axis={{
        x: { title: false, labelFormatter: timeAxisFormatter(window) },
        y: { title: 'active alerts' },
      }}
      point={{ shapeField: 'point', sizeField: 3, style: { fill: '#52c41a' } }}
      theme={isDark ? 'academy' : 'classic'}
    />
  );
}
```

- [ ] **Step 6.4: Update its test (`NWSAlertsArea.test.tsx`)**

Existing test mocks `@ant-design/charts` and asserts data length + empty placeholder. Add a `window` prop everywhere and add an assertion for theme + formatter wiring. Replace the file with:

```tsx
import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { NWSAlertsArea } from './NWSAlertsArea';

const mockArea = vi.fn();
vi.mock('@ant-design/charts', () => ({
  Area: (props: { data: Array<{ at: Date; activeCount: number }>; theme?: string }) => {
    mockArea(props);
    return <div data-testid="area-chart">{props.data.length} points (theme={props.theme ?? 'unset'})</div>;
  },
}));

vi.mock('../../theme', () => ({ useIsDark: () => true }));

describe('NWSAlertsArea', () => {
  it('renders the area chart with the supplied buckets and dark theme', () => {
    render(<NWSAlertsArea
      window="24h"
      data={[
        { at: '2026-05-03T10:00:00Z', activeCount: 12 },
        { at: '2026-05-03T11:00:00Z', activeCount: 18 },
      ]}
    />);
    expect(screen.getByTestId('area-chart').textContent).toContain('2 points');
    expect(screen.getByTestId('area-chart').textContent).toContain('theme=academy');
    // data is converted to Date for the time axis
    const props = mockArea.mock.calls.at(-1)![0];
    expect(props.data[0].at).toBeInstanceOf(Date);
  });

  it('renders an "empty" placeholder when data is empty', () => {
    render(<NWSAlertsArea window="24h" data={[]} />);
    expect(screen.queryByTestId('area-chart')).toBeNull();
    expect(screen.getByText(/no data in this window/i)).toBeTruthy();
  });
});
```

- [ ] **Step 6.5: Update `SWPCScalesLine.tsx`**

Replace the entire file with:

```tsx
import { Line } from '@ant-design/charts';
import { Empty } from 'antd';
import { useIsDark } from '../../theme';
import type { HistoryWindow } from '../../api/history';
import { timeAxisFormatter } from './timeFormat';

export interface SWPCScalesLineProps {
  data: Array<{ at: string; gScale: number; r1: number; s1: number }>;
  window: HistoryWindow;
}

interface Flat {
  at: Date;
  series: 'G-scale' | 'R1' | 'S1';
  value: number;
}

export function SWPCScalesLine({ data, window }: SWPCScalesLineProps) {
  const isDark = useIsDark();
  if (data.length === 0) {
    return <Empty description="No data in this window" />;
  }
  const flat: Flat[] = data.flatMap((b) => {
    const at = new Date(b.at);
    return [
      { at, series: 'G-scale', value: b.gScale },
      { at, series: 'R1', value: b.r1 },
      { at, series: 'S1', value: b.s1 },
    ];
  });
  return (
    <Line
      data={flat}
      xField="at"
      yField="value"
      colorField="series"
      shapeField="hv"
      height={180}
      scale={{ color: { range: ['#fa8c16', '#f5222d', '#1890ff'] } }}
      axis={{
        x: { title: false, labelFormatter: timeAxisFormatter(window) },
        y: { title: 'value' },
      }}
      point={{ shapeField: 'point', sizeField: 3 }}
      theme={isDark ? 'academy' : 'classic'}
    />
  );
}
```

Update its test analogously: pass `window="24h"`, mock `useIsDark`, assert theme on the props.

- [ ] **Step 6.6: Update `SWPCAlertsBars.tsx`**

Replace with:

```tsx
import { Column } from '@ant-design/charts';
import { Empty } from 'antd';
import { useIsDark } from '../../theme';
import type { HistoryWindow } from '../../api/history';
import { timeAxisFormatter } from './timeFormat';

export interface SWPCAlertsBarsProps {
  data: Array<{ at: string; warning: number; watch: number; alert: number }>;
  window: HistoryWindow;
}

interface Flat {
  at: Date;
  severity: 'warning' | 'watch' | 'alert';
  count: number;
}

export function SWPCAlertsBars({ data, window }: SWPCAlertsBarsProps) {
  const isDark = useIsDark();
  if (data.length === 0) {
    return <Empty description="No data in this window" />;
  }
  const flat: Flat[] = data.flatMap((b) => {
    const at = new Date(b.at);
    return [
      { at, severity: 'warning', count: b.warning },
      { at, severity: 'watch', count: b.watch },
      { at, severity: 'alert', count: b.alert },
    ];
  });
  return (
    <Column
      data={flat}
      xField="at"
      yField="count"
      colorField="severity"
      stack
      height={180}
      scale={{ color: { range: ['#f5222d', '#fa8c16', '#1890ff'] } }}
      axis={{
        x: { title: false, labelFormatter: timeAxisFormatter(window) },
        y: { title: 'count' },
      }}
      theme={isDark ? 'academy' : 'classic'}
    />
  );
}
```

Update its test analogously.

- [ ] **Step 6.7: Update `USGSQuakesScatter.tsx`**

Replace with:

```tsx
import { Scatter } from '@ant-design/charts';
import { Empty } from 'antd';
import { useIsDark } from '../../theme';
import type { HistoryWindow } from '../../api/history';
import { timeAxisFormatter } from './timeFormat';

export interface USGSQuakesScatterProps {
  data: Array<{ at: string; mag: number; place: string; depthKm: number }>;
  window: HistoryWindow;
}

export function USGSQuakesScatter({ data, window }: USGSQuakesScatterProps) {
  const isDark = useIsDark();
  if (data.length === 0) {
    return <Empty description="No events in this window" />;
  }
  const points = data.map((d) => ({ ...d, at: new Date(d.at) }));
  return (
    <Scatter
      data={points}
      xField="at"
      yField="mag"
      sizeField="mag"
      shapeField="circle"
      height={180}
      style={{ fill: '#722ed1', fillOpacity: 0.6 }}
      scale={{ y: { domainMin: 4, domainMax: 8 }, size: { range: [4, 16] } }}
      axis={{
        x: { title: false, labelFormatter: timeAxisFormatter(window) },
        y: { title: 'magnitude' },
      }}
      theme={isDark ? 'academy' : 'classic'}
    />
  );
}
```

Update its test analogously.

- [ ] **Step 6.8: Run tests + typecheck**

```bash
task web:test
task web:typecheck
```

Expected: all green / clean. Note: HistoryChartRow.test.tsx may fail because it now passes only `data` to chart components and the type requires `window` too — that's Task 8's problem; if it fails here, leave it failing for Task 8 to fix. (The failing test must be the one Task 8 owns; if any OTHER test fails, fix it as part of this task's typo allowance.)

- [ ] **Step 6.9: Commit**

```bash
git add \
  web/src/components/charts/timeFormat.ts \
  web/src/components/charts/timeFormat.test.ts \
  web/src/components/charts/NWSAlertsArea.tsx \
  web/src/components/charts/NWSAlertsArea.test.tsx \
  web/src/components/charts/SWPCScalesLine.tsx \
  web/src/components/charts/SWPCScalesLine.test.tsx \
  web/src/components/charts/SWPCAlertsBars.tsx \
  web/src/components/charts/SWPCAlertsBars.test.tsx \
  web/src/components/charts/USGSQuakesScatter.tsx \
  web/src/components/charts/USGSQuakesScatter.test.tsx
git commit -m "feat(p4): chart components — dark theme + X-axis time formatter" -- \
  web/src/components/charts/timeFormat.ts \
  web/src/components/charts/timeFormat.test.ts \
  web/src/components/charts/NWSAlertsArea.tsx \
  web/src/components/charts/NWSAlertsArea.test.tsx \
  web/src/components/charts/SWPCScalesLine.tsx \
  web/src/components/charts/SWPCScalesLine.test.tsx \
  web/src/components/charts/SWPCAlertsBars.tsx \
  web/src/components/charts/SWPCAlertsBars.test.tsx \
  web/src/components/charts/USGSQuakesScatter.tsx \
  web/src/components/charts/USGSQuakesScatter.test.tsx
```

---

## Task 7: Client — `NWSEventCountsStrip` component

**Files:**
- Create: `web/src/components/charts/NWSEventCountsStrip.tsx`
- Create: `web/src/components/charts/NWSEventCountsStrip.test.tsx`

**Dependencies:** Task 2 (`NWSAlertsHistory.buckets[].eventCounts` type), Task 3 (`useIsDark`).

**Why:** Design Task D, frontend half. Renders a 3-card strip (Tornado / Severe Thunderstorm / Flash Flood) above the NWS area chart, each card showing the latest count + a tiny sparkline of that category over the window.

- [ ] **Step 7.1: Write the failing test**

`web/src/components/charts/NWSEventCountsStrip.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { NWSEventCountsStrip } from './NWSEventCountsStrip';
import type { NWSAlertsHistory } from '../../api/history';

vi.mock('@ant-design/charts', () => ({
  TinyArea: ({ data }: { data: number[] }) => (
    <div data-testid="sparkline">{data.length} pts</div>
  ),
}));
vi.mock('../../theme', () => ({ useIsDark: () => true }));

const sample: NWSAlertsHistory['buckets'] = [
  { at: '2026-05-05T12:00:00Z', activeCount: 5,
    eventCounts: { tornado: 0, severeTstorm: 1, flashFlood: 2 } },
  { at: '2026-05-05T12:05:00Z', activeCount: 7,
    eventCounts: { tornado: 1, severeTstorm: 2, flashFlood: 0 } },
  { at: '2026-05-05T12:10:00Z', activeCount: 9,
    eventCounts: { tornado: 2, severeTstorm: 4, flashFlood: 1 } },
];

describe('NWSEventCountsStrip', () => {
  it('renders three cards with the latest counts', () => {
    render(<NWSEventCountsStrip buckets={sample} />);
    expect(screen.getByText(/Tornado/i)).toBeTruthy();
    expect(screen.getByText(/Severe Thunderstorm/i)).toBeTruthy();
    expect(screen.getByText(/Flash Flood/i)).toBeTruthy();
    // Latest bucket counts are 2 / 4 / 1
    expect(screen.getByTestId('count-tornado').textContent).toBe('2');
    expect(screen.getByTestId('count-severeTstorm').textContent).toBe('4');
    expect(screen.getByTestId('count-flashFlood').textContent).toBe('1');
  });

  it('renders three sparklines, each fed the bucket count history for its category', () => {
    render(<NWSEventCountsStrip buckets={sample} />);
    const sparks = screen.getAllByTestId('sparkline');
    expect(sparks).toHaveLength(3);
    sparks.forEach((s) => expect(s.textContent).toBe('3 pts'));
  });

  it('renders zeros and an empty-style card when buckets is empty', () => {
    render(<NWSEventCountsStrip buckets={[]} />);
    expect(screen.getByTestId('count-tornado').textContent).toBe('0');
    expect(screen.getByTestId('count-severeTstorm').textContent).toBe('0');
    expect(screen.getByTestId('count-flashFlood').textContent).toBe('0');
  });
});
```

- [ ] **Step 7.2: Run, confirm fail**

```bash
cd web && pnpm exec vitest run src/components/charts/NWSEventCountsStrip.test.tsx
```

Expected: import fail (component does not exist).

- [ ] **Step 7.3: Implement `NWSEventCountsStrip.tsx`**

```tsx
import { Card, Space, Typography } from 'antd';
import { TinyArea } from '@ant-design/charts';
import { useIsDark } from '../../theme';
import type { NWSAlertsHistory } from '../../api/history';

type Buckets = NWSAlertsHistory['buckets'];

interface Category {
  key: 'tornado' | 'severeTstorm' | 'flashFlood';
  label: string;
  color: string;
}

const CATEGORIES: Category[] = [
  { key: 'tornado',      label: 'Tornado',             color: '#f5222d' },
  { key: 'severeTstorm', label: 'Severe Thunderstorm', color: '#fa8c16' },
  { key: 'flashFlood',   label: 'Flash Flood',         color: '#1890ff' },
];

export interface NWSEventCountsStripProps {
  buckets: Buckets;
}

export function NWSEventCountsStrip({ buckets }: NWSEventCountsStripProps) {
  const isDark = useIsDark();
  const latest = buckets.length > 0 ? buckets[buckets.length - 1] : null;
  return (
    <Space size="small" wrap style={{ marginBottom: 12, width: '100%' }}>
      {CATEGORIES.map((c) => {
        const series = buckets.map((b) => b.eventCounts[c.key]);
        const latestCount = latest ? latest.eventCounts[c.key] : 0;
        return (
          <Card key={c.key} size="small" style={{ minWidth: 180, flex: '1 1 180px' }} styles={{ body: { padding: 12 } }}>
            <Space direction="vertical" size={2} style={{ width: '100%' }}>
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>{c.label}</Typography.Text>
              <Typography.Text strong style={{ fontSize: 24, color: c.color }} data-testid={`count-${c.key}`}>
                {latestCount}
              </Typography.Text>
              {series.length > 0 ? (
                <TinyArea
                  data={series}
                  height={28}
                  width={140}
                  smooth
                  style={{ fill: c.color, fillOpacity: 0.2, stroke: c.color }}
                  theme={isDark ? 'academy' : 'classic'}
                />
              ) : (
                <div data-testid="sparkline" style={{ height: 28, width: 140 }} />
              )}
            </Space>
          </Card>
        );
      })}
    </Space>
  );
}
```

NOTE: `TinyArea` props in v2 — confirm by browsing `web/node_modules/.pnpm/@ant-design+plots@*/...plots/es/components/tiny/area/index.d.ts` before committing. If a prop name differs (e.g. `data` is `number[]` in v2 — confirm the shape), adjust verbatim. Do not invent props.

- [ ] **Step 7.4: Run tests + typecheck**

```bash
task web:test
task web:typecheck
```

Expected: green / clean. (The mocked `TinyArea` returns a div with `data-testid="sparkline"`; the live component uses TinyArea; the empty-buckets case renders an inline div with the same `data-testid` so the empty-test sparkline assertion isn't needed there — that's why the empty test only asserts counts.)

- [ ] **Step 7.5: Commit**

```bash
git add web/src/components/charts/NWSEventCountsStrip.tsx web/src/components/charts/NWSEventCountsStrip.test.tsx
git commit -m "feat(p4): NWSEventCountsStrip — Tornado/SevereTstorm/FlashFlood cards" -- \
  web/src/components/charts/NWSEventCountsStrip.tsx web/src/components/charts/NWSEventCountsStrip.test.tsx
```

---

## Task 8: Client — `HistoryChartRow` integration (headline tag + SSE re-fetch + strip + max-width)

**Files:**
- Modify: `web/src/components/HistoryChartRow.tsx`
- Modify: `web/src/components/HistoryChartRow.test.tsx`

**Dependencies:** Task 2 (extended NWS type), Task 6 (charts now require `window` prop), Task 7 (NWSEventCountsStrip).

**Why:** The integration point. We add (a) a headline-stat tag in the card extra, computed from the response; (b) SSE-driven re-fetch via the snapshot store's per-source `fetchedAt` timestamp, debounced 250ms; (c) the `NWSEventCountsStrip` above the area chart for the NWS row only; (d) `width: 100%` on the card so it stretches to fill the row.

- [ ] **Step 8.1: Write the failing test**

Replace `web/src/components/HistoryChartRow.test.tsx` with:

```tsx
import { render, screen, waitFor, act } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { HistoryChartRow } from './HistoryChartRow';

vi.mock('../api/history', () => ({
  fetchNWSAlertsHistory: vi.fn(),
  fetchSWPCScalesHistory: vi.fn(),
  fetchSWPCAlertsHistory: vi.fn(),
  fetchUSGSQuakesHistory: vi.fn(),
  fetchUSGSVolcanoesHistory: vi.fn(),
}));

vi.mock('./charts/NWSAlertsArea', () => ({ NWSAlertsArea: ({ data }: { data: unknown[] }) => <div data-testid="nws-area">{data.length}</div> }));
vi.mock('./charts/NWSEventCountsStrip', () => ({ NWSEventCountsStrip: ({ buckets }: { buckets: unknown[] }) => <div data-testid="nws-strip">{buckets.length}</div> }));
vi.mock('./charts/SWPCScalesLine', () => ({ SWPCScalesLine: () => <div data-testid="swpc-scales" /> }));
vi.mock('./charts/SWPCAlertsBars', () => ({ SWPCAlertsBars: () => <div data-testid="swpc-alerts" /> }));
vi.mock('./charts/USGSQuakesScatter', () => ({ USGSQuakesScatter: () => <div data-testid="usgs-quakes" /> }));
vi.mock('./charts/VolcanoesTimeline', () => ({ VolcanoesTimeline: () => <div data-testid="volcanoes-timeline" /> }));

// Mock the snapshot store so we can drive SSE-tick simulation.
const subscribers = new Set<() => void>();
let mockFetchedAt = '2026-05-05T12:00:00Z';
vi.mock('../store/snapshot', () => ({
  useSnapshotStore: (selector: (s: { snapshot: unknown }) => unknown) => {
    // The HistoryChartRow selects the per-source fetchedAt; we return our
    // mockFetchedAt value, and call the selector lazily so subscribers fire
    // when we tick the value.
    return selector({
      snapshot: {
        sources: {
          nws_alerts:     { fetchedAt: mockFetchedAt },
          swpc_scales:    { fetchedAt: mockFetchedAt },
          swpc_alerts:    { fetchedAt: mockFetchedAt },
          usgs_quakes:    { fetchedAt: mockFetchedAt },
          usgs_volcanoes: { fetchedAt: mockFetchedAt },
        },
      },
    });
  },
}));

import { fetchNWSAlertsHistory } from '../api/history';

const NWS_RESP = {
  source: 'nws_alerts' as const,
  window: '24h' as const,
  buckets: [
    { at: '2026-05-05T11:00:00Z', activeCount: 7,
      eventCounts: { tornado: 1, severeTstorm: 2, flashFlood: 0 } },
    { at: '2026-05-05T12:00:00Z', activeCount: 12,
      eventCounts: { tornado: 0, severeTstorm: 3, flashFlood: 1 } },
  ],
  windowStart: '2026-05-04T12:00:00Z',
  dataStart: '2026-05-04T12:00:00Z',
};

describe('HistoryChartRow', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFetchedAt = '2026-05-05T12:00:00Z';
    subscribers.clear();
  });

  it('fetches the source on mount and renders the matching chart', async () => {
    (fetchNWSAlertsHistory as ReturnType<typeof vi.fn>).mockResolvedValue(NWS_RESP);
    render(<HistoryChartRow source="nws_alerts" window="24h" />);
    await waitFor(() => expect(fetchNWSAlertsHistory).toHaveBeenCalledWith('24h'));
    expect(await screen.findByTestId('nws-area')).toBeTruthy();
  });

  it('renders the NWSEventCountsStrip for NWS only', async () => {
    (fetchNWSAlertsHistory as ReturnType<typeof vi.fn>).mockResolvedValue(NWS_RESP);
    render(<HistoryChartRow source="nws_alerts" window="24h" />);
    expect(await screen.findByTestId('nws-strip')).toBeTruthy();
  });

  it('renders the headline-stat tag with max activeCount in the window', async () => {
    (fetchNWSAlertsHistory as ReturnType<typeof vi.fn>).mockResolvedValue(NWS_RESP);
    render(<HistoryChartRow source="nws_alerts" window="24h" />);
    expect(await screen.findByText(/12 active/)).toBeTruthy();
  });

  it('re-fetches when the snapshot store fetchedAt advances (SSE live-tail)', async () => {
    (fetchNWSAlertsHistory as ReturnType<typeof vi.fn>).mockResolvedValue(NWS_RESP);
    const { rerender } = render(<HistoryChartRow source="nws_alerts" window="24h" />);
    await waitFor(() => expect(fetchNWSAlertsHistory).toHaveBeenCalledTimes(1));
    // Tick the SSE timestamp — re-render to flush the new selector value.
    mockFetchedAt = '2026-05-05T12:00:30Z';
    rerender(<HistoryChartRow source="nws_alerts" window="24h" />);
    await waitFor(() => expect(fetchNWSAlertsHistory).toHaveBeenCalledTimes(2), { timeout: 1000 });
  });

  it('renders the "data starts here" marker when dataStart > windowStart', async () => {
    (fetchNWSAlertsHistory as ReturnType<typeof vi.fn>).mockResolvedValue({
      ...NWS_RESP,
      window: '30d' as const,
      windowStart: '2026-04-03T10:00:00Z',
      dataStart: '2026-05-01T10:00:00Z',
    });
    render(<HistoryChartRow source="nws_alerts" window="30d" />);
    await waitFor(() => expect(fetchNWSAlertsHistory).toHaveBeenCalled());
    expect(await screen.findByText(/data starts here/i)).toBeTruthy();
  });
});
```

- [ ] **Step 8.2: Run, confirm fail**

```bash
cd web && pnpm exec vitest run src/components/HistoryChartRow.test.tsx
```

Expected: multiple failures — strip not rendered, headline tag absent, no re-fetch on SSE tick.

- [ ] **Step 8.3: Replace `web/src/components/HistoryChartRow.tsx` with:**

```tsx
import { useEffect, useRef, useState } from 'react';
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
import { NWSEventCountsStrip } from './charts/NWSEventCountsStrip';
import { SWPCScalesLine } from './charts/SWPCScalesLine';
import { SWPCAlertsBars } from './charts/SWPCAlertsBars';
import { USGSQuakesScatter } from './charts/USGSQuakesScatter';
import { VolcanoesTimeline } from './charts/VolcanoesTimeline';
import { useSnapshotStore } from '../store/snapshot';

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

const SSE_REFETCH_DEBOUNCE_MS = 250;

function headlineFor(resp: Resp): string {
  switch (resp.source) {
    case 'nws_alerts': {
      const max = resp.buckets.reduce((m, b) => (b.activeCount > m ? b.activeCount : m), 0);
      return `${max} active`;
    }
    case 'swpc_scales': {
      const last = resp.buckets[resp.buckets.length - 1];
      return last ? `G${last.gScale}` : 'G0';
    }
    case 'swpc_alerts': {
      const total = resp.buckets.reduce((s, b) => s + b.warning + b.watch + b.alert, 0);
      return `${total} in window`;
    }
    case 'usgs_quakes':
      return `${resp.events.length} M4+`;
    case 'usgs_volcanoes':
      return `${resp.changes.length} elevated`;
  }
}

export function HistoryChartRow({ source, window }: HistoryChartRowProps) {
  const [resp, setResp] = useState<Resp | null>(null);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);

  // Per-source SSE timestamp from the snapshot store. When this advances we
  // re-fetch /api/history (debounced 250ms to coalesce same-tick storms).
  const sseFetchedAt = useSnapshotStore(
    (s) => s.snapshot?.sources?.[source]?.fetchedAt ?? null,
  );

  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    let cancelled = false;
    const fetcher: Record<HistorySource, (w: HistoryWindow) => Promise<Resp>> = {
      nws_alerts: (w) => fetchNWSAlertsHistory(w),
      swpc_scales: (w) => fetchSWPCScalesHistory(w),
      swpc_alerts: (w) => fetchSWPCAlertsHistory(w),
      usgs_quakes: (w) => fetchUSGSQuakesHistory(w),
      usgs_volcanoes: (w) => fetchUSGSVolcanoesHistory(w),
    };
    const run = () => {
      setLoading(true);
      setErr(null);
      fetcher[source](window)
        .then((r) => { if (!cancelled) setResp(r); })
        .catch((e: unknown) => { if (!cancelled) setErr(String(e)); })
        .finally(() => { if (!cancelled) setLoading(false); });
    };

    // Mount + (source,window) change → fetch immediately.
    // sseFetchedAt change → debounce-fetch.
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(run, SSE_REFETCH_DEBOUNCE_MS);

    return () => {
      cancelled = true;
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [source, window, sseFetchedAt]);

  const dataStartLag =
    resp && new Date(resp.dataStart).getTime() > new Date(resp.windowStart).getTime() + 60_000;

  let body: React.ReactNode;
  if (loading && !resp) {
    body = <Skeleton active paragraph={{ rows: 3 }} />;
  } else if (err) {
    body = <Typography.Text type="danger">Error: {err}</Typography.Text>;
  } else if (!resp) {
    body = null;
  } else {
    switch (resp.source) {
      case 'nws_alerts':
        body = (
          <>
            <NWSEventCountsStrip buckets={resp.buckets} />
            <NWSAlertsArea data={resp.buckets} window={window} />
          </>
        );
        break;
      case 'swpc_scales':
        body = <SWPCScalesLine data={resp.buckets} window={window} />;
        break;
      case 'swpc_alerts':
        body = <SWPCAlertsBars data={resp.buckets} window={window} />;
        break;
      case 'usgs_quakes':
        body = <USGSQuakesScatter data={resp.events} window={window} />;
        break;
      case 'usgs_volcanoes':
        body = <VolcanoesTimeline data={resp.changes} />;
        break;
    }
  }

  const extra = (
    <>
      {resp ? <Tag>{headlineFor(resp)}</Tag> : null}
      {dataStartLag && resp ? (
        <Tag>data starts here → {new Date(resp.dataStart).toUTCString()}</Tag>
      ) : null}
    </>
  );

  return (
    <Card
      title={TITLES[source]}
      extra={extra}
      style={{ marginBottom: 16, width: '100%' }}
      data-testid={`history-row-${source}`}
    >
      {body}
    </Card>
  );
}
```

- [ ] **Step 8.4: Run tests + typecheck**

```bash
task web:test
task web:typecheck
```

Expected: green / clean. The new test cases all pass; no other test regresses.

- [ ] **Step 8.5: Commit**

```bash
git add web/src/components/HistoryChartRow.tsx web/src/components/HistoryChartRow.test.tsx
git commit -m "feat(p4): HistoryChartRow — headline + SSE refetch + strip + width" -- \
  web/src/components/HistoryChartRow.tsx web/src/components/HistoryChartRow.test.tsx
```

---

## Task 9: Verify, Playwright tooltip smoke, restore webdist placeholder, push, PR

**Files:**
- Possibly modify: `web/src/pages/History.tsx` (only if Task 8's `width: '100%'` on the card isn't enough)
- Optional create: `web/tests/e2e/history-tooltip.spec.ts` (Playwright) — only if a Playwright runner is wired in the repo. If not, fall back to a vitest assertion that tooltip props are passed.

**Dependencies:** Tasks 1–8.

**Why:** Final integration. Validate the full polish round visually + by tests, restore the placeholder, push, and either open a new PR or update the existing one.

- [ ] **Step 9.1: Run the full verification gate**

```bash
go test ./... -race -count=1 -timeout 120s
golangci-lint run ./...
task web:test
task web:typecheck
```

All four must pass.

- [ ] **Step 9.2: Build the real frontend bundle**

```bash
task build:all
```

Expected: builds the SPA into `internal/webdist/dist/` and the binary into `bin/cwd`.

- [ ] **Step 9.3: Restore the webdist placeholder**

```bash
git checkout 665e40711738d00c593b16d014fbb29a665b624b -- internal/webdist/dist/index.html
git diff 665e40711738d00c593b16d014fbb29a665b624b -- internal/webdist/dist/index.html | head -3
```

The diff MUST be empty. If it isn't, run the checkout again.

- [ ] **Step 9.4: Tooltip smoke check**

Two paths — pick one:

**Path A — Playwright (preferred if `web/tests/e2e/` already exists):**

Write a small spec that navigates to `/history`, hovers over the NWS area chart, and asserts the AntD-charts tooltip element appears. Skeleton:

```ts
import { test, expect } from '@playwright/test';

test('NWS chart shows tooltip on hover', async ({ page }) => {
  await page.goto('http://127.0.0.1:8765/history');
  // Wait for the chart canvas to render
  const chart = page.getByTestId('history-row-nws_alerts').locator('canvas').first();
  await chart.waitFor({ state: 'visible' });
  const box = await chart.boundingBox();
  if (!box) throw new Error('no bounding box');
  await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
  // Tooltip is rendered into a sibling div managed by G2
  await expect(page.locator('.g2-tooltip')).toBeVisible({ timeout: 2000 });
});
```

Run:

```bash
cd web && pnpm exec playwright test tests/e2e/history-tooltip.spec.ts
```

**Path B — Unit assertion (fallback if no Playwright runner is wired):**

Add a vitest assertion in each chart's `.test.tsx` that the chart receives a defined `tooltip` config. Skip if v2's default tooltip is enabled-by-default (which it is) — in that case, document that the tooltip smoke check is a manual step and move on.

For this round, **default to Path B** (the simpler one) and add a one-line comment in `NWSAlertsArea.test.tsx` referencing manual verification. If a future task wires Playwright into CI we can promote to Path A.

- [ ] **Step 9.5: Manual browser smoke**

```bash
./bin/cwd serve &
sleep 2
open http://127.0.0.1:8765/history
```

Verify visually:
- All five rows render with legible axes/legend in dark mode.
- NWS row shows the 3-card strip (Tornado / Severe Tstorm / Flash Flood) above the area chart.
- Headline-stat tag in each card extra (`12 active`, `G0`, etc.).
- "Phase 0 — skeleton" tag is gone; build-version tag in its place.
- Footer source tags do NOT include `image:*` entries (since this is `task build:all` = production-mode build).
- X-axis tick format compact (`HH:mm` for 24h).
- Cards stretch to full width.

Kill the server (`kill %1`).

- [ ] **Step 9.6: Final protected-files check**

```bash
git diff 665e40711738d00c593b16d014fbb29a665b624b -- \
  internal/cache/cache.go internal/sse/hub.go internal/webdist/dist/index.html \
  'internal/sources/*.go' | head -3
echo "(empty=good)"
```

- [ ] **Step 9.7: Push + open or update PR**

```bash
git push -u origin feature/phase4-history
```

If the Phase 4 v1 PR is still open and the v1 work hasn't been merged: this push extends it (the new commits land on the same branch and show up in the existing PR). If a v1 PR was already merged: open a new PR titled `Phase 4 polish — chart theme, headline, SSE tail, NWS event-type counters` whose body summarizes Tasks 1–8 (use the design doc's §3 bullet list for the summary).

- [ ] **Step 9.8: Commit the manual-smoke note (optional)**

If during Step 9.5 you found a small UX nit you fixed (e.g. tweaking padding), commit it as a separate `fix(p4):` commit. Otherwise no commit needed for Task 9 itself — the prior 8 tasks are the substance.

---

## Out of scope

Verbatim from design §6:
- New per-source charts beyond NWS event-types.
- Configurable per-event-type list at runtime (server-side hardcoded for v1).
- Climatological comparisons / cross-source overlays.
- Restoring history-page state across hard refresh beyond the existing `?w=` URL param.
- Runtime theme toggle that switches charts without a page reload.

---

## Test plan

### Backend
- `internal/api/history_test.go` — `TestHistoryHandler_NWSAlerts_PerEventTypeCounts` covers Tornado/SevereTstorm/FlashFlood mapping + cumulative max-per-bucket.
- Existing `TestHistoryHandler_NWSAlerts_BucketsHaveActiveCount` continues to pass (the new struct keeps the same `activeCount` JSON field).

### Frontend
- `web/src/api/history.test.ts` — `eventCounts` shape on the typed envelope.
- `web/src/theme.test.ts` — `useIsDark` returns true under `darkAlgorithm`, false under `defaultAlgorithm`.
- `web/src/App.test.tsx` — header tag is the build version, not "Phase 0 — skeleton".
- `web/src/components/SourceHealthIndicator.test.tsx` — `image:*` tags hidden in non-dev.
- `web/src/components/charts/timeFormat.test.ts` — mask + formatter per window.
- Each chart component test asserts `theme="academy"` when `useIsDark` is mocked to true.
- `web/src/components/charts/NWSEventCountsStrip.test.tsx` — three cards, latest counts, three sparklines, empty-state handling.
- `web/src/components/HistoryChartRow.test.tsx` — strip rendered for NWS only, headline tag, SSE-driven re-fetch, data-starts-here marker.

### Smoke / E2E
- `task smoke` — unchanged from Phase 4 v1; the existing `/api/history?source=nws_alerts&window=24h` assertion still applies.
- Manual / Playwright tooltip verification (Step 9.4 / 9.5).

---

## Definition of done

- [ ] Approved design + this implementation plan committed under `docs/plans/`.
- [ ] All Go tests pass with `-race`; `golangci-lint run ./...` clean.
- [ ] All frontend tests pass; `task web:typecheck` clean; `task web:build` succeeds (then placeholder restored).
- [ ] `task smoke` passes.
- [ ] `internal/webdist/dist/index.html` byte-identical to BASE.
- [ ] `internal/cache/cache.go` and `internal/sse/hub.go` byte-identical to BASE.
- [ ] `internal/sources/*.go` byte-identical to BASE.
- [ ] Manual browser smoke: 5 rows render, NWS strip visible, headline tags visible, build-version tag in header, footer free of `image:*` tags in production build, X-axis tick format compact, cards stretch full width.
- [ ] Independent comprehensive review on the diff vs `665e40711738d00c593b16d014fbb29a665b624b` complete; material findings addressed.
- [ ] PR pushed (extending Phase 4 v1 PR or new), CI green.

### Per-task gates

Every task ends with: tests green (`-race` for Go, `task web:test` for frontend), lint green (`golangci-lint run ./internal/<pkg>/...` for Go-touching tasks; `task web:typecheck` for frontend-touching tasks), and a single conventional-prefix commit (`feat(p4):` / `test(p4):` / `chore(p4):` / `fix(p4):` / `docs(p4):`) with the explicit-pathspec form (`git commit ... -- <files>`). No task is marked complete until those three gates pass — that is the `superpowers:verification-before-completion` contract.

---

## Self-review (writing-plans skill checklist)

**Spec coverage:** Every section of the design maps to a task —
- Decision 1 (re-fetch live-tail) → Task 8 (SSE useEffect dependency on snapshot store fetchedAt + 250ms debounce).
- Decision 2 (event-type list scope: 3 only) → Task 1 (`nwsEventCounters` map with exactly 3 keys), Task 2 (typed envelope with exactly 3 fields), Task 7 (`CATEGORIES` array with exactly 3 entries).
- Decision 3 (headline from history response, not snapshot store) → Task 8's `headlineFor()` reads from `resp` only.
- Decision 4 (build-version tag) → Task 4.
- Decision 5 (page max-width) → Task 8 (`width: '100%'` on the Card).
- Decision 6 (Phase 0 tag) → Task 4.
- Decision 7 (image:* gating) → Task 5.
- Tasks A (dark theme), G (time format) → merged into Task 6.
- Task D (event-type counters) → Tasks 1 (server) + 2 (type) + 7 (component) + 8 (composition).
- Task I (tooltip smoke) → Task 9.4.

**Placeholder scan:** No "TBD" / "TODO" / "implement later" / "similar to Task N". Every code block is verbatim. Two notes flagged for the implementing subagent: confirm `TinyArea` v2 prop names (Step 7.3) and confirm AntD's existing `theme.ts` structure (Step 3.1) before writing — both are 30-second probes and don't change the contract.

**Type consistency:**
- `HistoryWindow` (`web/src/api/history.ts`, Phase 4 v1) is consumed by every chart component (Task 6) and `HistoryChartRow` (Task 8) under the same name.
- `NWSAlertsHistory.buckets[].eventCounts` (Task 2) is consumed by `NWSEventCountsStrip` (Task 7) and `HistoryChartRow` (Task 8).
- `useIsDark` (Task 3) is consumed by all 4 chart components (Task 6) and `NWSEventCountsStrip` (Task 7).
- `nwsEventCounters` keys (Task 1) match `CATEGORIES[].key` and `eventCounts` field names (Tasks 2, 7) — `tornado` / `severeTstorm` / `flashFlood` everywhere.
- The Go `nwsAlertsBucket.EventCounts.{Tornado,SevereTstorm,FlashFlood}` JSON tags `tornado` / `severeTstorm` / `flashFlood` match the TS field names.

**Known gaps to flag:**
- `TinyArea` v2 prop shape (Step 7.3) is documented as "confirm before commit" — if v2 wants `data: number[]` or `data: object[]`, adapt; the test mock is loose enough to absorb either.
- AntD's `colorBgBase` token assumption (Step 3.4) — if for some reason it's not literal-hex in this AntD version, swap to `colorBgLayout` or a similar token. The hook returns `false` on parse failure (safe default = light theme).

No other gaps.
