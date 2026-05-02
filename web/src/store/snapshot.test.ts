import { beforeEach, describe, it, expect } from 'vitest';
import { useSnapshotStore } from './snapshot';

describe('snapshot store', () => {
  beforeEach(() => {
    // Reset store state between tests.
    useSnapshotStore.setState({ snapshot: null, connection: 'connecting' });
  });

  it('starts empty', () => {
    const s = useSnapshotStore.getState();
    expect(s.snapshot).toBeNull();
    expect(s.connection).toBe('connecting');
  });

  it('replaces snapshot on setSnapshot', () => {
    useSnapshotStore.getState().setSnapshot({
      serverTime: '2026-05-02T12:00:00Z',
      sources: {
        nws_alerts: { source: 'nws_alerts', fetchedAt: 'a', payload: [] },
      },
    });
    expect(useSnapshotStore.getState().snapshot?.sources.nws_alerts?.payload).toEqual([]);
  });

  it('updates one source on applyUpdate', () => {
    useSnapshotStore.getState().setSnapshot({
      serverTime: '2026-05-02T12:00:00Z',
      sources: {
        nws_alerts: { source: 'nws_alerts', fetchedAt: 'a', payload: [] },
      },
    });
    useSnapshotStore.getState().applyUpdate('nws_alerts', {
      source: 'nws_alerts',
      fetchedAt: 'b',
      payload: [
        {
          id: '1',
          event: '',
          awips: '',
          headline: '',
          severity: 'Unknown',
          sent: '',
          effective: '',
          expires: '',
          areas: [],
          category: 'Unknown',
        },
      ],
    });
    const env = useSnapshotStore.getState().snapshot?.sources.nws_alerts;
    expect(env?.fetchedAt).toBe('b');
    expect(env?.payload.length).toBe(1);
  });
});
