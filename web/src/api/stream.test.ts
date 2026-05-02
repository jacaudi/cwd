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
});
