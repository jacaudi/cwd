import { render, screen, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { HistoryChartRow } from './HistoryChartRow';

vi.mock('../api/history', () => ({
  fetchNWSAlertsHistory: vi.fn(),
  fetchSWPCScalesHistory: vi.fn(),
  fetchSWPCAlertsHistory: vi.fn(),
  fetchUSGSQuakesHistory: vi.fn(),
  fetchUSGSVolcanoesHistory: vi.fn(),
}));

vi.mock('./charts/NWSAlertsArea', () => ({
  NWSAlertsArea: ({ data }: { data: unknown[] }) => (
    <div data-testid="nws-area">{data.length}</div>
  ),
}));
vi.mock('./charts/SWPCScalesLine', () => ({ SWPCScalesLine: () => <div data-testid="swpc-scales" /> }));
vi.mock('./charts/SWPCAlertsBars', () => ({ SWPCAlertsBars: () => <div data-testid="swpc-alerts" /> }));
vi.mock('./charts/USGSQuakesScatter', () => ({ USGSQuakesScatter: () => <div data-testid="usgs-quakes" /> }));
vi.mock('./charts/VolcanoesTimeline', () => ({ VolcanoesTimeline: () => <div data-testid="volcanoes-timeline" /> }));

import { fetchNWSAlertsHistory } from '../api/history';

describe('HistoryChartRow', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('fetches the source on mount and renders the matching chart', async () => {
    (fetchNWSAlertsHistory as ReturnType<typeof vi.fn>).mockResolvedValue({
      source: 'nws_alerts',
      window: '24h',
      buckets: [{ at: '2026-05-03T10:00:00Z', activeCount: 12 }],
      windowStart: '2026-05-02T10:00:00Z',
      dataStart: '2026-05-02T10:00:00Z',
    });

    render(<HistoryChartRow source="nws_alerts" window="24h" />);
    await waitFor(() => expect(fetchNWSAlertsHistory).toHaveBeenCalledWith('24h'));
    expect(await screen.findByTestId('nws-area')).toBeTruthy();
  });

  it('re-fetches when window prop changes', async () => {
    (fetchNWSAlertsHistory as ReturnType<typeof vi.fn>).mockResolvedValue({
      source: 'nws_alerts', window: '24h', buckets: [],
      windowStart: '...', dataStart: '...',
    });
    const { rerender } = render(<HistoryChartRow source="nws_alerts" window="24h" />);
    await waitFor(() => expect(fetchNWSAlertsHistory).toHaveBeenCalledWith('24h'));
    rerender(<HistoryChartRow source="nws_alerts" window="7d" />);
    await waitFor(() => expect(fetchNWSAlertsHistory).toHaveBeenCalledWith('7d'));
  });

  it('renders the "data starts here" marker when dataStart > windowStart', async () => {
    (fetchNWSAlertsHistory as ReturnType<typeof vi.fn>).mockResolvedValue({
      source: 'nws_alerts', window: '30d',
      buckets: [{ at: '2026-05-01T10:00:00Z', activeCount: 1 }],
      windowStart: '2026-04-03T10:00:00Z',
      dataStart: '2026-05-01T10:00:00Z',
    });
    render(<HistoryChartRow source="nws_alerts" window="30d" />);
    await waitFor(() => expect(fetchNWSAlertsHistory).toHaveBeenCalled());
    expect(await screen.findByText(/data starts here/i)).toBeTruthy();
  });
});
