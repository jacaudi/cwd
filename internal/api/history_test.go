package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jacaudi/cwd/internal/store"
)

func newTestStoreForAPI(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return s
}

func TestHistoryHandler_BadRequest_MissingSource(t *testing.T) {
	s := newTestStoreForAPI(t)
	h := NewHistoryHandler(s)
	req := httptest.NewRequest(http.MethodGet, "/api/history?window=24h", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if got, want := w.Code, http.StatusBadRequest; got != want {
		t.Errorf("Code = %d, want %d", got, want)
	}
}

func TestHistoryHandler_BadRequest_UnknownSource(t *testing.T) {
	s := newTestStoreForAPI(t)
	h := NewHistoryHandler(s)
	req := httptest.NewRequest(http.MethodGet, "/api/history?source=mystery&window=24h", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if got, want := w.Code, http.StatusBadRequest; got != want {
		t.Errorf("Code = %d, want %d", got, want)
	}
}

func TestHistoryHandler_BadRequest_BadWindow(t *testing.T) {
	s := newTestStoreForAPI(t)
	h := NewHistoryHandler(s)
	req := httptest.NewRequest(http.MethodGet, "/api/history?source=nws_alerts&window=42m", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if got, want := w.Code, http.StatusBadRequest; got != want {
		t.Errorf("Code = %d, want %d", got, want)
	}
}

func TestHistoryHandler_NWSAlerts_BucketsHaveActiveCount(t *testing.T) {
	s := newTestStoreForAPI(t)
	ctx := context.Background()
	t0 := time.Now().UTC().Add(-time.Hour)

	// Two snapshots: 3 alerts, then 5 alerts.
	type alert struct {
		ID string `json:"id"`
	}
	mk := func(n int) []byte {
		alerts := make([]alert, n)
		for i := range alerts {
			alerts[i] = alert{ID: "a" + string(rune('a'+i))}
		}
		b, _ := json.Marshal(alerts)
		return b
	}
	_ = s.Append(ctx, "nws_alerts", t0, "v1", mk(3))
	_ = s.Append(ctx, "nws_alerts", t0.Add(30*time.Minute), "v2", mk(5))

	h := NewHistoryHandler(s)
	req := httptest.NewRequest(http.MethodGet, "/api/history?source=nws_alerts&window=24h", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if got, want := w.Code, http.StatusOK; got != want {
		t.Fatalf("Code = %d, want %d (body=%s)", got, want, w.Body.String())
	}

	var body struct {
		Source  string `json:"source"`
		Window  string `json:"window"`
		Buckets []struct {
			At          string `json:"at"`
			ActiveCount int    `json:"activeCount"`
		} `json:"buckets"`
		WindowStart string `json:"windowStart"`
		DataStart   string `json:"dataStart"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Source != "nws_alerts" {
		t.Errorf("Source = %q", body.Source)
	}
	if body.Window != "24h" {
		t.Errorf("Window = %q", body.Window)
	}
	if len(body.Buckets) == 0 {
		t.Fatalf("no buckets")
	}
	// Aggregator is max(activeCount); should see at least one bucket with 5.
	maxSeen := 0
	for _, b := range body.Buckets {
		if b.ActiveCount > maxSeen {
			maxSeen = b.ActiveCount
		}
	}
	if maxSeen != 5 {
		t.Errorf("max activeCount = %d, want 5", maxSeen)
	}

	if got := w.Header().Get("Cache-Control"); !strings.Contains(got, "max-age=60") {
		t.Errorf("Cache-Control = %q, want max-age=60", got)
	}
}

func TestHistoryHandler_NWSAlerts_PerEventTypeCounts(t *testing.T) {
	s := newTestStoreForAPI(t)
	ctx := context.Background()
	t0 := time.Now().UTC().Add(-time.Hour)

	// Snapshot mixing event types — 1 tornado, 2 severe-tstorm, 1 flash flood, 1 unrelated.
	type alert struct {
		ID    string `json:"id"`
		Event string `json:"event"`
	}
	mk := func(events ...string) []byte {
		alerts := make([]alert, 0, len(events))
		for i, e := range events {
			alerts = append(alerts, alert{ID: "a" + string(rune('a'+i)), Event: e})
		}
		b, _ := json.Marshal(alerts)
		return b
	}
	_ = s.Append(ctx, "nws_alerts", t0, "v1", mk(
		"Tornado Warning",
		"Severe Thunderstorm Warning",
		"Severe Thunderstorm Watch",
		"Flash Flood Warning",
		"Winter Storm Watch",
	))

	h := NewHistoryHandler(s)
	req := httptest.NewRequest(http.MethodGet, "/api/history?source=nws_alerts&window=24h", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if got, want := w.Code, http.StatusOK; got != want {
		t.Fatalf("Code = %d, want %d (body=%s)", got, want, w.Body.String())
	}

	var body struct {
		Buckets []struct {
			ActiveCount int `json:"activeCount"`
			EventCounts struct {
				Tornado      int `json:"tornado"`
				SevereTstorm int `json:"severeTstorm"`
				FlashFlood   int `json:"flashFlood"`
			} `json:"eventCounts"`
		} `json:"buckets"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Buckets) == 0 {
		t.Fatal("no buckets")
	}
	// Find the bucket with the activity (max(activeCount) is 5).
	var seen *struct {
		ActiveCount int `json:"activeCount"`
		EventCounts struct {
			Tornado      int `json:"tornado"`
			SevereTstorm int `json:"severeTstorm"`
			FlashFlood   int `json:"flashFlood"`
		} `json:"eventCounts"`
	}
	for i := range body.Buckets {
		if body.Buckets[i].ActiveCount == 5 {
			seen = &body.Buckets[i]
			break
		}
	}
	if seen == nil {
		t.Fatalf("no bucket with activeCount=5; buckets=%+v", body.Buckets)
	}
	if seen.EventCounts.Tornado != 1 {
		t.Errorf("tornado = %d, want 1", seen.EventCounts.Tornado)
	}
	if seen.EventCounts.SevereTstorm != 2 {
		t.Errorf("severeTstorm = %d, want 2", seen.EventCounts.SevereTstorm)
	}
	if seen.EventCounts.FlashFlood != 1 {
		t.Errorf("flashFlood = %d, want 1", seen.EventCounts.FlashFlood)
	}
}

func TestHistoryHandler_USGSQuakes_RawEventsM4Plus(t *testing.T) {
	s := newTestStoreForAPI(t)
	ctx := context.Background()
	t0 := time.Now().UTC().Add(-time.Hour)

	// Each snapshot is a list of {features:[{properties:{mag, place, time}, geometry:{coordinates:[lon,lat,depth]}}, ...]}
	mk := func(mags ...float64) []byte {
		type props struct {
			Mag   float64 `json:"mag"`
			Place string  `json:"place"`
			Time  int64   `json:"time"`
		}
		type geom struct {
			Coordinates []float64 `json:"coordinates"`
		}
		type feat struct {
			Properties props `json:"properties"`
			Geometry   geom  `json:"geometry"`
		}
		feats := make([]feat, 0, len(mags))
		for _, m := range mags {
			feats = append(feats, feat{
				Properties: props{Mag: m, Place: "test", Time: t0.UnixMilli()},
				Geometry:   geom{Coordinates: []float64{0, 0, 10}},
			})
		}
		b, _ := json.Marshal(map[string]any{"features": feats})
		return b
	}

	_ = s.Append(ctx, "usgs_quakes", t0, "v1", mk(2.5, 4.2, 5.1, 3.9))

	h := NewHistoryHandler(s)
	req := httptest.NewRequest(http.MethodGet, "/api/history?source=usgs_quakes&window=24h", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if got, want := w.Code, http.StatusOK; got != want {
		t.Fatalf("Code = %d, want %d (body=%s)", got, want, w.Body.String())
	}

	var body struct {
		Events []struct {
			Mag float64 `json:"mag"`
		} `json:"events"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	// M ≥ 4 only: should keep 4.2 and 5.1, drop 2.5 and 3.9.
	if got := len(body.Events); got != 2 {
		t.Errorf("len(events) = %d, want 2 (M4+ filter)", got)
	}
}

func TestHistoryHandler_USGSVolcanoes_ChangesShape(t *testing.T) {
	s := newTestStoreForAPI(t)
	ctx := context.Background()
	t0 := time.Now().UTC().Add(-time.Hour)
	type vEntry struct {
		Name        string `json:"name"`
		Observatory string `json:"observatory"`
		AlertLevel  string `json:"alertLevel"`
	}
	mk := func(es ...vEntry) []byte { b, _ := json.Marshal(es); return b }
	_ = s.Append(ctx, "usgs_volcanoes", t0, "v1", mk())
	_ = s.Append(ctx, "usgs_volcanoes", t0.Add(time.Minute), "v2",
		mk(vEntry{Name: "Kilauea", Observatory: "HVO", AlertLevel: "WATCH"}))

	h := NewHistoryHandler(s)
	req := httptest.NewRequest(http.MethodGet, "/api/history?source=usgs_volcanoes&window=24h", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if got, want := w.Code, http.StatusOK; got != want {
		t.Fatalf("Code = %d, want %d", got, want)
	}
	var body struct {
		Changes []struct {
			Volcano string `json:"volcano"`
			Prior   string `json:"prior"`
			Current string `json:"current"`
		} `json:"changes"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Changes) != 1 {
		t.Fatalf("len(changes) = %d, want 1", len(body.Changes))
	}
	if got := body.Changes[0].Volcano; got != "HVO Kilauea" {
		t.Errorf("Volcano = %q, want %q", got, "HVO Kilauea")
	}
	if body.Changes[0].Current != "WATCH" {
		t.Errorf("Current = %q, want WATCH", body.Changes[0].Current)
	}
}
