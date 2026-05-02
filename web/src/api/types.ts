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

export interface Envelope<T> {
  source: string;
  fetchedAt: string;
  etag?: string;
  payload: T;
}

export interface Snapshot {
  serverTime: string;
  sources: {
    nws_alerts?: Envelope<Alert[]>;
    // swpc_scales, swpc_alerts, usgs_quakes, usgs_volcanoes: Phase 2
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
