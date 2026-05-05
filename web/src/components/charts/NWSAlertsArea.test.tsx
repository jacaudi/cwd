import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { NWSAlertsArea } from './NWSAlertsArea';

const mockArea = vi.fn();
vi.mock('@ant-design/charts', () => ({
  Area: (props: { data: Array<{ at: Date; activeCount: number }>; theme?: string }) => {
    mockArea(props);
    return <div data-testid="area-chart">{props.data.length} points (theme={props.theme ?? 'unset'})</div>;
  },
}));

vi.mock('../../theme', () => ({ useIsDark: () => true }));

describe('NWSAlertsArea', () => {
  it('renders the area chart with the supplied buckets and dark theme', () => {
    render(<NWSAlertsArea
      window="24h"
      data={[
        { at: '2026-05-03T10:00:00Z', activeCount: 12 },
        { at: '2026-05-03T11:00:00Z', activeCount: 18 },
      ]}
    />);
    expect(screen.getByTestId('area-chart').textContent).toContain('2 points');
    expect(screen.getByTestId('area-chart').textContent).toContain('theme=academy');
    // data is converted to Date for the time axis
    const props = mockArea.mock.calls.at(-1)![0];
    expect(props.data[0].at).toBeInstanceOf(Date);
  });

  it('renders an "empty" placeholder when data is empty', () => {
    render(<NWSAlertsArea window="24h" data={[]} />);
    expect(screen.queryByTestId('area-chart')).toBeNull();
    expect(screen.getByText(/no data in this window/i)).toBeTruthy();
  });
});
