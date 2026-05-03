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

// SWPCAlertsName is the canonical source name for the SWPC alerts feed.
const SWPCAlertsName = "swpc_alerts"

// swpcMessageMaxBytes truncates upstream message bodies to keep wire payloads small.
const swpcMessageMaxBytes = 512

// swpcIssueDatetimeLayout is the upstream timestamp shape: "2026-05-02 10:00:00.000".
const swpcIssueDatetimeLayout = "2006-01-02 15:04:05.000"

// SWPCAlert is a parsed SWPC alert (post-allowlist, post-window-filter, post-truncate).
type SWPCAlert struct {
	Code        string    `json:"code"`          // upstream product_id, e.g. "K08A", "P12A", "WARK04W"
	Series      string    `json:"series"`        // resolved label family, e.g. "K-Index", "Proton-Event"
	Description string    `json:"description"`   // resolved short human label; "" for unknown codes
	Issued      time.Time `json:"issued"`        // upstream issue_datetime parsed as UTC
	Message     string    `json:"message"`       // first 512 bytes of upstream message
	URL         string    `json:"url,omitempty"` // reserved for future per-product permalinks
}

// SWPC product code namespace (operator menu — NOT all of these are in the
// default swpc_alert_products allowlist; extend your config to opt in).
//
// K-Index series (planetary geomagnetic, K = 0..9):
//
//	K04A — K-index of 4 (active conditions; not yet a storm)
//	K05A — K-index of 5 = G1 minor storm
//	K06A — K-index of 6 = G2 moderate storm
//	K07A — K-index of 7 = G3 strong storm
//	K08A — K-index of 8 = G4 severe storm                          [DEFAULT]
//	K09A — K-index of 9 = G5 extreme storm                         [DEFAULT]
//
// Geomagnetic warnings + watches (forecasts, not retrospective):
//
//	WARK04W — K-index ≥ 4 expected (warning)
//	WARK05W — K-index ≥ 5 expected
//	WARK06W — K-index ≥ 6 expected
//	WARK07W — K-index ≥ 7 expected
//	WARK08W — K-index ≥ 8 expected
//	WARK09W — K-index ≥ 9 expected
//	WATA50W — Geomagnetic A-index ≥ 50 watch
//
// Solar proton event (≥10 MeV flux at GOES, in pfu):
//
//	P10A — ≥ 10 pfu (S1)
//	P11A — ≥ 100 pfu (S2)
//	P12A — ≥ 1,000 pfu (S2/S3 boundary)                            [DEFAULT]
//	P13A — ≥ 10,000 pfu (S3)                                       [DEFAULT]
//	P14A — ≥ 100,000 pfu (S4)
//	P15A — ≥ 1,000,000 pfu (S5)
//
// Radio blackout / X-ray flare:
//
//	RWAR  — Radio blackout warning
//	X1XW  — X-ray flux ≥ X1 (R3 radio blackout)
//	X10XW — X-ray flux ≥ X10 (R5)
//
// Type II / Type IV radio sweeps (CME signatures):
//
//	SUM01R — Type II radio sweep
//	SUM02R — Type IV radio sweep
//
// SWPC summary / discussion:
//
//	SUM01D — Daily 3-day forecast discussion
//
// To extend coverage, add codes to derived.thresholds.swpc_alert_products
// and (if needed) add an entry to swpcCodeTable below for the human label.
var swpcCodeTable = map[string]struct{ Series, Description string }{
	"K04A":    {"K-Index", "K-index 4 (active)"},
	"K05A":    {"K-Index", "K-index 5 = G1 minor"},
	"K06A":    {"K-Index", "K-index 6 = G2 moderate"},
	"K07A":    {"K-Index", "K-index 7 = G3 strong"},
	"K08A":    {"K-Index", "K-index 8 = G4 severe"},
	"K09A":    {"K-Index", "K-index 9 = G5 extreme"},
	"WARK04W": {"Geomagnetic-Watch", "K≥4 expected"},
	"WARK05W": {"Geomagnetic-Watch", "K≥5 expected"},
	"WARK06W": {"Geomagnetic-Watch", "K≥6 expected"},
	"WARK07W": {"Geomagnetic-Watch", "K≥7 expected"},
	"WARK08W": {"Geomagnetic-Watch", "K≥8 expected"},
	"WARK09W": {"Geomagnetic-Watch", "K≥9 expected"},
	"WATA50W": {"Geomagnetic-Watch", "A≥50 watch"},
	"P10A":    {"Proton-Event", "≥10 pfu (S1)"},
	"P11A":    {"Proton-Event", "≥100 pfu (S2)"},
	"P12A":    {"Proton-Event", "≥1,000 pfu"},
	"P13A":    {"Proton-Event", "≥10,000 pfu (S3)"},
	"P14A":    {"Proton-Event", "≥100,000 pfu (S4)"},
	"P15A":    {"Proton-Event", "≥1,000,000 pfu (S5)"},
	"RWAR":    {"Radio-Blackout", "Radio blackout warning"},
	"X1XW":    {"X-Ray-Flare", "X-ray flux ≥ X1 (R3)"},
	"X10XW":   {"X-Ray-Flare", "X-ray flux ≥ X10 (R5)"},
	"SUM01R":  {"Radio-Sweep", "Type II radio sweep"},
	"SUM02R":  {"Radio-Sweep", "Type IV radio sweep"},
	"SUM01D":  {"Discussion", "Daily 3-day forecast discussion"},
}

type rawSWPCAlert struct {
	ProductID     string `json:"product_id"`
	IssueDatetime string `json:"issue_datetime"`
	Message       string `json:"message"`
}

// ParseSWPCAlerts decodes alerts.json, drops entries whose product_id is not in
// allow, drops entries older than window relative to now(), resolves the
// series+description from swpcCodeTable, and truncates message to 512 bytes.
// Bad timestamps cause the entry to be dropped (defensive).
func ParseSWPCAlerts(body []byte, allow []string, window time.Duration, now func() time.Time, logger *slog.Logger) ([]SWPCAlert, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if now == nil {
		now = time.Now
	}
	allowSet := make(map[string]struct{}, len(allow))
	for _, c := range allow {
		allowSet[c] = struct{}{}
	}
	var raw []rawSWPCAlert
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("swpc_alerts: unmarshal: %w", err)
	}
	cutoff := now().Add(-window)
	out := make([]SWPCAlert, 0, len(raw))
	for _, r := range raw {
		if _, ok := allowSet[r.ProductID]; !ok {
			continue
		}
		issued, err := time.Parse(swpcIssueDatetimeLayout, r.IssueDatetime)
		if err != nil {
			logger.Debug("swpc_alerts.bad_timestamp", "product_id", r.ProductID, "value", r.IssueDatetime, "err", err.Error())
			continue
		}
		issued = issued.UTC()
		if issued.Before(cutoff) {
			continue
		}
		msg := r.Message
		if len(msg) > swpcMessageMaxBytes {
			// SWPC messages are ASCII in practice, but slice on a byte boundary
			// can still split a multi-byte rune if upstream ever ships UTF-8.
			// strings.ToValidUTF8 drops the broken trailing sequence (if any).
			msg = strings.ToValidUTF8(msg[:swpcMessageMaxBytes], "")
		}
		entry := SWPCAlert{
			Code:    r.ProductID,
			Issued:  issued,
			Message: msg,
		}
		if meta, ok := swpcCodeTable[r.ProductID]; ok {
			entry.Series = meta.Series
			entry.Description = meta.Description
		}
		out = append(out, entry)
	}
	return out, nil
}

// SWPCAlerts implements Source for the SWPC alerts feed.
type SWPCAlerts struct {
	url       string
	userAgent string
	allow     []string
	window    time.Duration
	now       func() time.Time
	interval  time.Duration
	client    *http.Client
	logger    *slog.Logger
}

// NewSWPCAlerts constructs a SWPCAlerts source. allow is the configured
// product-id allowlist (derived.thresholds.swpc_alert_products); window is
// the configured age cutoff (derived.thresholds.swpc_alert_window_hours).
func NewSWPCAlerts(url, userAgent string, allow []string, window time.Duration, logger *slog.Logger) *SWPCAlerts {
	return &SWPCAlerts{
		url:       url,
		userAgent: userAgent,
		allow:     append([]string(nil), allow...),
		window:    window,
		now:       time.Now,
		interval:  60 * time.Second,
		client:    &http.Client{Timeout: 15 * time.Second},
		logger:    logger,
	}
}

// Name returns the source identifier.
func (s *SWPCAlerts) Name() string { return SWPCAlertsName }

// Interval returns the polling interval.
func (s *SWPCAlerts) Interval() time.Duration { return s.interval }

// SetInterval is the boot-time setter used by the config env walker.
func (s *SWPCAlerts) SetInterval(d time.Duration) { s.interval = d }

// Fetch retrieves the SWPC alerts feed and returns a filtered slice + sha256 validator.
// Like noaa-scales, the alerts.json endpoint sends Cache-Control: max-age=60 with no
// usable ETag — content-hash is the right validator strategy.
func (s *SWPCAlerts) Fetch(ctx context.Context) (FetchResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return FetchResult{}, fmt.Errorf("swpc_alerts: build request: %w", err)
	}
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return FetchResult{}, fmt.Errorf("swpc_alerts: do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return FetchResult{}, fmt.Errorf("swpc_alerts: upstream status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return FetchResult{}, fmt.Errorf("swpc_alerts: read body: %w", err)
	}
	alerts, err := ParseSWPCAlerts(body, s.allow, s.window, s.now, s.logger)
	if err != nil {
		return FetchResult{}, err
	}
	// Hash the parsed payload (not the raw body) so the validator changes when
	// the window filter ages an alert out — even if upstream stops publishing
	// updates and the body bytes stay identical. Otherwise cache.Set sees an
	// unchanged validator and never broadcasts the disappearance to SSE clients.
	// json.Marshal of []SWPCAlert is canonical: slice order is preserved by the
	// parser and struct field order is fixed at compile time.
	canon, err := json.Marshal(alerts)
	if err != nil {
		return FetchResult{}, fmt.Errorf("swpc_alerts: marshal payload for validator: %w", err)
	}
	sum := sha256.Sum256(canon)
	return FetchResult{
		Payload:   alerts,
		Validator: "sha256:" + hex.EncodeToString(sum[:]),
	}, nil
}
