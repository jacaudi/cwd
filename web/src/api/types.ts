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
