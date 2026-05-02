# NCEP Critical Weather Day Status — Reconnaissance & Catalog

**Target:** `https://www.nco.ncep.noaa.gov/status/cwd/`
**Title:** *NWS/NCO Critical Weather Day Status*
**Date of audit:** 2026-05-01
**Auditor:** Claude (gentle reconnaissance — single page load + 5 upstream HEADs + 3 file pulls)
**Purpose:** Inventory every image, data source, refresh cadence, and behavior so we can plan a self‑hosted re‑implementation.

---

## 1. Top‑level architecture (one‑paragraph summary)

The CWD page is a **server‑rendered PHP/HTML shell** (Apache + PHP 8.2.30) hosted at `nco.ncep.noaa.gov` whose body is mostly **static markup with hot‑linked images** from sister NOAA centers, plus **two small client‑side JavaScript files** that periodically call **five third‑party JSON/XML endpoints** to populate the alert badges, the Space‑Weather forecast block, and the Earthquakes / Tsunamis / Volcanoes panels. There is no API of its own, no database calls visible to the client, no auth, and no CAPTCHA. All upstream data is unauthenticated, CORS‑open, browser‑cacheable for ≈ 60 seconds, and re‑pulled on a fixed `setInterval(…, 60000)` schedule. The whole page also performs a **hard meta‑refresh every 900 s (15 min)**.

```
┌───────────────────── Browser ─────────────────────┐
│  HTML shell (PHP-rendered, mostly static)         │
│   ├── 31 <img> hotlinks → other NOAA/USGS/Navy    │
│   │   sites (PNG/GIF/JPG, no per-request auth)    │
│   ├── get_cwd_info.js  (272 lines) ── periodic    │
│   │     fetch(urla..urld)                         │
│   └── get_count_alerts.js (208 lines) ── periodic │
│         fetch(urlc, urle)                         │
│                                                   │
│  Schedule                                         │
│   ├── meta refresh = 900 s   (full page reload)   │
│   └── setInterval(gatheralerts, 60_000) — calls   │
│         all five fetches every minute             │
└───────────────────────────────────────────────────┘
```

**Implication for a refactor:** there is *no* server‑side aggregation today. Each browser is independently hitting SWPC, USGS, USGS volcanoes, the NWS API, and dozens of static map PNGs every minute. A self‑hosted version that adds a small server‑side cache layer is *strictly nicer to upstreams* than the current design.

---

## 2. Server fingerprint & security headers

`curl -I https://www.nco.ncep.noaa.gov/status/cwd/` →

```
HTTP/1.1 200 OK
Server: Apache
X-Frame-Options: SAMEORIGIN
X-Content-Type-Options: nosniff
X-XSS-Protection: 1; mode=block
Content-Security-Policy: connect-src 'self'
   https://www.googletagmanager.com
   https://www.google-analytics.com
   https://services.swpc.noaa.gov
   https://earthquake.usgs.gov
   https://api.weather.gov
   https://volcanoes.usgs.gov;
Referrer-Policy: no-referrer
X-Powered-By: PHP/8.2.30
Content-Type: text/html; charset=UTF-8
Strict-Transport-Security: max-age=31536000; includeSubdomains; preload
```

Notes:
- **CSP `connect-src` is the easiest external inventory of dynamic data sources.** It exactly matches `urla..urle` in the inline `<script>`.
- No `Cache-Control` was returned for the HTML itself — the meta `<http-equiv="refresh" content="900">` is the only TTL on the shell.
- No `frame-ancestors`; `X-Frame-Options: SAMEORIGIN` is the only clickjacking control.
- `Referrer-Policy: no-referrer` is why hotlinked images from `forecast.weather.gov`, `spc.noaa.gov`, `wpc.ncep.noaa.gov`, `nhc.noaa.gov`, `metoc.navy.mil` work without referer disputes.

`robots.txt` (full):
```
User-agent: *
Disallow: /pmb/nwpara/
Disallow: /pmb/nwtest/
```
→ `/status/cwd/` is unrestricted.

---

## 3. Page sections (top‑to‑bottom)

Identified from `cwd-index.html` and the rendered screenshot (`cwd-fullpage.png`):

| # | Section / banner | DOM hook | Content |
|---|---|---|---|
| 1 | NOAA / NWS / DOC header | `/images/header-noaa.png`, `header-nws.png`, `header_doc.png` | Static brand bar |
| 2 | "Critical Weather Day Status" sub‑banner | `cwd-banner-fade-oceanblue.png` | Static |
| 3 | **Current Status** (NORMAL / CWD / Enhanced Caution) | `class="row-opt-a"` block | Server‑rendered (PHP) — text + WWA map |
| 4 | Page‑load timestamps in 6 US time zones | inline server text | PHP `date()` |
| 5 | Active alerts badge row | `<span id="DisplayAlerts">` | JS — built by `get_count_alerts.js` |
| 6 | **Outlook** (3‑day timeline, Fri/Sat/Sun) | `hd-opt-d` "Outlook" | Server‑rendered |
| 7 | **Hazards** group of image grids: | `hd-opt-d` "Hazards" | Hot‑linked PNGs |
|   | • Severe Thunderstorms (SPC Day 1/2/3 convective) | | |
|   | • Wildfire (SPC fire wx Day 1/2 + Day 3‑8 experimental) | | |
|   | • Excessive Rainfall (WPC ERO Day 1/2/3) | | |
|   | • Winter Weather (WPC WSSI Day 1/2/3) | | |
|   | • Heat (WPC HeatRisk Day 1/2/3) — *broken `alt"…"` attr* | | |
|   | • Space Weather (3‑day forecast block) | `id="SWPCy"` / `id="SWPCn"` | JS from SWPC |
|   | • Tropical Cyclones (JTWC + NHC × 3 basins) | | |
| 8 | **Flooding** (WPC FHO image) + Tsunamis / Earthquakes / Volcanoes panels | `id="ts"`, `id="eq"`, `id="vl"` | JS |
| 9 | NWS / DOC / Privacy footer | static | |

---

## 4. Static (image) sources — full inventory

These are loaded directly by the browser as `<img src="…">` with no proxy. They refresh because the entire page reloads every 900 s.

### 4.1 Watch/Warning area (WWA) maps (forecast.weather.gov)
| URL | Region | Width displayed |
|---|---|---|
| `https://forecast.weather.gov/wwamap/png/US.png` | CONUS | 160 px tall, responsive |
| `https://forecast.weather.gov/wwamap/png/ak.png` | Alaska | 60×46 |
| `https://forecast.weather.gov/wwamap/png/hi.png` | Hawaii | 60×46 |
| `https://forecast.weather.gov/wwamap/png/ppg.png` | Pago Pago | 42×22 |
| `https://forecast.weather.gov/wwamap/png/gum.png` | Guam | 42×22 |
| `https://forecast.weather.gov/wwamap/png/sju.png` | Puerto Rico | 36×28 |

### 4.2 Severe Thunderstorms — SPC Convective Outlooks
| URL | Day |
|---|---|
| `https://www.spc.noaa.gov/products/outlook/day1otlk.png` | 1 |
| `https://www.spc.noaa.gov/products/outlook/day2otlk.png` | 2 |
| `https://www.spc.noaa.gov/products/outlook/day3otlk.png` | 3 |

### 4.3 Wildfire — SPC Fire Weather Outlooks
| URL | Day |
|---|---|
| `https://www.spc.noaa.gov/products/fire_wx/day1otlk_fire.png` | 1 |
| `https://www.spc.noaa.gov/products/fire_wx/day2otlk_fire.png` | 2 |
| `https://www.spc.noaa.gov/products/exper/fire_wx/imgs/day38otlk_fire.gif` | 3‑8 (experimental) |

### 4.4 Excessive Rainfall — WPC ERO
| URL | Day |
|---|---|
| `https://www.wpc.ncep.noaa.gov/qpf/94ewbg.gif` | 1 |
| `https://www.wpc.ncep.noaa.gov/qpf/98ewbg.gif` | 2 |
| `https://www.wpc.ncep.noaa.gov/qpf/99ewbg.gif` | 3 |

### 4.5 Winter Weather — WPC WSSI Overall CONUS
| URL | Day |
|---|---|
| `https://www.wpc.ncep.noaa.gov/wwd/wssi/images/WSSI_Overall_Day1_CONUS_Day1.png` | 1 |
| `https://www.wpc.ncep.noaa.gov/wwd/wssi/images/WSSI_Overall_Day2_CONUS_Day2.png` | 2 |
| `https://www.wpc.ncep.noaa.gov/wwd/wssi/images/WSSI_Overall_Day3_CONUS_Day3.png` | 3 |

### 4.6 Heat — WPC HeatRisk
| URL | Day |
|---|---|
| `https://www.wpc.ncep.noaa.gov/heatrisk/graphics/HeatRisk_Day1_CONUS.png` | 1 |
| `https://www.wpc.ncep.noaa.gov/heatrisk/graphics/HeatRisk_Day2_CONUS.png` | 2 |
| `https://www.wpc.ncep.noaa.gov/heatrisk/graphics/HeatRisk_Day3_CONUS.png` | 3 |

> **HTML bug observed:** the HeatRisk `<img>` tags say `alt"WPC HeatRisk"` (missing `=`). It's an `alt` attribute that won't validate but renders fine. Worth fixing in the refactor.

### 4.7 Tropical Cyclones
| URL | Source |
|---|---|
| `https://www.metoc.navy.mil/jtwc/products/abpwsair.jpg` | US Navy Joint Typhoon Warning Center |
| `https://www.nhc.noaa.gov/xgtwo/two_cpac_7d0.png` | NHC — Central Pacific 7‑day |
| `https://www.nhc.noaa.gov/xgtwo/two_pac_7d0.png` | NHC — East Pacific 7‑day |
| `https://www.nhc.noaa.gov/xgtwo/two_atl_7d0.png` | NHC — Atlantic 7‑day |

### 4.8 Flooding
| URL | Source |
|---|---|
| `https://www.weather.gov/images/owp/FHO/National/National_FHO.png` | NWC National Flood Hazard Outlook |

### 4.9 Local site assets (decorative)
- `/images/header-noaa.png`, `header-nws.png`, `header_doc.png`, `bg.png`, `head_shadow.png`, `skipgraphic.gif`
- `/status/css/images/cwd-banner-fade-oceanblue.png`
- Section icons: `icon-lightning.png`, `icon-wildfire.png`, `icon-rainstorm.png`, `icon-winter.png`, `icon-space.png`, `icon-tropical.png`, `icon-flood.png`, `icon-tsunami.png`, `icon-earthquake.png`, `icon-volcano.png`, `icon-wind.png`, plus alert‑bar variants `icon-tornado.png`, `icon-heat.png`, `icon-cold.png`

### 4.10 External CSS / JS
| URL | Notes |
|---|---|
| `/css/nco_main_structural.css` | NCO global layout |
| `/css/nco_main_style.css` | NCO global theme |
| `/status/css/status_style_cwd.css?version=3.0` | Page‑specific styles |
| `/status/javascript/get_cwd_info.js?version=3.0` | Earthquake/SWPC forecast/Volcano/Tsunami pulls |
| `/status/javascript/get_count_alerts.js?version=3.1` | Active alerts badge bar |
| `https://dap.digitalgov.gov/Universal-Federated-Analytics-Min.js?...` | DAP analytics — *failed in our test* (NS_ERROR_CONNECTION_REFUSED ×4) |

---

## 5. Dynamic data sources — full inventory

All five are declared inline in `cwd-index.html` (lines 607‑611) and bound to global `urla..urle`:

```js
var urla = 'https://services.swpc.noaa.gov/products/noaa-scales.json';
var urlb = 'https://earthquake.usgs.gov/earthquakes/feed/v1.0/summary/significant_day.geojson';
var urlc = 'https://api.weather.gov/alerts/active';
var urld = 'https://volcanoes.usgs.gov/rss/vhpcaprss.xml';
var urle = 'https://services.swpc.noaa.gov/products/alerts.json';
```

### 5.1 `urla` — SWPC NOAA Scales 3‑day forecast
- **Endpoint:** `https://services.swpc.noaa.gov/products/noaa-scales.json`
- **Cache:** `Cache-Control: max-age=60`, `Expires: +60s`, `ETag` present, `Last-Modified` updated each cache miss, `Access-Control-Allow-Origin: *`
- **Content‑Type:** `application/json`
- **Shape consumed by `get_cwd_info.js`:**

  ```text
  json[1..3] = {                 // index 0 is "current"; 1,2,3 are 3-day forecast
    DateStamp:  "YYYY-MM-DD",
    R: { MinorProb: "5",  MajorProb: "1" },   // Radio blackout (R-scale) probabilities
    S: { Prob:      "1" },                     // Solar radiation (S-scale)
    G: { Scale:     "0",  Text:      "none" }  // Geomagnetic (G-scale): 0..5
  }
  ```
- Page binds these to `swpc-fd-N`, `swpc-r1-N`, `swpc-r3-N`, `swpc-s1-N`, `swpc-g-N`, `swpc-gtc-N`, `swpc-gt-N` for N=1..3.
- **QC:** if any value isn't an integer after parse, the SWPC block is hidden (`donotdisplay()`).

### 5.2 `urlb` — USGS Significant Earthquakes (24 h)
- **Endpoint:** `https://earthquake.usgs.gov/earthquakes/feed/v1.0/summary/significant_day.geojson`
- **Cache:** `Cache-Control: public, max-age=60`, CORS `*`
- **Content‑Type:** `application/json; charset=utf-8` (GeoJSON)
- **Shape consumed:**

  ```text
  features[i] = {
    properties: { url, title, time /* ms epoch */ },
    geometry:   { coordinates: [lon, lat, depth] }
  }
  ```
- Each feature is rendered as `<title link><br>lat°N/S lon°E/W | depth meters<br>YYYY-MM-DD HH:MM:SS(UTC)`.
- Used to populate `id="eq"` and increment global `neqw` (badge counter).

### 5.3 `urlc` — NWS API active alerts (used twice)
- **Endpoint:** `https://api.weather.gov/alerts/active`
- **Cache:** `Cache-Control: public, max-age=9, s-maxage=30`, `Vary: Accept,Feature-Flags,Accept-Language`, CORS `*`
- **Content‑Type:** `application/geo+json`
- **Two consumers:**
  1. `getts()` in `get_cwd_info.js` — filters where `properties.parameters.AWIPSidentifier[0:3] == "TSU"`, builds Tsunami panel `id="ts"` and direct‑product link `https://forecast.weather.gov/product.php?site=NWS&product=TSU&issuedby=<wfo>`.
  2. `getapialerts()` in `get_count_alerts.js` — counts by `properties.event` text (string match):
     - `"Tornado Warning"` → `ntor`
     - `"Severe Thunderstorm Warning"` → `nsvr`
     - `"Flash Flood Warning"` → `nffw`
     - `"Storm Surge Warning"` | `"Hurricane Warning"` | `"Typhoon Warning"` | `"Tropical Storm Warning"` → `trpw`
     - `"High Wind Warning"` | `"Extreme Wind Warning"` → `nhww`
     - `"Red Flag Warning"` → `rdfw`
     - `"Winter Storm Warning"` | `"Blizzard Warning"` | `"Ice Storm Warning"` | `"Snow Squall Warning"` → `wwbw`
     - `"Extreme Heat Warning"` → `exhw`  (note: header comment says it was changed from "Excessive Heat" 2025‑03‑23 per HazSimp SCN 24‑88)
     - `"Extreme Cold Warning"` → `excw`
     - AWIPSidentifier prefix `"TSU"` → `ntsu` (also counted here for the badge)
- The two consumers each call `fetch(urlc)` separately → **two requests per minute to api.weather.gov per browser** (unnecessary; a refactor target).

### 5.4 `urld` — USGS Volcano Hazards CAP RSS
- **Endpoint:** `https://volcanoes.usgs.gov/rss/vhpcaprss.xml`
- **Cache:** No explicit `Cache-Control`; `Last-Modified` + `ETag` for revalidation, CORS `*`
- **Content‑Type:** `text/xml`
- **Shape consumed (DOMParser):** loops `<item>` elements, reads `<volcano:alertlevel>` (filter to `WATCH`/`WARNING`) and `<volcano:colorcode>` (`RED` → red badge, `ORANGE` → orange badge), plus `<title>`, `<description>`, `<link>`. Threshold was lowered to "Watch" in April 2024 per the file header comment.
- Renders into `id="vl"` and `id="vol"`; counts WARNING‑level into `nvlw`.

### 5.5 `urle` — SWPC alerts feed (G/S badges)
- **Endpoint:** `https://services.swpc.noaa.gov/products/alerts.json`
- **Cache:** `Cache-Control: max-age=60`, CORS `*`
- **Content‑Type:** `application/json`
- **Shape consumed:** array of `{ product_id, issue_datetime }`. The page filters to events issued in the last 24 hours (`x = 24` in `get_count_alerts.js`) where `product_id` is in the set:
  - `K08A` → Geomagnetic K=8 (G4) reached
  - `K09A` → Geomagnetic K=9 (G5) reached
  - `P12A` → Solar radiation S2 observed   *(added 2026‑02‑19)*
  - `P13A` → Solar radiation S3 observed   *(added 2026‑02‑19)*
- Increments `nswp` for the badge bar.

---

## 6. Refresh cadence (the budget per browser per hour)

| Mechanism | Where | Period |
|---|---|---|
| `<meta http-equiv="refresh" content="900">` | `cwd-index.html:50` | 900 s — full page reload, re‑fetches HTML + CSS + JS + every static image |
| `setInterval(gatheralerts, 60000)` | `cwd-index.html:627` | 60 s — calls `getdata`, `getdatab`, `getdatac`, `getdatad`, `getAlertsSWPC`, `getAlertsApi` |
| Initial fire | `cwd-index.html:614` | once on load |

**Per‑browser API request rate:**
- 60 requests/hr to SWPC `noaa-scales.json`
- 60 requests/hr to USGS `significant_day.geojson`
- **120 requests/hr to api.weather.gov/alerts/active** (called by both JS files)
- 60 requests/hr to USGS volcanoes RSS
- 60 requests/hr to SWPC `alerts.json`
- Plus one full page + ~31 hot‑linked images every 15 min = 4×/hr

**Per‑page‑load static image fetches (≈31):** SPC (3+3+1), WPC ERO (3), WSSI (3), HeatRisk (3), JTWC (1), NHC (3), FHO (1), WWA (6), local icons + chrome (~8).

This per‑browser load is the main argument for putting a **server‑side cache + ETag passthrough** in front of all of it in our refactor.

---

## 7. Behavior quirks worth knowing

1. **Alerts badge double‑pull.** `getapialerts()` (count_alerts.js) and `getts()` (cwd_info.js) both call `fetch(urlc)` separately every minute. A refactor should call once and fan out.
2. **Time fabrication (mild).** `get_cwd_info.js geteq()` reconstructs `vdate` from `properties.time` (epoch ms) — fine. But `get_count_alerts.js getswpcalerts()` builds `cdate` and `idate` as zero‑padded `YYYYMMDDHHMM` *strings* and compares with `>` — works only because both strings are the same fixed length.
3. **No error UI for SWPC.** If the SWPC fetch fails, the existing `<div id="SWPCn">` ("currently unavailable") branch is shown — there's no retry.
4. **Hard‑coded "K08A/K09A/P12A/P13A" set.** Anything beyond G4/G5 or S2/S3 won't badge; refactor should make the threshold configurable.
5. **Alert event matching is by exact `properties.event` string.** If NWS renames an event (as happened with Extreme Heat in 2025), the count silently goes to zero. The HazSimp transition note in the file header is a real artifact of this fragility.
6. **CSP omission.** `connect-src` allows `googletagmanager.com` + `google-analytics.com`, but `<script src="https://dap.digitalgov.gov/...">` is loaded as `script-src`; we observed it failing with `NS_ERROR_CONNECTION_REFUSED` four times in our session — likely network‑local, but it tells you analytics is *not* essential to the page.
7. **Volcano alert RSS lacks Cache‑Control** — only ETag/Last‑Modified. Refactor should send `If-None-Match` to be polite.
8. **The "Active alerts" badge counter for volcanoes (`nvlw`) only counts WARNING level**, even though the panel itself displays both WATCH and WARNING. Mild inconsistency.
9. **`alt"…"` typo** on three HeatRisk `<img>` tags — see §4.6.
10. **Hot‑link bandwidth.** Every browser fetches every map PNG directly from SPC/WPC/NHC/etc on every 15‑min reload; a self‑hosted version that caches images for the 5–15 min publication cadence would cut image traffic to those origins by orders of magnitude per active viewer.

---

## 8. Cache‑Control summary (HEAD probes, 2026‑05‑01 22:23 UTC)

| Endpoint | Status | Cache‑Control | Has ETag | Has Last‑Modified | CORS |
|---|---|---|---|---|---|
| `services.swpc.noaa.gov/products/noaa-scales.json` | 200 | `max-age=60` | ✅ | ✅ | `*` |
| `services.swpc.noaa.gov/products/alerts.json` | 200 | `max-age=60` | ✅ | ✅ | `*` |
| `earthquake.usgs.gov/.../significant_day.geojson` | 200 | `public, max-age=60` | (no) | ✅ | `*` |
| `api.weather.gov/alerts/active` | 200 | `public, max-age=9, s-maxage=30` | (no) | (no) | `*` |
| `volcanoes.usgs.gov/rss/vhpcaprss.xml` | 200 | (none) | ✅ | ✅ | `*` |

**Implication for refactor:** all five are CORS‑open and conditional‑GET friendly. A Go backend doing `If-None-Match`/`If-Modified-Since` on the three with strong validators, and respecting `max-age=60`/`max-age=9` on the others, is enough — no auth, no API keys, no rate limit headers visible.

---

## 9. Local sample artifacts captured during this audit

All saved at the repo root for follow‑on use:

- `cwd-index.html` (638 lines) — server‑rendered shell
- `cwd-get_cwd_info.js` (272 lines) — Earthquake / Volcano / Tsunami / SWPC forecast pulls
- `cwd-get_count_alerts.js` (208 lines) — alert‑bar counter logic
- `cwd-snapshot.md` (581 lines) — Playwright a11y snapshot
- `cwd-network.txt` (56 lines) — full network waterfall, single page load
- `cwd-fullpage.png` — full‑page screenshot @ 1440×900 viewport

These should move into `docs/recon/artifacts/` as part of any refactor PR.

---

## 10. Open questions for the refactor design

These are the choices that came out of the audit and that the brainstorm should answer:

1. **Authority model.** Is the self‑hosted version intended to be a *mirror* of NCO CWD's behavior, or a *superset* (more thresholds, configurable WFO filters, history retention)?
2. **Server‑side caching tier.** Do we serve cached JSON only (passthrough proxy), or do we **normalize** upstream payloads into our own schema once and serve that?
3. **Image strategy.** Cache PNGs server‑side and serve from our origin (relieves upstream)? Or 302‑redirect the browser? Or just pass through with `If-Modified-Since`?
4. **Auth / multi‑tenant.** Single‑tenant self‑host (most likely) vs. allow per‑user thresholds.
5. **History.** Today nothing is persisted. Do we want a SQLite/Postgres ring buffer of alerts/badges so we can show "what was active at 14:00Z"?
6. **Push vs poll.** Today it's pure poll. With a server‑side cache we can offer **SSE/WebSocket** to the browser and drop the per‑browser timers entirely.
7. **Theming / accessibility.** The current page is 2002‑era Apache template. Ant Design opens up a real responsive layout, dark mode, a11y improvements (keyboard nav for the hazard grids, alt text for HeatRisk).
