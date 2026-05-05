package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/jacaudi/cwd/internal/sources"
	"github.com/jacaudi/cwd/internal/store"
)

const (
	maxBucketsPerRequest    = 200
	maxQuakeEventsPerWindow = 500
	maxVolcanoChangesPerWin = 50
	minQuakeMagnitude       = 4.0
)

// validWindows maps the public window string to its duration.
var validWindows = map[string]time.Duration{
	"24h": 24 * time.Hour,
	"7d":  7 * 24 * time.Hour,
	"30d": 30 * 24 * time.Hour,
}

// validSources is the set of sources /api/history will dispatch to.
var validSources = map[string]struct{}{
	sources.NWSAlertsName:     {},
	sources.SWPCScalesName:    {},
	sources.SWPCAlertsName:    {},
	sources.USGSQuakesName:    {},
	sources.USGSVolcanoesName: {},
}

// nwsEventCounters defines which NWS upstream "event" strings count toward
// each per-event-type category surfaced on the History page. Names are matched
// case-sensitive against the upstream NWS feed's `event` field. Unmatched
// events are simply not counted (they still count toward activeCount).
var nwsEventCounters = map[string][]string{
	"tornado":      {"Tornado Warning", "Tornado Watch", "Tornado Emergency"},
	"severeTstorm": {"Severe Thunderstorm Warning", "Severe Thunderstorm Watch"},
	"flashFlood":   {"Flash Flood Warning", "Flash Flood Watch", "Flash Flood Emergency"},
}

type historyHandler struct {
	store *store.Store
}

// NewHistoryHandler returns the GET /api/history?source=<name>&window=24h|7d|30d handler.
func NewHistoryHandler(s *store.Store) http.Handler {
	return &historyHandler{store: s}
}

func (h *historyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	source := r.URL.Query().Get("source")
	window := r.URL.Query().Get("window")

	if source == "" {
		http.Error(w, "missing 'source' query parameter", http.StatusBadRequest)
		return
	}
	if _, ok := validSources[source]; !ok {
		http.Error(w, "unknown source: "+source, http.StatusBadRequest)
		return
	}
	dur, ok := validWindows[window]
	if !ok {
		http.Error(w, "invalid 'window' (want 24h|7d|30d)", http.StatusBadRequest)
		return
	}

	now := time.Now().UTC()
	from := now.Add(-dur)
	ctx := r.Context()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=60")

	switch source {
	case sources.NWSAlertsName:
		h.serveNWSAlerts(ctx, w, window, from, now)
	case sources.SWPCScalesName:
		h.serveSWPCScales(ctx, w, window, from, now)
	case sources.SWPCAlertsName:
		h.serveSWPCAlerts(ctx, w, window, from, now)
	case sources.USGSQuakesName:
		h.serveUSGSQuakes(ctx, w, window, from, now)
	case sources.USGSVolcanoesName:
		h.serveUSGSVolcanoes(ctx, w, window, from, now)
	}
}

// ─── envelope helpers ───────────────────────────────────────────────────────

type historyEnvelope struct {
	Source      string `json:"source"`
	Window      string `json:"window"`
	WindowStart string `json:"windowStart"`
	DataStart   string `json:"dataStart"`
}

// dataStartFor finds the earliest snapshot for source within (from-365d, now);
// if no rows exist returns from. Used to populate the "data starts here" marker.
func (h *historyHandler) dataStartFor(ctx context.Context, source string, from, now time.Time) time.Time {
	rows, err := h.store.Range(ctx, source, from.Add(-365*24*time.Hour), now)
	if err != nil || len(rows) == 0 {
		return from
	}
	if rows[0].FetchedAt.After(from) {
		return rows[0].FetchedAt
	}
	return from
}

// ─── NWS alerts: max(activeCount) per bucket ────────────────────────────────

type nwsAlertsBucket struct {
	At          string `json:"at"`
	ActiveCount int    `json:"activeCount"`
	EventCounts struct {
		Tornado      int `json:"tornado"`
		SevereTstorm int `json:"severeTstorm"`
		FlashFlood   int `json:"flashFlood"`
	} `json:"eventCounts"`
}

func (h *historyHandler) serveNWSAlerts(ctx context.Context, w http.ResponseWriter, window string, from, now time.Time) {
	agg := func(rows []store.Row) ([]byte, error) {
		maxCount := 0
		maxByCategory := map[string]int{"tornado": 0, "severeTstorm": 0, "flashFlood": 0}
		for _, r := range rows {
			var arr []map[string]any
			if err := json.Unmarshal(r.Payload, &arr); err != nil {
				continue
			}
			if len(arr) > maxCount {
				maxCount = len(arr)
			}
			rowByCat := map[string]int{"tornado": 0, "severeTstorm": 0, "flashFlood": 0}
			for _, alert := range arr {
				event, _ := alert["event"].(string)
				for cat, names := range nwsEventCounters {
					for _, name := range names {
						if name == event {
							rowByCat[cat]++
							break
						}
					}
				}
			}
			for cat, count := range rowByCat {
				if count > maxByCategory[cat] {
					maxByCategory[cat] = count
				}
			}
		}
		return json.Marshal(map[string]any{
			"activeCount": maxCount,
			"eventCounts": maxByCategory,
		})
	}
	buckets, err := h.store.RangeBuckets(ctx, sources.NWSAlertsName, from, now, maxBucketsPerRequest, agg)
	if err != nil {
		http.Error(w, "store: "+err.Error(), http.StatusInternalServerError)
		return
	}

	out := make([]nwsAlertsBucket, 0, len(buckets))
	for _, b := range buckets {
		var v struct {
			ActiveCount int            `json:"activeCount"`
			EventCounts map[string]int `json:"eventCounts"`
		}
		_ = json.Unmarshal(b.Payload, &v)
		bucket := nwsAlertsBucket{
			At:          b.BucketStart.UTC().Format(time.RFC3339Nano),
			ActiveCount: v.ActiveCount,
		}
		bucket.EventCounts.Tornado = v.EventCounts["tornado"]
		bucket.EventCounts.SevereTstorm = v.EventCounts["severeTstorm"]
		bucket.EventCounts.FlashFlood = v.EventCounts["flashFlood"]
		out = append(out, bucket)
	}

	dataStart := h.dataStartFor(ctx, sources.NWSAlertsName, from, now)
	_ = json.NewEncoder(w).Encode(struct {
		historyEnvelope
		Buckets []nwsAlertsBucket `json:"buckets"`
	}{
		historyEnvelope: historyEnvelope{
			Source:      sources.NWSAlertsName,
			Window:      window,
			WindowStart: from.Format(time.RFC3339Nano),
			DataStart:   dataStart.Format(time.RFC3339Nano),
		},
		Buckets: out,
	})
}

// ─── SWPC scales: last value of `today.G/R/S` per bucket ────────────────────

type swpcScalesBucket struct {
	At     string  `json:"at"`
	GScale int     `json:"gScale"`
	R1     float64 `json:"r1"`
	S1     float64 `json:"s1"`
}

func (h *historyHandler) serveSWPCScales(ctx context.Context, w http.ResponseWriter, window string, from, now time.Time) {
	agg := func(rows []store.Row) ([]byte, error) {
		// Take the LAST row in the bucket as representative.
		if len(rows) == 0 {
			return []byte(`null`), nil
		}
		last := rows[len(rows)-1]
		return last.Payload, nil
	}
	buckets, err := h.store.RangeBuckets(ctx, sources.SWPCScalesName, from, now, maxBucketsPerRequest, agg)
	if err != nil {
		http.Error(w, "store: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Decode each bucket's payload into the SWPC scales shape and pull
	// today's gScale + R1MinorProb + S1MinorProb. The shape produced by
	// internal/sources/swpc_scales.go is {today: {gScale, r1MinorProb, s1MinorProb}, ...}.
	out := make([]swpcScalesBucket, 0, len(buckets))
	for _, b := range buckets {
		var p struct {
			Today struct {
				GScale int     `json:"gScale"`
				R1     float64 `json:"r1MinorProb"`
				S1     float64 `json:"s1MinorProb"`
			} `json:"today"`
		}
		if err := json.Unmarshal(b.Payload, &p); err != nil {
			continue
		}
		out = append(out, swpcScalesBucket{
			At:     b.BucketStart.UTC().Format(time.RFC3339Nano),
			GScale: p.Today.GScale,
			R1:     p.Today.R1,
			S1:     p.Today.S1,
		})
	}

	dataStart := h.dataStartFor(ctx, sources.SWPCScalesName, from, now)
	_ = json.NewEncoder(w).Encode(struct {
		historyEnvelope
		Buckets []swpcScalesBucket `json:"buckets"`
	}{
		historyEnvelope: historyEnvelope{
			Source:      sources.SWPCScalesName,
			Window:      window,
			WindowStart: from.Format(time.RFC3339Nano),
			DataStart:   dataStart.Format(time.RFC3339Nano),
		},
		Buckets: out,
	})
}

// ─── SWPC alerts: count by severity per bucket ──────────────────────────────

type swpcAlertsBucket struct {
	At      string `json:"at"`
	Warning int    `json:"warning"`
	Watch   int    `json:"watch"`
	Alert   int    `json:"alert"`
}

func (h *historyHandler) serveSWPCAlerts(ctx context.Context, w http.ResponseWriter, window string, from, now time.Time) {
	agg := func(rows []store.Row) ([]byte, error) {
		var warning, watch, alert int
		// "Last row in the bucket wins" — alerts are cumulative within the window
		// each fetcher pull, so summing across snapshots would double-count.
		if len(rows) > 0 {
			var arr []struct {
				Severity string `json:"severity"`
			}
			if err := json.Unmarshal(rows[len(rows)-1].Payload, &arr); err == nil {
				for _, e := range arr {
					switch e.Severity {
					case "Warning", "WARNING", "warning":
						warning++
					case "Watch", "WATCH", "watch":
						watch++
					default:
						alert++
					}
				}
			}
		}
		return json.Marshal(map[string]int{"warning": warning, "watch": watch, "alert": alert})
	}
	buckets, err := h.store.RangeBuckets(ctx, sources.SWPCAlertsName, from, now, maxBucketsPerRequest, agg)
	if err != nil {
		http.Error(w, "store: "+err.Error(), http.StatusInternalServerError)
		return
	}

	out := make([]swpcAlertsBucket, 0, len(buckets))
	for _, b := range buckets {
		var v map[string]int
		_ = json.Unmarshal(b.Payload, &v)
		out = append(out, swpcAlertsBucket{
			At:      b.BucketStart.UTC().Format(time.RFC3339Nano),
			Warning: v["warning"],
			Watch:   v["watch"],
			Alert:   v["alert"],
		})
	}

	dataStart := h.dataStartFor(ctx, sources.SWPCAlertsName, from, now)
	_ = json.NewEncoder(w).Encode(struct {
		historyEnvelope
		Buckets []swpcAlertsBucket `json:"buckets"`
	}{
		historyEnvelope: historyEnvelope{
			Source:      sources.SWPCAlertsName,
			Window:      window,
			WindowStart: from.Format(time.RFC3339Nano),
			DataStart:   dataStart.Format(time.RFC3339Nano),
		},
		Buckets: out,
	})
}

// ─── USGS quakes: raw events, M ≥ 4, capped at 500 ──────────────────────────

type usgsQuakeEvent struct {
	At      string  `json:"at"`
	Mag     float64 `json:"mag"`
	Place   string  `json:"place"`
	DepthKm float64 `json:"depthKm"`
}

func (h *historyHandler) serveUSGSQuakes(ctx context.Context, w http.ResponseWriter, window string, from, now time.Time) {
	rows, err := h.store.Range(ctx, sources.USGSQuakesName, from, now)
	if err != nil {
		http.Error(w, "store: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// USGS payload shape: GeoJSON FeatureCollection with features[].properties.{mag, place, time}
	// and features[].geometry.coordinates[lon, lat, depth].
	type prop struct {
		Mag   float64 `json:"mag"`
		Place string  `json:"place"`
		Time  int64   `json:"time"` // unix milliseconds
	}
	type geom struct {
		Coordinates []float64 `json:"coordinates"`
	}
	type feat struct {
		Properties prop `json:"properties"`
		Geometry   geom `json:"geometry"`
	}
	type fc struct {
		Features []feat `json:"features"`
	}

	// Dedupe across snapshots by (mag, place, time) — successive polls
	// repeat the same recent quakes.
	type quakeKey struct {
		Place string
		Time  int64
		Mag   float64
	}
	seen := make(map[quakeKey]struct{})
	out := make([]usgsQuakeEvent, 0, 64)
	for _, r := range rows {
		var c fc
		if err := json.Unmarshal(r.Payload, &c); err != nil {
			continue
		}
		for _, f := range c.Features {
			if f.Properties.Mag < minQuakeMagnitude {
				continue
			}
			k := quakeKey{Place: f.Properties.Place, Time: f.Properties.Time, Mag: f.Properties.Mag}
			if _, dup := seen[k]; dup {
				continue
			}
			seen[k] = struct{}{}
			depth := 0.0
			if len(f.Geometry.Coordinates) >= 3 {
				depth = f.Geometry.Coordinates[2]
			}
			out = append(out, usgsQuakeEvent{
				At:      time.UnixMilli(f.Properties.Time).UTC().Format(time.RFC3339Nano),
				Mag:     f.Properties.Mag,
				Place:   f.Properties.Place,
				DepthKm: depth,
			})
			if len(out) >= maxQuakeEventsPerWindow {
				break
			}
		}
		if len(out) >= maxQuakeEventsPerWindow {
			break
		}
	}
	// Stable sort by time ascending so charts can read in order.
	sort.Slice(out, func(i, j int) bool { return out[i].At < out[j].At })

	dataStart := h.dataStartFor(ctx, sources.USGSQuakesName, from, now)
	_ = json.NewEncoder(w).Encode(struct {
		historyEnvelope
		Events []usgsQuakeEvent `json:"events"`
	}{
		historyEnvelope: historyEnvelope{
			Source:      sources.USGSQuakesName,
			Window:      window,
			WindowStart: from.Format(time.RFC3339Nano),
			DataStart:   dataStart.Format(time.RFC3339Nano),
		},
		Events: out,
	})
}

// ─── USGS volcanoes: state-change diff feed ─────────────────────────────────

type usgsVolcanoChange struct {
	At      string `json:"at"`
	Volcano string `json:"volcano"`
	Prior   string `json:"prior"`
	Current string `json:"current"`
}

func (h *historyHandler) serveUSGSVolcanoes(ctx context.Context, w http.ResponseWriter, window string, from, now time.Time) {
	changes, err := h.store.VolcanoStateChanges(ctx, from, now, maxVolcanoChangesPerWin)
	if err != nil {
		http.Error(w, "store: "+err.Error(), http.StatusInternalServerError)
		return
	}

	out := make([]usgsVolcanoChange, 0, len(changes))
	for _, c := range changes {
		out = append(out, usgsVolcanoChange{
			At:      c.At.UTC().Format(time.RFC3339Nano),
			Volcano: c.Volcano,
			Prior:   c.Prior,
			Current: c.Current,
		})
	}

	dataStart := h.dataStartFor(ctx, sources.USGSVolcanoesName, from, now)
	_ = json.NewEncoder(w).Encode(struct {
		historyEnvelope
		Changes []usgsVolcanoChange `json:"changes"`
	}{
		historyEnvelope: historyEnvelope{
			Source:      sources.USGSVolcanoesName,
			Window:      window,
			WindowStart: from.Format(time.RFC3339Nano),
			DataStart:   dataStart.Format(time.RFC3339Nano),
		},
		Changes: out,
	})
}
