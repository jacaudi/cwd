package sources

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"
)

func TestUSGSVolcanoes_ParseFixture(t *testing.T) {
	body := loadFixture(t, "usgs_volcanoes_typical.json")
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	got, err := ParseUSGSVolcanoes(body, logger)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, v := range got {
		if v.Name == "" {
			t.Errorf("missing Name: %+v", v)
		}
		if v.Alert == "NORMAL" {
			t.Errorf("NORMAL should be filtered: %+v", v)
		}
	}
}

func TestUSGSVolcanoes_DropsNORMAL(t *testing.T) {
	// Field-name shape verified against live capture 2026-05-02:
	// snake_case (volcano_name, alert_level, color_code, vnum, notice_url,
	// sent_utc, obs_fullname). No latitude/longitude/region/synopsis fields
	// exist on this endpoint.
	body := []byte(`[
		{"vnum":"a1","volcano_name":"A","obs_abbr":"avo","obs_fullname":"Alaska Volcano Observatory","alert_level":"NORMAL","color_code":"GREEN","sent_utc":"2026-05-02 10:00:00","notice_url":"https://a"},
		{"vnum":"a2","volcano_name":"B","obs_abbr":"avo","obs_fullname":"Alaska Volcano Observatory","alert_level":"WATCH","color_code":"ORANGE","sent_utc":"2026-05-02 10:00:00","notice_url":"https://b"},
		{"vnum":"a3","volcano_name":"C","obs_abbr":"cvo","obs_fullname":"California Volcano Observatory","alert_level":"ADVISORY","color_code":"YELLOW","sent_utc":"2026-05-02 10:00:00","notice_url":"https://c"}
	]`)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	got, err := ParseUSGSVolcanoes(body, logger)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 (NORMAL dropped), got %d", len(got))
	}
	for _, v := range got {
		if v.Alert == "NORMAL" {
			t.Errorf("NORMAL leaked: %+v", v)
		}
	}
}

func TestUSGSVolcanoes_SortsByRegionThenName(t *testing.T) {
	body := []byte(`[
		{"vnum":"3","volcano_name":"Zeta","obs_fullname":"Alaska Volcano Observatory","alert_level":"WATCH","color_code":"ORANGE","sent_utc":"2026-05-02 10:00:00"},
		{"vnum":"1","volcano_name":"Beta","obs_fullname":"Alaska Volcano Observatory","alert_level":"WATCH","color_code":"ORANGE","sent_utc":"2026-05-02 10:00:00"},
		{"vnum":"2","volcano_name":"Alpha","obs_fullname":"California Volcano Observatory","alert_level":"WATCH","color_code":"ORANGE","sent_utc":"2026-05-02 10:00:00"}
	]`)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	got, err := ParseUSGSVolcanoes(body, logger)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !sort.SliceIsSorted(got, func(i, j int) bool {
		if got[i].Region != got[j].Region {
			return got[i].Region < got[j].Region
		}
		return got[i].Name < got[j].Name
	}) {
		t.Errorf("not sorted by region/name: %+v", got)
	}
	if got[0].Name != "Beta" || got[1].Name != "Zeta" || got[2].Name != "Alpha" {
		t.Errorf("order: %+v", got)
	}
	if got[0].Region != "Alaska Volcano Observatory" {
		t.Errorf("Region should come from obs_fullname, got %q", got[0].Region)
	}
}

func TestUSGSVolcanoes_FetchETagPassthrough(t *testing.T) {
	body := []byte(`[]`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `W/"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `W/"v1"`)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	src := NewUSGSVolcanoes(srv.URL, "ua", slog.New(slog.NewJSONHandler(io.Discard, nil)))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	res1, err := src.Fetch(ctx)
	if err != nil {
		t.Fatalf("fetch1: %v", err)
	}
	if res1.Validator != `W/"v1"` {
		t.Errorf("first validator: %q", res1.Validator)
	}
	res2, err := src.Fetch(ctx)
	if err != nil {
		t.Fatalf("fetch2: %v", err)
	}
	if res2.Validator != `W/"v1"` {
		t.Errorf("second validator: %q", res2.Validator)
	}
	if src.Name() != "usgs_volcanoes" {
		t.Errorf("Name() = %q", src.Name())
	}
	if src.Interval() != 5*time.Minute {
		t.Errorf("default Interval = %v, want 5m", src.Interval())
	}
	_ = io.Discard
}

func TestUSGSVolcanoes_ParseMalformedJSON(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	if _, err := ParseUSGSVolcanoes([]byte("nope"), logger); err == nil {
		t.Fatalf("want error")
	}
}
