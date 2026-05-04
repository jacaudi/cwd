import type { Snapshot, Envelope, ImageInvalidate } from './types';
import type { SourcePayloadMap } from '../store/snapshot';

export interface ConnectOpts {
  onSnapshot: (s: Snapshot) => void;
  onUpdate: <K extends keyof SourcePayloadMap>(
    source: K,
    env: Envelope<SourcePayloadMap[K]>,
  ) => void;
  onImageInvalidate?: (ev: ImageInvalidate) => void;
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
 * - subscribes to image.invalidate.update for image proxy invalidations
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
  es.addEventListener('image.invalidate.update', (e: MessageEvent) => {
    try {
      const env = JSON.parse(e.data) as Envelope<ImageInvalidate>;
      opts.onImageInvalidate?.(env.payload);
    } catch (err) {
      opts.onError?.(err);
    }
  });
  return () => es.close();
}
