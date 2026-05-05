import { create } from 'zustand';
import type { HistoryWindow } from '../api/history';

interface HistoryState {
  window: HistoryWindow;
  setWindow: (w: HistoryWindow) => void;
  /**
   * Parse the ?w= query parameter from a URL search string. Returns the
   * window if it's one of the three valid values, otherwise null. Does not
   * touch state — call sites decide whether to seed the store with the
   * parsed value (typically once at History.tsx mount time).
   */
  parseWindowFromURL: (search: string) => HistoryWindow | null;
}

const VALID: ReadonlyArray<HistoryWindow> = ['24h', '7d', '30d'];

export const useHistoryStore = create<HistoryState>((set) => ({
  window: '24h',
  setWindow: (w) => set({ window: w }),
  parseWindowFromURL: (search) => {
    const params = new URLSearchParams(search);
    const v = params.get('w');
    if (!v) return null;
    if ((VALID as readonly string[]).includes(v)) return v as HistoryWindow;
    return null;
  },
}));
