// Typed client for GET /api/history. One function per source so call sites get
// autocomplete for the source-specific response shape.

export type HistoryWindow = '24h' | '7d' | '30d';

interface HistoryEnvelope<S extends string> {
  source: S;
  window: HistoryWindow;
  windowStart: string;  // ISO-8601 UTC
  dataStart: string;    // ISO-8601 UTC; earliest available snapshot
}

export interface NWSAlertsHistory extends HistoryEnvelope<'nws_alerts'> {
  buckets: Array<{ at: string; activeCount: number }>;
}

export interface SWPCScalesHistory extends HistoryEnvelope<'swpc_scales'> {
  buckets: Array<{ at: string; gScale: number; r1: number; s1: number }>;
}

export interface SWPCAlertsHistory extends HistoryEnvelope<'swpc_alerts'> {
  buckets: Array<{ at: string; warning: number; watch: number; alert: number }>;
}

export interface USGSQuakesHistory extends HistoryEnvelope<'usgs_quakes'> {
  events: Array<{ at: string; mag: number; place: string; depthKm: number }>;
}

export interface USGSVolcanoesHistory extends HistoryEnvelope<'usgs_volcanoes'> {
  changes: Array<{ at: string; volcano: string; prior: string; current: string }>;
}

async function fetchHistory<T>(source: string, window: HistoryWindow): Promise<T> {
  const res = await fetch(`/api/history?source=${source}&window=${window}`);
  if (!res.ok) {
    throw new Error(`GET /api/history?source=${source}&window=${window}: ${res.status}`);
  }
  return (await res.json()) as T;
}

export const fetchNWSAlertsHistory = (w: HistoryWindow) =>
  fetchHistory<NWSAlertsHistory>('nws_alerts', w);
export const fetchSWPCScalesHistory = (w: HistoryWindow) =>
  fetchHistory<SWPCScalesHistory>('swpc_scales', w);
export const fetchSWPCAlertsHistory = (w: HistoryWindow) =>
  fetchHistory<SWPCAlertsHistory>('swpc_alerts', w);
export const fetchUSGSQuakesHistory = (w: HistoryWindow) =>
  fetchHistory<USGSQuakesHistory>('usgs_quakes', w);
export const fetchUSGSVolcanoesHistory = (w: HistoryWindow) =>
  fetchHistory<USGSVolcanoesHistory>('usgs_volcanoes', w);
