import { afterEach, describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import Overview from './Overview';
import { useSnapshotStore } from '../store/snapshot';

// Spy on stream.connect so we can assert Overview does NOT open its own
// EventSource — the lifecycle is owned by App after the Phase 2 hoist.
const connectMock = vi.fn(async (_opts: unknown) => () => {});
vi.mock('../api/stream', () => ({
  connect: (opts: unknown) => connectMock(opts),
}));

describe('Overview page', () => {
  afterEach(() => {
    cleanup();
    connectMock.mockClear();
    vi.restoreAllMocks();
    useSnapshotStore.setState({ snapshot: null, connection: 'connecting' });
  });

  function renderWithRouter() {
    return render(
      <MemoryRouter initialEntries={['/']}>
        <Routes>
          <Route path="/" element={<Overview />} />
          <Route path="/events" element={<div data-testid="events-page" />} />
        </Routes>
      </MemoryRouter>,
    );
  }

  it('does not call stream.connect — the SSE lifecycle is owned by App', async () => {
    renderWithRouter();
    // Give React a tick; the connect MUST NOT be invoked from Overview.
    await new Promise((r) => setTimeout(r, 10));
    expect(connectMock).not.toHaveBeenCalled();
  });

  it('renders badge bar from the shared snapshot store', async () => {
    useSnapshotStore.setState({
      snapshot: {
        serverTime: 't',
        sources: { nws_alerts: { source: 'nws_alerts', fetchedAt: 't', payload: [] } },
      },
      connection: 'live',
    });
    renderWithRouter();
    await waitFor(() => expect(screen.getByLabelText('Tornado')).toBeInTheDocument());
  });

  it('navigates to /events#tsunami when Tsunami badge is clicked with non-zero count', async () => {
    useSnapshotStore.setState({
      snapshot: {
        serverTime: 't',
        sources: {
          nws_alerts: {
            source: 'nws_alerts', fetchedAt: 't',
            payload: [{
              id: 'tsu1', event: 'Tsunami Warning', awips: 'TSUWCA',
              headline: 'Tsunami', severity: 'Extreme',
              sent: 't', effective: 't', expires: 't', areas: [], category: 'Tsunami',
            }],
          },
        },
      },
      connection: 'live',
    });

    renderWithRouter();
    await screen.findByLabelText('Tsunami');
    // The Tag element (not the outer aria-labelled wrapper) carries the onClick handler.
    const tag = screen.getByText('Tsunami');
    fireEvent.click(tag);
    await waitFor(() => expect(screen.getByTestId('events-page')).toBeInTheDocument());
  });
});
