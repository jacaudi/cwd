import { afterEach, describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import Overview from './Overview';
import { useSnapshotStore } from '../store/snapshot';

describe('Overview page', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    useSnapshotStore.setState({ snapshot: null, connection: 'connecting' });
  });

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

    render(<Overview />);
    await waitFor(() => expect(screen.getByLabelText('Tornado')).toBeInTheDocument());
  });
});
