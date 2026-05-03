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

func TestSWPCAlerts_AppliesAllowlistAndWindow(t *testing.T) {
	body := []byte(`[
		{"product_id":"K08A","issue_datetime":"2026-05-02 10:00:00.000","message":"Geomagnetic K=8 reached. Detail follows..."},
		{"product_id":"K05A","issue_datetime":"2026-05-02 09:00:00.000","message":"K=5 (G1)."},
		{"product_id":"P12A","issue_datetime":"2026-05-01 23:00:00.000","message":"Solar proton event observed."},
		{"product_id":"P13A","issue_datetime":"2026-04-28 12:00:00.000","message":"Older proton event — outside 24h window."},
		{"product_id":"WARK04W","issue_datetime":"2026-05-02 08:00:00.000","message":"Geomagnetic K>=4 expected."}
	]`)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	now := func() time.Time { return time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC) }
	allow := []string{"K08A", "K09A", "P12A", "P13A"}
	got, err := ParseSWPCAlerts(body, allow, 24*time.Hour, now, logger)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Want: K08A (in allow + within 24h) and P12A (in allow + within 24h)
	// Drop: K05A (not in allow), P13A (outside 24h), WARK04W (not in allow)
	if len(got) != 2 {
		t.Fatalf("want 2, got %d: %+v", len(got), got)
	}
	codes := []string{got[0].Code, got[1].Code}
	if !contains2(codes, "K08A") || !contains2(codes, "P12A") {
		t.Errorf("want K08A and P12A, got %v", codes)
	}
}

func TestSWPCAlerts_ResolvesSeriesAndDescription(t *testing.T) {
	body := []byte(`[
		{"product_id":"K08A","issue_datetime":"2026-05-02 10:00:00.000","message":"detail"},
		{"product_id":"P12A","issue_datetime":"2026-05-02 10:00:00.000","message":"detail"}
	]`)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	now := func() time.Time { return time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC) }
	got, err := ParseSWPCAlerts(body, []string{"K08A", "P12A"}, 24*time.Hour, now, logger)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, a := range got {
		if a.Series == "" {
			t.Errorf("%s missing Series", a.Code)
		}
		if a.Description == "" {
			t.Errorf("%s missing Description", a.Code)
		}
	}
}

func TestSWPCAlerts_TruncatesMessageTo512(t *testing.T) {
	long := strings.Repeat("x", 1024)
	body := []byte(`[{"product_id":"K08A","issue_datetime":"2026-05-02 10:00:00.000","message":"` + long + `"}]`)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	now := func() time.Time { return time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC) }
	got, err := ParseSWPCAlerts(body, []string{"K08A"}, 24*time.Hour, now, logger)
	if err != nil || len(got) != 1 {
		t.Fatalf("parse: %v, len=%d", err, len(got))
	}
	if len(got[0].Message) != 512 {
		t.Errorf("Message len = %d, want 512", len(got[0].Message))
	}
}

func TestSWPCAlerts_UnknownCodePassesThroughWithEmptyDescription(t *testing.T) {
	body := []byte(`[{"product_id":"ZZZ99X","issue_datetime":"2026-05-02 10:00:00.000","message":"..."}]`)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	now := func() time.Time { return time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC) }
	got, err := ParseSWPCAlerts(body, []string{"ZZZ99X"}, 24*time.Hour, now, logger)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
	if got[0].Description != "" {
		t.Errorf("unknown code should have empty description, got %q", got[0].Description)
	}
}

func TestSWPCAlerts_FixtureRoundTrip(t *testing.T) {
	body := loadFixture(t, "swpc_alerts_typical.json")
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	// Use a far-future "now" so the window filter doesn't drop everything;
	// then a wide window. The point is to exercise the parser against real
	// upstream shapes, not to assert specific counts.
	now := func() time.Time { return time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC) }
	allow := []string{"K05A", "K06A", "K07A", "K08A", "K09A", "P10A", "P11A", "P12A", "P13A", "WARK04W", "WARK05W", "WARK06W", "WARK07W", "WARK08W", "WARK09W"}
	got, err := ParseSWPCAlerts(body, allow, 365*24*time.Hour, now, logger)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, a := range got {
		if a.Code == "" {
			t.Errorf("alert missing Code: %+v", a)
		}
		if a.Issued.IsZero() {
			t.Errorf("alert %s missing Issued", a.Code)
		}
	}
}

func TestSWPCAlerts_ParseMalformedJSON(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	now := func() time.Time { return time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC) }
	if _, err := ParseSWPCAlerts([]byte("not json"), []string{"K08A"}, 24*time.Hour, now, logger); err == nil {
		t.Fatalf("want error, got nil")
	}
}

func TestSWPCAlerts_FetchHTTP(t *testing.T) {
	body := []byte(`[{"product_id":"K08A","issue_datetime":"2026-05-02 10:00:00.000","message":"x"}]`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	src := NewSWPCAlerts(srv.URL, "ua", []string{"K08A"}, 24*time.Hour, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	src.now = func() time.Time { return time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC) }
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	res, err := src.Fetch(ctx)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if !strings.HasPrefix(res.Validator, "sha256:") {
		t.Errorf("validator: %q", res.Validator)
	}
	alerts, ok := res.Payload.([]SWPCAlert)
	if !ok {
		t.Fatalf("payload type %T", res.Payload)
	}
	if len(alerts) != 1 {
		t.Errorf("want 1 alert, got %d", len(alerts))
	}
	if src.Name() != "swpc_alerts" {
		t.Errorf("Name() = %q", src.Name())
	}
	_ = io.Discard // keep import
}

// TestSWPCAlerts_ValidatorChangesWhenAlertAgesOutOfWindow guards against the
// cache-staleness bug where the validator was the sha256 of the raw upstream
// body. If upstream stops publishing updates, the body bytes never change, so
// the validator never changes — but the parsed payload changes because the
// 24h window filter ages alerts out using now(). With the bug, cache.Set sees
// identical hashes and never broadcasts, so SSE clients never see the alert
// disappear. The fix: hash the parsed payload, not the raw body.
func TestSWPCAlerts_ValidatorChangesWhenAlertAgesOutOfWindow(t *testing.T) {
	// One alert issued at a fixed wall-clock time. Body bytes are constant.
	body := []byte(`[{"product_id":"K08A","issue_datetime":"2026-05-02 10:00:00.000","message":"x"}]`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	src := NewSWPCAlerts(srv.URL, "ua", []string{"K08A"}, 24*time.Hour, slog.New(slog.NewJSONHandler(io.Discard, nil)))

	// First fetch: T = 2026-05-02 23:00 UTC. Alert is 13h old → inside window.
	src.now = func() time.Time { return time.Date(2026, 5, 2, 23, 0, 0, 0, time.UTC) }
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	res1, err := src.Fetch(ctx)
	if err != nil {
		t.Fatalf("fetch1: %v", err)
	}
	alerts1, ok := res1.Payload.([]SWPCAlert)
	if !ok {
		t.Fatalf("payload1 type %T", res1.Payload)
	}
	if len(alerts1) != 1 {
		t.Fatalf("fetch1 want 1 alert, got %d", len(alerts1))
	}

	// Second fetch: T = 2026-05-03 11:00 UTC. Alert is 25h old → outside window.
	// SAME body bytes from upstream, but parsed payload is now empty.
	src.now = func() time.Time { return time.Date(2026, 5, 3, 11, 0, 0, 0, time.UTC) }
	res2, err := src.Fetch(ctx)
	if err != nil {
		t.Fatalf("fetch2: %v", err)
	}
	alerts2, ok := res2.Payload.([]SWPCAlert)
	if !ok {
		t.Fatalf("payload2 type %T", res2.Payload)
	}
	if len(alerts2) != 0 {
		t.Fatalf("fetch2 want 0 alerts (aged out), got %d", len(alerts2))
	}

	// The bug: validators identical because raw body unchanged → cache silently
	// drops the change → SSE clients never learn the alert is gone.
	// The fix: validators differ because parsed payload differs.
	if res1.Validator == res2.Validator {
		t.Errorf("validator must change when parsed payload changes (window aging); got identical %q", res1.Validator)
	}
}

// TestSWPCAlerts_ValidatorStableWhenPayloadIdentical confirms the fix preserves
// determinism — two fetches with the same body AND same now() must produce the
// same validator (otherwise every poll would broadcast a spurious change).
func TestSWPCAlerts_ValidatorStableWhenPayloadIdentical(t *testing.T) {
	body := []byte(`[{"product_id":"K08A","issue_datetime":"2026-05-02 10:00:00.000","message":"x"}]`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	src := NewSWPCAlerts(srv.URL, "ua", []string{"K08A"}, 24*time.Hour, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	src.now = func() time.Time { return time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC) }
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	res1, err := src.Fetch(ctx)
	if err != nil {
		t.Fatalf("fetch1: %v", err)
	}
	res2, err := src.Fetch(ctx)
	if err != nil {
		t.Fatalf("fetch2: %v", err)
	}
	if res1.Validator != res2.Validator {
		t.Errorf("validator must be stable when payload identical; got %q vs %q", res1.Validator, res2.Validator)
	}
}

func contains2(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
