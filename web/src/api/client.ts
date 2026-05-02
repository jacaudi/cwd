import type { UIConfig, VersionInfo } from "./types";

async function getJSON<T>(path: string): Promise<T> {
  const res = await fetch(path, { headers: { Accept: "application/json" } });
  if (!res.ok) throw new Error(`${path}: ${res.status} ${res.statusText}`);
  return (await res.json()) as T;
}

export const api = {
  uiconfig: () => getJSON<UIConfig>("/api/uiconfig"),
  version:  () => getJSON<VersionInfo>("/api/version"),
};
