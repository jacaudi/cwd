# NWS Alerts Test Fixtures

These are immutable captured (or synthetic) snapshots of `https://api.weather.gov/alerts/active`
used by `nws_alerts_test.go`. Recapture only when the parser needs to lock new behavior;
the captured-on dates document expected drift over time.

| File | Captured | Notes |
|---|---|---|
| `alerts_active_mixed.json` | 2026-07-01 | Re-captured for issue #3 (expanded category map). Curated from a live `alerts/active` pull — up to 2 features per distinct event type — plus two synthetic features (`Tornado Watch`, `Storm Surge Watch`) modeled on real NWS structure to exercise the new Watch mappings that were seasonally absent from the July pull. Exercises the new `Flood` and `Marine` families and leaves zero Severe/Extreme events uncategorized (drift canary silent). |
| `alerts_active_empty.json` | 2026-05-02 | Hand-crafted, zero features |
| `alerts_active_tsunami.json` | 2026-05-02 | Synthetic TSU AWIPS prefix |

## SWPC fixtures

| File | Captured | Notes |
|---|---|---|
| `swpc_scales_typical.json` | 2026-05-02 | Live capture of noaa-scales.json; keys "0".."3" populated. Recapture if SWPC reshapes the upstream contract. |
| `swpc_alerts_typical.json` | 2026-05-02 | Live capture of alerts.json; ~30-day rolling window with mixed K/P/WARK/SUM products. |

## USGS fixtures

| File | Captured | Notes |
|---|---|---|
| `usgs_volcanoes_typical.json` | 2026-05-02 | Live capture of getElevatedVolcanoes; non-NORMAL volcanoes only by upstream design. |
| `usgs_quakes_typical.geojson` | 2026-05-02 | Live capture of significant_day.geojson. Feature count varies 0–20 depending on day. |
| `usgs_quakes_empty.geojson` | 2026-05-02 | Hand-crafted, zero features. Locks the empty-day path. |
