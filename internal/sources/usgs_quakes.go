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

// USGSQuakesName is the canonical source name for the USGS earthquake feed.
const USGSQuakesName = "usgs_quakes"

// Quake is a normalized USGS earthquake event.
type Quake struct {
	ID        string    `json:"id"`
	Magnitude float64   `json:"magnitude"`
	Place     string    `json:"place"`
	Time      time.Time `json:"time"`
	UpdatedAt time.Time `json:"updatedAt"`
	Lat       float64   `json:"lat"`
	Lon       float64   `json:"lon"`
	DepthKm   float64   `json:"depthKm"`
	Tsunami   bool      `json:"tsunami"`
	Alert     string    `json:"alert,omitempty"`
	URL       string    `json:"url,omitempty"`
}

// USGS earthquake feeds — operator menu.
//
// We use significant_day.geojson because USGS already curates it to
// newsworthy events (M4.5+ globally, plus felt events ≥ M2.5 in CONUS,
// plus tsunami-flagged quakes regardless of magnitude). Polled every 60s
// matches USGS Cache-Control. Each feed: same GeoJSON shape, different cutoffs.
//
// Alternatives (not used in v1; flip the URL constant to switch):
//
//	significant_hour.geojson      — past hour, ~1-3 events
//	significant_day.geojson       — past 24h, ~5-20 events                  [DEFAULT]
//	significant_week.geojson      — past 7 days
//	significant_month.geojson     — past 30 days
//	4.5_day.geojson               — all M4.5+, ~30-60/day
//	2.5_day.geojson               — all M2.5+, ~150/day
//	1.0_day.geojson               — all M1.0+, ~500/day (CONUS heavy)
//	all_day.geojson               — every detection, ~5000/day
//
// If you switch to a higher-volume feed, add a server-side threshold:
//
//	derived:
//	  thresholds:
//	    quake_min_magnitude: 5.0          # drop everything below
//	    quake_max_depth_km:  300          # drop deep-focus events
//
// (Knob shape is reserved; not wired in v1.)
type rawQuakeFC struct {
	Features []rawQuakeFeature `json:"features"`
}

type rawQuakeFeature struct {
	ID         string        `json:"id"`
	Properties rawQuakeProps `json:"properties"`
	Geometry   rawPointGeom  `json:"geometry"`
}

type rawQuakeProps struct {
	Mag     float64 `json:"mag"`
	Place   string  `json:"place"`
	Time    int64   `json:"time"`
	Updated int64   `json:"updated"`
	Tsunami int     `json:"tsunami"`
	Alert   *string `json:"alert"`
	URL     string  `json:"url"`
}

type rawPointGeom struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"`
}

// ParseUSGSQuakes decodes a GeoJSON FeatureCollection of significant_day.geojson
// shape and returns a slice sorted by magnitude descending.
func ParseUSGSQuakes(body []byte, logger *slog.Logger) ([]Quake, error) {
	_ = logger // reserved for future per-feature warn-logging; v1 parser is silent
	var fc rawQuakeFC
	if err := json.Unmarshal(body, &fc); err != nil {
		return nil, fmt.Errorf("usgs_quakes: unmarshal: %w", err)
	}
	out := make([]Quake, 0, len(fc.Features))
	for _, f := range fc.Features {
		var lat, lon, depth float64
		if len(f.Geometry.Coordinates) >= 3 {
			lon = f.Geometry.Coordinates[0]
			lat = f.Geometry.Coordinates[1]
			depth = f.Geometry.Coordinates[2]
		}
		alert := ""
		if f.Properties.Alert != nil {
			alert = *f.Properties.Alert
		}
		out = append(out, Quake{
			ID:        f.ID,
			Magnitude: f.Properties.Mag,
			Place:     f.Properties.Place,
			Time:      time.UnixMilli(f.Properties.Time).UTC(),
			UpdatedAt: time.UnixMilli(f.Properties.Updated).UTC(),
			Lat:       lat,
			Lon:       lon,
			DepthKm:   depth,
			Tsunami:   f.Properties.Tsunami != 0,
			Alert:     alert,
			URL:       f.Properties.URL,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Magnitude > out[j].Magnitude })
	return out, nil
}

// USGSQuakes implements Source for USGS significant_day.geojson, with
// upstream-ETag passthrough so 304 responses keep the cached payload.
type USGSQuakes struct {
	url       string
	userAgent string
	interval  time.Duration
	client    *http.Client
	logger    *slog.Logger

	mu         sync.Mutex
	lastETag   string
	lastQuakes []Quake
}

// NewUSGSQuakes constructs a USGSQuakes source.
func NewUSGSQuakes(url, userAgent string, logger *slog.Logger) *USGSQuakes {
	return &USGSQuakes{
		url:       url,
		userAgent: userAgent,
		interval:  60 * time.Second,
		client:    &http.Client{Timeout: 15 * time.Second},
		logger:    logger,
	}
}

// Name returns the source identifier.
func (q *USGSQuakes) Name() string { return USGSQuakesName }

// Interval returns the polling interval.
func (q *USGSQuakes) Interval() time.Duration { return q.interval }

// SetInterval is the boot-time setter for the config env walker.
func (q *USGSQuakes) SetInterval(d time.Duration) { q.interval = d }

// Fetch retrieves the USGS feed with If-None-Match passthrough. On 304 it
// returns the prior payload + the prior ETag (the fetcher's diff-on-write
// then sees identical Validator and skips broadcast).
func (q *USGSQuakes) Fetch(ctx context.Context) (FetchResult, error) {
	q.mu.Lock()
	priorETag := q.lastETag
	priorPayload := append([]Quake(nil), q.lastQuakes...)
	q.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, q.url, nil)
	if err != nil {
		return FetchResult{}, fmt.Errorf("usgs_quakes: build request: %w", err)
	}
	req.Header.Set("User-Agent", q.userAgent)
	req.Header.Set("Accept", "application/geo+json")
	if priorETag != "" {
		req.Header.Set("If-None-Match", priorETag)
	}
	resp, err := q.client.Do(req)
	if err != nil {
		return FetchResult{}, fmt.Errorf("usgs_quakes: do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotModified {
		return FetchResult{Payload: priorPayload, Validator: priorETag}, nil
	}
	if resp.StatusCode/100 != 2 {
		return FetchResult{}, fmt.Errorf("usgs_quakes: upstream status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return FetchResult{}, fmt.Errorf("usgs_quakes: read body: %w", err)
	}
	quakes, err := ParseUSGSQuakes(body, q.logger)
	if err != nil {
		return FetchResult{}, err
	}
	etag := resp.Header.Get("ETag")
	q.mu.Lock()
	q.lastETag = etag
	q.lastQuakes = append([]Quake(nil), quakes...)
	q.mu.Unlock()
	return FetchResult{Payload: quakes, Validator: etag}, nil
}
