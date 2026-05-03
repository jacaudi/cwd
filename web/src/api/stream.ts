import type { Snapshot, Envelope } from './types';
import type { SourcePayloadMap } from '../store/snapshot';

export interface ConnectOpts {
  onSnapshot: (s: Snapshot) => void;
  onUpdate: <K extends keyof SourcePayloadMap>(
    source: K,
    env: Envelope<SourcePayloadMap[K]>,
  ) => void;
  onError?: (e: unknown) => void;
}

const SOURCE_NAMES: (keyof SourcePayloadMap)[] = [
  'nws_alerts',
  'swpc_scales',
  'swpc_alerts',
  'usgs_quakes',
  'usgs_volcanoes',
];

/**
 * Connects the client to the live snapshot stream:
 * - fetches /api/snapshot for the initial paint
 * - subscribes to /api/stream for SSE updates (one listener per source)
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
  for (const name of SOURCE_NAMES) {
    es.addEventListener(`${name}.update`, (e: MessageEvent) => {
      try {
        opts.onUpdate(name, JSON.parse(e.data));
      } catch (err) {
        opts.onError?.(err);
      }
    });
  }
  return () => es.close();
}
