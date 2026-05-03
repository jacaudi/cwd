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
	"strconv"
	"time"
)

// SWPCScalesName is the canonical source name for the SWPC NOAA-Scales 3-day forecast.
const SWPCScalesName = "swpc_scales"

// GScale is a geomagnetic G-scale label ("G1".."G5"); zero scale returns "".
type GScale string

// SWPCDay is a single day of the SWPC NOAA scales forecast.
type SWPCDay struct {
	Date  string `json:"date"`            // "YYYY-MM-DD" UTC, copied from upstream DateStamp
	R1    int    `json:"r1"`              // % chance R1+ event (R MinorProb)
	R3    int    `json:"r3"`              // % chance R3+ event (R MajorProb)
	S1    int    `json:"s1"`              // % chance S1+ event (S Prob)
	G     GScale `json:"g,omitempty"`     // worst predicted G-scale (G1..G5, or "" when scale=0)
	GText string `json:"gText,omitempty"` // free-text note (G.Text)
}

// SWPCForecast is the parsed wire shape of /products/noaa-scales.json,
// covering the 3-day forecast (indices "1","2","3" upstream). The
// "current" (index "0") and "yesterday" (index "-1") slots are deliberately
// skipped in v1 — Phase 6 is when they get rendered.
type SWPCForecast struct {
	Days [3]SWPCDay `json:"days"`
}

// rawSWPC is the upstream JSON shape: a JSON object keyed by day-offset
// (string ints "-1".."3"). All sub-values are strings (SWPC quirk).
type rawSWPC map[string]rawSWPCDay

type rawSWPCDay struct {
	DateStamp string   `json:"DateStamp"`
	R         rawSWPCR `json:"R"`
	S         rawSWPCS `json:"S"`
	G         rawSWPCG `json:"G"`
}

type rawSWPCR struct {
	MinorProb string `json:"MinorProb"`
	MajorProb string `json:"MajorProb"`
}

type rawSWPCS struct {
	Prob string `json:"Prob"`
}

type rawSWPCG struct {
	Scale string `json:"Scale"`
	Text  string `json:"Text"`
}

func atoiOrZero(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

func gScaleFrom(s string) GScale {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return ""
	}
	if n > 5 {
		n = 5
	}
	return GScale("G" + strconv.Itoa(n))
}

// ParseSWPCScales decodes the noaa-scales.json body into a SWPCForecast covering
// indices "1","2","3" (the 3-day forecast). Missing day keys produce zero-valued
// SWPCDay slots; malformed JSON returns an error. Logger is reserved for future
// drift-canary use; nil is tolerated.
func ParseSWPCScales(body []byte, logger *slog.Logger) (SWPCForecast, error) {
	// logger reserved for future drift-canary use; currently unused but kept in
	// the signature for symmetry with ParseNWSAlerts and to absorb the change
	// when drift logging lands.
	_ = logger
	var raw rawSWPC
	if err := json.Unmarshal(body, &raw); err != nil {
		return SWPCForecast{}, fmt.Errorf("swpc_scales: unmarshal: %w", err)
	}
	var out SWPCForecast
	for i, key := range []string{"1", "2", "3"} {
		d, ok := raw[key]
		if !ok {
			continue // missing day → zero-valued slot
		}
		out.Days[i] = SWPCDay{
			Date:  d.DateStamp,
			R1:    atoiOrZero(d.R.MinorProb),
			R3:    atoiOrZero(d.R.MajorProb),
			S1:    atoiOrZero(d.S.Prob),
			G:     gScaleFrom(d.G.Scale),
			GText: d.G.Text,
		}
	}
	return out, nil
}

// SWPCScales implements Source for the SWPC NOAA-scales 3-day forecast.
type SWPCScales struct {
	url       string
	userAgent string
	interval  time.Duration
	client    *http.Client
	logger    *slog.Logger
}

// NewSWPCScales constructs a SWPCScales source.
func NewSWPCScales(url, userAgent string, logger *slog.Logger) *SWPCScales {
	return &SWPCScales{
		url:       url,
		userAgent: userAgent,
		interval:  60 * time.Second,
		client:    &http.Client{Timeout: 15 * time.Second},
		logger:    logger,
	}
}

// Name returns the source identifier.
func (s *SWPCScales) Name() string { return SWPCScalesName }

// Interval returns the polling interval.
func (s *SWPCScales) Interval() time.Duration { return s.interval }

// SetInterval is the boot-time setter used by the config env walker. Not safe
// for concurrent use once the fetcher is running.
func (s *SWPCScales) SetInterval(d time.Duration) { s.interval = d }

// Fetch retrieves the current SWPC NOAA-scales forecast and returns a SWPCForecast
// payload + sha256 content hash validator. SWPC's noaa-scales endpoint sends only
// Cache-Control: max-age=60 (no usable ETag), so content-hash is the right
// validator strategy here.
func (s *SWPCScales) Fetch(ctx context.Context) (FetchResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return FetchResult{}, fmt.Errorf("swpc_scales: build request: %w", err)
	}
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return FetchResult{}, fmt.Errorf("swpc_scales: do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return FetchResult{}, fmt.Errorf("swpc_scales: upstream status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return FetchResult{}, fmt.Errorf("swpc_scales: read body: %w", err)
	}
	fc, err := ParseSWPCScales(body, s.logger)
	if err != nil {
		return FetchResult{}, err
	}
	sum := sha256.Sum256(body)
	return FetchResult{
		Payload:   fc,
		Validator: "sha256:" + hex.EncodeToString(sum[:]),
	}, nil
}
