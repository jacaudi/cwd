import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { SWPCAlertsBars } from './SWPCAlertsBars';

const mockColumn = vi.fn();
vi.mock('@ant-design/charts', () => ({
  Column: (props: { data: Array<{ at: Date; severity: string; count: number }>; theme?: string }) => {
    mockColumn(props);
    return <div data-testid="column-chart">{props.data.length} points (theme={props.theme ?? 'unset'})</div>;
  },
}));

vi.mock('../../theme', () => ({ useIsDark: () => true }));

describe('SWPCAlertsBars', () => {
  it('flattens the buckets into per-severity rows (warning/watch/alert) with dark theme', () => {
    render(<SWPCAlertsBars
      window="24h"
      data={[
        { at: '2026-05-03T10:00:00Z', warning: 1, watch: 2, alert: 4 },
        { at: '2026-05-03T11:00:00Z', warning: 0, watch: 1, alert: 3 },
      ]}
    />);
    // 2 buckets * 3 severities = 6 rows
    expect(screen.getByTestId('column-chart').textContent).toContain('6 points');
    expect(screen.getByTestId('column-chart').textContent).toContain('theme=academy');
    const props = mockColumn.mock.calls.at(-1)![0];
    expect(props.data[0].at).toBeInstanceOf(Date);
  });

  it('renders Empty when data is empty', () => {
    render(<SWPCAlertsBars window="24h" data={[]} />);
    expect(screen.getByText(/no data in this window/i)).toBeTruthy();
  });
});
