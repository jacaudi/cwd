import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { USGSQuakesScatter } from './USGSQuakesScatter';

const mockScatter = vi.fn();
vi.mock('@ant-design/charts', () => ({
  Scatter: (props: { data: Array<{ at: Date; mag: number }>; theme?: string }) => {
    mockScatter(props);
    return <div data-testid="scatter-chart">{props.data.length} events (theme={props.theme ?? 'unset'})</div>;
  },
}));

vi.mock('../../theme', () => ({ useIsDark: () => true }));

describe('USGSQuakesScatter', () => {
  it('renders all events as scatter points with dark theme', () => {
    render(<USGSQuakesScatter
      window="24h"
      data={[
        { at: '2026-05-03T10:00:00Z', mag: 4.2, place: 'Alaska', depthKm: 12 },
        { at: '2026-05-03T11:00:00Z', mag: 5.7, place: 'Chile', depthKm: 30 },
      ]}
    />);
    expect(screen.getByTestId('scatter-chart').textContent).toContain('2 events');
    expect(screen.getByTestId('scatter-chart').textContent).toContain('theme=academy');
    const props = mockScatter.mock.calls.at(-1)![0];
    expect(props.data[0].at).toBeInstanceOf(Date);
  });

  it('renders Empty when no events', () => {
    render(<USGSQuakesScatter window="24h" data={[]} />);
    expect(screen.getByText(/no events in this window/i)).toBeTruthy();
  });
});
