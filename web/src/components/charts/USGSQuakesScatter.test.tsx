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
