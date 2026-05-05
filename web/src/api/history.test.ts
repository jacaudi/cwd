import { describe, it, expect, beforeEach, vi } from 'vitest';
import {
  fetchNWSAlertsHistory,
  fetchSWPCScalesHistory,
  fetchSWPCAlertsHistory,
  fetchUSGSQuakesHistory,
  fetchUSGSVolcanoesHistory,
  type HistoryWindow,
} from './history';

describe('history client', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('fetchNWSAlertsHistory composes the URL and parses the typed payload', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify({
      source: 'nws_alerts',
      window: '24h',
      buckets: [{ at: '2026-05-03T12:00:00Z', activeCount: 42 }],
      windowStart: '2026-05-02T12:00:00Z',
      dataStart: '2026-05-02T12:00:00Z',
    }), { status: 200 }));

    const got = await fetchNWSAlertsHistory('24h');
    expect(fetchSpy).toHaveBeenCalledWith('/api/history?source=nws_alerts&window=24h');
    expect(got.source).toBe('nws_alerts');
    expect(got.buckets).toHaveLength(1);
    expect(got.buckets[0].activeCount).toBe(42);
  });

  it('fetchUSGSQuakesHistory returns the events array shape', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify({
      source: 'usgs_quakes',
      window: '7d',
      events: [{ at: '2026-05-03T12:00:00Z', mag: 5.2, place: 'Alaska', depthKm: 12 }],
      windowStart: '2026-04-26T12:00:00Z',
      dataStart: '2026-04-26T12:00:00Z',
    }), { status: 200 }));

    const got = await fetchUSGSQuakesHistory('7d');
    expect(got.events).toHaveLength(1);
    expect(got.events[0].mag).toBe(5.2);
  });

  it('fetchUSGSVolcanoesHistory returns the changes array shape', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify({
      source: 'usgs_volcanoes',
      window: '30d',
      changes: [{ at: '2026-05-03T12:00:00Z', volcano: 'AVO Great Sitkin', prior: 'YELLOW', current: 'ORANGE' }],
      windowStart: '2026-04-03T12:00:00Z',
      dataStart: '2026-04-03T12:00:00Z',
    }), { status: 200 }));

    const got = await fetchUSGSVolcanoesHistory('30d');
    expect(got.changes).toHaveLength(1);
    expect(got.changes[0].volcano).toBe('AVO Great Sitkin');
  });

  it('throws on non-200 response', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response('bad', { status: 400 }));
    await expect(fetchNWSAlertsHistory('24h')).rejects.toThrow();
  });

  it('NWSAlertsHistory bucket includes eventCounts shape', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify({
      source: 'nws_alerts',
      window: '24h',
      buckets: [{
        at: '2026-05-05T12:00:00Z',
        activeCount: 42,
        eventCounts: { tornado: 1, severeTstorm: 2, flashFlood: 0 },
      }],
      windowStart: '2026-05-04T12:00:00Z',
      dataStart: '2026-05-04T12:00:00Z',
    }), { status: 200 }));

    const got = await fetchNWSAlertsHistory('24h');
    expect(got.buckets[0].eventCounts.tornado).toBe(1);
    expect(got.buckets[0].eventCounts.severeTstorm).toBe(2);
    expect(got.buckets[0].eventCounts.flashFlood).toBe(0);
  });

  it('exports the HistoryWindow union with the three documented values', () => {
    const windows: HistoryWindow[] = ['24h', '7d', '30d'];
    expect(windows).toEqual(['24h', '7d', '30d']);
    // Reference all five fetchers so this file documents the public surface.
    expect(typeof fetchSWPCScalesHistory).toBe('function');
    expect(typeof fetchSWPCAlertsHistory).toBe('function');
  });
});
