export type ThemeMode = "dark" | "light" | "auto";

export interface UIConfig {
  defaultTheme: ThemeMode;
  defaultLanding: string;
  enableHistory: boolean;
}

export interface VersionInfo {
  version: string;
  commit: string;
  date: string;
  go: string;
}

// ── Phase 1: NWS Alerts wire types ────────────────────────────────────────

export type Severity = 'Extreme' | 'Severe' | 'Moderate' | 'Minor' | 'Unknown';

export type Category =
  | 'Tornado' | 'SevereThunderstorm' | 'FlashFlood' | 'Tropical'
  | 'HighWind' | 'RedFlag' | 'Winter' | 'ExtremeHeat' | 'ExtremeCold'
  | 'Tsunami' | 'Unknown';

export interface Alert {
  id: string;
  event: string;
  awips: string;
  headline: string;
  severity: Severity;
  sent: string;       // RFC3339
  effective: string;
  onset?: string;
  expires: string;
  ends?: string;
  areas: string[];
  ugcs?: string[];
  sames?: string[];
  wfo?: string;
  vtecEtn?: string;
  url?: string;
  category: Category;
}

// ── Phase 2: SWPC + USGS wire types ───────────────────────────────────────

export type GScale = 'G1' | 'G2' | 'G3' | 'G4' | 'G5';

export interface SWPCDay {
  date: string;
  r1: number;
  r3: number;
  s1: number;
  g?: GScale;
  gText?: string;
}

export interface SWPCForecast {
  days: [SWPCDay, SWPCDay, SWPCDay];
}

export interface SWPCAlert {
  code: string;
  series: string;
  description: string;
  issued: string;
  message: string;
  url?: string;
}

export interface Quake {
  id: string;
  magnitude: number;
  place: string;
  time: string;
  updatedAt: string;
  lat: number;
  lon: number;
  depthKm: number;
  tsunami: boolean;
  alert?: string;
  url?: string;
}

export type AlertLevel = 'NORMAL' | 'ADVISORY' | 'WATCH' | 'WARNING';
export type ColorCode  = 'GREEN'  | 'YELLOW'   | 'ORANGE' | 'RED';

// Volcano shape verified against live getElevatedVolcanoes 2026-05-02.
// Design §6.4 also listed lat/lon/synopsis but the upstream does not return
// them on this endpoint; see Task 8 plan body for the deviation rationale.
export interface Volcano {
  id: string;
  name: string;
  region: string;       // derived from upstream obs_fullname
  alert: AlertLevel;
  color: ColorCode;
  updatedAt: string;
  url?: string;
}

export interface Envelope<T> {
  source: string;
  fetchedAt: string;
  etag?: string;
  payload: T;
}

export interface Snapshot {
  serverTime: string;
  sources: {
    nws_alerts?:     Envelope<Alert[]>;
    swpc_scales?:    Envelope<SWPCForecast>;
    swpc_alerts?:    Envelope<SWPCAlert[]>;
    usgs_quakes?:    Envelope<Quake[]>;
    usgs_volcanoes?: Envelope<Volcano[]>;
  };
}

export interface SourceHealth {
  intervalSec: number;
  lastAttempt: string;
  lastSuccess: string;
  lastError?: string;
  etag?: string;
  ageSec: number;
  consecutiveFailures: number;
}
