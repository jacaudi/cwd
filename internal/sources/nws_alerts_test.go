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
	"sync"
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
		{"Tornado Watch", CatTornado},
		{"Severe Thunderstorm Warning", CatSevereThunderstorm},
		{"Severe Thunderstorm Watch", CatSevereThunderstorm},
		{"Flash Flood Warning", CatFlashFlood},
		{"Flash Flood Watch", CatFlashFlood},
		{"Flood Warning", CatFlood},
		{"Flood Watch", CatFlood},
		{"Flood Advisory", CatFlood},
		{"Coastal Flood Warning", CatFlood},
		{"Coastal Flood Watch", CatFlood},
		{"Coastal Flood Advisory", CatFlood},
		{"Coastal Flood Statement", CatFlood},
		{"Storm Surge Warning", CatTropical},
		{"Storm Surge Watch", CatTropical},
		{"Hurricane Warning", CatTropical},
		{"Hurricane Watch", CatTropical},
		{"Typhoon Warning", CatTropical},
		{"Typhoon Watch", CatTropical},
		{"Tropical Storm Warning", CatTropical},
		{"Tropical Storm Watch", CatTropical},
		{"High Wind Warning", CatHighWind},
		{"High Wind Watch", CatHighWind},
		{"Extreme Wind Warning", CatHighWind},
		{"Wind Advisory", CatHighWind},
		{"Red Flag Warning", CatRedFlag},
		{"Fire Weather Watch", CatRedFlag},
		{"Winter Storm Warning", CatWinter},
		{"Winter Storm Watch", CatWinter},
		{"Winter Weather Advisory", CatWinter},
		{"Blizzard Warning", CatWinter},
		{"Blizzard Watch", CatWinter},
		{"Ice Storm Warning", CatWinter},
		{"Snow Squall Warning", CatWinter},
		{"Extreme Heat Warning", CatExtremeHeat},
		{"Extreme Heat Watch", CatExtremeHeat},
		{"Heat Advisory", CatExtremeHeat},
		{"Extreme Cold Warning", CatExtremeCold},
		{"Extreme Cold Watch", CatExtremeCold},
		{"Freeze Warning", CatExtremeCold},
		{"Freeze Watch", CatExtremeCold},
		{"Frost Advisory", CatExtremeCold},
		{"Wind Chill Warning", CatExtremeCold},
		{"Wind Chill Watch", CatExtremeCold},
		{"Wind Chill Advisory", CatExtremeCold},
		{"Special Marine Warning", CatMarine},
		{"Gale Warning", CatMarine},
		{"Gale Watch", CatMarine},
		{"Storm Warning", CatMarine},
		{"Storm Watch", CatMarine},
		{"Hurricane Force Wind Warning", CatMarine},
		{"Hazardous Seas Warning", CatMarine},
		{"Hazardous Seas Watch", CatMarine},
		{"Special Weather Statement", CatUnknown},
		{"Air Quality Alert", CatUnknown},
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

func TestNWSAlerts_ParseMalformedJSON(t *testing.T) {
	logger, _ := newWarnCapturingLogger()
	if _, err := ParseNWSAlerts([]byte("not json"), logger); err == nil {
		t.Fatalf("want error for malformed JSON, got nil")
	}
}

func TestNWSAlerts_ParseEmptyBody(t *testing.T) {
	logger, _ := newWarnCapturingLogger()
	if _, err := ParseNWSAlerts([]byte{}, logger); err == nil {
		t.Fatalf("want error for empty body, got nil")
	}
}

func TestNWSAlerts_ParseMissingParameters(t *testing.T) {
	// Feature with no `parameters` key at all — must not panic; AWIPS and VTEC
	// fields should be empty strings; result should be a single CatUnknown alert
	// (no canary log because severity is Minor, not Severe/Extreme).
	body := []byte(`{"type":"FeatureCollection","features":[{
		"id":"https://api.weather.gov/alerts/urn:oid:test-noparam",
		"type":"Feature","geometry":null,
		"properties":{
			"id":"urn:oid:test-noparam",
			"areaDesc":"Test Area",
			"sent":"2026-05-02T12:00:00+00:00",
			"effective":"2026-05-02T12:00:00+00:00",
			"expires":"2026-05-02T18:00:00+00:00",
			"severity":"Minor",
			"event":"Special Weather Statement",
			"headline":"Test",
			"senderName":"NWS Test"
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
	if got[0].AWIPS != "" {
		t.Errorf("want AWIPS=\"\" with no parameters, got %q", got[0].AWIPS)
	}
	if got[0].VTECEtn != "" {
		t.Errorf("want VTECEtn=\"\" with no parameters, got %q", got[0].VTECEtn)
	}
	if got[0].Category != CatUnknown {
		t.Errorf("want CatUnknown with no parameters, got %q", got[0].Category)
	}
	if buf.Len() != 0 {
		t.Errorf("Minor-severity unmapped should not log canary, got: %s", buf.String())
	}
}

func TestWFOFromAWIPS(t *testing.T) {
	cases := []struct {
		name  string
		awips string
		want  string
	}{
		{"standard 6-char", "WCNTBW", "TBW"},
		{"flood warning", "FLSPAH", "PAH"},
		{"red flag", "RFWBIS", "BIS"},
		{"tsunami WCA", "TSUWCA", "WCA"},
		{"too short", "TSU", ""},
		{"empty", "", ""},
		{"contains digit", "WCN1BW", ""},
		{"lowercase", "wcntbw", ""},
	}
	for _, tc := range cases {
		if got := wfoFromAWIPS(tc.awips); got != tc.want {
			t.Errorf("%s: wfoFromAWIPS(%q) = %q, want %q", tc.name, tc.awips, got, tc.want)
		}
	}
}

func TestNWSAlerts_ParseSetsWFO(t *testing.T) {
	body := loadFixture(t, "alerts_active_tsunami.json")
	logger, _ := newWarnCapturingLogger()
	got, _ := ParseNWSAlerts(body, logger)
	if len(got) != 1 || got[0].WFO != "WCA" {
		t.Errorf("expected WFO=WCA, got %q (len=%d)", got[0].WFO, len(got))
	}
}

func TestNWSAlerts_AreasEmptyMarshalsAsArray(t *testing.T) {
	// Lock the wire-shape contract: an alert with empty areaDesc must marshal
	// areas as `[]`, not `null` — TS frontend types `Alert.areas: string[]`.
	body := []byte(`{"type":"FeatureCollection","features":[{
		"id":"https://api.weather.gov/alerts/urn:oid:test-noareas",
		"type":"Feature","geometry":null,
		"properties":{
			"id":"urn:oid:test-noareas",
			"areaDesc":"",
			"sent":"2026-05-02T12:00:00+00:00",
			"effective":"2026-05-02T12:00:00+00:00",
			"expires":"2026-05-02T18:00:00+00:00",
			"severity":"Minor",
			"event":"Special Weather Statement",
			"headline":"Test",
			"senderName":"NWS Test"
		}
	}]}`)
	logger, _ := newWarnCapturingLogger()
	got, err := ParseNWSAlerts(body, logger)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 alert, got %d", len(got))
	}
	b, err := json.Marshal(got[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !bytes.Contains(b, []byte(`"areas":[]`)) {
		t.Errorf("want areas:[] in JSON, got: %s", b)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Issue #3 — expanded HazSimp category map. Tests below are organized by the
// seven Go test categories the change is expected to cover:
//   1. Happy-path        2. Table-driven       3. Edge/boundary
//   4. Error handling    5. Concurrency/race   6. Integration (HTTP)
//   7. Benchmark (+ Fuzz)
// ─────────────────────────────────────────────────────────────────────────

// Category 1 — Happy path: the freshly re-captured mixed fixture must exercise
// the two new hazard families (Flood, Marine) and, crucially, must leave NO
// Severe/Extreme event uncategorized — i.e. the production drift canary
// (`nws_alerts.unmapped_severe_event`) is now silent on the real event mix that
// used to trip it constantly (Tornado Watch, Flood Warning, Freeze Watch, …).
func TestNWSAlerts_MixedFixtureExercisesNewMappings(t *testing.T) {
	body := loadFixture(t, "alerts_active_mixed.json")
	logger, buf := newWarnCapturingLogger()
	got, err := ParseNWSAlerts(body, logger)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	seen := map[Category]int{}
	for _, a := range got {
		seen[a.Category]++
		if a.Category == CatUnknown && (a.Severity == SevExtreme || a.Severity == SevSevere) {
			t.Errorf("re-capture leaves a Severe/Extreme event uncategorized (canary would fire): event=%q severity=%q", a.Event, a.Severity)
		}
	}
	for _, c := range []Category{CatFlood, CatMarine, CatTropical, CatExtremeCold, CatRedFlag} {
		if seen[c] == 0 {
			t.Errorf("fixture does not exercise expanded category %q", c)
		}
	}
	// The canary is a drift tripwire, not removed by this change; on the fresh
	// capture it should stay silent.
	if bytes.Contains(buf.Bytes(), []byte("nws_alerts.unmapped_severe_event")) {
		t.Errorf("unexpected drift-canary WARN on fresh fixture: %s", buf.String())
	}
}

// Category 2 — Table-driven: the new Watch/Advisory names and the two new
// families resolve to the right Category, and the AWIPS TSU prefix still wins
// over event text even for a newly-mapped event.
func TestNWSAlerts_ExpandedCategoryMapTable(t *testing.T) {
	cases := []struct {
		event, awips string
		want         Category
	}{
		{"Tornado Watch", "WCNOUN", CatTornado},
		{"Storm Surge Watch", "TCVMFL", CatTropical},
		{"Fire Weather Watch", "RFWXXX", CatRedFlag},
		{"Freeze Watch", "NPWXXX", CatExtremeCold},
		{"Flood Warning", "FLWXXX", CatFlood},
		{"Coastal Flood Advisory", "CFWXXX", CatFlood},
		{"Special Marine Warning", "SMWXXX", CatMarine},
		{"Gale Warning", "MWWXXX", CatMarine},
		// TSU AWIPS prefix overrides even a mapped event name.
		{"Flood Warning", "TSUWCA", CatTsunami},
	}
	for _, tc := range cases {
		if got := categorize(tc.event, tc.awips); got != tc.want {
			t.Errorf("categorize(%q, %q) = %q, want %q", tc.event, tc.awips, got, tc.want)
		}
	}
}

// Category 3 — Edge/boundary: a Severe event that is STILL unmapped after the
// expansion must remain Unknown and must trip the canary, proving the tripwire
// was preserved (acceptance criterion) rather than mapped into oblivion.
func TestNWSAlerts_ExpandedMapPreservesCanary(t *testing.T) {
	body := []byte(`{"type":"FeatureCollection","features":[{
		"id":"https://api.weather.gov/alerts/urn:oid:test-still-unmapped",
		"type":"Feature","geometry":null,
		"properties":{
			"id":"urn:oid:test-still-unmapped",
			"areaDesc":"Test Area",
			"sent":"2026-07-01T12:00:00+00:00",
			"effective":"2026-07-01T12:00:00+00:00",
			"expires":"2026-07-01T18:00:00+00:00",
			"severity":"Severe",
			"event":"Some Brand New HazSimp Warning",
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
	if got[0].Category != CatUnknown {
		t.Errorf("want CatUnknown for still-unmapped event, got %q", got[0].Category)
	}
	if !bytes.Contains(buf.Bytes(), []byte("nws_alerts.unmapped_severe_event")) {
		t.Errorf("canary must still fire for genuinely unmapped Severe events")
	}
}

// Category 4 — Error handling: a newly-mapped event carried in otherwise
// malformed JSON must surface a parse error rather than panic or silently
// mis-categorize.
func TestNWSAlerts_ExpandedMapMalformedJSON(t *testing.T) {
	logger, _ := newWarnCapturingLogger()
	_, err := ParseNWSAlerts([]byte(`{"type":"FeatureCollection","features":[{"event":"Flood Warning"`), logger)
	if err == nil {
		t.Fatalf("want error on truncated JSON, got nil")
	}
}

// Category 5 — Concurrency/race: categorize() reads a package-level map with no
// synchronization; run it from many goroutines under `-race` to prove the read
// path is safe (the map is never mutated after init).
func TestNWSAlerts_CategorizeConcurrent(t *testing.T) {
	events := []string{"Tornado Watch", "Flood Warning", "Special Marine Warning", "Freeze Watch", "Storm Surge Watch"}
	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, e := range events {
				if categorize(e, "XYZ") == CatUnknown {
					t.Errorf("event %q unexpectedly Unknown", e)
				}
			}
		}()
	}
	wg.Wait()
}

// Category 6 — Integration (HTTP): serve the re-captured fixture over an httptest
// server and confirm the full Fetch path decodes the expanded categories.
func TestNWSAlerts_FetchServesExpandedFixture(t *testing.T) {
	body := loadFixture(t, "alerts_active_mixed.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
	alerts, ok := res.Payload.([]Alert)
	if !ok {
		t.Fatalf("payload type %T not []Alert", res.Payload)
	}
	var flood, marine int
	for _, a := range alerts {
		switch a.Category {
		case CatFlood:
			flood++
		case CatMarine:
			marine++
		}
	}
	if flood == 0 || marine == 0 {
		t.Errorf("expected Flood and Marine alerts via Fetch, got flood=%d marine=%d", flood, marine)
	}
}

// Category 7a — Benchmark: parsing throughput over the mixed fixture.
func BenchmarkParseNWSAlertsMixed(b *testing.B) {
	body, err := os.ReadFile(filepath.Join("testdata", "alerts_active_mixed.json"))
	if err != nil {
		b.Fatalf("read fixture: %v", err)
	}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ParseNWSAlerts(body, logger); err != nil {
			b.Fatalf("parse: %v", err)
		}
	}
}

// Category 7b — Fuzz: categorize() must never panic and must return a
// well-formed Category for arbitrary event/awips input.
func FuzzCategorize(f *testing.F) {
	f.Add("Tornado Watch", "WCNOUN")
	f.Add("Special Marine Warning", "SMWXXX")
	f.Add("", "")
	f.Add("Flood Warning", "TSU")
	valid := map[Category]bool{
		CatTornado: true, CatSevereThunderstorm: true, CatFlashFlood: true,
		CatFlood: true, CatTropical: true, CatHighWind: true, CatRedFlag: true,
		CatWinter: true, CatExtremeHeat: true, CatExtremeCold: true,
		CatMarine: true, CatTsunami: true, CatUnknown: true,
	}
	f.Fuzz(func(t *testing.T, event, awips string) {
		got := categorize(event, awips)
		if !valid[got] {
			t.Errorf("categorize(%q,%q) returned invalid Category %q", event, awips, got)
		}
	})
}
