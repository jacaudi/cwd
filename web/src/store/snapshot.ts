import { create } from 'zustand';
import type { Envelope, Alert, Snapshot } from '../api/types';

export type Connection = 'connecting' | 'live' | 'polling' | 'error';

interface State {
  snapshot: Snapshot | null;
  connection: Connection;
  setSnapshot: (s: Snapshot) => void;
  applyUpdate: (source: keyof Snapshot['sources'], env: Envelope<Alert[]>) => void;
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
        },
      };
    }),
  setConnection: (c) => set({ connection: c }),
}));
