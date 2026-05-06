import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, vi } from 'vitest';
import History from './History';

// Stub HistoryChartRow so we can assert page composition without dragging
// in chart libs. The recording mock for ChartsConfigProvider below records
// what `common.theme.type` was passed to it on render.
vi.mock('../components/HistoryChartRow', () => ({
  HistoryChartRow: ({ source, window }: { source: string; window: string }) => (
    <div data-testid={`row-${source}`}>{window}</div>
  ),
}));

// Recording mock — captures the most recent ChartsConfigProvider props.
let recordedThemeType: string | undefined;
vi.mock('@ant-design/charts', () => ({
  ConfigProvider: (props: {
    common?: { theme?: { type?: string } };
    children: React.ReactNode;
  }) => {
    recordedThemeType = props.common?.theme?.type;
    return <>{props.children}</>;
  },
}));

// Force the dark branch for the wrapping assertion.
vi.mock('../theme', () => ({ useIsDark: () => true }));

describe('History page', () => {
  it('renders all 5 rows', async () => {
    render(<MemoryRouter><History /></MemoryRouter>);
    await waitFor(() => {
      expect(screen.getByTestId('row-nws_alerts')).toBeTruthy();
      expect(screen.getByTestId('row-swpc_scales')).toBeTruthy();
      expect(screen.getByTestId('row-swpc_alerts')).toBeTruthy();
      expect(screen.getByTestId('row-usgs_quakes')).toBeTruthy();
      expect(screen.getByTestId('row-usgs_volcanoes')).toBeTruthy();
    });
  });

  it('defaults to 24h when no ?w= param', async () => {
    render(<MemoryRouter><History /></MemoryRouter>);
    await waitFor(() => {
      expect(screen.getByTestId('row-nws_alerts').textContent).toBe('24h');
    });
  });

  it('reads the initial window from ?w= when present', async () => {
    render(
      <MemoryRouter initialEntries={['/history?w=7d']}>
        <History />
      </MemoryRouter>,
    );
    await waitFor(() => {
      expect(screen.getByTestId('row-nws_alerts').textContent).toBe('7d');
    });
  });

  it('wraps the row map in ChartsConfigProvider with common.theme.type matching useIsDark', async () => {
    recordedThemeType = undefined;
    render(<MemoryRouter><History /></MemoryRouter>);
    // The wrapper renders synchronously on mount; recordedThemeType should
    // be set by the time the rows show up.
    await waitFor(() => {
      expect(screen.getByTestId('row-nws_alerts')).toBeTruthy();
    });
    expect(recordedThemeType).toBe('dark');
  });
});
