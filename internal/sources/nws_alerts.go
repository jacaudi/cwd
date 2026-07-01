package sources

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// NWSAlertsName is the canonical source name for NWS active alerts.
const NWSAlertsName = "nws_alerts"

// Severity represents NWS alert severity levels.
type Severity string

// Severity level constants as defined by the NWS CAP specification.
const (
	SevExtreme  Severity = "Extreme"
	SevSevere   Severity = "Severe"
	SevModerate Severity = "Moderate"
	SevMinor    Severity = "Minor"
	SevUnknown  Severity = "Unknown"
)

// Category groups NWS alert event types into broad hazard families.
type Category string

// Category constants for supported NWS hazard families.
const (
	CatTornado            Category = "Tornado"
	CatSevereThunderstorm Category = "SevereThunderstorm"
	CatFlashFlood         Category = "FlashFlood"
	CatFlood              Category = "Flood"
	CatTropical           Category = "Tropical"
	CatHighWind           Category = "HighWind"
	CatRedFlag            Category = "RedFlag"
	CatWinter             Category = "Winter"
	CatExtremeHeat        Category = "ExtremeHeat"
	CatExtremeCold        Category = "ExtremeCold"
	CatMarine             Category = "Marine"
	CatTsunami            Category = "Tsunami"
	CatUnknown            Category = "Unknown"
)

// Alert is a parsed, normalized NWS active alert.
type Alert struct {
	ID        string    `json:"id"`
	Event     string    `json:"event"`
	AWIPS     string    `json:"awips"`
	Headline  string    `json:"headline"`
	Severity  Severity  `json:"severity"`
	Sent      time.Time `json:"sent"`
	Effective time.Time `json:"effective"`
	Onset     time.Time `json:"onset,omitempty"`
	Expires   time.Time `json:"expires"`
	Ends      time.Time `json:"ends,omitempty"`
	Areas     []string  `json:"areas"`
	UGCs      []string  `json:"ugcs,omitempty"`
	SAMEs     []string  `json:"sames,omitempty"`
	WFO       string    `json:"wfo,omitempty"`
	VTECEtn   string    `json:"vtecEtn,omitempty"`
	URL       string    `json:"url,omitempty"`
	Category  Category  `json:"category"`
}

// eventCategoryMap maps NWS `properties.event` text to a badge Category.
//
// Phase 1 (PR #2) intentionally restricted this to Warnings only to match the
// original NCEP CWD page. Issue #3 expands it to cover the corresponding
// Watches and a handful of Advisories/Statements, and adds two new hazard
// families — Flood (areal/coastal flooding, distinct from FlashFlood) and
// Marine (offshore/coastal-waters products) — so the production drift canary
// (`nws_alerts.unmapped_severe_event`) stops firing on routine Severe/Extreme
// events such as Tornado Watch, Flood Warning, Freeze Watch, Fire Weather Watch,
// Special Marine Warning, and Storm Surge Watch.
//
// Tsunami is deliberately absent here: it is matched by the AWIPS "TSU" prefix
// in categorize() rather than by event text.
var eventCategoryMap = map[string]Category{
	// Tornado
	"Tornado Warning": CatTornado,
	"Tornado Watch":   CatTornado,
	// Severe thunderstorm
	"Severe Thunderstorm Warning": CatSevereThunderstorm,
	"Severe Thunderstorm Watch":   CatSevereThunderstorm,
	// Flash flood (rapid-onset)
	"Flash Flood Warning": CatFlashFlood,
	"Flash Flood Watch":   CatFlashFlood,
	// Flood (areal / river / coastal, slower-onset)
	"Flood Warning":           CatFlood,
	"Flood Watch":             CatFlood,
	"Flood Advisory":          CatFlood,
	"Coastal Flood Warning":   CatFlood,
	"Coastal Flood Watch":     CatFlood,
	"Coastal Flood Advisory":  CatFlood,
	"Coastal Flood Statement": CatFlood,
	// Tropical
	"Storm Surge Warning":    CatTropical,
	"Storm Surge Watch":      CatTropical,
	"Hurricane Warning":      CatTropical,
	"Hurricane Watch":        CatTropical,
	"Typhoon Warning":        CatTropical,
	"Typhoon Watch":          CatTropical,
	"Tropical Storm Warning": CatTropical,
	"Tropical Storm Watch":   CatTropical,
	// High wind
	"High Wind Warning":    CatHighWind,
	"High Wind Watch":      CatHighWind,
	"Extreme Wind Warning": CatHighWind,
	"Wind Advisory":        CatHighWind,
	// Red flag / fire weather
	"Red Flag Warning":   CatRedFlag,
	"Fire Weather Watch": CatRedFlag,
	// Winter
	"Winter Storm Warning":    CatWinter,
	"Winter Storm Watch":      CatWinter,
	"Winter Weather Advisory": CatWinter,
	"Blizzard Warning":        CatWinter,
	"Blizzard Watch":          CatWinter,
	"Ice Storm Warning":       CatWinter,
	"Snow Squall Warning":     CatWinter,
	// Extreme heat
	"Extreme Heat Warning": CatExtremeHeat,
	"Extreme Heat Watch":   CatExtremeHeat,
	"Heat Advisory":        CatExtremeHeat,
	// Extreme cold
	"Extreme Cold Warning": CatExtremeCold,
	"Extreme Cold Watch":   CatExtremeCold,
	"Freeze Warning":       CatExtremeCold,
	"Freeze Watch":         CatExtremeCold,
	"Frost Advisory":       CatExtremeCold,
	"Wind Chill Warning":   CatExtremeCold,
	"Wind Chill Watch":     CatExtremeCold,
	"Wind Chill Advisory":  CatExtremeCold,
	// Marine (offshore / coastal waters)
	"Special Marine Warning":       CatMarine,
	"Gale Warning":                 CatMarine,
	"Gale Watch":                   CatMarine,
	"Storm Warning":                CatMarine,
	"Storm Watch":                  CatMarine,
	"Hurricane Force Wind Warning": CatMarine,
	"Hazardous Seas Warning":       CatMarine,
	"Hazardous Seas Watch":         CatMarine,
}

func categorize(event, awips string) Category {
	if len(awips) >= 3 && awips[:3] == "TSU" {
		return CatTsunami
	}
	if c, ok := eventCategoryMap[event]; ok {
		return c
	}
	return CatUnknown
}

type rawFeatureCollection struct {
	Features []rawFeature `json:"features"`
}

type rawFeature struct {
	ID         string        `json:"id"`
	Properties rawProperties `json:"properties"`
	Geometry   any           `json:"geometry"`
}

type rawProperties struct {
	ID          string        `json:"id"`
	AreaDesc    string        `json:"areaDesc"`
	Sent        time.Time     `json:"sent"`
	Effective   time.Time     `json:"effective"`
	Onset       *time.Time    `json:"onset"`
	Expires     time.Time     `json:"expires"`
	Ends        *time.Time    `json:"ends"`
	Status      string        `json:"status"`
	MessageType string        `json:"messageType"`
	Severity    string        `json:"severity"`
	Event       string        `json:"event"`
	Headline    string        `json:"headline"`
	SenderName  string        `json:"senderName"`
	Geocode     rawGeocode    `json:"geocode"`
	Parameters  rawParameters `json:"parameters"`
}

type rawGeocode struct {
	UGC  []string `json:"UGC"`
	SAME []string `json:"SAME"`
}

type rawParameters struct {
	AWIPSidentifier     []string `json:"AWIPSidentifier"`
	EventTrackingNumber []string `json:"eventTrackingNumber"`
}

func parseSeverity(s string) Severity {
	switch s {
	case "Extreme":
		return SevExtreme
	case "Severe":
		return SevSevere
	case "Moderate":
		return SevModerate
	case "Minor":
		return SevMinor
	default:
		return SevUnknown
	}
}

// wfoFromAWIPS extracts the 3-letter WFO code from an AWIPS identifier.
// NWS convention: AWIPSidentifier is typically a 6-char string whose last
// 3 chars are the issuing office code (WCNTBW → TBW, FLSPAH → PAH).
// Returns "" when the AWIPS code is too short or not all-uppercase letters.
func wfoFromAWIPS(awips string) string {
	if len(awips) < 6 {
		return ""
	}
	code := awips[len(awips)-3:]
	for i := 0; i < 3; i++ {
		c := code[i]
		if c < 'A' || c > 'Z' {
			return ""
		}
	}
	return code
}

// ParseNWSAlerts decodes a NWS GeoJSON FeatureCollection response body into a
// slice of normalized Alert values. An unmapped severe/extreme event is logged
// at WARN level as a drift-canary signal.
func ParseNWSAlerts(body []byte, logger *slog.Logger) ([]Alert, error) {
	if logger == nil {
		logger = slog.Default()
	}
	var fc rawFeatureCollection
	if err := json.Unmarshal(body, &fc); err != nil {
		return nil, fmt.Errorf("nws_alerts: unmarshal: %w", err)
	}
	out := make([]Alert, 0, len(fc.Features))
	for _, f := range fc.Features {
		p := f.Properties
		awips := ""
		if len(p.Parameters.AWIPSidentifier) > 0 {
			awips = p.Parameters.AWIPSidentifier[0]
		}
		etn := ""
		if len(p.Parameters.EventTrackingNumber) > 0 {
			etn = p.Parameters.EventTrackingNumber[0]
		}
		areas := splitAreas(p.AreaDesc)
		sev := parseSeverity(p.Severity)
		cat := categorize(p.Event, awips)
		if cat == CatUnknown && (sev == SevExtreme || sev == SevSevere) {
			logger.Warn("nws_alerts.unmapped_severe_event",
				"event", p.Event,
				"severity", sev,
				"id", p.ID,
				"awips", awips,
			)
		}
		var onset, ends time.Time
		if p.Onset != nil {
			onset = *p.Onset
		}
		if p.Ends != nil {
			ends = *p.Ends
		}
		out = append(out, Alert{
			ID:        p.ID,
			Event:     p.Event,
			AWIPS:     awips,
			Headline:  p.Headline,
			Severity:  sev,
			Sent:      p.Sent,
			Effective: p.Effective,
			Onset:     onset,
			Expires:   p.Expires,
			Ends:      ends,
			Areas:     areas,
			UGCs:      append([]string(nil), p.Geocode.UGC...),
			SAMEs:     append([]string(nil), p.Geocode.SAME...),
			WFO:       wfoFromAWIPS(awips),
			VTECEtn:   etn,
			URL:       f.ID,
			Category:  cat,
		})
	}
	return out, nil
}

func splitAreas(s string) []string {
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// NWSAlerts implements Source for the NWS active alerts API.
type NWSAlerts struct {
	url       string
	userAgent string
	interval  time.Duration
	client    *http.Client
	logger    *slog.Logger
}

// NewNWSAlerts constructs an NWSAlerts source targeting the given URL with the
// provided User-Agent string.
func NewNWSAlerts(url, userAgent string, logger *slog.Logger) *NWSAlerts {
	return &NWSAlerts{
		url:       url,
		userAgent: userAgent,
		interval:  30 * time.Second,
		client:    &http.Client{Timeout: 15 * time.Second},
		logger:    logger,
	}
}

// Name returns the source identifier.
func (n *NWSAlerts) Name() string { return NWSAlertsName }

// Interval returns the polling interval for this source.
func (n *NWSAlerts) Interval() time.Duration { return n.interval }

// SetInterval updates the polling interval. Must be called before the fetcher
// loop begins (i.e., during boot, by the config walker in Task 14); not safe
// for concurrent use with Interval() or Fetch() once the fetcher is running.
func (n *NWSAlerts) SetInterval(d time.Duration) { n.interval = d }

// Fetch retrieves the current NWS active alerts and returns parsed results with
// a SHA-256 content hash as the validator for change detection.
func (n *NWSAlerts) Fetch(ctx context.Context) (FetchResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, n.url, nil)
	if err != nil {
		return FetchResult{}, fmt.Errorf("nws_alerts: build request: %w", err)
	}
	req.Header.Set("User-Agent", n.userAgent)
	req.Header.Set("Accept", "application/geo+json")
	resp, err := n.client.Do(req)
	if err != nil {
		return FetchResult{}, fmt.Errorf("nws_alerts: do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return FetchResult{}, fmt.Errorf("nws_alerts: upstream status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return FetchResult{}, fmt.Errorf("nws_alerts: read body: %w", err)
	}
	alerts, err := ParseNWSAlerts(body, n.logger)
	if err != nil {
		return FetchResult{}, err
	}
	sum := sha256.Sum256(body)
	return FetchResult{
		Payload:   alerts,
		Validator: "sha256:" + hex.EncodeToString(sum[:]),
	}, nil
}
