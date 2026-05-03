package server

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jacaudi/cwd/internal/sources"
)

func TestHotStartDecode_KnownSources(t *testing.T) {
	cases := []struct {
		name    string
		payload any
		check   func(t *testing.T, got any)
	}{
		{
			name:    "nws_alerts",
			payload: []sources.Alert{{ID: "a1", Category: sources.CatTornado}},
			check: func(t *testing.T, got any) {
				as, ok := got.([]sources.Alert)
				if !ok {
					t.Fatalf("type %T", got)
				}
				if len(as) != 1 || as[0].ID != "a1" {
					t.Errorf("got %+v", as)
				}
			},
		},
		{
			name:    "swpc_scales",
			payload: sources.SWPCForecast{Days: [3]sources.SWPCDay{{Date: "2026-05-03", R1: 5}, {}, {}}},
			check: func(t *testing.T, got any) {
				fc, ok := got.(sources.SWPCForecast)
				if !ok {
					t.Fatalf("type %T", got)
				}
				if fc.Days[0].Date != "2026-05-03" {
					t.Errorf("Days[0].Date = %q", fc.Days[0].Date)
				}
			},
		},
		{
			name:    "swpc_alerts",
			payload: []sources.SWPCAlert{{Code: "K08A", Issued: time.Now().UTC()}},
			check: func(t *testing.T, got any) {
				as, ok := got.([]sources.SWPCAlert)
				if !ok {
					t.Fatalf("type %T", got)
				}
				if len(as) != 1 || as[0].Code != "K08A" {
					t.Errorf("got %+v", as)
				}
			},
		},
		{
			name:    "usgs_quakes",
			payload: []sources.Quake{{ID: "q1", Magnitude: 5.5}},
			check: func(t *testing.T, got any) {
				qs, ok := got.([]sources.Quake)
				if !ok {
					t.Fatalf("type %T", got)
				}
				if len(qs) != 1 || qs[0].ID != "q1" {
					t.Errorf("got %+v", qs)
				}
			},
		},
		{
			name:    "usgs_volcanoes",
			payload: []sources.Volcano{{ID: "v1", Name: "Test", Alert: sources.AlertWATCH}},
			check: func(t *testing.T, got any) {
				vs, ok := got.([]sources.Volcano)
				if !ok {
					t.Fatalf("type %T", got)
				}
				if len(vs) != 1 || vs[0].Name != "Test" {
					t.Errorf("got %+v", vs)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(tc.payload)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			got, err := hotStartDecode(tc.name, raw)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			tc.check(t, got)
		})
	}
}

func TestHotStartDecode_UnknownSource(t *testing.T) {
	if _, err := hotStartDecode("bogus", []byte(`{}`)); err == nil {
		t.Fatalf("want error for unknown source")
	}
}

func TestHotStartDecode_MalformedJSON(t *testing.T) {
	if _, err := hotStartDecode("nws_alerts", []byte("not json")); err == nil {
		t.Fatalf("want error for malformed JSON")
	}
}
