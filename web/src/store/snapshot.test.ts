import { beforeEach, describe, it, expect } from 'vitest';
import { useSnapshotStore } from './snapshot';

describe('snapshot store', () => {
  beforeEach(() => {
    // Reset store state between tests.
    useSnapshotStore.setState({ snapshot: null, connection: 'connecting', imageRefresh: {} });
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

  it('routes updates by source key', () => {
    useSnapshotStore.getState().setSnapshot({ serverTime: 't', sources: {} });
    useSnapshotStore.getState().applyUpdate('swpc_scales', {
      source: 'swpc_scales',
      fetchedAt: 't',
      payload: {
        days: [
          { date: '2026-05-03', r1: 5, r3: 0, s1: 0 },
          { date: '2026-05-04', r1: 5, r3: 0, s1: 0 },
          { date: '2026-05-05', r1: 5, r3: 0, s1: 0 },
        ],
      },
    });
    useSnapshotStore.getState().applyUpdate('usgs_quakes', {
      source: 'usgs_quakes',
      fetchedAt: 't',
      payload: [
        {
          id: 'q1',
          magnitude: 6.0,
          place: 'X',
          time: 't',
          updatedAt: 't',
          lat: 0,
          lon: 0,
          depthKm: 5,
          tsunami: false,
        },
      ],
    });
    useSnapshotStore.getState().applyUpdate('swpc_alerts', {
      source: 'swpc_alerts',
      fetchedAt: 't',
      payload: [
        {
          code: 'K07',
          series: 'ALERT',
          description: 'Geomagnetic K=7',
          issued: 't',
          message: 'msg',
        },
      ],
    });
    useSnapshotStore.getState().applyUpdate('usgs_volcanoes', {
      source: 'usgs_volcanoes',
      fetchedAt: 't',
      payload: [
        {
          id: 'v1',
          name: 'Kilauea',
          region: 'Hawaii',
          alert: 'WATCH',
          color: 'ORANGE',
          updatedAt: 't',
        },
      ],
    });
    const s = useSnapshotStore.getState().snapshot!;
    expect(s.sources.swpc_scales?.payload.days[0].date).toBe('2026-05-03');
    expect(s.sources.usgs_quakes?.payload[0].id).toBe('q1');
    expect(s.sources.swpc_alerts?.payload[0].code).toBe('K07');
    expect(s.sources.usgs_volcanoes?.payload[0].alert).toBe('WATCH');
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

  it('routes image.invalidate events into imageRefresh', () => {
    useSnapshotStore.setState({ snapshot: { serverTime: 't', sources: {} }, connection: 'live', imageRefresh: {} });
    useSnapshotStore.getState().applyImageInvalidate({
      source: 'spc', name: 'day1otlk', fetchedAt: '2026-05-03T22:14:33Z',
    });
    useSnapshotStore.getState().applyImageInvalidate({
      source: 'nhc', name: 'atl_7d', fetchedAt: '2026-05-03T22:14:34Z',
    });
    const refresh = useSnapshotStore.getState().imageRefresh;
    expect(refresh['spc.day1otlk']).toBe('2026-05-03T22:14:33Z');
    expect(refresh['nhc.atl_7d']).toBe('2026-05-03T22:14:34Z');
  });

  it('seeds imageRefresh from snapshot.sources["image.invalidate"]', () => {
    useSnapshotStore.getState().setSnapshot({
      serverTime: 't',
      sources: {
        'image.invalidate': {
          source: 'image.invalidate', fetchedAt: 't',
          payload: { source: 'spc', name: 'day1otlk', fetchedAt: '2026-05-03T22:14:33Z' },
        },
      },
    });
    expect(useSnapshotStore.getState().imageRefresh['spc.day1otlk']).toBe('2026-05-03T22:14:33Z');
  });
});
