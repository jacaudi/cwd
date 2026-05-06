import { render, screen, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import type { ReactNode } from 'react';
import { HistoryChartRow } from './HistoryChartRow';

vi.mock('../api/history', () => ({
  fetchNWSAlertsHistory: vi.fn(),
  fetchSWPCScalesHistory: vi.fn(),
  fetchSWPCAlertsHistory: vi.fn(),
  fetchUSGSQuakesHistory: vi.fn(),
  fetchUSGSVolcanoesHistory: vi.fn(),
}));

// Passthrough mock for the chart-package ConfigProvider — set up by Task 8's
// page-level wrapping in pages/History.tsx. Not strictly needed by these
// tests today (they stub HistoryChartRow's chart subcomponents), but kept
// here so a future test that renders pages/History through the real wrapper
// doesn't have to reintroduce it.
vi.mock('@ant-design/charts', () => ({
  ConfigProvider: ({ children }: { children: ReactNode }) => <>{children}</>,
}));

vi.mock('./charts/NWSAlertsArea', () => ({ NWSAlertsArea: ({ data }: { data: unknown[] }) => <div data-testid="nws-area">{data.length}</div> }));
vi.mock('./charts/NWSEventCountsStrip', () => ({ NWSEventCountsStrip: ({ buckets }: { buckets: unknown[] }) => <div data-testid="nws-strip">{buckets.length}</div> }));
vi.mock('./charts/SWPCScalesLine', () => ({ SWPCScalesLine: () => <div data-testid="swpc-scales" /> }));
vi.mock('./charts/SWPCAlertsBars', () => ({ SWPCAlertsBars: () => <div data-testid="swpc-alerts" /> }));
vi.mock('./charts/USGSQuakesScatter', () => ({ USGSQuakesScatter: () => <div data-testid="usgs-quakes" /> }));
vi.mock('./charts/VolcanoesTimeline', () => ({ VolcanoesTimeline: () => <div data-testid="volcanoes-timeline" /> }));

// Mock the snapshot store so we can drive SSE-tick simulation.
let mockFetchedAt = '2026-05-05T12:00:00Z';
vi.mock('../store/snapshot', () => ({
  useSnapshotStore: (selector: (s: { snapshot: unknown }) => unknown) => {
    return selector({
      snapshot: {
        sources: {
          nws_alerts:     { fetchedAt: mockFetchedAt },
          swpc_scales:    { fetchedAt: mockFetchedAt },
          swpc_alerts:    { fetchedAt: mockFetchedAt },
          usgs_quakes:    { fetchedAt: mockFetchedAt },
          usgs_volcanoes: { fetchedAt: mockFetchedAt },
        },
      },
    });
  },
}));

import { fetchNWSAlertsHistory } from '../api/history';

const NWS_RESP = {
  source: 'nws_alerts' as const,
  window: '24h' as const,
  buckets: [
    { at: '2026-05-05T11:00:00Z', activeCount: 7,
      eventCounts: { tornado: 1, severeTstorm: 2, flashFlood: 0 } },
    { at: '2026-05-05T12:00:00Z', activeCount: 12,
      eventCounts: { tornado: 0, severeTstorm: 3, flashFlood: 1 } },
  ],
  windowStart: '2026-05-04T12:00:00Z',
  dataStart: '2026-05-04T12:00:00Z',
};

describe('HistoryChartRow', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFetchedAt = '2026-05-05T12:00:00Z';
  });

  it('fetches the source on mount and renders the matching chart', async () => {
    (fetchNWSAlertsHistory as ReturnType<typeof vi.fn>).mockResolvedValue(NWS_RESP);
    render(<HistoryChartRow source="nws_alerts" window="24h" />);
    await waitFor(() => expect(fetchNWSAlertsHistory).toHaveBeenCalledWith('24h'));
    expect(await screen.findByTestId('nws-area')).toBeTruthy();
  });

  it('renders the NWSEventCountsStrip for NWS only', async () => {
    (fetchNWSAlertsHistory as ReturnType<typeof vi.fn>).mockResolvedValue(NWS_RESP);
    render(<HistoryChartRow source="nws_alerts" window="24h" />);
    expect(await screen.findByTestId('nws-strip')).toBeTruthy();
  });

  it('renders the headline-stat tag with max activeCount in the window', async () => {
    (fetchNWSAlertsHistory as ReturnType<typeof vi.fn>).mockResolvedValue(NWS_RESP);
    render(<HistoryChartRow source="nws_alerts" window="24h" />);
    expect(await screen.findByText(/12 active/)).toBeTruthy();
  });

  it('re-fetches when the snapshot store fetchedAt advances (SSE live-tail)', async () => {
    (fetchNWSAlertsHistory as ReturnType<typeof vi.fn>).mockResolvedValue(NWS_RESP);
    const { rerender } = render(<HistoryChartRow source="nws_alerts" window="24h" />);
    await waitFor(() => expect(fetchNWSAlertsHistory).toHaveBeenCalledTimes(1));
    // Tick the SSE timestamp — re-render to flush the new selector value.
    mockFetchedAt = '2026-05-05T12:00:30Z';
    rerender(<HistoryChartRow source="nws_alerts" window="24h" />);
    await waitFor(() => expect(fetchNWSAlertsHistory).toHaveBeenCalledTimes(2), { timeout: 1000 });
  });

  it('re-fetches when window prop changes', async () => {
    (fetchNWSAlertsHistory as ReturnType<typeof vi.fn>).mockResolvedValue({
      ...NWS_RESP, window: '24h',
    });
    const { rerender } = render(<HistoryChartRow source="nws_alerts" window="24h" />);
    await waitFor(() => expect(fetchNWSAlertsHistory).toHaveBeenCalledWith('24h'));
    (fetchNWSAlertsHistory as ReturnType<typeof vi.fn>).mockResolvedValue({
      ...NWS_RESP, window: '7d',
    });
    rerender(<HistoryChartRow source="nws_alerts" window="7d" />);
    await waitFor(() => expect(fetchNWSAlertsHistory).toHaveBeenCalledWith('7d'));
  });

  it('renders SWPC alerts headline with window-aware label ("N in <window>")', async () => {
    const { fetchSWPCAlertsHistory } = await import('../api/history');
    (fetchSWPCAlertsHistory as ReturnType<typeof vi.fn>).mockResolvedValue({
      source: 'swpc_alerts' as const,
      window: '24h' as const,
      buckets: [
        { at: '2026-05-05T11:00:00Z', warning: 2, watch: 3, alert: 1 },
        { at: '2026-05-05T12:00:00Z', warning: 1, watch: 4, alert: 1 },
      ],
      windowStart: '2026-05-04T12:00:00Z',
      dataStart: '2026-05-04T12:00:00Z',
    });
    render(<HistoryChartRow source="swpc_alerts" window="24h" />);
    expect(await screen.findByText(/12 in 24h/)).toBeTruthy();
  });

  it('renders volcanoes headline with "N changes" wording', async () => {
    const { fetchUSGSVolcanoesHistory } = await import('../api/history');
    (fetchUSGSVolcanoesHistory as ReturnType<typeof vi.fn>).mockResolvedValue({
      source: 'usgs_volcanoes' as const,
      window: '24h' as const,
      changes: [
        { at: '2026-05-05T11:00:00Z', name: 'Mount A', from: 'green', to: 'yellow' },
        { at: '2026-05-05T12:00:00Z', name: 'Mount B', from: 'yellow', to: 'orange' },
      ],
      windowStart: '2026-05-04T12:00:00Z',
      dataStart: '2026-05-04T12:00:00Z',
    });
    render(<HistoryChartRow source="usgs_volcanoes" window="24h" />);
    expect(await screen.findByText(/2 changes/)).toBeTruthy();
  });

  it('renders the "data starts here" marker when dataStart > windowStart', async () => {
    (fetchNWSAlertsHistory as ReturnType<typeof vi.fn>).mockResolvedValue({
      ...NWS_RESP,
      window: '30d' as const,
      windowStart: '2026-04-03T10:00:00Z',
      dataStart: '2026-05-01T10:00:00Z',
    });
    render(<HistoryChartRow source="nws_alerts" window="30d" />);
    await waitFor(() => expect(fetchNWSAlertsHistory).toHaveBeenCalled());
    expect(await screen.findByText(/data starts here/i)).toBeTruthy();
  });
});
