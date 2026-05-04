// Package imageproxy implements the lazy-with-prewarm proxy that fronts the
// 20 third-party static maps the original NCEP CWD page hot-links. See
// docs/plans/2026-05-03-phase3-image-proxy-design.md for the full design.
package imageproxy

import "time"

// Image describes one upstream map proxied by /img/{source}/{name}.
type Image struct {
	// Source is URL-path segment 1 (e.g. "spc"). Must not contain '.' or '/'.
	Source string
	// Name is URL-path segment 2 (e.g. "day1otlk"). Must not contain '.' or '/'.
	Name string
	// URL is the upstream absolute URL.
	URL string
	// MIME is the fallback content-type if upstream omits Content-Type.
	MIME string
	// DefaultInterval is the poll cadence absent operator override.
	// Must be >= 60s (politeness floor).
	DefaultInterval time.Duration
	// Description is human-readable "what this map shows" text. Doubles as
	// documentation; surfaced to operators in future settings panel.
	Description string
}

// Key returns the dot-key form used in config and SSE payloads (e.g. "spc.day1otlk").
func (i Image) Key() string { return i.Source + "." + i.Name }

// Path returns the URL form served at /img/{source}/{name}.
func (i Image) Path() string { return "/img/" + i.Source + "/" + i.Name }

// Per-key string constants. One per registry entry, mirroring the Phase 1/2
// pattern of exporting per-source name constants (sources.NWSAlertsName etc).
// These guard cross-file references against typos at compile time.
const (
	KeySPCDay1Otlk      = "spc.day1otlk"
	KeySPCDay2Otlk      = "spc.day2otlk"
	KeySPCDay3Otlk      = "spc.day3otlk"
	KeySPCDay1OtlkFire  = "spc.day1otlk_fire"
	KeySPCDay2OtlkFire  = "spc.day2otlk_fire"
	KeySPCDay38OtlkFire = "spc.day38otlk_fire"
	KeyWPCEroDay1       = "wpc.ero_day1"
	KeyWPCEroDay2       = "wpc.ero_day2"
	KeyWPCEroDay3       = "wpc.ero_day3"
	KeyWPCWSSIDay1      = "wpc.wssi_day1"
	KeyWPCWSSIDay2      = "wpc.wssi_day2"
	KeyWPCWSSIDay3      = "wpc.wssi_day3"
	KeyWPCHeatRiskDay1  = "wpc.heatrisk_day1"
	KeyWPCHeatRiskDay2  = "wpc.heatrisk_day2"
	KeyWPCHeatRiskDay3  = "wpc.heatrisk_day3"
	KeyNHCAtl7D         = "nhc.atl_7d"
	KeyNHCEpac7D        = "nhc.epac_7d"
	KeyNHCCpac7D        = "nhc.cpac_7d"
	KeyNavyJTWCABPW     = "navy.jtwc_abpw"
	KeyNWCFHONational   = "nwc.fho_national"
)

// Registry is the immutable list of all proxied images.
//
// To add a new image: append a new struct literal AND add a Key* constant
// AND extend the front-end CATEGORIES table in HazardCategoryCard.tsx if
// the new image belongs to a category card.
//
// Each entry's comment names the upstream agency, the product, and the time
// horizon it depicts so the registry doubles as documentation.
//
//nolint:gochecknoglobals // intentional package-level immutable config
var Registry = []Image{
	// Severe Storms — SPC Convective Outlooks
	// Day 1: today's convective threat (tornado / wind / hail) over CONUS.
	{Source: "spc", Name: "day1otlk", URL: "https://www.spc.noaa.gov/products/outlook/day1otlk.png", MIME: "image/png", DefaultInterval: 2 * time.Minute, Description: "SPC Convective Outlook — Day 1 (today)"},
	// Day 2: tomorrow's convective threat.
	{Source: "spc", Name: "day2otlk", URL: "https://www.spc.noaa.gov/products/outlook/day2otlk.png", MIME: "image/png", DefaultInterval: 5 * time.Minute, Description: "SPC Convective Outlook — Day 2 (tomorrow)"},
	// Day 3: the day after tomorrow.
	{Source: "spc", Name: "day3otlk", URL: "https://www.spc.noaa.gov/products/outlook/day3otlk.png", MIME: "image/png", DefaultInterval: 10 * time.Minute, Description: "SPC Convective Outlook — Day 3"},

	// Wildfire — SPC Fire Weather Outlooks
	{Source: "spc", Name: "day1otlk_fire", URL: "https://www.spc.noaa.gov/products/fire_wx/day1otlk_fire.png", MIME: "image/png", DefaultInterval: 5 * time.Minute, Description: "SPC Fire Weather Outlook — Day 1"},
	{Source: "spc", Name: "day2otlk_fire", URL: "https://www.spc.noaa.gov/products/fire_wx/day2otlk_fire.png", MIME: "image/png", DefaultInterval: 10 * time.Minute, Description: "SPC Fire Weather Outlook — Day 2"},
	// Day 3-8 experimental: a longer outlook; animated GIF.
	{Source: "spc", Name: "day38otlk_fire", URL: "https://www.spc.noaa.gov/products/exper/fire_wx/imgs/day38otlk_fire.gif", MIME: "image/gif", DefaultInterval: 30 * time.Minute, Description: "SPC Fire Weather Outlook — Day 3-8 (experimental, animated)"},

	// Excessive Rainfall — WPC ERO
	{Source: "wpc", Name: "ero_day1", URL: "https://www.wpc.ncep.noaa.gov/qpf/94ewbg.gif", MIME: "image/gif", DefaultInterval: 5 * time.Minute, Description: "WPC Excessive Rainfall Outlook — Day 1"},
	{Source: "wpc", Name: "ero_day2", URL: "https://www.wpc.ncep.noaa.gov/qpf/98ewbg.gif", MIME: "image/gif", DefaultInterval: 10 * time.Minute, Description: "WPC Excessive Rainfall Outlook — Day 2"},
	{Source: "wpc", Name: "ero_day3", URL: "https://www.wpc.ncep.noaa.gov/qpf/99ewbg.gif", MIME: "image/gif", DefaultInterval: 15 * time.Minute, Description: "WPC Excessive Rainfall Outlook — Day 3"},

	// Winter — WPC WSSI Overall CONUS
	{Source: "wpc", Name: "wssi_day1", URL: "https://www.wpc.ncep.noaa.gov/wwd/wssi/images/WSSI_Overall_Day1_CONUS_Day1.png", MIME: "image/png", DefaultInterval: 10 * time.Minute, Description: "WPC Winter Storm Severity Index — Day 1 (overall, CONUS)"},
	{Source: "wpc", Name: "wssi_day2", URL: "https://www.wpc.ncep.noaa.gov/wwd/wssi/images/WSSI_Overall_Day2_CONUS_Day2.png", MIME: "image/png", DefaultInterval: 15 * time.Minute, Description: "WPC Winter Storm Severity Index — Day 2 (overall, CONUS)"},
	{Source: "wpc", Name: "wssi_day3", URL: "https://www.wpc.ncep.noaa.gov/wwd/wssi/images/WSSI_Overall_Day3_CONUS_Day3.png", MIME: "image/png", DefaultInterval: 20 * time.Minute, Description: "WPC Winter Storm Severity Index — Day 3 (overall, CONUS)"},

	// Heat — WPC HeatRisk
	{Source: "wpc", Name: "heatrisk_day1", URL: "https://www.wpc.ncep.noaa.gov/heatrisk/graphics/HeatRisk_Day1_CONUS.png", MIME: "image/png", DefaultInterval: 15 * time.Minute, Description: "WPC HeatRisk — Day 1 (CONUS)"},
	{Source: "wpc", Name: "heatrisk_day2", URL: "https://www.wpc.ncep.noaa.gov/heatrisk/graphics/HeatRisk_Day2_CONUS.png", MIME: "image/png", DefaultInterval: 20 * time.Minute, Description: "WPC HeatRisk — Day 2 (CONUS)"},
	{Source: "wpc", Name: "heatrisk_day3", URL: "https://www.wpc.ncep.noaa.gov/heatrisk/graphics/HeatRisk_Day3_CONUS.png", MIME: "image/png", DefaultInterval: 30 * time.Minute, Description: "WPC HeatRisk — Day 3 (CONUS)"},

	// Tropical — NHC + Navy JTWC
	{Source: "nhc", Name: "atl_7d", URL: "https://www.nhc.noaa.gov/xgtwo/two_atl_7d0.png", MIME: "image/png", DefaultInterval: 30 * time.Minute, Description: "NHC Tropical Weather Outlook — Atlantic (7 day)"},
	{Source: "nhc", Name: "epac_7d", URL: "https://www.nhc.noaa.gov/xgtwo/two_pac_7d0.png", MIME: "image/png", DefaultInterval: 30 * time.Minute, Description: "NHC Tropical Weather Outlook — East Pacific (7 day)"},
	{Source: "nhc", Name: "cpac_7d", URL: "https://www.nhc.noaa.gov/xgtwo/two_cpac_7d0.png", MIME: "image/png", DefaultInterval: 30 * time.Minute, Description: "NHC Tropical Weather Outlook — Central Pacific (7 day)"},
	// US Navy Joint Typhoon Warning Center — Western Pacific basin tropical advisory.
	{Source: "navy", Name: "jtwc_abpw", URL: "https://www.metoc.navy.mil/jtwc/products/abpwsair.jpg", MIME: "image/jpeg", DefaultInterval: 30 * time.Minute, Description: "Navy JTWC ABPW — Western Pacific tropical synopsis"},

	// Flooding — WPC NWC National Flood Hazard Outlook
	{Source: "nwc", Name: "fho_national", URL: "https://www.weather.gov/images/owp/FHO/National/National_FHO.png", MIME: "image/png", DefaultInterval: 30 * time.Minute, Description: "NWC National Flood Hazard Outlook"},
}

// ByKey returns a map view of the registry indexed by Image.Key(). The map is
// rebuilt on every call (cheap — 20 entries) so callers can mutate the result
// without affecting the package-level Registry.
func ByKey() map[string]Image {
	m := make(map[string]Image, len(Registry))
	for _, img := range Registry {
		m[img.Key()] = img
	}
	return m
}

// DefaultPrewarmKeys returns the registry keys seeded into ImagesConfig.Prewarm
// by config defaults() — the 4 highest-traffic maps per design Decision 4.
func DefaultPrewarmKeys() []string {
	return []string{
		KeySPCDay1Otlk,
		KeySPCDay2Otlk,
		KeySPCDay3Otlk,
		KeyNHCAtl7D,
	}
}
