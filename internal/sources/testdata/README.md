# NWS Alerts Test Fixtures

These are immutable captured (or synthetic) snapshots of `https://api.weather.gov/alerts/active`
used by `nws_alerts_test.go`. Recapture only when the parser needs to lock new behavior;
the captured-on dates document expected drift over time.

| File | Captured | Notes |
|---|---|---|
| `alerts_active_mixed.json` | 2026-05-02 | Live capture, mixed event types |
| `alerts_active_empty.json` | 2026-05-02 | Hand-crafted, zero features |
| `alerts_active_tsunami.json` | 2026-05-02 | Synthetic TSU AWIPS prefix |

## SWPC fixtures

| File | Captured | Notes |
|---|---|---|
| `swpc_scales_typical.json` | 2026-05-02 | Live capture of noaa-scales.json; keys "0".."3" populated. Recapture if SWPC reshapes the upstream contract. |
| `swpc_alerts_typical.json` | 2026-05-02 | Live capture of alerts.json; ~30-day rolling window with mixed K/P/WARK/SUM products. |
