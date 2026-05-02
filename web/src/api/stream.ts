import type { Snapshot, Envelope, Alert } from './types';

export interface ConnectOpts {
  onSnapshot: (s: Snapshot) => void;
  onUpdate: (source: keyof Snapshot['sources'], env: Envelope<Alert[]>) => void;
  onError?: (e: unknown) => void;
}

/**
 * Connects the client to the live snapshot stream:
 * - fetches /api/snapshot for the initial paint
 * - subscribes to /api/stream for SSE updates
 * Returns a teardown function that closes the EventSource.
 */
export async function connect(opts: ConnectOpts): Promise<() => void> {
  try {
    const res = await fetch('/api/snapshot');
    if (res.ok) opts.onSnapshot(await res.json());
  } catch (e) {
    opts.onError?.(e);
  }

  const es = new EventSource('/api/stream');
  es.addEventListener('snapshot', (e: MessageEvent) => {
    try {
      opts.onSnapshot(JSON.parse(e.data));
    } catch (err) {
      opts.onError?.(err);
    }
  });
  es.addEventListener('nws_alerts.update', (e: MessageEvent) => {
    try {
      opts.onUpdate('nws_alerts', JSON.parse(e.data));
    } catch (err) {
      opts.onError?.(err);
    }
  });
  return () => es.close();
}
