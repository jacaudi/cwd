package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"sync"
	"time"
)

// USGSVolcanoesName is the canonical source name for the USGS volcano feed.
const USGSVolcanoesName = "usgs_volcanoes"

// AlertLevel is the USGS volcanic alert level.
type AlertLevel string

// ColorCode is the USGS aviation color code.
type ColorCode string

// Volcano alert levels (NORMAL is filtered out before storage).
const (
	AlertNORMAL   AlertLevel = "NORMAL"
	AlertADVISORY AlertLevel = "ADVISORY"
	AlertWATCH    AlertLevel = "WATCH"
	AlertWARNING  AlertLevel = "WARNING"
)

// Volcano is a normalized non-NORMAL USGS volcano entry.
//
// Design §6.4 sketched lat/lon/synopsis fields, but the chosen
// `getElevatedVolcanoes` upstream does not return them. Region is derived
// from the upstream `obs_fullname` (e.g. "Alaska Volcano Observatory").
// Lat/lon/synopsis would require N+1 fetches against the per-notice
// `notice_data` URL — out of scope for v1.
type Volcano struct {
	ID        string     `json:"id"`            // upstream "vnum"
	Name      string     `json:"name"`          // upstream "volcano_name"
	Region    string     `json:"region"`        // derived from upstream "obs_fullname"
	Alert     AlertLevel `json:"alert"`         // upstream "alert_level"
	Color     ColorCode  `json:"color"`         // upstream "color_code"
	UpdatedAt time.Time  `json:"updatedAt"`     // upstream "sent_utc"
	URL       string     `json:"url,omitempty"` // upstream "notice_url"
}

// USGS volcano feeds — operator menu.
//
// Primary: hans-public/api/volcano/getElevatedVolcanoes (JSON, ETag).
// Cleaner contract than the legacy RSS feed: structured fields, no XML
// dependency, fewer regex-heavy edge cases.
//
// Legacy fallback (vhpss/api/volcano/elevatedRSS — ATOM/RSS, ETag) exists
// and is what the parent design originally listed. If the JSON endpoint is
// ever deprecated upstream, swap to the RSS feed and add a small XML-to-
// Volcano parser. Polling cadence stays 5min.
// rawVolcano matches the actual `getElevatedVolcanoes` JSON shape
// (verified live 2026-05-02 — snake_case keys).
type rawVolcano struct {
	Vnum         string `json:"vnum"`
	VolcanoName  string `json:"volcano_name"`
	ObsFullname  string `json:"obs_fullname"`
	ObsAbbr      string `json:"obs_abbr"`
	AlertLevel   string `json:"alert_level"`
	ColorCode    string `json:"color_code"`
	SentUTC      string `json:"sent_utc"`
	SentUnixtime int64  `json:"sent_unixtime"`
	NoticeURL    string `json:"notice_url"`
}

// usgsSentLayout is the upstream `sent_utc` shape — "YYYY-MM-DD HH:MM:SS"
// (no timezone suffix; documented by field name as UTC).
const usgsSentLayout = "2006-01-02 15:04:05"

// ParseUSGSVolcanoes decodes the getElevatedVolcanoes JSON array, drops
// NORMAL-level entries, and sorts by region then name. Bad timestamps fall
// back to sent_unixtime (or zero time) rather than dropping the entry.
func ParseUSGSVolcanoes(body []byte, logger *slog.Logger) ([]Volcano, error) {
	if logger == nil {
		logger = slog.Default()
	}
	var raw []rawVolcano
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("usgs_volcanoes: unmarshal: %w", err)
	}
	out := make([]Volcano, 0, len(raw))
	for _, r := range raw {
		alert := AlertLevel(r.AlertLevel)
		if alert == AlertNORMAL || alert == "" {
			continue
		}
		ts, err := time.Parse(usgsSentLayout, r.SentUTC)
		if err != nil {
			if r.SentUnixtime > 0 {
				ts = time.Unix(r.SentUnixtime, 0)
			} else {
				logger.Debug("usgs_volcanoes.bad_timestamp", "volcano", r.VolcanoName, "value", r.SentUTC)
				ts = time.Time{}
			}
		}
		out = append(out, Volcano{
			ID:        r.Vnum,
			Name:      r.VolcanoName,
			Region:    r.ObsFullname,
			Alert:     alert,
			Color:     ColorCode(r.ColorCode),
			UpdatedAt: ts.UTC(),
			URL:       r.NoticeURL,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Region != out[j].Region {
			return out[i].Region < out[j].Region
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// USGSVolcanoes implements Source for the USGS getElevatedVolcanoes endpoint.
type USGSVolcanoes struct {
	url       string
	userAgent string
	interval  time.Duration
	client    *http.Client
	logger    *slog.Logger

	mu        sync.Mutex
	lastETag  string
	lastVolcs []Volcano
}

// NewUSGSVolcanoes constructs a USGSVolcanoes source.
func NewUSGSVolcanoes(url, userAgent string, logger *slog.Logger) *USGSVolcanoes {
	return &USGSVolcanoes{
		url:       url,
		userAgent: userAgent,
		interval:  5 * time.Minute,
		client:    &http.Client{Timeout: 15 * time.Second},
		logger:    logger,
	}
}

// Name returns the source identifier.
func (v *USGSVolcanoes) Name() string { return USGSVolcanoesName }

// Interval returns the polling interval.
func (v *USGSVolcanoes) Interval() time.Duration { return v.interval }

// SetInterval is the boot-time setter for the config env walker.
func (v *USGSVolcanoes) SetInterval(d time.Duration) { v.interval = d }

// Fetch retrieves the USGS volcano feed with If-None-Match passthrough.
func (v *USGSVolcanoes) Fetch(ctx context.Context) (FetchResult, error) {
	v.mu.Lock()
	priorETag := v.lastETag
	priorPayload := append([]Volcano(nil), v.lastVolcs...)
	v.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.url, nil)
	if err != nil {
		return FetchResult{}, fmt.Errorf("usgs_volcanoes: build request: %w", err)
	}
	req.Header.Set("User-Agent", v.userAgent)
	req.Header.Set("Accept", "application/json")
	if priorETag != "" {
		req.Header.Set("If-None-Match", priorETag)
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return FetchResult{}, fmt.Errorf("usgs_volcanoes: do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotModified {
		return FetchResult{Payload: priorPayload, Validator: priorETag}, nil
	}
	if resp.StatusCode/100 != 2 {
		return FetchResult{}, fmt.Errorf("usgs_volcanoes: upstream status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return FetchResult{}, fmt.Errorf("usgs_volcanoes: read body: %w", err)
	}
	volcs, err := ParseUSGSVolcanoes(body, v.logger)
	if err != nil {
		return FetchResult{}, err
	}
	etag := resp.Header.Get("ETag")
	v.mu.Lock()
	v.lastETag = etag
	v.lastVolcs = append([]Volcano(nil), volcs...)
	v.mu.Unlock()
	return FetchResult{Payload: volcs, Validator: etag}, nil
}
