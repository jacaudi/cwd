import React from "react";
import ReactDOM from "react-dom/client";
import { App as AntdApp, ConfigProvider } from "antd";
import { BrowserRouter } from "react-router-dom";
import App from "./App";
import { buildThemeConfig, loadStoredTheme } from "./theme";

// Server-default-theme is fetched async by App (from /api/uiconfig). Until then,
// honour any stored choice or fall back to "dark" (matches design §6.1 default).
const initialMode = loadStoredTheme() ?? "dark";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <ConfigProvider theme={buildThemeConfig(initialMode)}>
      <AntdApp>
        <BrowserRouter>
          <App initialThemeMode={initialMode} />
        </BrowserRouter>
      </AntdApp>
    </ConfigProvider>
  </React.StrictMode>,
);
