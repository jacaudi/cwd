package sources

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestUSGSQuakes_ParseEmpty(t *testing.T) {
	body := loadFixture(t, "usgs_quakes_empty.geojson")
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	got, err := ParseUSGSQuakes(body, logger)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want 0, got %d", len(got))
	}
}

func TestUSGSQuakes_ParseFixture(t *testing.T) {
	body := loadFixture(t, "usgs_quakes_typical.geojson")
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	got, err := ParseUSGSQuakes(body, logger)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, q := range got {
		if q.ID == "" {
			t.Errorf("quake missing ID: %+v", q)
		}
		if q.Time.IsZero() {
			t.Errorf("quake %s missing Time", q.ID)
		}
	}
}

func TestUSGSQuakes_SortsByMagnitudeDescending(t *testing.T) {
	body := []byte(`{"type":"FeatureCollection","features":[
		{"id":"a","properties":{"mag":4.5,"place":"X","time":1714651200000,"updated":1714651200000,"tsunami":0,"alert":null,"url":"https://x"},"geometry":{"type":"Point","coordinates":[10,20,5]}},
		{"id":"b","properties":{"mag":7.0,"place":"Y","time":1714651300000,"updated":1714651300000,"tsunami":1,"alert":"yellow","url":"https://y"},"geometry":{"type":"Point","coordinates":[30,40,15]}},
		{"id":"c","properties":{"mag":5.5,"place":"Z","time":1714651400000,"updated":1714651400000,"tsunami":0,"alert":null,"url":"https://z"},"geometry":{"type":"Point","coordinates":[50,60,25]}}
	]}`)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	got, err := ParseUSGSQuakes(body, logger)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3, got %d", len(got))
	}
	if !sort.SliceIsSorted(got, func(i, j int) bool { return got[i].Magnitude > got[j].Magnitude }) {
		t.Errorf("not sorted desc by magnitude: %+v", got)
	}
	if got[0].ID != "b" {
		t.Errorf("largest first should be b, got %s", got[0].ID)
	}
	if !got[0].Tsunami {
		t.Errorf("b should carry tsunami=true")
	}
	if got[0].Alert != "yellow" {
		t.Errorf("b alert: %q", got[0].Alert)
	}
}

func TestUSGSQuakes_FetchPassesThroughETag(t *testing.T) {
	body := []byte(`{"type":"FeatureCollection","features":[]}`)
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Header.Get("If-None-Match") == `"abc"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"abc"`)
		w.Header().Set("Content-Type", "application/geo+json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	src := NewUSGSQuakes(srv.URL, "ua", slog.New(slog.NewJSONHandler(io.Discard, nil)))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	res1, err := src.Fetch(ctx)
	if err != nil {
		t.Fatalf("fetch1: %v", err)
	}
	if res1.Validator != `"abc"` {
		t.Errorf("first validator = %q, want \"abc\"", res1.Validator)
	}

	res2, err := src.Fetch(ctx)
	if err != nil {
		t.Fatalf("fetch2: %v", err)
	}
	if res2.Validator != `"abc"` {
		t.Errorf("second validator = %q, want \"abc\"", res2.Validator)
	}
	if hits != 2 {
		t.Errorf("upstream hits = %d, want 2", hits)
	}
	if _, ok := res2.Payload.([]Quake); !ok {
		t.Errorf("304 payload type %T, want []Quake", res2.Payload)
	}

	if src.Name() != "usgs_quakes" {
		t.Errorf("Name() = %q", src.Name())
	}
	if src.Interval() != 60*time.Second {
		t.Errorf("default Interval = %v", src.Interval())
	}
	_ = strings.Contains
}

func TestUSGSQuakes_ParseMalformedJSON(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	if _, err := ParseUSGSQuakes([]byte("nope"), logger); err == nil {
		t.Fatalf("want error")
	}
}
