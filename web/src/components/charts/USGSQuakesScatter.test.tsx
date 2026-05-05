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
