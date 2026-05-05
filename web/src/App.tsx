import { useEffect, useMemo, useState } from "react";
import { ConfigProvider, Space, Tag } from "antd";
import enUS from "antd/locale/en_US";
import { ProLayout } from "@ant-design/pro-components";
import { Route, Routes, useNavigate, useLocation } from "react-router-dom";
import {
  AppstoreOutlined,
  ExperimentOutlined,
  FireOutlined,
  HistoryOutlined,
  SettingOutlined,
  ThunderboltOutlined,
} from "@ant-design/icons";

import { api } from "./api/client";
import { connect } from "./api/stream";
import { useSnapshotStore } from "./store/snapshot";
import type { ThemeMode, UIConfig, VersionInfo } from "./api/types";
import { buildThemeConfig, persistTheme } from "./theme";
import SettingsDrawer from "./components/SettingsDrawer";
import { SourceHealthIndicator } from "./components/SourceHealthIndicator";

import Overview from "./pages/Overview";
import Hazards from "./pages/Hazards";
import SpaceWeather from "./pages/SpaceWeather";
import Events from "./pages/Events";
import History from "./pages/History";

interface AppProps {
  initialThemeMode: ThemeMode;
}

const route = {
  path: "/",
  routes: [
    { path: "/",        name: "Overview",      icon: <AppstoreOutlined /> },
    { path: "/hazards", name: "Hazards",       icon: <ThunderboltOutlined /> },
    { path: "/space",   name: "Space Weather", icon: <ExperimentOutlined /> },
    { path: "/events",  name: "Events",        icon: <FireOutlined /> },
    { path: "/history", name: "History",       icon: <HistoryOutlined /> },
  ],
};

export default function App({ initialThemeMode }: AppProps) {
  const navigate = useNavigate();
  const location = useLocation();

  const [themeMode, setThemeMode] = useState<ThemeMode>(initialThemeMode);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [serverVersion, setServerVersion] = useState<VersionInfo | null>(null);
  const [uiConfig, setUIConfig] = useState<UIConfig | null>(null);

  const setSnapshot = useSnapshotStore((s) => s.setSnapshot);
  const applyUpdate = useSnapshotStore((s) => s.applyUpdate);
  const applyImageInvalidate = useSnapshotStore((s) => s.applyImageInvalidate);
  const setConnection = useSnapshotStore((s) => s.setConnection);

  // Open the live snapshot stream once for the whole app — every route
  // (Overview, Hazards, Space, Events, History) reads from the shared
  // snapshot store, so a single EventSource serves them all. Hoisting the
  // lifecycle here means deep-links and hard-refreshes on non-Overview
  // routes also get the initial /api/snapshot fetch + SSE updates.
  useEffect(() => {
    let cancel: (() => void) | undefined;
    let unmounted = false;
    void connect({
      onSnapshot: (s) => {
        setSnapshot(s);
        setConnection('live');
      },
      onUpdate: (name, env) => applyUpdate(name, env),
      onImageInvalidate: (ev) => applyImageInvalidate(ev),
      onError: () => setConnection('error'),
    }).then((c) => {
      // If the component unmounted before connect resolved, tear down
      // immediately so we never leak an EventSource.
      if (unmounted) c();
      else cancel = c;
    });
    return () => {
      unmounted = true;
      cancel?.();
    };
  }, [setSnapshot, applyUpdate, applyImageInvalidate, setConnection]);

  // First-paint: load server config + version. If user has no stored choice and the server
  // says a different default, adopt it. (loadStoredTheme already ran in main.tsx, so we
  // only adopt when the user hasn't explicitly chosen.)
  useEffect(() => {
    let cancelled = false;
    Promise.allSettled([api.uiconfig(), api.version()]).then(([cfg, ver]) => {
      if (cancelled) return;
      if (cfg.status === "fulfilled") {
        setUIConfig(cfg.value);
        const stored = localStorage.getItem("cwd.themeMode");
        if (!stored && cfg.value.defaultTheme !== themeMode) {
          setThemeMode(cfg.value.defaultTheme);
        }
      }
      if (ver.status === "fulfilled") setServerVersion(ver.value);
    });
    return () => { cancelled = true; };
    // themeMode intentionally not in deps: this only runs once on mount.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const themeConfig = useMemo(() => buildThemeConfig(themeMode), [themeMode]);

  const onThemeChange = (mode: ThemeMode) => {
    setThemeMode(mode);
    persistTheme(mode);
  };

  return (
    <ConfigProvider theme={themeConfig} locale={enUS}>
      <ProLayout
        title="cwd"
        logo={false}
        layout="mix"
        fixSiderbar
        location={{ pathname: location.pathname }}
        route={route}
        menuItemRender={(item, dom) => (
          <a onClick={() => item.path && navigate(item.path)}>{dom}</a>
        )}
        rightContentRender={() => (
          <Space>
            <Tag color="default">Phase 0 — skeleton</Tag>
            <a aria-label="Settings" onClick={() => setSettingsOpen(true)}>
              <SettingOutlined />
            </a>
          </Space>
        )}
        footerRender={() => (
          <div style={{ textAlign: "center", padding: 8, opacity: 0.85 }}>
            <div style={{ marginBottom: 4 }}>
              <SourceHealthIndicator />
            </div>
            <div style={{ opacity: 0.65 }}>
              cwd {serverVersion?.version ?? "…"} ·{" "}
              UI default: <code>{uiConfig?.defaultTheme ?? "…"}</code> ·{" "}
              <a href="/api/version">version</a> · <a href="/api/uiconfig">uiconfig</a>
            </div>
          </div>
        )}
      >
        <Routes>
          <Route path="/"        element={<Overview />} />
          <Route path="/hazards" element={<Hazards />} />
          <Route path="/space"   element={<SpaceWeather />} />
          <Route path="/events"  element={<Events />} />
          <Route path="/history" element={<History />} />
        </Routes>

        <SettingsDrawer
          open={settingsOpen}
          onClose={() => setSettingsOpen(false)}
          themeMode={themeMode}
          onThemeChange={onThemeChange}
        />
      </ProLayout>
    </ConfigProvider>
  );
}
