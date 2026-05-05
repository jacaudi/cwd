import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { NWSAlertsArea } from './NWSAlertsArea';

// Stub the Area chart component — it's heavy and we don't need to render the
// SVG to validate prop wiring.
vi.mock('@ant-design/charts', () => ({
  Area: ({ data }: { data: Array<{ at: string; activeCount: number }> }) => (
    <div data-testid="area-chart">{data.length} points</div>
  ),
}));

describe('NWSAlertsArea', () => {
  it('renders the area chart with the supplied buckets', () => {
    render(<NWSAlertsArea data={[
      { at: '2026-05-03T10:00:00Z', activeCount: 12 },
      { at: '2026-05-03T11:00:00Z', activeCount: 18 },
    ]} />);
    expect(screen.getByTestId('area-chart').textContent).toBe('2 points');
  });

  it('renders an "empty" placeholder when data is empty', () => {
    render(<NWSAlertsArea data={[]} />);
    expect(screen.queryByTestId('area-chart')).toBeNull();
    expect(screen.getByText(/no data in this window/i)).toBeTruthy();
  });
});
