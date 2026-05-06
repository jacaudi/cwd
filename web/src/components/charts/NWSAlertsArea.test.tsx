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
