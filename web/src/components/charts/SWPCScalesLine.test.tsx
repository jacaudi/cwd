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
