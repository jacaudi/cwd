package sources

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func newWarnCapturingLogger() (*slog.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	h := slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	return slog.New(h), buf
}

func TestNWSAlerts_ParseEmpty(t *testing.T) {
	body := loadFixture(t, "alerts_active_empty.json")
	logger, _ := newWarnCapturingLogger()
	got, err := ParseNWSAlerts(body, logger)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want 0 alerts, got %d", len(got))
	}
}

func TestNWSAlerts_ParseTsunamiByAWIPSPrefix(t *testing.T) {
	body := loadFixture(t, "alerts_active_tsunami.json")
	logger, _ := newWarnCapturingLogger()
	got, err := ParseNWSAlerts(body, logger)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 alert, got %d", len(got))
	}
	a := got[0]
	if a.Category != CatTsunami {
		t.Errorf("want Category=Tsunami, got %q", a.Category)
	}
	if !strings.HasPrefix(a.AWIPS, "TSU") {
		t.Errorf("want AWIPS prefix TSU, got %q", a.AWIPS)
	}
	if a.Severity != SevExtreme {
		t.Errorf("want Severity=Extreme, got %q", a.Severity)
	}
	if want := []string{"Coastal Areas of Northern California", "Coastal Areas of Southern Oregon"}; !equalStringSlices(a.Areas, want) {
		t.Errorf("areas mismatch: got %v, want %v", a.Areas, want)
	}
	if !equalStringSlices(a.UGCs, []string{"CAZ505", "ORZ021"}) {
		t.Errorf("UGCs mismatch: got %v", a.UGCs)
	}
}

func TestNWSAlerts_ParseMixedFixture(t *testing.T) {
	body := loadFixture(t, "alerts_active_mixed.json")
	logger, _ := newWarnCapturingLogger()
	got, err := ParseNWSAlerts(body, logger)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, a := range got {
		if a.ID == "" {
			t.Errorf("alert missing ID: %+v", a)
		}
		if a.Sent.IsZero() {
			t.Errorf("alert %s missing sent timestamp", a.ID)
		}
	}
}

func TestNWSAlerts_UnmappedSevereLogsWarn(t *testing.T) {
	body := []byte(`{"type":"FeatureCollection","features":[{
		"id":"https://api.weather.gov/alerts/urn:oid:test-unmapped",
		"type":"Feature","geometry":null,
		"properties":{
			"id":"urn:oid:test-unmapped",
			"areaDesc":"Test Area",
			"sent":"2026-05-02T12:00:00+00:00",
			"effective":"2026-05-02T12:00:00+00:00",
			"expires":"2026-05-02T18:00:00+00:00",
			"severity":"Severe",
			"event":"Hypothetical Future HazSimp Warning",
			"headline":"Test",
			"senderName":"NWS Test",
			"parameters":{"AWIPSidentifier":["XYZTST"]}
		}
	}]}`)
	logger, buf := newWarnCapturingLogger()
	got, err := ParseNWSAlerts(body, logger)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 alert, got %d", len(got))
	}
	if got[0].Category != CatUnknown {
		t.Errorf("want Category=Unknown for unmapped event, got %q", got[0].Category)
	}
	if !bytes.Contains(buf.Bytes(), []byte("nws_alerts.unmapped_severe_event")) {
		t.Errorf("expected canary WARN log, got: %s", buf.String())
	}
}

func TestNWSAlerts_CategoryMap(t *testing.T) {
	cases := []struct {
		event string
		want  Category
	}{
		{"Tornado Warning", CatTornado},
		{"Severe Thunderstorm Warning", CatSevereThunderstorm},
		{"Flash Flood Warning", CatFlashFlood},
		{"Storm Surge Warning", CatTropical},
		{"Hurricane Warning", CatTropical},
		{"Typhoon Warning", CatTropical},
		{"Tropical Storm Warning", CatTropical},
		{"High Wind Warning", CatHighWind},
		{"Extreme Wind Warning", CatHighWind},
		{"Red Flag Warning", CatRedFlag},
		{"Winter Storm Warning", CatWinter},
		{"Blizzard Warning", CatWinter},
		{"Ice Storm Warning", CatWinter},
		{"Snow Squall Warning", CatWinter},
		{"Extreme Heat Warning", CatExtremeHeat},
		{"Extreme Cold Warning", CatExtremeCold},
		{"Special Weather Statement", CatUnknown},
	}
	for _, tc := range cases {
		if got := categorize(tc.event, "XYZ"); got != tc.want {
			t.Errorf("categorize(%q) = %q, want %q", tc.event, got, tc.want)
		}
	}
	if got := categorize("Severe Thunderstorm Warning", "TSUWCA"); got != CatTsunami {
		t.Errorf("AWIPS TSU prefix should override event text; got %q", got)
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestNWSAlerts_FetchHTTP(t *testing.T) {
	body := loadFixture(t, "alerts_active_empty.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got == "" || !strings.Contains(got, "cwd-self-host") {
			t.Errorf("missing/bad User-Agent: %q", got)
		}
		w.Header().Set("Content-Type", "application/geo+json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	src := NewNWSAlerts(srv.URL, "cwd-self-host/test (test@example.com)", slog.New(slog.NewJSONHandler(io.Discard, nil)))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	res, err := src.Fetch(ctx)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if res.Validator == "" {
		t.Errorf("validator empty")
	}
	alerts, ok := res.Payload.([]Alert)
	if !ok {
		t.Fatalf("payload type %T not []Alert", res.Payload)
	}
	if len(alerts) != 0 {
		t.Errorf("want 0 alerts, got %d", len(alerts))
	}
	res2, _ := src.Fetch(ctx)
	if res.Validator != res2.Validator {
		t.Errorf("validator not deterministic: %q vs %q", res.Validator, res2.Validator)
	}
	if _, err := json.Marshal(alerts); err != nil {
		t.Errorf("marshal alerts: %v", err)
	}
}
