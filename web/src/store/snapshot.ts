import { create } from 'zustand';
import type {
  Envelope,
  Alert,
  ImageInvalidate,
  Quake,
  SWPCAlert,
  SWPCForecast,
  Snapshot,
  Volcano,
} from '../api/types';

export type Connection = 'connecting' | 'live' | 'polling' | 'error';

// SourcePayloadMap maps each source key to its envelope payload type. The
// stream client + store both narrow against this so a misrouted update
// (e.g. quake payload routed to nws_alerts) is a TS compile error.
export interface SourcePayloadMap {
  nws_alerts:     Alert[];
  swpc_scales:    SWPCForecast;
  swpc_alerts:    SWPCAlert[];
  usgs_quakes:    Quake[];
  usgs_volcanoes: Volcano[];
}

interface State {
  snapshot: Snapshot | null;
  connection: Connection;
  // imageRefresh maps "<source>.<name>" → fetchedAt ISO string. Components
  // key their <Image> element on this so SSE-driven invalidations force a
  // browser-side reload (React unmount-remount of <img>).
  imageRefresh: Record<string, string>;
  setSnapshot: (s: Snapshot) => void;
  applyUpdate: <K extends keyof SourcePayloadMap>(
    source: K,
    env: Envelope<SourcePayloadMap[K]>,
  ) => void;
  applyImageInvalidate: (ev: ImageInvalidate) => void;
  setConnection: (c: Connection) => void;
}

export const useSnapshotStore = create<State>((set) => ({
  snapshot: null,
  connection: 'connecting',
  imageRefresh: {},
  setSnapshot: (s) => set((prev) => {
    // Preserve previously accumulated invalidates so an SSE reconnect (which
    // re-delivers the cached snapshot) does not wipe per-image refresh state.
    const nextRefresh: Record<string, string> = { ...prev.imageRefresh };
    const inv = s.sources['image.invalidate'];
    if (inv) {
      const ev = inv.payload;
      nextRefresh[`${ev.source}.${ev.name}`] = ev.fetchedAt;
    }
    return { snapshot: s, imageRefresh: nextRefresh };
  }),
  applyUpdate: (source, env) =>
    set((prev) => {
      const base: Snapshot =
        prev.snapshot ?? { serverTime: env.fetchedAt, sources: {} };
      return {
        snapshot: {
          ...base,
          sources: { ...base.sources, [source]: env },
        } as Snapshot,
      };
    }),
  applyImageInvalidate: (ev) =>
    set((prev) => ({
      imageRefresh: { ...prev.imageRefresh, [`${ev.source}.${ev.name}`]: ev.fetchedAt },
    })),
  setConnection: (c) => set({ connection: c }),
}));
