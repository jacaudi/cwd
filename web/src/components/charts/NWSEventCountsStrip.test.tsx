import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { NWSEventCountsStrip } from './NWSEventCountsStrip';
import type { NWSAlertsHistory } from '../../api/history';

const mockTinyArea = vi.fn();
vi.mock('@ant-design/charts', () => ({
  Tiny: {
    Area: (props: { data: number[] }) => {
      mockTinyArea(props);
      return <div data-testid="sparkline">{props.data.length} pts</div>;
    },
  },
}));

const sample: NWSAlertsHistory['buckets'] = [
  { at: '2026-05-05T12:00:00Z', activeCount: 5,
    eventCounts: { tornado: 0, severeTstorm: 1, flashFlood: 2 } },
  { at: '2026-05-05T12:05:00Z', activeCount: 7,
    eventCounts: { tornado: 1, severeTstorm: 2, flashFlood: 0 } },
  { at: '2026-05-05T12:10:00Z', activeCount: 9,
    eventCounts: { tornado: 2, severeTstorm: 4, flashFlood: 1 } },
];

describe('NWSEventCountsStrip', () => {
  it('renders three cards with the latest counts', () => {
    render(<NWSEventCountsStrip buckets={sample} />);
    expect(screen.getByText(/Tornado/i)).toBeTruthy();
    expect(screen.getByText(/Severe Thunderstorm/i)).toBeTruthy();
    expect(screen.getByText(/Flash Flood/i)).toBeTruthy();
    // Latest bucket counts are 2 / 4 / 1
    expect(screen.getByTestId('count-tornado').textContent).toBe('2');
    expect(screen.getByTestId('count-severeTstorm').textContent).toBe('4');
    expect(screen.getByTestId('count-flashFlood').textContent).toBe('1');
  });

  it('renders three sparklines, each fed the bucket count history for its category', () => {
    render(<NWSEventCountsStrip buckets={sample} />);
    const sparks = screen.getAllByTestId('sparkline');
    expect(sparks).toHaveLength(3);
    sparks.forEach((s) => expect(s.textContent).toBe('3 pts'));
  });

  it('renders zeros and an empty-style card when buckets is empty', () => {
    render(<NWSEventCountsStrip buckets={[]} />);
    expect(screen.getByTestId('count-tornado').textContent).toBe('0');
    expect(screen.getByTestId('count-severeTstorm').textContent).toBe('0');
    expect(screen.getByTestId('count-flashFlood').textContent).toBe('0');
  });

  it('does NOT pass a theme prop on the sparkline (handled by page-level ChartsConfigProvider)', () => {
    render(<NWSEventCountsStrip buckets={sample} />);
    const props = mockTinyArea.mock.calls.at(-1)![0] as Record<string, unknown>;
    expect(props.theme).toBeUndefined();
  });
});
