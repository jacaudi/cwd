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
