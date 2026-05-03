import { afterEach, describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import Overview from './Overview';
import { useSnapshotStore } from '../store/snapshot';

describe('Overview page', () => {
  afterEach(() => {
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

  it('renders badge bar after initial snapshot', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        serverTime: 't',
        sources: { nws_alerts: { source: 'nws_alerts', fetchedAt: 't', payload: [] } },
      }),
    }));
    class FakeES {
      addEventListener() {}
      close() {}
      onerror: any = null;
    }
    vi.stubGlobal('EventSource', FakeES as any);

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
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => ({ serverTime: 't', sources: {} }) }));
    class FakeES { addEventListener() {} close() {} onerror: any = null; }
    vi.stubGlobal('EventSource', FakeES as any);

    renderWithRouter();
    await screen.findByLabelText('Tsunami');
    // The Tag element (not the outer aria-labelled wrapper) carries the onClick handler.
    const tag = screen.getByText('Tsunami');
    fireEvent.click(tag);
    await waitFor(() => expect(screen.getByTestId('events-page')).toBeInTheDocument());
  });
});
