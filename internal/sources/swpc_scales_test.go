package sources

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSWPCScales_ParseFixture(t *testing.T) {
	body := loadFixture(t, "swpc_scales_typical.json")
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	got, err := ParseSWPCScales(body, logger)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got.Days) != 3 {
		t.Fatalf("want 3 forecast days, got %d", len(got.Days))
	}
	for i, d := range got.Days {
		if d.Date == "" {
			t.Errorf("day %d missing Date", i)
		}
		if d.R1 < 0 || d.R1 > 100 {
			t.Errorf("day %d R1 out of range: %d", i, d.R1)
		}
		if d.R3 < 0 || d.R3 > 100 {
			t.Errorf("day %d R3 out of range: %d", i, d.R3)
		}
		if d.S1 < 0 || d.S1 > 100 {
			t.Errorf("day %d S1 out of range: %d", i, d.S1)
		}
	}
}

func TestSWPCScales_DerivesWorstGScale(t *testing.T) {
	body := []byte(`{
		"0":{"DateStamp":"2026-05-02","R":{"MinorProb":"5","MajorProb":"1"},"S":{"Prob":"1"},"G":{"Scale":"0","Text":"none"}},
		"1":{"DateStamp":"2026-05-03","R":{"MinorProb":"10","MajorProb":"1"},"S":{"Prob":"1"},"G":{"Scale":"2","Text":"G2 expected"}},
		"2":{"DateStamp":"2026-05-04","R":{"MinorProb":"20","MajorProb":"5"},"S":{"Prob":"1"},"G":{"Scale":"4","Text":"G4 watch"}},
		"3":{"DateStamp":"2026-05-05","R":{"MinorProb":"15","MajorProb":"3"},"S":{"Prob":"1"},"G":{"Scale":"5","Text":"G5 extreme"}}
	}`)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	got, err := ParseSWPCScales(body, logger)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.Days[0].G != "G2" {
		t.Errorf("day 1 G = %q, want G2", got.Days[0].G)
	}
	if got.Days[1].G != "G4" {
		t.Errorf("day 2 G = %q, want G4", got.Days[1].G)
	}
	if got.Days[2].G != "G5" {
		t.Errorf("day 3 G = %q, want G5", got.Days[2].G)
	}
	if got.Days[0].GText != "G2 expected" {
		t.Errorf("day 1 GText = %q", got.Days[0].GText)
	}
}

func TestSWPCScales_GScaleZeroIsEmptyString(t *testing.T) {
	body := []byte(`{
		"1":{"DateStamp":"2026-05-03","R":{"MinorProb":"5","MajorProb":"0"},"S":{"Prob":"0"},"G":{"Scale":"0","Text":"none"}},
		"2":{"DateStamp":"2026-05-04","R":{"MinorProb":"5","MajorProb":"0"},"S":{"Prob":"0"},"G":{"Scale":"0","Text":"none"}},
		"3":{"DateStamp":"2026-05-05","R":{"MinorProb":"5","MajorProb":"0"},"S":{"Prob":"0"},"G":{"Scale":"0","Text":"none"}}
	}`)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	got, err := ParseSWPCScales(body, logger)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for i, d := range got.Days {
		if d.G != "" {
			t.Errorf("day %d: want G=\"\" when scale is 0, got %q", i, d.G)
		}
	}
}

func TestSWPCScales_ParseMalformedJSON(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	if _, err := ParseSWPCScales([]byte("not json"), logger); err == nil {
		t.Fatalf("want error, got nil")
	}
}

func TestSWPCScales_MissingDayIsZeroValued(t *testing.T) {
	// SWPC has been observed to drop a day under maintenance; defensive parse
	// should not panic — missing day stays zero-valued.
	body := []byte(`{
		"1":{"DateStamp":"2026-05-03","R":{"MinorProb":"5","MajorProb":"0"},"S":{"Prob":"0"},"G":{"Scale":"0","Text":"none"}},
		"3":{"DateStamp":"2026-05-05","R":{"MinorProb":"15","MajorProb":"3"},"S":{"Prob":"1"},"G":{"Scale":"2","Text":"G2"}}
	}`)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	got, err := ParseSWPCScales(body, logger)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.Days[0].Date != "2026-05-03" {
		t.Errorf("day 1 date wrong: %q", got.Days[0].Date)
	}
	if got.Days[1].Date != "" {
		t.Errorf("day 2 should be zero-valued, got Date=%q", got.Days[1].Date)
	}
	if got.Days[2].Date != "2026-05-05" {
		t.Errorf("day 3 date wrong: %q", got.Days[2].Date)
	}
}

func TestSWPCScales_FetchHTTP(t *testing.T) {
	body := loadFixture(t, "swpc_scales_typical.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); !strings.Contains(got, "cwd-self-host") {
			t.Errorf("missing/bad UA: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	src := NewSWPCScales(srv.URL, "cwd-self-host/test (test@example.com)", slog.New(slog.NewJSONHandler(io.Discard, nil)))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	res, err := src.Fetch(ctx)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if !strings.HasPrefix(res.Validator, "sha256:") {
		t.Errorf("validator should be sha256-prefixed: %q", res.Validator)
	}
	fc, ok := res.Payload.(SWPCForecast)
	if !ok {
		t.Fatalf("payload type %T not SWPCForecast", res.Payload)
	}
	// SWPCForecast.Days is a fixed-size [3]SWPCDay; verify the slot dates
	// were populated from the fixture rather than checking len() which is a
	// compile-time constant.
	if fc.Days[0].Date == "" || fc.Days[1].Date == "" || fc.Days[2].Date == "" {
		t.Errorf("expected all 3 forecast slots populated, got dates: %q %q %q",
			fc.Days[0].Date, fc.Days[1].Date, fc.Days[2].Date)
	}
	res2, _ := src.Fetch(ctx)
	if res.Validator != res2.Validator {
		t.Errorf("validator not deterministic: %q vs %q", res.Validator, res2.Validator)
	}
}

func TestSWPCScales_NameAndInterval(t *testing.T) {
	src := NewSWPCScales("http://x", "ua", slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if src.Name() != "swpc_scales" {
		t.Errorf("Name() = %q, want swpc_scales", src.Name())
	}
	if src.Interval() != 60*time.Second {
		t.Errorf("default Interval() = %v, want 60s", src.Interval())
	}
	src.SetInterval(120 * time.Second)
	if src.Interval() != 120*time.Second {
		t.Errorf("SetInterval did not stick: %v", src.Interval())
	}
}
