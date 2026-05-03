import { create } from 'zustand';
import type {
  Envelope,
  Alert,
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
  setSnapshot: (s: Snapshot) => void;
  applyUpdate: <K extends keyof SourcePayloadMap>(
    source: K,
    env: Envelope<SourcePayloadMap[K]>,
  ) => void;
  setConnection: (c: Connection) => void;
}

export const useSnapshotStore = create<State>((set) => ({
  snapshot: null,
  connection: 'connecting',
  setSnapshot: (s) => set({ snapshot: s }),
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
  setConnection: (c) => set({ connection: c }),
}));
