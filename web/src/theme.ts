import { theme as antdTheme, type ThemeConfig } from "antd";
import type { ThemeMode } from "./api/types";

const tokenOverrides: ThemeConfig["token"] = {
  colorPrimary: "#0a4f8a",
  borderRadius: 6,
  fontFamilyCode:
    "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace",
};

/**
 * resolveAlgorithm picks the AntD algorithm for the current theme mode.
 * "auto" honours `prefers-color-scheme`.
 */
export function resolveAlgorithm(mode: ThemeMode): typeof antdTheme.darkAlgorithm {
  if (mode === "dark") return antdTheme.darkAlgorithm;
  if (mode === "light") return antdTheme.defaultAlgorithm;
  // auto
  if (typeof window !== "undefined" && window.matchMedia?.("(prefers-color-scheme: dark)").matches) {
    return antdTheme.darkAlgorithm;
  }
  return antdTheme.defaultAlgorithm;
}

export function buildThemeConfig(mode: ThemeMode): ThemeConfig {
  return {
    algorithm: resolveAlgorithm(mode),
    token: tokenOverrides,
  };
}

const STORAGE_KEY = "cwd.themeMode";

export function loadStoredTheme(): ThemeMode | null {
  try {
    const v = localStorage.getItem(STORAGE_KEY);
    if (v === "dark" || v === "light" || v === "auto") return v;
  } catch {
    /* ignore */
  }
  return null;
}

export function persistTheme(mode: ThemeMode): void {
  try {
    localStorage.setItem(STORAGE_KEY, mode);
  } catch {
    /* ignore */
  }
}
