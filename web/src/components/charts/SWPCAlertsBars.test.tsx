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
