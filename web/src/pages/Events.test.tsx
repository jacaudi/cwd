import { afterEach, describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import Events from './Events';
import { useSnapshotStore } from '../store/snapshot';

afterEach(() => {
  useSnapshotStore.setState({ snapshot: null, connection: 'connecting' });
  vi.restoreAllMocks();
});

describe('Events page', () => {
  it('renders the page title even with no data', () => {
    render(<Events />);
    expect(screen.getByRole('heading', { name: /Events/i })).toBeInTheDocument();
  });

  it('exposes the tsunami anchor id for deep-link from Overview', () => {
    const { container } = render(<Events />);
    const anchor = container.querySelector('#tsunami');
    expect(anchor).not.toBeNull();
  });

  it('renders all three blocks when snapshot is fully populated', () => {
    useSnapshotStore.setState({
      snapshot: {
        serverTime: 't',
        sources: {
          nws_alerts: {
            source: 'nws_alerts', fetchedAt: 't',
            payload: [{
              id: 'tsu1', event: 'Tsunami Warning', awips: 'TSUWCA',
              headline: 'Tsunami Warning issued', severity: 'Extreme',
              sent: '2026-05-02T11:00:00Z', effective: '2026-05-02T11:00:00Z',
              expires: '2026-05-02T17:00:00Z', areas: ['Coastal CA'],
              category: 'Tsunami', wfo: 'NTW',
            }],
          },
          usgs_quakes: {
            source: 'usgs_quakes', fetchedAt: 't',
            payload: [{ id: 'q1', magnitude: 6.5, place: 'Ocean', time: '2026-05-02T10:00:00Z', updatedAt: 't', lat: 0, lon: 0, depthKm: 12, tsunami: false }],
          },
          usgs_volcanoes: {
            source: 'usgs_volcanoes', fetchedAt: 't',
            payload: [{ id: '311100', name: 'Redoubt', region: 'Alaska Volcano Observatory', alert: 'WATCH', color: 'ORANGE', updatedAt: '2026-05-02T10:00:00Z' }],
          },
        },
      },
      connection: 'live',
    });
    render(<Events />);
    expect(screen.getByText(/Tsunami Warning issued/)).toBeInTheDocument();
    expect(screen.getByText('Ocean')).toBeInTheDocument();
    expect(screen.getByText('Redoubt')).toBeInTheDocument();
  });

  it('hides quake + volcano blocks (returns null) when those sources are empty', () => {
    useSnapshotStore.setState({
      snapshot: {
        serverTime: 't',
        sources: {
          usgs_quakes:    { source: 'usgs_quakes',    fetchedAt: 't', payload: [] },
          usgs_volcanoes: { source: 'usgs_volcanoes', fetchedAt: 't', payload: [] },
        },
      },
      connection: 'live',
    });
    render(<Events />);
    expect(screen.queryByText(/Significant earthquakes/i)).toBeNull();
    expect(screen.queryByText(/Volcanoes at elevated alert/i)).toBeNull();
  });
});
