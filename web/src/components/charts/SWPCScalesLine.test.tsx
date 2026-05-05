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
