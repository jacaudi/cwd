import { afterEach, describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import SpaceWeather from './SpaceWeather';
import { useSnapshotStore } from '../store/snapshot';

afterEach(() => {
  useSnapshotStore.setState({ snapshot: null, connection: 'connecting' });
  vi.restoreAllMocks();
});

describe('SpaceWeather page', () => {
  it('renders skeleton forecast cards when snapshot is null', () => {
    render(<SpaceWeather />);
    expect(screen.getAllByLabelText(/forecast-skeleton/i).length).toBe(3);
  });

  it('renders forecast cards + alerts list when snapshot is populated', () => {
    useSnapshotStore.setState({
      snapshot: {
        serverTime: 't',
        sources: {
          swpc_scales: {
            source: 'swpc_scales', fetchedAt: '2026-05-02T12:00:00Z',
            payload: {
              days: [
                { date: '2026-05-03', r1: 5,  r3: 1, s1: 1 },
                { date: '2026-05-04', r1: 30, r3: 5, s1: 5, g: 'G4', gText: 'G4 watch' },
                { date: '2026-05-05', r1: 60, r3: 25, s1: 80, g: 'G5' },
              ],
            },
          },
          swpc_alerts: {
            source: 'swpc_alerts', fetchedAt: '2026-05-02T12:00:00Z',
            payload: [
              { code: 'K08A', series: 'K-Index', description: 'K=8 = G4', issued: '2026-05-02T11:00:00Z', message: 'event observed.' },
            ],
          },
        },
      },
      connection: 'live',
    });
    render(<SpaceWeather />);
    expect(screen.getByText('2026-05-03')).toBeInTheDocument();
    expect(screen.getByText('K08A')).toBeInTheDocument();
    expect(screen.getByText(/K-Index/)).toBeInTheDocument();
  });

  it('shows Empty state for the alerts block when alerts are empty', () => {
    useSnapshotStore.setState({
      snapshot: {
        serverTime: 't',
        sources: {
          swpc_scales: {
            source: 'swpc_scales', fetchedAt: '2026-05-02T12:00:00Z',
            payload: {
              days: [
                { date: '2026-05-03', r1: 5, r3: 0, s1: 0 },
                { date: '2026-05-04', r1: 5, r3: 0, s1: 0 },
                { date: '2026-05-05', r1: 5, r3: 0, s1: 0 },
              ],
            },
          },
          swpc_alerts: {
            source: 'swpc_alerts', fetchedAt: '2026-05-02T12:00:00Z',
            payload: [],
          },
        },
      },
      connection: 'live',
    });
    render(<SpaceWeather />);
    expect(screen.getByText(/No active SWPC alerts/i)).toBeInTheDocument();
  });
});
