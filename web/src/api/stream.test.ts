import { afterEach, describe, it, expect, vi } from 'vitest';
import { connect } from './stream';

class FakeES {
  static last: FakeES | null = null;
  listeners: Record<string, ((e: any) => void)[]> = {};
  onerror: any = null;
  constructor(public url: string) { FakeES.last = this; }
  addEventListener(name: string, fn: (e: any) => void) {
    (this.listeners[name] ||= []).push(fn);
  }
  emit(name: string, data: any) {
    (this.listeners[name] || []).forEach((fn) => fn({ data: JSON.stringify(data) }));
  }
  close() {}
}

describe('stream client', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    FakeES.last = null;
  });

  it('fetches initial snapshot then routes SSE events', async () => {
    const fakeFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ serverTime: 't', sources: {} }),
    });
    vi.stubGlobal('fetch', fakeFetch);
    vi.stubGlobal('EventSource', FakeES as any);

    const events: any[] = [];
    await connect({
      onSnapshot: (e) => events.push({ type: 'snap', e }),
      onUpdate: (n, e) => events.push({ type: 'upd', n, e }),
    });

    expect(fakeFetch).toHaveBeenCalledWith('/api/snapshot');
    FakeES.last!.emit('nws_alerts.update', {
      source: 'nws_alerts',
      fetchedAt: 'b',
      payload: [],
    });
    expect(events.find((x) => x.type === 'upd' && x.n === 'nws_alerts')).toBeTruthy();
  });

  it('registers a listener for every source.update event', async () => {
    const fakeFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ serverTime: 't', sources: {} }),
    });
    vi.stubGlobal('fetch', fakeFetch);
    vi.stubGlobal('EventSource', FakeES as any);

    await connect({ onSnapshot: () => {}, onUpdate: () => {} });
    for (const name of [
      'nws_alerts',
      'swpc_scales',
      'swpc_alerts',
      'usgs_quakes',
      'usgs_volcanoes',
    ]) {
      expect(FakeES.last!.listeners[`${name}.update`]).toBeTruthy();
    }
  });

  it('registers a listener for image.invalidate.update', async () => {
    const fakeFetch = vi.fn().mockResolvedValue({
      ok: true, json: async () => ({ serverTime: 't', sources: {} }),
    });
    vi.stubGlobal('fetch', fakeFetch);
    vi.stubGlobal('EventSource', FakeES as any);
    await connect({
      onSnapshot: () => {},
      onUpdate: () => {},
      onImageInvalidate: () => {},
    });
    expect(FakeES.last!.listeners['image.invalidate.update']).toBeTruthy();
  });

  it('routes image.invalidate.update payloads to onImageInvalidate', async () => {
    const fakeFetch = vi.fn().mockResolvedValue({
      ok: true, json: async () => ({ serverTime: 't', sources: {} }),
    });
    vi.stubGlobal('fetch', fakeFetch);
    vi.stubGlobal('EventSource', FakeES as any);
    const seen: any[] = [];
    await connect({
      onSnapshot: () => {},
      onUpdate: () => {},
      onImageInvalidate: (ev) => seen.push(ev),
    });
    // simulate the SSE event using FakeES — call the registered listener.
    const ev = { data: JSON.stringify({ source: 'image.invalidate', fetchedAt: 't', payload: { source: 'spc', name: 'day1otlk', fetchedAt: 't' } }) };
    FakeES.last!.listeners['image.invalidate.update'][0](ev as any);
    expect(seen[0]).toEqual({ source: 'spc', name: 'day1otlk', fetchedAt: 't' });
  });
});
