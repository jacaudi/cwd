# Phase 4 — follow-ups (issue #17): dark-mode chart theme + UX polish — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve the 5 UX issues in [issue #17](https://github.com/jacaudi/cwd/issues/17) — dark-mode chart contrast, SWPC empty-state, "Missing encoding" console errors, AlertBadgeBar zero-badge clutter, VolcanoList duplicate chip — without introducing custom G2 theme tokens.

**Architecture:** Wrap the `<HistoryChartRow>` map in `pages/History.tsx` with `<ConfigProvider common={{theme:{type:'dark'|'light'}}}>` from `@ant-design/charts`; drop the dead per-chart `theme={…}` prop and the `useIsDark` import from each chart; drop `scale.x.type='time'` so v2 infers from `Date` data; tighten the SWPC bars empty-state to `every`-zero; flip AlertBadgeBar to `<Badge showZero={false}>`; delete the redundant `COLOR_CHIP` `<Tag>` in VolcanoList.

**Tech Stack:** React 18 + TS 5 + AntD 5 + `@ant-design/charts` 2.6.7, Vitest + Testing Library + jsdom, Taskfile. No backend changes this round.

**Source documents:**
- Approved design: [`docs/plans/2026-05-05-phase4-followups-issue-17-design.md`](2026-05-05-phase4-followups-issue-17-design.md)
- Phase 4 v1 design (load-bearing for chart shapes): [`docs/plans/2026-05-03-phase4-history-design.md`](2026-05-03-phase4-history-design.md)
- Polish-round implementation (structural template): [`docs/plans/2026-05-05-phase4-history-polish-implementation.md`](2026-05-05-phase4-history-polish-implementation.md)

**Branch:** `feature/p4-followups-issue-17` off `origin/main` at `e5dba8d` (Phase 4 polish-round merge). **BASE for diffs:** `e5dba8d68efa6f32e6e5536f88209fb6f8049194`.

> **For Claude:** REQUIRED EXECUTION WORKFLOW (follow in order):
> 1. `superpowers:using-git-worktrees` — Worktree already exists at `.worktrees/p4-followups-issue-17`; reuse it.
> 2. `superpowers:subagent-driven-development` — Dispatch a fresh subagent per task.
> 3. `superpowers:test-driven-development` — All subagents use TDD.
> 4. `superpowers:verification-before-completion` — Verify all tests pass per task.
> 5. `superpowers:requesting-code-review` — Per-task review (built-in).
> 6. After all tasks: independent comprehensive code review on the full diff vs `e5dba8d`.
> 7. `superpowers:finishing-a-development-branch` — Push + open PR.
>
> Skills carry their own model and effort settings. Do not override them.

---

## Conventions

- All commits prefixed `feat(p4-followups):`, `test(p4-followups):`, `chore(p4-followups):`, `fix(p4-followups):`, `docs(p4-followups):` — distinguishes from Phase 4 v1 + polish-round (`feat(p4):`).
- Frontend tests live next to the component (`Component.test.tsx`).
- `golangci-lint` v2 must stay clean (no Go changes this round, but verify nothing regressed at Task 9).
- Never `git add -A` / `git add .`. Never skip hooks (`--no-verify`, `--no-gpg-sign`).

### Commit author identity

Already configured at the repo level (`jacaudi <47005674+jacaudi@users.noreply.github.com>`). Do NOT run `git config`.

### Stage + commit by explicit pathspec

```bash
git add path/to/file_a.tsx path/to/file_b.tsx
git commit -m "feat(p4-followups): explanatory message" -- path/to/file_a.tsx path/to/file_b.tsx
```

The trailing `-- <pathspec>` form scopes the commit even if a sibling subagent stages something between your `git add` and `git commit`.

### Worktree

Created earlier in this session — `.worktrees/p4-followups-issue-17`, branch `feature/p4-followups-issue-17` off `origin/main` at `e5dba8d`. All commands assume CWD is the worktree.

### Trap — webdist placeholder file (do NOT stage)

`internal/webdist/dist/index.html` is a committed placeholder. `task web:build` overwrites it with a real Vite build. Only Task 9 (final integration) may run `task web:build`. Earlier tasks verify with `task web:test` and `task web:typecheck` only. After running `task web:build`, restore via:

```bash
git checkout e5dba8d68efa6f32e6e5536f88209fb6f8049194 -- internal/webdist/dist/index.html
```

### Trap — protected files contract (full list)

Before each commit AND before the PR, verify these four paths are byte-identical to BASE:
- `internal/cache/cache.go`
- `internal/sse/hub.go`
- `internal/webdist/dist/index.html`
- `internal/sources/*.go` (every file)

```bash
git diff e5dba8d68efa6f32e6e5536f88209fb6f8049194 -- \
  internal/cache/cache.go internal/sse/hub.go internal/webdist/dist/index.html \
  'internal/sources/*.go' | head -3
```

Empty = good. This round changes only `web/src/**` and `docs/plans/*` so the contract is honored trivially.

### Trap — `useIsDark` is still exported by `web/src/theme.ts`

We are dropping the per-chart `useIsDark` import — but the function itself stays in `theme.ts`. `pages/History.tsx` still uses it. Do NOT delete it from `theme.ts`.

---

## Dependency graph

```
0 setup: install web deps, sanity-check baseline ─┐
                                                   │
                                                   ▼
        ┌──────────────────┬─────────────┬─────────────┬──────────────┬──────────────┬─────────────┬────────────┐
        ▼                  ▼             ▼             ▼              ▼              ▼             ▼            ▼
1 NWSAlertsArea   2 SWPCScalesLine  3 SWPCAlertsBars 4 USGSQuakes  5 AlertBadgeBar 6 VolcanoList 7 HistRow.test 8 History wrap
        │                  │             │             │              │              │             │            │
        └──────────────────┴─────────────┴─────────────┴──────────────┴──────────────┴─────────────┴────────────┴──► 9 verify + push
```

**Parallelism windows:**
- **Wave 0** (1): Task 0 — setup. Must complete before anything else.
- **Wave 1** (8 parallel): Tasks 1, 2, 3, 4, 5, 6, 7, 8 — eight different file regions, no path overlaps.
- **Wave 2** (1): Task 9 — verify, build, restore webdist, browser smoke, push.

No two parallel-runnable tasks touch the same file. Per-task pathspec discipline provides belt-and-suspenders.

---

## Task 0: Setup — install web deps + verify baseline

**Files:** none modified.

**Dependencies:** none.

**Why:** `web/node_modules` is missing in fresh clones / fresh worktrees. Subsequent tasks need `pnpm` packages installed before `task web:test` will work. Also asserts the worktree starts clean against `e5dba8d` so any later failure is unambiguously caused by a task's own changes.

- [ ] **Step 0.1: Install web deps**

```bash
task web:install
```

Or, if Taskfile target is named differently, fall through to the project-equivalent (`pnpm -C web install`).

- [ ] **Step 0.2: Run baseline tests + typecheck**

```bash
task web:test
task web:typecheck
```

Expected: PASS, both. If either fails on the untouched baseline, STOP and surface to the user — the worktree's BASE is broken before we've made any change.

- [ ] **Step 0.3: Verify protected-files contract is satisfied at BASE (sanity)**

```bash
git diff e5dba8d68efa6f32e6e5536f88209fb6f8049194 -- \
  internal/cache/cache.go internal/sse/hub.go internal/webdist/dist/index.html \
  'internal/sources/*.go' | head -3
```

Expected: empty output.

- [ ] **Step 0.4: No commit.**

Setup-only task; no files modified.

---

## Task 1: NWSAlertsArea — drop `useIsDark`, drop `theme` prop, drop `scale.x.type`

**Files:**
- Modify: `web/src/components/charts/NWSAlertsArea.tsx`
- Modify: `web/src/components/charts/NWSAlertsArea.test.tsx`

**Dependencies:** Task 0.

**Why:** Decisions 2 + 3 — once the page-level `ChartsConfigProvider` (Task 8) supplies the theme, the per-chart `theme={isDark ? 'academy' : 'classic'}` prop is dead code. Dropping `scale: { x: { type: 'time' } }` lets G2 v2 infer the time scale from the `Date` objects in `at`, which is required to clear the "Missing encoding for channel: x." console errors (finding 3).

- [ ] **Step 1.1: Update the test to fail**

Replace the entire content of `web/src/components/charts/NWSAlertsArea.test.tsx` with:

```tsx
import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { NWSAlertsArea } from './NWSAlertsArea';

const mockArea = vi.fn();
vi.mock('@ant-design/charts', () => ({
  Area: (props: { data: Array<{ at: Date; activeCount: number }> }) => {
    mockArea(props);
    return <div data-testid="area-chart">{props.data.length} points</div>;
  },
}));

describe('NWSAlertsArea', () => {
  it('renders the area chart with the supplied buckets', () => {
    render(<NWSAlertsArea
      window="24h"
      data={[
        { at: '2026-05-03T10:00:00Z', activeCount: 12 },
        { at: '2026-05-03T11:00:00Z', activeCount: 18 },
      ]}
    />);
    expect(screen.getByTestId('area-chart').textContent).toContain('2 points');
    const props = mockArea.mock.calls.at(-1)![0];
    expect(props.data[0].at).toBeInstanceOf(Date);
  });

  it('does NOT pass a theme prop (handled by page-level ChartsConfigProvider)', () => {
    render(<NWSAlertsArea
      window="24h"
      data={[{ at: '2026-05-03T10:00:00Z', activeCount: 1 }]}
    />);
    const props = mockArea.mock.calls.at(-1)![0] as Record<string, unknown>;
    expect(props.theme).toBeUndefined();
  });

  it('does NOT set scale.x.type (G2 v2 infers from Date data)', () => {
    render(<NWSAlertsArea
      window="24h"
      data={[{ at: '2026-05-03T10:00:00Z', activeCount: 1 }]}
    />);
    const props = mockArea.mock.calls.at(-1)![0] as { scale?: { x?: { type?: string } } };
    expect(props.scale?.x?.type).toBeUndefined();
  });

  it('renders an "empty" placeholder when data is empty', () => {
    render(<NWSAlertsArea window="24h" data={[]} />);
    expect(screen.queryByTestId('area-chart')).toBeNull();
    expect(screen.getByText(/no data in this window/i)).toBeTruthy();
  });
});
```

- [ ] **Step 1.2: Run, confirm fail**

```bash
task web:test -- src/components/charts/NWSAlertsArea.test.tsx
```

Or, equivalent: `pnpm -C web vitest run src/components/charts/NWSAlertsArea.test.tsx`.

Expected: FAIL — the existing component still passes `theme=…` and `scale.x.type='time'`.

- [ ] **Step 1.3: Update the component**

Replace the entire content of `web/src/components/charts/NWSAlertsArea.tsx` with:

```tsx
import { Area } from '@ant-design/charts';
import { Empty } from 'antd';
import type { HistoryWindow } from '../../api/history';
import { timeAxisFormatter } from './timeFormat';

export interface NWSAlertsAreaProps {
  data: Array<{ at: string; activeCount: number }>;
  window: HistoryWindow;
}

export function NWSAlertsArea({ data, window }: NWSAlertsAreaProps) {
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
    />
  );
}
```

Diff from BASE: drop `import { useIsDark } from '../../theme';`; drop `const isDark = useIsDark();`; drop the `theme={isDark ? 'academy' : 'classic'}` prop; drop the `x: { type: 'time' }` clause from `scale`.

- [ ] **Step 1.4: Run, confirm pass**

```bash
task web:test -- src/components/charts/NWSAlertsArea.test.tsx
```

Expected: PASS, all 4 tests.

- [ ] **Step 1.5: Commit**

```bash
git add web/src/components/charts/NWSAlertsArea.tsx web/src/components/charts/NWSAlertsArea.test.tsx
git commit -m "refactor(p4-followups): NWSAlertsArea — drop per-chart theme prop and explicit time scale

ChartsConfigProvider at the History page level now drives theming for all
G2 charts in the subtree (Decisions 1 + 2 of issue #17 follow-up design).
Drop the dead theme={isDark ? 'academy' : 'classic'} prop and useIsDark
import. Drop scale.x.type='time' so v2 infers from Date data and clears
the 'Missing encoding for channel: x.' console errors (finding 3)." -- \
  web/src/components/charts/NWSAlertsArea.tsx web/src/components/charts/NWSAlertsArea.test.tsx
```

---

## Task 2: SWPCScalesLine — drop `useIsDark`, drop `theme` prop, drop `scale.x.type`

**Files:**
- Modify: `web/src/components/charts/SWPCScalesLine.tsx`
- Modify: `web/src/components/charts/SWPCScalesLine.test.tsx`

**Dependencies:** Task 0.

**Why:** Same as Task 1, applied to `SWPCScalesLine` (the multi-series Line chart for G-scale + R1 + S1).

- [ ] **Step 2.1: Update the test to fail**

Replace the entire content of `web/src/components/charts/SWPCScalesLine.test.tsx` with:

```tsx
import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { SWPCScalesLine } from './SWPCScalesLine';

const mockLine = vi.fn();
vi.mock('@ant-design/charts', () => ({
  Line: (props: { data: Array<{ at: Date; series: string; value: number }> }) => {
    mockLine(props);
    return <div data-testid="line-chart">{props.data.length} points</div>;
  },
}));

describe('SWPCScalesLine', () => {
  it('flattens the buckets into per-series rows', () => {
    render(<SWPCScalesLine
      window="24h"
      data={[
        { at: '2026-05-03T10:00:00Z', gScale: 1, r1: 0.4, s1: 0.05 },
        { at: '2026-05-03T11:00:00Z', gScale: 2, r1: 0.5, s1: 0.05 },
      ]}
    />);
    // 2 buckets * 3 series = 6 rows
    expect(screen.getByTestId('line-chart').textContent).toContain('6 points');
    const props = mockLine.mock.calls.at(-1)![0];
    expect(props.data[0].at).toBeInstanceOf(Date);
  });

  it('does NOT pass a theme prop (handled by page-level ChartsConfigProvider)', () => {
    render(<SWPCScalesLine
      window="24h"
      data={[{ at: '2026-05-03T10:00:00Z', gScale: 0, r1: 0, s1: 0 }]}
    />);
    const props = mockLine.mock.calls.at(-1)![0] as Record<string, unknown>;
    expect(props.theme).toBeUndefined();
  });

  it('does NOT set scale.x.type (G2 v2 infers from Date data)', () => {
    render(<SWPCScalesLine
      window="24h"
      data={[{ at: '2026-05-03T10:00:00Z', gScale: 0, r1: 0, s1: 0 }]}
    />);
    const props = mockLine.mock.calls.at(-1)![0] as { scale?: { x?: { type?: string } } };
    expect(props.scale?.x?.type).toBeUndefined();
  });

  it('renders Empty when data is empty', () => {
    render(<SWPCScalesLine window="24h" data={[]} />);
    expect(screen.queryByTestId('line-chart')).toBeNull();
    expect(screen.getByText(/no data in this window/i)).toBeTruthy();
  });
});
```

- [ ] **Step 2.2: Run, confirm fail**

```bash
task web:test -- src/components/charts/SWPCScalesLine.test.tsx
```

Expected: FAIL.

- [ ] **Step 2.3: Update the component**

Replace the entire content of `web/src/components/charts/SWPCScalesLine.tsx` with:

```tsx
import { Line } from '@ant-design/charts';
import { Empty } from 'antd';
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
    />
  );
}
```

- [ ] **Step 2.4: Run, confirm pass**

```bash
task web:test -- src/components/charts/SWPCScalesLine.test.tsx
```

Expected: PASS, all 4 tests.

- [ ] **Step 2.5: Commit**

```bash
git add web/src/components/charts/SWPCScalesLine.tsx web/src/components/charts/SWPCScalesLine.test.tsx
git commit -m "refactor(p4-followups): SWPCScalesLine — drop per-chart theme prop and explicit time scale

Same shape as the NWSAlertsArea change: theme handled at the History page
level via ChartsConfigProvider; v2 infers the time scale from Date data." -- \
  web/src/components/charts/SWPCScalesLine.tsx web/src/components/charts/SWPCScalesLine.test.tsx
```

---

## Task 3: SWPCAlertsBars — drop `useIsDark`/`theme`, replace empty-state with every-zero check

**Files:**
- Modify: `web/src/components/charts/SWPCAlertsBars.tsx`
- Modify: `web/src/components/charts/SWPCAlertsBars.test.tsx`

**Dependencies:** Task 0.

**Why:** Same theme-prop drop as Tasks 1/2/4. Plus Decision 4 — the `<Empty>` placeholder must render not only when `data.length === 0` but also when the response carries 200 buckets that all sum to zero severities (finding 2). NOTE: `SWPCAlertsBars` does NOT have `scale.x.type` today — there is no time-scale clause to drop.

- [ ] **Step 3.1: Update the test to fail**

Replace the entire content of `web/src/components/charts/SWPCAlertsBars.test.tsx` with:

```tsx
import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { SWPCAlertsBars } from './SWPCAlertsBars';

const mockColumn = vi.fn();
vi.mock('@ant-design/charts', () => ({
  Column: (props: { data: Array<{ at: Date; severity: string; count: number }> }) => {
    mockColumn(props);
    return <div data-testid="column-chart">{props.data.length} points</div>;
  },
}));

describe('SWPCAlertsBars', () => {
  it('flattens the buckets into per-severity rows (warning/watch/alert)', () => {
    render(<SWPCAlertsBars
      window="24h"
      data={[
        { at: '2026-05-03T10:00:00Z', warning: 1, watch: 2, alert: 4 },
        { at: '2026-05-03T11:00:00Z', warning: 0, watch: 1, alert: 3 },
      ]}
    />);
    // 2 buckets * 3 severities = 6 rows
    expect(screen.getByTestId('column-chart').textContent).toContain('6 points');
    const props = mockColumn.mock.calls.at(-1)![0];
    expect(props.data[0].at).toBeInstanceOf(Date);
  });

  it('does NOT pass a theme prop (handled by page-level ChartsConfigProvider)', () => {
    render(<SWPCAlertsBars
      window="24h"
      data={[{ at: '2026-05-03T10:00:00Z', warning: 1, watch: 0, alert: 0 }]}
    />);
    const props = mockColumn.mock.calls.at(-1)![0] as Record<string, unknown>;
    expect(props.theme).toBeUndefined();
  });

  it('renders Empty when data is empty', () => {
    render(<SWPCAlertsBars window="24h" data={[]} />);
    expect(screen.queryByTestId('column-chart')).toBeNull();
    expect(screen.getByText(/no alerts in this window/i)).toBeTruthy();
  });

  it('renders Empty when every bucket sums to zero across all severities', () => {
    render(<SWPCAlertsBars
      window="24h"
      data={[
        { at: '2026-05-03T10:00:00Z', warning: 0, watch: 0, alert: 0 },
        { at: '2026-05-03T11:00:00Z', warning: 0, watch: 0, alert: 0 },
        { at: '2026-05-03T12:00:00Z', warning: 0, watch: 0, alert: 0 },
      ]}
    />);
    expect(screen.queryByTestId('column-chart')).toBeNull();
    expect(screen.getByText(/no alerts in this window/i)).toBeTruthy();
  });

  it('renders the chart when at least one bucket has a non-zero severity', () => {
    render(<SWPCAlertsBars
      window="24h"
      data={[
        { at: '2026-05-03T10:00:00Z', warning: 0, watch: 0, alert: 0 },
        { at: '2026-05-03T11:00:00Z', warning: 0, watch: 1, alert: 0 },
      ]}
    />);
    expect(screen.getByTestId('column-chart')).toBeInTheDocument();
  });
});
```

Note the empty-state copy changes from `"No data in this window"` to `"No alerts in this window"` to match the existing wording philosophy of other charts (`USGSQuakesScatter` uses "No events in this window"; SWPC alerts having no alerts is the natural phrasing).

- [ ] **Step 3.2: Run, confirm fail**

```bash
task web:test -- src/components/charts/SWPCAlertsBars.test.tsx
```

Expected: FAIL.

- [ ] **Step 3.3: Update the component**

Replace the entire content of `web/src/components/charts/SWPCAlertsBars.tsx` with:

```tsx
import { Column } from '@ant-design/charts';
import { Empty } from 'antd';
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
  if (data.every((b) => b.warning + b.watch + b.alert === 0)) {
    return <Empty description="No alerts in this window" />;
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
    />
  );
}
```

The `every` predicate is vacuously true on an empty array, so the new guard subsumes the old `data.length === 0` check.

- [ ] **Step 3.4: Run, confirm pass**

```bash
task web:test -- src/components/charts/SWPCAlertsBars.test.tsx
```

Expected: PASS, all 5 tests.

- [ ] **Step 3.5: Commit**

```bash
git add web/src/components/charts/SWPCAlertsBars.tsx web/src/components/charts/SWPCAlertsBars.test.tsx
git commit -m "fix(p4-followups): SWPCAlertsBars — render Empty for all-zero windows + drop theme prop

Issue #17 finding 2: when every bucket carries zero severities, the column
chart was rendering empty bars over a populated time axis (looked broken).
Tighten the empty-state guard to data.every(b => b.warning+b.watch+b.alert
=== 0) — vacuously true on empty arrays, so subsumes the prior length===0
check. Also drop the per-chart theme prop (Decisions 1+2)." -- \
  web/src/components/charts/SWPCAlertsBars.tsx web/src/components/charts/SWPCAlertsBars.test.tsx
```

---

## Task 4: USGSQuakesScatter — drop `useIsDark`, drop `theme` prop, drop `scale.x.type`

**Files:**
- Modify: `web/src/components/charts/USGSQuakesScatter.tsx`
- Modify: `web/src/components/charts/USGSQuakesScatter.test.tsx`

**Dependencies:** Task 0.

**Why:** Same as Tasks 1 + 2 applied to the magnitude scatter chart.

- [ ] **Step 4.1: Update the test to fail**

Replace the entire content of `web/src/components/charts/USGSQuakesScatter.test.tsx` with:

```tsx
import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { USGSQuakesScatter } from './USGSQuakesScatter';

const mockScatter = vi.fn();
vi.mock('@ant-design/charts', () => ({
  Scatter: (props: { data: Array<{ at: Date; mag: number }> }) => {
    mockScatter(props);
    return <div data-testid="scatter-chart">{props.data.length} events</div>;
  },
}));

describe('USGSQuakesScatter', () => {
  it('renders all events as scatter points', () => {
    render(<USGSQuakesScatter
      window="24h"
      data={[
        { at: '2026-05-03T10:00:00Z', mag: 4.2, place: 'Alaska', depthKm: 12 },
        { at: '2026-05-03T11:00:00Z', mag: 5.7, place: 'Chile', depthKm: 30 },
      ]}
    />);
    expect(screen.getByTestId('scatter-chart').textContent).toContain('2 events');
    const props = mockScatter.mock.calls.at(-1)![0];
    expect(props.data[0].at).toBeInstanceOf(Date);
  });

  it('does NOT pass a theme prop (handled by page-level ChartsConfigProvider)', () => {
    render(<USGSQuakesScatter
      window="24h"
      data={[{ at: '2026-05-03T10:00:00Z', mag: 4.2, place: 'A', depthKm: 1 }]}
    />);
    const props = mockScatter.mock.calls.at(-1)![0] as Record<string, unknown>;
    expect(props.theme).toBeUndefined();
  });

  it('does NOT set scale.x.type (G2 v2 infers from Date data)', () => {
    render(<USGSQuakesScatter
      window="24h"
      data={[{ at: '2026-05-03T10:00:00Z', mag: 4.2, place: 'A', depthKm: 1 }]}
    />);
    const props = mockScatter.mock.calls.at(-1)![0] as { scale?: { x?: { type?: string } } };
    expect(props.scale?.x?.type).toBeUndefined();
  });

  it('renders Empty when no events', () => {
    render(<USGSQuakesScatter window="24h" data={[]} />);
    expect(screen.queryByTestId('scatter-chart')).toBeNull();
    expect(screen.getByText(/no events in this window/i)).toBeTruthy();
  });
});
```

- [ ] **Step 4.2: Run, confirm fail**

```bash
task web:test -- src/components/charts/USGSQuakesScatter.test.tsx
```

Expected: FAIL.

- [ ] **Step 4.3: Update the component**

Replace the entire content of `web/src/components/charts/USGSQuakesScatter.tsx` with:

```tsx
import { Scatter } from '@ant-design/charts';
import { Empty } from 'antd';
import type { HistoryWindow } from '../../api/history';
import { timeAxisFormatter } from './timeFormat';

export interface USGSQuakesScatterProps {
  data: Array<{ at: string; mag: number; place: string; depthKm: number }>;
  window: HistoryWindow;
}

export function USGSQuakesScatter({ data, window }: USGSQuakesScatterProps) {
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
    />
  );
}
```

- [ ] **Step 4.4: Run, confirm pass**

```bash
task web:test -- src/components/charts/USGSQuakesScatter.test.tsx
```

Expected: PASS, all 4 tests.

- [ ] **Step 4.5: Commit**

```bash
git add web/src/components/charts/USGSQuakesScatter.tsx web/src/components/charts/USGSQuakesScatter.test.tsx
git commit -m "refactor(p4-followups): USGSQuakesScatter — drop per-chart theme prop and explicit time scale

Same shape as the NWSAlertsArea change." -- \
  web/src/components/charts/USGSQuakesScatter.tsx web/src/components/charts/USGSQuakesScatter.test.tsx
```

---

## Task 5: AlertBadgeBar — `<Badge showZero={false}>`, drop dead `color` prop

**Files:**
- Modify: `web/src/components/AlertBadgeBar.tsx`
- Modify: `web/src/components/AlertBadgeBar.test.tsx`

**Dependencies:** Task 0.

**Why:** Issue #17 finding 4 + Decision 5. The chip's gray `'default'` color (line 47 of `AlertBadgeBar.tsx`) already conveys the inactive state when count is zero; the corner badge added clutter without information. Flipping to `showZero={false}` removes the badge for zero counts; the now-unused `color={n > 0 ? undefined : '#999'}` prop on `Badge` is dropped.

- [ ] **Step 5.1: Update the test to fail**

Replace the entire content of `web/src/components/AlertBadgeBar.test.tsx` with:

```tsx
import { describe, it, expect } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { AlertBadgeBar } from './AlertBadgeBar';
import type { Alert, Category } from '../api/types';

const a = (cat: Category, over: Partial<Alert> = {}): Alert => ({
  id: Math.random().toString(),
  event: cat,
  awips: 'XXX',
  headline: 'h',
  severity: 'Severe',
  sent: 't',
  effective: 't',
  expires: 't',
  areas: [],
  category: cat,
  ...over,
});

describe('AlertBadgeBar', () => {
  it('renders all 10 chips with zero counts when payload empty', () => {
    render(<AlertBadgeBar alerts={[]} onTsunamiClick={() => {}} />);
    for (const label of [
      'Tornado',
      'Severe Thunderstorm',
      'Flash Flood',
      'Tropical',
      'High Wind',
      'Red Flag',
      'Winter',
      'Extreme Heat',
      'Extreme Cold',
      'Tsunami',
    ]) {
      expect(screen.getByLabelText(label)).toBeInTheDocument();
    }
  });

  it('does NOT render a 0 badge for inactive categories (showZero=false)', () => {
    render(<AlertBadgeBar alerts={[]} onTsunamiClick={() => {}} />);
    // Each chip's wrapper carries the aria-label; the chip text "Tornado" is
    // inside, but no "0" textnode should appear under that wrapper.
    const tornadoWrapper = screen.getByLabelText('Tornado');
    expect(within(tornadoWrapper).queryByText('0')).toBeNull();
  });

  it('renders the count badge when n > 0', () => {
    const alerts = [a('Tornado'), a('Tornado'), a('Tsunami')];
    render(<AlertBadgeBar alerts={alerts} onTsunamiClick={() => {}} />);
    const tornadoWrapper = screen.getByLabelText('Tornado');
    expect(within(tornadoWrapper).getByText('2')).toBeInTheDocument();
    const tsunamiWrapper = screen.getByLabelText('Tsunami');
    expect(within(tsunamiWrapper).getByText('1')).toBeInTheDocument();
  });

  it('counts alerts by category and ignores Unknown', () => {
    const alerts = [a('Tornado'), a('Tornado'), a('Tsunami'), a('Unknown')];
    render(<AlertBadgeBar alerts={alerts} onTsunamiClick={() => {}} />);
    expect(screen.getByLabelText('Tornado').textContent).toContain('2');
    expect(screen.getByLabelText('Tsunami').textContent).toContain('1');
  });
});
```

- [ ] **Step 5.2: Run, confirm fail**

```bash
task web:test -- src/components/AlertBadgeBar.test.tsx
```

Expected: FAIL — the existing component still renders a `0` badge for inactive categories. The new "does NOT render a 0 badge" test fails.

- [ ] **Step 5.3: Update the component**

Replace the entire content of `web/src/components/AlertBadgeBar.tsx` with:

```tsx
import { useMemo } from 'react';
import { Badge, Space, Tag, Tooltip } from 'antd';
import type { Alert, Category } from '../api/types';

const ORDER: { key: Exclude<Category, 'Unknown'>; label: string }[] = [
  { key: 'Tornado', label: 'Tornado' },
  { key: 'SevereThunderstorm', label: 'Severe Thunderstorm' },
  { key: 'FlashFlood', label: 'Flash Flood' },
  { key: 'Tropical', label: 'Tropical' },
  { key: 'HighWind', label: 'High Wind' },
  { key: 'RedFlag', label: 'Red Flag' },
  { key: 'Winter', label: 'Winter' },
  { key: 'ExtremeHeat', label: 'Extreme Heat' },
  { key: 'ExtremeCold', label: 'Extreme Cold' },
  { key: 'Tsunami', label: 'Tsunami' },
];

const COLOR: Record<Exclude<Category, 'Unknown'>, string> = {
  Tornado: 'red',
  SevereThunderstorm: 'volcano',
  FlashFlood: 'cyan',
  Tropical: 'magenta',
  HighWind: 'gold',
  RedFlag: 'orange',
  Winter: 'blue',
  ExtremeHeat: 'red',
  ExtremeCold: 'geekblue',
  Tsunami: 'purple',
};

interface Props {
  alerts: Alert[];
  onTsunamiClick: () => void;
}

export function AlertBadgeBar({ alerts, onTsunamiClick }: Props) {
  const counts = useMemo(() => {
    const m: Record<string, number> = {};
    for (const a of alerts) m[a.category] = (m[a.category] ?? 0) + 1;
    return m;
  }, [alerts]);

  return (
    <Space size="small" wrap aria-label="active-alerts">
      {ORDER.map(({ key, label }) => {
        const n = counts[key] ?? 0;
        const color = n > 0 ? COLOR[key] : 'default';
        const onClick = key === 'Tsunami' ? onTsunamiClick : undefined;
        return (
          <Tooltip key={key} title={label}>
            <span aria-label={label}>
              <Badge count={n} showZero={false}>
                <Tag
                  color={color}
                  onClick={onClick}
                  style={{ cursor: onClick && n > 0 ? 'pointer' : 'default' }}
                >
                  {label}
                </Tag>
              </Badge>
            </span>
          </Tooltip>
        );
      })}
    </Space>
  );
}
```

Diff from BASE: `<Badge count={n} showZero color={n > 0 ? undefined : '#999'}>` → `<Badge count={n} showZero={false}>`. The `showZero` prop flipped from implicit-true (with `showZero` written bare) to explicit-false; the dead `color={…}` prop is removed.

- [ ] **Step 5.4: Run, confirm pass**

```bash
task web:test -- src/components/AlertBadgeBar.test.tsx
```

Expected: PASS, all 4 tests.

- [ ] **Step 5.5: Commit**

```bash
git add web/src/components/AlertBadgeBar.tsx web/src/components/AlertBadgeBar.test.tsx
git commit -m "fix(p4-followups): AlertBadgeBar — hide zero badges (showZero={false})

Issue #17 finding 4: gray '0' corner badges on inactive hazard chips
overlapped chip text and added clutter without information. The chip's
own gray default color already encodes 'inactive', so flipping the badge
to showZero={false} de-clutters significantly while preserving the
'we know it's zero' signal via the chip color." -- \
  web/src/components/AlertBadgeBar.tsx web/src/components/AlertBadgeBar.test.tsx
```

---

## Task 6: VolcanoList — drop the `COLOR_CHIP` chip + map

**Files:**
- Modify: `web/src/components/VolcanoList.tsx`
- Modify: `web/src/components/VolcanoList.test.tsx`

**Dependencies:** Task 0.

**Why:** Issue #17 finding 5 + Decision 6. The `ALERT_COLOR` and `COLOR_CHIP` mappings are 1:1 (NORMAL/GREEN, ADVISORY/YELLOW, WATCH/ORANGE, WARNING/RED), so the second `<Tag>` is always a redundant restatement of the first chip's semantic color.

- [ ] **Step 6.1: Update the test to fail**

Replace the entire content of `web/src/components/VolcanoList.test.tsx` with:

```tsx
import { describe, it, expect } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { VolcanoList } from './VolcanoList';
import type { Volcano } from '../api/types';

function mk(over: Partial<Volcano>): Volcano {
  return {
    id: '311100',
    name: 'Redoubt',
    region: 'Alaska Volcano Observatory',
    alert: 'WATCH',
    color: 'ORANGE',
    updatedAt: '2026-05-02T10:00:00Z',
    url: 'https://volcanoes.usgs.gov/hans-public/notice/example',
    ...over,
  };
}

describe('VolcanoList', () => {
  it('returns null when payload is empty', () => {
    const { container } = render(<VolcanoList volcanoes={[]} />);
    expect(container.firstChild).toBeNull();
  });

  it('renders a row per volcano with name + region', () => {
    render(<VolcanoList volcanoes={[mk({}), mk({ id: 'b', name: 'Shasta', region: 'California Volcano Observatory' })]} />);
    expect(screen.getByText('Redoubt')).toBeInTheDocument();
    expect(screen.getByText('Shasta')).toBeInTheDocument();
    expect(screen.getByText(/Alaska Volcano Observatory/)).toBeInTheDocument();
    expect(screen.getByText(/California Volcano Observatory/)).toBeInTheDocument();
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

  it('does NOT render a separate color-code chip (color is conveyed by the alert-level chip)', () => {
    render(<VolcanoList volcanoes={[mk({ color: 'ORANGE' })]} />);
    // The literal "ORANGE" (or any color-code label) should not appear as a Tag.
    expect(screen.queryByText('ORANGE')).toBeNull();
    expect(screen.queryByText('YELLOW')).toBeNull();
    expect(screen.queryByText('RED')).toBeNull();
    expect(screen.queryByText('GREEN')).toBeNull();
  });

  it('renders exactly one chip per volcano (the alert-level chip)', () => {
    render(<VolcanoList volcanoes={[mk({ alert: 'WATCH', color: 'ORANGE' })]} />);
    const tags = screen.getAllByTestId('volcano-alert-tag');
    expect(tags).toHaveLength(1);
    // Sanity: scope a query inside the row's Space to assert no second Tag sibling
    // by looking at the AntD .ant-tag class count within the alert tag's parent.
    const row = tags[0].closest('.ant-list-item');
    expect(row).not.toBeNull();
    if (row) {
      const allTags = within(row as HTMLElement).getAllByText(/.+/, { selector: '.ant-tag' });
      expect(allTags).toHaveLength(1);
    }
  });

  it('renders external link', () => {
    render(<VolcanoList volcanoes={[mk({ url: 'https://volcanoes.usgs.gov/hans-public/notice/example' })]} />);
    const link = screen.getByRole('link') as HTMLAnchorElement;
    expect(link.href).toContain('hans-public/notice/example');
    expect(link.target).toBe('_blank');
    expect(link.rel).toContain('noopener');
  });

  // Guards against an upstream introducing a new AlertLevel value we haven't
  // mapped — the alert chip should still render with antd's `default` color
  // (a defined fallback) rather than passing `undefined` to <Tag color>.
  it('falls back to antd default color when alert value is unknown', () => {
    const rogue = mk({
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      alert: 'FUTURE_LEVEL' as any,
    });
    expect(() => render(<VolcanoList volcanoes={[rogue]} />)).not.toThrow();
    const alertTag = screen.getByTestId('volcano-alert-tag');
    expect(alertTag.className).toContain('ant-tag-default');
  });
});
```

Removed: the original `it('renders the color-code chip', ...)` test. The original `it('falls back to antd default color when alert/color values are unknown', …)` is also updated to drop the color-tag assertion (since there is no color tag anymore).

- [ ] **Step 6.2: Run, confirm fail**

```bash
task web:test -- src/components/VolcanoList.test.tsx
```

Expected: FAIL — the existing component still renders the color chip (`screen.queryByText('ORANGE')` is non-null), and the per-row chip count is 2.

- [ ] **Step 6.3: Update the component**

Replace the entire content of `web/src/components/VolcanoList.tsx` with:

```tsx
import { List, Space, Tag, Typography } from 'antd';
import { ExportOutlined } from '@ant-design/icons';
import type { AlertLevel, Volcano } from '../api/types';

interface Props {
  volcanoes: Volcano[];
}

const ALERT_COLOR: Record<AlertLevel, string> = {
  NORMAL:   'green',
  ADVISORY: 'gold',
  WATCH:    'orange',
  WARNING:  'red',
};

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
                  color={ALERT_COLOR[v.alert] ?? 'default'}
                  data-alert={v.alert}
                  data-testid="volcano-alert-tag"
                >
                  {v.alert}
                </Tag>
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

Diff from BASE: deleted the `COLOR_CHIP` map; deleted the `ColorCode` type import; deleted the `<Tag color={COLOR_CHIP[v.color] ?? 'default'}>{v.color}</Tag>` element. The `v.color` field on the upstream type is unchanged (still on `Volcano`); we just don't render it.

- [ ] **Step 6.4: Run, confirm pass**

```bash
task web:test -- src/components/VolcanoList.test.tsx
```

Expected: PASS, all 7 tests.

- [ ] **Step 6.5: Commit**

```bash
git add web/src/components/VolcanoList.tsx web/src/components/VolcanoList.test.tsx
git commit -m "fix(p4-followups): VolcanoList — drop redundant color-code chip

Issue #17 finding 5: each volcano row showed two side-by-side chips —
the alert-level chip ('WATCH', 'ADVISORY', etc.) colored by ALERT_COLOR,
and a second chip labeling its own color ('ORANGE', 'YELLOW', etc.) via
the 1:1-mapped COLOR_CHIP. Drop the redundant chip; the level chip's
color carries the same signal." -- \
  web/src/components/VolcanoList.tsx web/src/components/VolcanoList.test.tsx
```

---

## Task 7: HistoryChartRow.test — passthrough mock for ChartsConfigProvider

**Files:**
- Modify: `web/src/components/HistoryChartRow.test.tsx`

**Dependencies:** Task 0.

**Why:** Risk 4 of the design — `HistoryChartRow.test.tsx` doesn't import `@ant-design/charts` directly today (it stubs the chart components instead), but once Task 8 wraps the History page in a `ChartsConfigProvider`, any test that renders `<History>` will need to mock that wrapper. `HistoryChartRow.test` doesn't render `<History>` (only `<HistoryChartRow>` directly), so it doesn't strictly need the wrapper mock — but adding a passthrough now means we don't have to come back to this file when a future test refactors. **Cheap, low-risk preventive change.**

If at execution time it turns out `HistoryChartRow.test.tsx` truly never needs the passthrough (e.g. no test in the file ever causes `@ant-design/charts` to be imported), this task can be a no-op — verify by re-running the test suite after Task 8 and confirming green. Either way the mock entry is harmless.

- [ ] **Step 7.1: Add the passthrough mock**

In `web/src/components/HistoryChartRow.test.tsx`, locate the existing stub-mock block (lines 13-18 — the `vi.mock('./charts/...', ...)` calls) and add a new mock block immediately ABOVE that block:

```ts
// Passthrough mock for the chart-package ConfigProvider — set up by Task 8's
// page-level wrapping in pages/History.tsx. Not strictly needed by these
// tests today (they stub HistoryChartRow's chart subcomponents), but kept
// here so a future test that renders pages/History through the real wrapper
// doesn't have to reintroduce it.
vi.mock('@ant-design/charts', () => ({
  ConfigProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));
```

The exact insertion point: after the `vi.mock('../api/history', …)` block on line 11 and before the chart-stub block on line 13.

- [ ] **Step 7.2: Run, confirm pass (no test behavior changes)**

```bash
task web:test -- src/components/HistoryChartRow.test.tsx
```

Expected: PASS, same 8 tests as before. The mock entry is additive.

- [ ] **Step 7.3: Commit**

```bash
git add web/src/components/HistoryChartRow.test.tsx
git commit -m "test(p4-followups): HistoryChartRow.test — passthrough ConfigProvider mock

Belt-and-suspenders for Task 8's page-level ChartsConfigProvider wrapping.
Not strictly needed by these tests (they stub HistoryChartRow's chart
subcomponents) but kept here to avoid re-adding it in a future refactor." -- \
  web/src/components/HistoryChartRow.test.tsx
```

---

## Task 8: pages/History.tsx — wrap with ChartsConfigProvider; add recording-mock test

**Files:**
- Modify: `web/src/pages/History.tsx`
- Modify: `web/src/pages/History.test.tsx`

**Dependencies:** Task 0.

**Why:** Issue #17 finding 1 + Decisions 1 + 2 — the load-bearing fix. Page-level wrapping with `<ChartsConfigProvider key={isDark ? 'dark' : 'light'} common={{theme:{type: isDark ? 'dark' : 'light'}}}>` drives theming for all G2 charts in the subtree (NWS area, SWPC line, SWPC bars, USGS scatter, plus the `Tiny` sparklines inside `NWSEventCountsStrip`). The `key` prop forces a remount on theme toggle.

- [ ] **Step 8.1: Update the test to fail**

Replace the entire content of `web/src/pages/History.test.tsx` with:

```tsx
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, vi } from 'vitest';
import History from './History';

// Stub HistoryChartRow so we can assert page composition without dragging
// in chart libs. The recording mock for ChartsConfigProvider below records
// what `common.theme.type` was passed to it on render.
vi.mock('../components/HistoryChartRow', () => ({
  HistoryChartRow: ({ source, window }: { source: string; window: string }) => (
    <div data-testid={`row-${source}`}>{window}</div>
  ),
}));

// Recording mock — captures the most recent ChartsConfigProvider props.
let recordedThemeType: string | undefined;
vi.mock('@ant-design/charts', () => ({
  ConfigProvider: (props: {
    common?: { theme?: { type?: string } };
    children: React.ReactNode;
  }) => {
    recordedThemeType = props.common?.theme?.type;
    return <>{props.children}</>;
  },
}));

// Force the dark branch for the wrapping assertion.
vi.mock('../theme', () => ({ useIsDark: () => true }));

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

  it('wraps the row map in ChartsConfigProvider with common.theme.type matching useIsDark', async () => {
    recordedThemeType = undefined;
    render(<MemoryRouter><History /></MemoryRouter>);
    // The wrapper renders synchronously on mount; recordedThemeType should
    // be set by the time the rows show up.
    await waitFor(() => {
      expect(screen.getByTestId('row-nws_alerts')).toBeTruthy();
    });
    expect(recordedThemeType).toBe('dark');
  });
});
```

- [ ] **Step 8.2: Run, confirm fail**

```bash
task web:test -- src/pages/History.test.tsx
```

Expected: FAIL — the new "wraps the row map in ChartsConfigProvider" test fails because the existing component doesn't render the wrapper, so `recordedThemeType` stays `undefined`.

- [ ] **Step 8.3: Update the component**

Replace the entire content of `web/src/pages/History.tsx` with:

```tsx
import { useEffect } from 'react';
import { Segmented, Typography } from 'antd';
import { ConfigProvider as ChartsConfigProvider } from '@ant-design/charts';
import { useLocation, useNavigate } from 'react-router-dom';
import { HistoryChartRow, type HistorySource } from '../components/HistoryChartRow';
import { useHistoryStore } from '../store/history';
import { useIsDark } from '../theme';
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
  const isDark = useIsDark();

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
      {/*
        Issue #17 finding 1: G2's default ('classic') theme renders axis
        labels, gridlines, and legend in dark text — invisible on AntD's
        dark card background. Wrap the chart subtree with the package's
        ConfigProvider so all charts in /history pick up a built-in
        dark/light theme. The `key` prop forces a remount on theme toggle
        as belt-and-suspenders against G2 caching its theme on first
        mount. No custom palette tokens — the user has explicitly directed
        Ant-Design built-in `type` only.
      */}
      <ChartsConfigProvider
        key={isDark ? 'dark' : 'light'}
        common={{ theme: { type: isDark ? 'dark' : 'light' } }}
      >
        {SOURCES.map((s) => (
          <HistoryChartRow key={s} source={s} window={window} />
        ))}
      </ChartsConfigProvider>
    </div>
  );
}
```

Diff from BASE: import `ConfigProvider as ChartsConfigProvider` from `@ant-design/charts`; import `useIsDark` from `../theme`; call `useIsDark()`; wrap the `SOURCES.map(...)` block in `<ChartsConfigProvider key={...} common={{theme:{type:...}}}>`.

- [ ] **Step 8.4: Run, confirm pass**

```bash
task web:test -- src/pages/History.test.tsx
```

Expected: PASS, all 4 tests.

- [ ] **Step 8.5: Run typecheck**

```bash
task web:typecheck
```

Expected: clean (no errors). Catches any prop-shape mismatch on the wrapper.

- [ ] **Step 8.6: Commit**

```bash
git add web/src/pages/History.tsx web/src/pages/History.test.tsx
git commit -m "feat(p4-followups): History — wrap chart rows with ChartsConfigProvider for dark mode

Issue #17 finding 1: G2's default theme renders axis/legend/gridlines in
dark text that's invisible against AntD's dark card. Wrap the
<HistoryChartRow> map with @ant-design/charts' ConfigProvider and pass
common.theme.type built from useIsDark() so the entire chart subtree
re-themes consistently. The key prop forces a remount on theme toggle
in case G2 caches its theme on first mount. No custom palette tokens —
user-explicit constraint." -- \
  web/src/pages/History.tsx web/src/pages/History.test.tsx
```

---

## Task 9: Final verification + browser smoke + restore webdist + push

**Files:**
- Verify: all of the above + the protected files
- Modify (transient): `internal/webdist/dist/index.html` is overwritten by `task web:build` and MUST be restored before commit
- No new commits unless a smoke-test finding requires one (per Decision 3 fallback path or Risk 1 escalation)

**Dependencies:** Tasks 1–8 all complete and passing.

**Why:** Final integration gate. Validates that the unit tests didn't paint over a real-browser regression, and that the visual claims of the design (legible dark mode, clean console, clean Active Alerts strip, single chip per volcano) actually hold against the running binary.

- [ ] **Step 9.1: Run the full unit suite**

```bash
task web:test
task web:typecheck
task lint
task test
```

All four expected: PASS / clean. (`task lint` and `task test` cover Go; this round didn't touch Go, so they're a regression check.)

- [ ] **Step 9.2: Verify the protected-files contract**

```bash
git diff e5dba8d68efa6f32e6e5536f88209fb6f8049194 -- \
  internal/cache/cache.go internal/sse/hub.go internal/webdist/dist/index.html \
  'internal/sources/*.go' | head -3
```

Expected: empty output. If non-empty, STOP — investigate and reset the offending file(s) to BASE.

- [ ] **Step 9.3: Build the web SPA + the binary**

```bash
task web:build
task build:all
```

Both expected: SUCCESS. `task web:build` will overwrite `internal/webdist/dist/index.html` with a real Vite build — this is normal, but MUST be restored before any commit (next step).

- [ ] **Step 9.4: Restore the webdist placeholder**

```bash
git checkout e5dba8d68efa6f32e6e5536f88209fb6f8049194 -- internal/webdist/dist/index.html
git status
```

`git status` should show no changes to `internal/webdist/dist/index.html`. If it still shows changes, repeat the `git checkout` command.

- [ ] **Step 9.5: Boot the binary + open in a browser**

```bash
./bin/cwd serve &
# Wait ~90s for sources to warm up.
```

Then open `http://localhost:8080` (or whatever port the binary uses; check the binary's startup log). Use Playwright via the MCP if available, otherwise use a real browser.

- [ ] **Step 9.6: Manual smoke — `/history` in DARK mode**

In the browser:
1. Toggle the SettingsDrawer to "dark".
2. Navigate to `/history`.
3. Open DevTools console.

Verify:
- [ ] Axis tick labels (Y values like "300, 250, 200…", X times like "07:50, 07:55…") are clearly legible against the dark card background.
- [ ] Legend item text (`G-scale R1 S1`, `warning watch alert`) is legible.
- [ ] Y-axis titles (`active alerts`, `value`, `count`, `magnitude`) are legible.
- [ ] Gridlines are visible but subordinate to the data (not invisible, not overpowering).
- [ ] **No** "Missing encoding for channel: x." console errors at mount.
- [ ] After waiting ~60s for an SSE tick, **no** new "Missing encoding" console errors.
- [ ] After toggling 24h → 7d → 30d → 24h, **no** new "Missing encoding" console errors.

**STOP-AND-ESCALATE GATE:** If any chart element remains illegible after the `ChartsConfigProvider type:'dark'` switch, halt the verify pass, capture a screenshot of the specific failing element(s), and surface to the user. Resolution paths (per the design doc, Risk 1, all in this branch — no follow-up issue):
1. A non-palette `ConfigProvider` field that respects the no-custom-themes constraint.
2. Pin or upgrade `@ant-design/charts` (confirm version change with the user).
3. Discuss with the user; user may choose to relax the no-custom-themes constraint (NEVER apply unilaterally) or to merge with a documented caveat.

**FALLBACK FOR `scale.x.type` (per Decision 3):** If "Missing encoding for channel: x." errors persist after dropping `scale.x.type='time'`, switch the affected charts (`NWSAlertsArea`, `SWPCScalesLine`, `USGSQuakesScatter`) to use `axis: { x: { type: 'time' }, …existing axis fields }` instead. Re-run the verify pass; if errors clear, commit as a separate `fix(p4-followups):` commit.

- [ ] **Step 9.7: Manual smoke — `/history` in LIGHT mode**

Toggle theme back to "light" (or `auto` resolving to light). Re-verify all of 9.6's checklist items in light mode. Light mode was passing on BASE; this step asserts no regression.

- [ ] **Step 9.8: Manual smoke — `/` (Overview)**

Navigate to `/`. In BOTH themes:
- [ ] Active Alerts chips render without a corner `0` badge on inactive categories.
- [ ] Active Alerts chips with non-zero counts (if any present) DO render the count badge in the chip's semantic color.
- [ ] Inactive chips are visually subordinate (gray `default` color) but still readable.
- [ ] Tooltip-on-hover still shows the full category name.

- [ ] **Step 9.9: Manual smoke — `/events`**

Navigate to `/events`. In BOTH themes:
- [ ] Each volcano row shows EXACTLY ONE chip (the alert level), in the level's semantic color.
- [ ] No `ORANGE` / `YELLOW` / `RED` / `GREEN` literal-text chips appear next to the level.
- [ ] The level chip's color matches the underlying USGS color code (visual sanity: WATCH should look orange, ADVISORY gold/yellow, WARNING red).
- [ ] The USGS link, region text, and relative-time text all render unchanged.

- [ ] **Step 9.10: Manual smoke — theme toggle while on `/history`**

Open `/history`. Toggle theme dark → light → dark via SettingsDrawer. Verify:
- [ ] All 5 chart rows re-render with the new theme without a stuck-state.
- [ ] The brief flicker (5 charts unmount + remount due to the `key` prop) is the expected behavior — not a bug.
- [ ] After settle, the new theme's contrast is correct (axis labels legible).

- [ ] **Step 9.11: Stop the binary**

```bash
fg
# Ctrl-C to stop ./bin/cwd serve
```

Or `kill %1` if backgrounded.

- [ ] **Step 9.12: Final protected-files re-check**

```bash
git diff e5dba8d68efa6f32e6e5536f88209fb6f8049194 -- \
  internal/cache/cache.go internal/sse/hub.go internal/webdist/dist/index.html \
  'internal/sources/*.go' | head -3
```

Expected: empty. If `internal/webdist/dist/index.html` shows changes from 9.3's `task web:build`, re-run `git checkout BASE -- internal/webdist/dist/index.html`.

- [ ] **Step 9.13: Push the branch**

```bash
git push -u origin feature/p4-followups-issue-17
```

- [ ] **Step 9.14: Open the PR**

```bash
gh pr create --title "Phase 4 follow-ups (issue #17): dark-mode chart theme + UX polish" --body "$(cat <<'EOF'
## Summary

Resolves issue #17 — five UX issues from the post-Phase-4 dark/light review.

- **Major** — dark-mode chart axis text / legend / gridlines now legible on `/history`. Wraps the chart subtree in `<ConfigProvider common={{theme:{type:'dark'|'light'}}}>` from `@ant-design/charts`. No custom G2 token map — built-in `type` only, per user direction.
- **Minor** — SWPC alerts column now renders `<Empty>` when every bucket sums to zero severities (was rendering empty bars over a populated time axis).
- **Minor** — "Missing encoding for channel: x." console errors cleared by dropping the explicit `scale.x.type='time'` config; G2 v2 infers from the `Date` data.
- **Minor** — Active Alerts hazard chips on Overview now hide their `0` badges (`<Badge showZero={false}>`); the chip's gray `default` color carries the inactive signal.
- **Minor** — Volcano rows on `/events` now show a single alert-level chip (`WATCH` in orange) instead of two redundant chips (`WATCH` + `ORANGE`).

## Test plan

- [x] `task web:test` — all unit tests pass
- [x] `task web:typecheck` — clean
- [x] `task web:build && task build:all` — both succeed
- [x] `task lint` + `task test` — Go suite still clean (no Go changes this round)
- [x] Protected files (`internal/cache/cache.go`, `internal/sse/hub.go`, `internal/webdist/dist/index.html`, `internal/sources/*.go`) byte-identical to BASE
- [x] Manual browser smoke in dark + light modes on `/history`, `/`, and `/events`
- [x] No "Missing encoding" console errors at mount / window-toggle / SSE tick / theme toggle on `/history`

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

- [ ] **Step 9.15: No commit unless smoke surfaced a finding**

If steps 9.6–9.10 surfaced no findings beyond what the unit tests already cover, no additional commit is needed in this task. The PR is the final artifact. If a finding was surfaced (per the stop-and-escalate gate or the `axis.x.type` fallback), commit the resolving change as a separate `fix(p4-followups):` commit before pushing.

---

## Self-review

Run this checklist after writing the plan, before handing it off.

### Spec coverage

| Design § | Concept | Plan task |
|---|---|---|
| §2 Decision 1 (page-level wrapper) | Wrap rows with ChartsConfigProvider, `key` prop | Task 8 |
| §2 Decision 2 (drop per-chart prop) | Drop `theme={…}` and `useIsDark` from each chart | Tasks 1, 2, 3, 4 |
| §2 Decision 3 (drop scale.x.type) | Drop time-scale clause from Area/Line/Scatter | Tasks 1, 2, 4 |
| §2 Decision 3 fallback | `axis.x.type='time'` if errors persist | Task 9.6 fallback callout |
| §2 Decision 4 (every-zero empty) | SWPC bars empty when all severities zero | Task 3 |
| §2 Decision 5 (zero-badge hide) | `<Badge showZero={false}>` | Task 5 |
| §2 Decision 6 (drop volcano color chip) | Single chip per volcano | Task 6 |
| §2 Decision 7 (test mock strategy) | Per-chart passthrough; HistoryChartRow.test passthrough; History.test recording | Tasks 1-4 (passthrough), Task 7 (HistoryChartRow), Task 8 (recording) |
| §2 Decision 8 (one cohesive plan) | Single PR | this plan |
| §3.1 ConfigProvider import alias | `import { ConfigProvider as ChartsConfigProvider }` | Task 8 component code |
| §3.2 useIsDark continues to work | Single useIsDark call in History.tsx | Task 8 |
| §3.3 key={isDark} for theme toggle | `key` prop on wrapper | Task 8 |
| §3.4 Protected files | Re-verified at multiple steps | Task 0.3, Task 9.2, Task 9.12 |
| §3.5 web/node_modules missing | Install before tests | Task 0 |
| §8.3 Manual verify | Dark + light on /history /, /events; theme toggle | Task 9.6 – 9.10 |
| Risk 1 stop-and-escalate | In-branch resolution paths | Task 9.6 callout |

All design sections covered.

### Placeholder scan

- No `TBD`, `TODO`, `…implement later…`.
- No "add appropriate error handling" — every step shows the actual code.
- No "similar to Task N" — each task carries its own verbatim code.
- Every code step has a complete code block (not a description of what to write).
- All function names referenced (`useIsDark`, `ConfigProvider`, `headlineFor`, `timeAxisFormatter`) are either in BASE files (verified during exploration) or in the verbatim code blocks above.

### Type consistency

- Decisions table uses `'dark'` / `'light'` for `common.theme.type`; Task 8 component code uses `'dark'` / `'light'`; Task 8 test uses `'dark'`. Match.
- `ChartsConfigProvider` is the consistent alias for `@ant-design/charts`'s `ConfigProvider` across Tasks 7, 8 (recording mock), and the Task 8 component code.
- The empty-state copy `"No alerts in this window"` in Task 3 is consistent across the component and the test.
- `useIsDark` import path `../theme` (from `pages/History.tsx`) and `../../theme` (from chart components, where it's now removed) match the existing tree depth.
- The protected-files BASE hash `e5dba8d68efa6f32e6e5536f88209fb6f8049194` is consistent across Tasks 0, 9, and the conventions section.

### Scope check

Five findings, eight implementation tasks (one per file region), one final integration task. Sized similarly to the polish round (9 tasks). Single PR, single deploy.
