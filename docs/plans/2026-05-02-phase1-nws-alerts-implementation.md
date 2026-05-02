# Self-Hosted CWD — Phase 1 (NWS Alerts End-to-End) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire `https://api.weather.gov/alerts/active` end-to-end (source → fetcher → cache → store → snapshot/history/SSE endpoints → AntD Overview page with live alert badges + tsunami pane), and address the five Phase 0 reviewer carryovers along the way.

**Architecture:** Three concurrent subsystems inside one Go binary: (1) one fetcher goroutine per enabled source doing content-hash diff-on-write, (2) typed in-memory cache + SQLite ring buffer + SSE hub, (3) HTTP API + embedded SPA. Phase 1 brings the first source online and ships the AntD Overview-page consumers (badge bar + tsunami panel).

**Tech Stack:**
- Backend: Go 1.24, `chi` router (already in Phase 0), `gopkg.in/yaml.v3` (Phase 0), `log/slog` stdlib, `modernc.org/sqlite` (pure Go — added in this phase), `compress/gzip` for payload storage
- Frontend: Node 24, pnpm, Vite 5, React 18, TypeScript 5, AntD 5, `@ant-design/pro-components`, Zustand (added this phase), Vitest + `@testing-library/react` + jsdom (added this phase)
- Tooling: `golangci-lint` v2, GitHub Actions via `jacaudi/github-actions` reusable workflows, Renovate, `Taskfile.yml` (go-task)

**Source documents:**
- Design: [`docs/plans/2026-05-02-phase1-nws-alerts-design.md`](2026-05-02-phase1-nws-alerts-design.md)
- Parent: [`docs/plans/2026-05-01-self-hosted-cwd-design.md`](2026-05-01-self-hosted-cwd-design.md)
- Phase 0 plan (for conventions): [`docs/plans/2026-05-01-self-hosted-cwd-phase0-skeleton-implementation.md`](2026-05-01-self-hosted-cwd-phase0-skeleton-implementation.md)
- Recon: [`docs/recon/2026-05-01-ncep-cwd-status-recon.md`](../recon/2026-05-01-ncep-cwd-status-recon.md) §5.3, §8

**Branch:** `feature/phase1-nws-alerts` off main (currently `cea2eee`, ahead by the design doc; merge base for the final review remains `8a43b7a`).

> **For Claude:** REQUIRED EXECUTION WORKFLOW (follow in order):
> 1. `superpowers:using-git-worktrees` — Isolate work in a dedicated worktree (`.worktrees/phase1-nws-alerts`)
> 2. `superpowers:subagent-driven-development` — Dispatch a fresh subagent per task
> 3. `superpowers:test-driven-development` — All subagents use TDD
> 4. `superpowers:verification-before-completion` — Verify all tests pass per task
> 5. `superpowers:requesting-code-review` — Code review after each task (built in)
> 6. After all tasks: comprehensive code review on full diff from branch point (automatic)
> 7. `superpowers:finishing-a-development-branch` — Complete the branch
>
> Skills carry their own model and effort settings. Do not override them.

---

## Conventions

- All commits prefixed `feat(p1):`, `test(p1):`, `chore(p1):`, `fix(p1):`, `docs(p1):`. Co-authored trailer per repo norm.
- All tests use Go stdlib `testing` + `httptest`; no testing frameworks. Table-driven where appropriate.
- All upstream HTTP must include `User-Agent: cwd-self-host/<version> (<contact>)`. Phase 1 adds the threading; the warning when `contact == ""` was wired in Phase 0.
- Logging: stdlib `log/slog`. Use structured key/value fields; never format into the message.
- No CGO. SQLite is `modernc.org/sqlite` exclusively.
- Frontend tests live next to the component (`Component.test.tsx`).

## Dependency graph (visual)

```
1 fixtures ─┐
            ▼
2 nws_alerts parser ─┬─▶ 3 filter
                     │
4 cache              │
5 store              │
                     ▼
6 fetcher ◀───────── 2,4,5
7 sse hub ◀───────── 4
                     │
8 /api/sources ◀──── 6
9 /api/snapshot ◀─── 4,3
10 /api/history ◀── 5,3
11 /api/stream ◀──── 4,7
                     │
12 HEAD healthz/readyz (independent)
13 per-route WriteTimeout ◀── 8,9,10,11
14 config env walker + WARN (independent)
15 server wiring ◀── 4,5,6,7
16 vitest scaffold (independent)
17 frontend types/store/stream client ◀── 16
18 AlertBadgeBar ◀── 16,17
19 TsunamiPanel ◀── 16,17
20 SourceHealthIndicator ◀── 16,17,8
21 Overview wiring ◀── 17,18,19,20
22 README + smoke task ◀── all
```

---

## Task 1: Capture NWS alerts test fixtures

**Files:**
- Create: `internal/sources/testdata/alerts_active_mixed.json`
- Create: `internal/sources/testdata/alerts_active_empty.json`
- Create: `internal/sources/testdata/alerts_active_tsunami.json`
- Create: `internal/sources/testdata/README.md`

**Dependencies:** none.

**Why:** The Phase 0 reviewer noted no captured fixtures yet. Every parser test in Task 2 reads these. Capture once, commit, then never hit the network in unit tests.

- [ ] **Step 1.1: Capture a live "mixed events" fixture**

```bash
curl -sS \
  -H 'User-Agent: cwd-self-host-fixture-capture/0.0 (adam.caudill@proton.me)' \
  -H 'Accept: application/geo+json' \
  'https://api.weather.gov/alerts/active' \
  > internal/sources/testdata/alerts_active_mixed.json
```

If the response has no Tornado/Severe Thunderstorm/Tsunami at capture time, retry until a representative mix is captured (re-run during severe weather, or pull from a quiet day and proceed — the test suite is tolerant of either).

- [ ] **Step 1.2: Hand-craft a known-empty fixture**

```bash
cat > internal/sources/testdata/alerts_active_empty.json <<'EOF'
{"@context":[],"type":"FeatureCollection","features":[],"title":"current watches, warnings, and advisories","updated":"2026-05-02T12:00:00+00:00"}
EOF
```

- [ ] **Step 1.3: Hand-craft a tsunami-only fixture**

Locate a current TSU* product on https://forecast.weather.gov/, OR copy a feature from `alerts_active_mixed.json` and edit `properties.parameters.AWIPSidentifier[0]` to start with `"TSU"`. If no live tsunami exists, build a synthetic one based on the Tsunami Warning shape — the parser will be tested against the AWIPS prefix rule, not against round-tripping a literal NOAA payload.

A minimal valid synthetic example (only the fields the parser reads):

```json
{
  "type": "FeatureCollection",
  "features": [
    {
      "id": "https://api.weather.gov/alerts/urn:oid:2.49.0.1.840.synthetic-tsu",
      "type": "Feature",
      "geometry": null,
      "properties": {
        "id": "urn:oid:2.49.0.1.840.synthetic-tsu",
        "areaDesc": "Coastal Areas of Northern California; Coastal Areas of Southern Oregon",
        "geocode": {"UGC": ["CAZ505","ORZ021"], "SAME": ["006015","041011"]},
        "sent":      "2026-05-02T11:00:00+00:00",
        "effective": "2026-05-02T11:00:00+00:00",
        "expires":   "2026-05-02T17:00:00+00:00",
        "severity":  "Extreme",
        "event":     "Tsunami Warning",
        "headline":  "Tsunami Warning issued for coastal Northern California",
        "senderName":"NWS National Tsunami Warning Center",
        "parameters": {
          "AWIPSidentifier": ["TSUWCA"],
          "eventTrackingNumber": ["0001"]
        }
      }
    }
  ]
}
```

Save as `internal/sources/testdata/alerts_active_tsunami.json`.

- [ ] **Step 1.4: Document the fixtures**

Create `internal/sources/testdata/README.md`:

```markdown
# NWS Alerts Test Fixtures

These are immutable captured (or synthetic) snapshots of `https://api.weather.gov/alerts/active`
used by `nws_alerts_test.go`. Recapture only when the parser needs to lock new behavior;
the captured-on dates document expected drift over time.

| File | Captured | Notes |
|---|---|---|
| `alerts_active_mixed.json` | 2026-05-02 | Live capture, mixed event types |
| `alerts_active_empty.json` | 2026-05-02 | Hand-crafted, zero features |
| `alerts_active_tsunami.json` | 2026-05-02 | Synthetic TSU AWIPS prefix |
```

- [ ] **Step 1.5: Commit**

```bash
git add internal/sources/testdata/
git commit -m "test(p1): capture nws_alerts test fixtures (mixed, empty, tsunami)"
```

---

## Task 2: `Source` interface + `Alert` types + `nws_alerts.go` parser + category map + canary log

**Files:**
- Create: `internal/sources/source.go`
- Create: `internal/sources/nws_alerts.go`
- Create: `internal/sources/nws_alerts_test.go`

**Dependencies:** — depends on: Task 1 (Capture NWS alerts test fixtures).

- [ ] **Step 2.1: Write the failing test (`nws_alerts_test.go`)**

```go
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
	// Mixed fixture is a live capture; assert structural invariants only.
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

// equalStringSlices is a tiny helper; lives next to the test that uses it.
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

// Roundtrip sanity to keep the fetcher contract honest:
// the source consumes an io.Reader, not just []byte.
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
	// Validator should be deterministic for identical input.
	res2, _ := src.Fetch(ctx)
	if res.Validator != res2.Validator {
		t.Errorf("validator not deterministic: %q vs %q", res.Validator, res2.Validator)
	}
	// Smoke that JSON round-trips fine for downstream API.
	if _, err := json.Marshal(alerts); err != nil {
		t.Errorf("marshal alerts: %v", err)
	}
}
```

- [ ] **Step 2.2: Run test, verify it fails**

```bash
go test ./internal/sources/...
```

Expected: fail with "undefined: ParseNWSAlerts" (and similar).

- [ ] **Step 2.3: Implement `internal/sources/source.go`**

```go
package sources

import "context"

// FetchResult is the contract every Source returns.
// Validator is opaque to the fetcher: real ETag if upstream provides one,
// content-hash ("sha256:hex") otherwise.
type FetchResult struct {
	Payload   any
	Validator string
}

// Source is one upstream data source. One goroutine per Source at runtime.
type Source interface {
	Name() string
	Interval() ConfigInterval // (resolved at construction; fetcher uses for ticker + backoff cap)
	Fetch(ctx context.Context) (FetchResult, error)
}

// ConfigInterval is just time.Duration aliased for clarity at call sites.
// Defined here so internal/fetcher doesn't need to import internal/config.
type ConfigInterval = interface{ /* sentinel; replace with time.Duration in actual code */ }
```

NOTE for the implementing engineer: the `ConfigInterval` sentinel above is illustrative — in the actual file, just use `time.Duration` directly:

```go
package sources

import (
	"context"
	"time"
)

type FetchResult struct {
	Payload   any
	Validator string
}

type Source interface {
	Name() string
	Interval() time.Duration
	Fetch(ctx context.Context) (FetchResult, error)
}
```

- [ ] **Step 2.4: Implement `internal/sources/nws_alerts.go`**

Full design spec for the `Alert` type, severity enum, and category map is in `docs/plans/2026-05-02-phase1-nws-alerts-design.md` §5 + §5.1. Implementation:

```go
package sources

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const NWSAlertsName = "nws_alerts"

type Severity string

const (
	SevExtreme  Severity = "Extreme"
	SevSevere   Severity = "Severe"
	SevModerate Severity = "Moderate"
	SevMinor    Severity = "Minor"
	SevUnknown  Severity = "Unknown"
)

type Category string

const (
	CatTornado            Category = "Tornado"
	CatSevereThunderstorm Category = "SevereThunderstorm"
	CatFlashFlood         Category = "FlashFlood"
	CatTropical           Category = "Tropical"
	CatHighWind           Category = "HighWind"
	CatRedFlag            Category = "RedFlag"
	CatWinter             Category = "Winter"
	CatExtremeHeat        Category = "ExtremeHeat"
	CatExtremeCold        Category = "ExtremeCold"
	CatTsunami            Category = "Tsunami"
	CatUnknown            Category = "Unknown"
)

type Alert struct {
	ID        string    `json:"id"`
	Event     string    `json:"event"`
	AWIPS     string    `json:"awips"`
	Headline  string    `json:"headline"`
	Severity  Severity  `json:"severity"`
	Sent      time.Time `json:"sent"`
	Effective time.Time `json:"effective"`
	Onset     time.Time `json:"onset,omitempty"`
	Expires   time.Time `json:"expires"`
	Ends      time.Time `json:"ends,omitempty"`
	Areas     []string  `json:"areas"`
	UGCs      []string  `json:"ugcs,omitempty"`
	SAMEs     []string  `json:"sames,omitempty"`
	WFO       string    `json:"wfo,omitempty"`
	VTEC_ETN  string    `json:"vtecEtn,omitempty"`
	URL       string    `json:"url,omitempty"`
	Category  Category  `json:"category"`
}

var eventCategoryMap = map[string]Category{
	"Tornado Warning":             CatTornado,
	"Severe Thunderstorm Warning": CatSevereThunderstorm,
	"Flash Flood Warning":         CatFlashFlood,
	"Storm Surge Warning":         CatTropical,
	"Hurricane Warning":           CatTropical,
	"Typhoon Warning":             CatTropical,
	"Tropical Storm Warning":      CatTropical,
	"High Wind Warning":           CatHighWind,
	"Extreme Wind Warning":        CatHighWind,
	"Red Flag Warning":            CatRedFlag,
	"Winter Storm Warning":        CatWinter,
	"Blizzard Warning":            CatWinter,
	"Ice Storm Warning":           CatWinter,
	"Snow Squall Warning":         CatWinter,
	"Extreme Heat Warning":        CatExtremeHeat,
	"Extreme Cold Warning":        CatExtremeCold,
}

// categorize implements the rule from design §5.1: AWIPS-prefix Tsunami wins,
// otherwise look up by event text, else CatUnknown.
func categorize(event, awips string) Category {
	if len(awips) >= 3 && awips[:3] == "TSU" {
		return CatTsunami
	}
	if c, ok := eventCategoryMap[event]; ok {
		return c
	}
	return CatUnknown
}

// rawFeatureCollection mirrors only the fields we read.
type rawFeatureCollection struct {
	Features []rawFeature `json:"features"`
}

type rawFeature struct {
	ID         string         `json:"id"`
	Properties rawProperties  `json:"properties"`
	Geometry   any            `json:"geometry"` // skipped per design §2 decision A
}

type rawProperties struct {
	ID         string         `json:"id"`
	AreaDesc   string         `json:"areaDesc"`
	Sent       time.Time      `json:"sent"`
	Effective  time.Time      `json:"effective"`
	Onset      *time.Time     `json:"onset"`
	Expires    time.Time      `json:"expires"`
	Ends       *time.Time     `json:"ends"`
	Status     string         `json:"status"`
	MessageType string        `json:"messageType"`
	Severity   string         `json:"severity"`
	Event      string         `json:"event"`
	Headline   string         `json:"headline"`
	SenderName string         `json:"senderName"`
	Geocode    rawGeocode     `json:"geocode"`
	Parameters rawParameters  `json:"parameters"`
}

type rawGeocode struct {
	UGC  []string `json:"UGC"`
	SAME []string `json:"SAME"`
}

type rawParameters struct {
	AWIPSidentifier     []string `json:"AWIPSidentifier"`
	EventTrackingNumber []string `json:"eventTrackingNumber"`
}

func parseSeverity(s string) Severity {
	switch s {
	case "Extreme":
		return SevExtreme
	case "Severe":
		return SevSevere
	case "Moderate":
		return SevModerate
	case "Minor":
		return SevMinor
	default:
		return SevUnknown
	}
}

// wfoFromSenderName extracts the 3-letter office code from strings like
// "NWS Sterling VA" → not derivable; fall back to senderName as-is unless
// id is parseable. Returns "" if unknown — the caller treats that as no-WFO.
// This is intentionally conservative; the recon notes WFO derivation is
// fragile across product types. Better empty than wrong.
func wfoFromSenderName(sender, id string) string {
	// Many alert IDs are URLs ending with "...?wfo=XXX" or contain WFO triplet
	// in path segments; senderName is usually "NWS <City> <State>". For Phase 1
	// we only attempt the conservative path and return "" if uncertain — the
	// region filter tolerates empty WFO (it just won't match WFO predicates).
	_ = sender
	_ = id
	return ""
}

// ParseNWSAlerts is the pure parser — fed []byte, returns typed alerts and a
// drift-canary side effect (slog.Warn on Extreme/Severe events not in the
// category map).
func ParseNWSAlerts(body []byte, logger *slog.Logger) ([]Alert, error) {
	if logger == nil {
		logger = slog.Default()
	}
	var fc rawFeatureCollection
	if err := json.Unmarshal(body, &fc); err != nil {
		return nil, fmt.Errorf("nws_alerts: unmarshal: %w", err)
	}
	out := make([]Alert, 0, len(fc.Features))
	for _, f := range fc.Features {
		p := f.Properties
		awips := ""
		if len(p.Parameters.AWIPSidentifier) > 0 {
			awips = p.Parameters.AWIPSidentifier[0]
		}
		etn := ""
		if len(p.Parameters.EventTrackingNumber) > 0 {
			etn = p.Parameters.EventTrackingNumber[0]
		}
		areas := splitAreas(p.AreaDesc)
		sev := parseSeverity(p.Severity)
		cat := categorize(p.Event, awips)
		if cat == CatUnknown && (sev == SevExtreme || sev == SevSevere) {
			logger.Warn("nws_alerts.unmapped_severe_event",
				"event", p.Event,
				"severity", sev,
				"id", p.ID,
				"awips", awips,
			)
		}
		var onset, ends time.Time
		if p.Onset != nil {
			onset = *p.Onset
		}
		if p.Ends != nil {
			ends = *p.Ends
		}
		out = append(out, Alert{
			ID:        p.ID,
			Event:     p.Event,
			AWIPS:     awips,
			Headline:  p.Headline,
			Severity:  sev,
			Sent:      p.Sent,
			Effective: p.Effective,
			Onset:     onset,
			Expires:   p.Expires,
			Ends:      ends,
			Areas:     areas,
			UGCs:      append([]string(nil), p.Geocode.UGC...),
			SAMEs:     append([]string(nil), p.Geocode.SAME...),
			WFO:       wfoFromSenderName(p.SenderName, f.ID),
			VTEC_ETN:  etn,
			URL:       f.ID,
			Category:  cat,
		})
	}
	return out, nil
}

func splitAreas(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// NWSAlerts is the Source implementation.
type NWSAlerts struct {
	url       string
	userAgent string
	interval  time.Duration
	client    *http.Client
	logger    *slog.Logger
}

func NewNWSAlerts(url, userAgent string, logger *slog.Logger) *NWSAlerts {
	return &NWSAlerts{
		url:       url,
		userAgent: userAgent,
		interval:  30 * time.Second, // matches upstream s-maxage; per-source env override comes from Task 14
		client:    &http.Client{Timeout: 15 * time.Second},
		logger:    logger,
	}
}

func (n *NWSAlerts) Name() string              { return NWSAlertsName }
func (n *NWSAlerts) Interval() time.Duration   { return n.interval }
func (n *NWSAlerts) SetInterval(d time.Duration) { n.interval = d }

func (n *NWSAlerts) Fetch(ctx context.Context) (FetchResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, n.url, nil)
	if err != nil {
		return FetchResult{}, fmt.Errorf("nws_alerts: build request: %w", err)
	}
	req.Header.Set("User-Agent", n.userAgent)
	req.Header.Set("Accept", "application/geo+json")
	resp, err := n.client.Do(req)
	if err != nil {
		return FetchResult{}, fmt.Errorf("nws_alerts: do: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return FetchResult{}, fmt.Errorf("nws_alerts: upstream status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return FetchResult{}, fmt.Errorf("nws_alerts: read body: %w", err)
	}
	alerts, err := ParseNWSAlerts(body, n.logger)
	if err != nil {
		return FetchResult{}, err
	}
	sum := sha256.Sum256(body)
	return FetchResult{
		Payload:   alerts,
		Validator: "sha256:" + hex.EncodeToString(sum[:]),
	}, nil
}
```

- [ ] **Step 2.5: Run tests, verify they pass**

```bash
go test ./internal/sources/...
```

Expected: PASS (all tests in `nws_alerts_test.go`).

- [ ] **Step 2.6: Lint**

```bash
golangci-lint run ./internal/sources/...
```

Expected: clean.

- [ ] **Step 2.7: Commit**

```bash
git add internal/sources/source.go internal/sources/nws_alerts.go internal/sources/nws_alerts_test.go
git commit -m "feat(p1): add Source interface + nws_alerts parser with category map and drift canary"
```

---

## Task 3: Region filter

**Files:**
- Create: `internal/sources/filter.go`
- Create: `internal/sources/filter_test.go`

**Dependencies:** — depends on: Task 2 (uses `Alert` type).

- [ ] **Step 3.1: Write the failing test**

```go
package sources

import "testing"

func mkAlert(ugcs []string, wfo string) Alert {
	return Alert{UGCs: ugcs, WFO: wfo}
}

func TestFilter_EmptyMatchesAll(t *testing.T) {
	f := NewFilter(nil, nil)
	if !f.Match(mkAlert(nil, "")) {
		t.Errorf("empty filter should match alert with no fields")
	}
	if !f.Match(mkAlert([]string{"VAC059"}, "LWX")) {
		t.Errorf("empty filter should match populated alert")
	}
}

func TestFilter_UGCMatch(t *testing.T) {
	f := NewFilter([]string{"VAC059"}, nil)
	if !f.Match(mkAlert([]string{"VAC059", "MDC031"}, "")) {
		t.Errorf("UGC list intersection should match")
	}
	if f.Match(mkAlert([]string{"CAZ505"}, "")) {
		t.Errorf("non-overlapping UGCs should not match")
	}
}

func TestFilter_WFOMatch(t *testing.T) {
	f := NewFilter(nil, []string{"LWX"})
	if !f.Match(mkAlert(nil, "LWX")) {
		t.Errorf("WFO match expected")
	}
	if f.Match(mkAlert(nil, "OAX")) {
		t.Errorf("non-matching WFO should not match")
	}
	if f.Match(mkAlert(nil, "")) {
		t.Errorf("empty WFO with non-empty filter should not match")
	}
}

func TestFilter_UnionSemantics(t *testing.T) {
	// Union (UGC OR WFO): both filters non-empty → either match suffices.
	f := NewFilter([]string{"VAC059"}, []string{"OAX"})
	if !f.Match(mkAlert([]string{"VAC059"}, "")) {
		t.Errorf("UGC-only match should pass under union")
	}
	if !f.Match(mkAlert(nil, "OAX")) {
		t.Errorf("WFO-only match should pass under union")
	}
	if f.Match(mkAlert([]string{"CAZ505"}, "BOX")) {
		t.Errorf("no-match in either should fail")
	}
}

func TestFilter_Apply(t *testing.T) {
	f := NewFilter([]string{"VAC059"}, nil)
	in := []Alert{
		mkAlert([]string{"VAC059"}, ""),
		mkAlert([]string{"CAZ505"}, ""),
		mkAlert([]string{"VAC059", "MDC031"}, ""),
	}
	got := f.Apply(in)
	if len(got) != 2 {
		t.Fatalf("want 2 matching alerts, got %d", len(got))
	}
}
```

- [ ] **Step 3.2: Run test, verify it fails**

```bash
go test ./internal/sources/ -run TestFilter
```

Expected: fail with "undefined: NewFilter".

- [ ] **Step 3.3: Implement `internal/sources/filter.go`**

```go
package sources

// Filter is the per-operator region filter from design §6.5.
// Empty UGCs AND empty WFOs ⇒ match-all.
// Non-empty: UGC list-intersection OR WFO equality (union semantics).
type Filter struct {
	ugcs map[string]struct{}
	wfos map[string]struct{}
}

func NewFilter(ugcs, wfos []string) Filter {
	f := Filter{ugcs: map[string]struct{}{}, wfos: map[string]struct{}{}}
	for _, u := range ugcs {
		if u != "" {
			f.ugcs[u] = struct{}{}
		}
	}
	for _, w := range wfos {
		if w != "" {
			f.wfos[w] = struct{}{}
		}
	}
	return f
}

// Match returns true iff the alert passes the filter.
func (f Filter) Match(a Alert) bool {
	if len(f.ugcs) == 0 && len(f.wfos) == 0 {
		return true
	}
	for _, u := range a.UGCs {
		if _, ok := f.ugcs[u]; ok {
			return true
		}
	}
	if a.WFO != "" {
		if _, ok := f.wfos[a.WFO]; ok {
			return true
		}
	}
	return false
}

// Apply returns a new slice of alerts that pass the filter.
// Stable order; never returns nil for non-nil input.
func (f Filter) Apply(in []Alert) []Alert {
	if len(f.ugcs) == 0 && len(f.wfos) == 0 {
		out := make([]Alert, len(in))
		copy(out, in)
		return out
	}
	out := make([]Alert, 0, len(in))
	for _, a := range in {
		if f.Match(a) {
			out = append(out, a)
		}
	}
	return out
}
```

- [ ] **Step 3.4: Run tests, verify pass**

```bash
go test ./internal/sources/ -run TestFilter -v
golangci-lint run ./internal/sources/...
```

- [ ] **Step 3.5: Commit**

```bash
git add internal/sources/filter.go internal/sources/filter_test.go
git commit -m "feat(p1): add region filter with UGC/WFO union semantics"
```

---

## Task 4: Cache (`Get`/`Set`/`Subscribe`)

**Files:**
- Create: `internal/cache/cache.go`
- Create: `internal/cache/cache_test.go`

**Dependencies:** none.

- [ ] **Step 4.1: Write the failing test**

```go
package cache

import (
	"context"
	"testing"
	"time"
)

type fakeEnv struct {
	Source    string
	FetchedAt time.Time
	Validator string
	Payload   any
}

func TestCache_GetMissReturnsZero(t *testing.T) {
	c := New()
	got, ok := c.Get("nws_alerts")
	if ok {
		t.Errorf("expected miss, got hit: %+v", got)
	}
}

func TestCache_SetThenGet(t *testing.T) {
	c := New()
	env := Envelope{Source: "nws_alerts", FetchedAt: time.Now(), Validator: "sha256:abc", Payload: []int{1, 2, 3}}
	c.Set(env)
	got, ok := c.Get("nws_alerts")
	if !ok {
		t.Fatalf("expected hit")
	}
	if got.Validator != env.Validator {
		t.Errorf("validator mismatch: %q vs %q", got.Validator, env.Validator)
	}
}

func TestCache_SubscribeReceivesSet(t *testing.T) {
	c := New()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ch := c.Subscribe(ctx, "nws_alerts")
	env := Envelope{Source: "nws_alerts", FetchedAt: time.Now(), Validator: "v1"}
	c.Set(env)
	select {
	case got := <-ch:
		if got.Validator != "v1" {
			t.Errorf("got validator %q, want v1", got.Validator)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("subscriber did not receive event")
	}
}

func TestCache_NoBroadcastOnIdenticalValidator(t *testing.T) {
	c := New()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ch := c.Subscribe(ctx, "nws_alerts")
	env := Envelope{Source: "nws_alerts", FetchedAt: time.Now(), Validator: "vA"}
	c.Set(env)
	<-ch // drain first
	c.Set(env) // identical validator
	select {
	case unwanted := <-ch:
		t.Fatalf("did not expect a second broadcast, got %+v", unwanted)
	case <-time.After(150 * time.Millisecond):
		// success
	}
}

func TestCache_SubscribeUnsubsOnContextDone(t *testing.T) {
	c := New()
	ctx, cancel := context.WithCancel(context.Background())
	ch := c.Subscribe(ctx, "nws_alerts")
	cancel()
	// Cache must close the channel and stop dispatching.
	c.Set(Envelope{Source: "nws_alerts", Validator: "v"})
	// Drain everything; chan should be closed.
	deadline := time.After(300 * time.Millisecond)
	for {
		select {
		case _, open := <-ch:
			if !open {
				return
			}
		case <-deadline:
			t.Fatal("channel never closed after ctx cancel")
		}
	}
}
```

- [ ] **Step 4.2: Run test, verify fail**

```bash
go test ./internal/cache/...
```

Expected: fail with "undefined: New" / "undefined: Envelope".

- [ ] **Step 4.3: Implement `internal/cache/cache.go`**

```go
// Package cache holds the latest Envelope per source name and broadcasts
// changes to subscribers. Diff-on-write: identical validators do not broadcast.
package cache

import (
	"context"
	"sync"
	"time"
)

// Envelope is the payload-with-metadata stored per source. The Payload is
// source-typed (`[]sources.Alert` for nws_alerts) and downcast at the consumer.
type Envelope struct {
	Source    string    `json:"source"`
	FetchedAt time.Time `json:"fetchedAt"`
	Validator string    `json:"etag,omitempty"` // serialized as "etag" for the wire shape
	Payload   any       `json:"payload"`
}

type subscriber struct {
	ch     chan Envelope
	cancel <-chan struct{}
}

// Cache is the typed in-memory store + change broadcaster.
type Cache struct {
	mu          sync.RWMutex
	latest      map[string]Envelope
	subscribers map[string][]*subscriber
}

func New() *Cache {
	return &Cache{
		latest:      map[string]Envelope{},
		subscribers: map[string][]*subscriber{},
	}
}

// Get returns the latest envelope for source name and a hit indicator.
func (c *Cache) Get(name string) (Envelope, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.latest[name]
	return e, ok
}

// Set stores an envelope and broadcasts if the validator changed.
// Identical-validator writes update FetchedAt but do not broadcast.
func (c *Cache) Set(env Envelope) {
	c.mu.Lock()
	prev, ok := c.latest[env.Source]
	c.latest[env.Source] = env
	subs := c.subscribers[env.Source]
	c.mu.Unlock()

	if ok && prev.Validator == env.Validator && env.Validator != "" {
		return
	}
	for _, s := range subs {
		select {
		case s.ch <- env:
		case <-s.cancel:
			// subscriber's context is done; will be GC'd next time we prune
		default:
			// slow subscriber; skip rather than block. SSE hub uses a per-client
			// chan with its own backpressure handling.
		}
	}
}

// Subscribe returns a channel of envelopes for the given source. Closes when
// ctx is done. Channel is buffered (size 4) for mild burst tolerance.
func (c *Cache) Subscribe(ctx context.Context, name string) <-chan Envelope {
	s := &subscriber{
		ch:     make(chan Envelope, 4),
		cancel: ctx.Done(),
	}
	c.mu.Lock()
	c.subscribers[name] = append(c.subscribers[name], s)
	c.mu.Unlock()

	go func() {
		<-ctx.Done()
		c.mu.Lock()
		subs := c.subscribers[name]
		out := subs[:0]
		for _, x := range subs {
			if x != s {
				out = append(out, x)
			}
		}
		c.subscribers[name] = out
		c.mu.Unlock()
		close(s.ch)
	}()

	return s.ch
}
```

- [ ] **Step 4.4: Run tests, verify pass**

```bash
go test ./internal/cache/... -v -race
golangci-lint run ./internal/cache/...
```

- [ ] **Step 4.5: Commit**

```bash
git add internal/cache/cache.go internal/cache/cache_test.go
git commit -m "feat(p1): add typed cache with Subscribe + diff-on-write"
```

---

## Task 5: SQLite store (schema + Open + Append + Latest + At + Prune)

**Files:**
- Create: `internal/store/schema.sql`
- Create: `internal/store/store.go`
- Create: `internal/store/snapshots.go`
- Create: `internal/store/store_test.go`
- Modify: `go.mod` (add `modernc.org/sqlite`)

**Dependencies:** none.

- [ ] **Step 5.1: Add the sqlite dependency**

```bash
go get modernc.org/sqlite
go mod tidy
```

- [ ] **Step 5.2: Write the failing test (`store_test.go`)**

```go
package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func openTempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "cwd.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestStore_OpenMigrateIdempotent(t *testing.T) {
	s := openTempStore(t)
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Calling twice must not fail (CREATE TABLE IF NOT EXISTS).
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate twice: %v", err)
	}
}

func TestStore_AppendAndLatest(t *testing.T) {
	s := openTempStore(t)
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	if err := s.Append(context.Background(), "nws_alerts", now, "vA", []byte(`["a"]`)); err != nil {
		t.Fatalf("append: %v", err)
	}
	got, ok, err := s.Latest(context.Background(), "nws_alerts")
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if !ok {
		t.Fatal("expected a row")
	}
	if got.Validator != "vA" {
		t.Errorf("validator: got %q", got.Validator)
	}
	if string(got.Payload) != `["a"]` {
		t.Errorf("payload: got %s", got.Payload)
	}
	if !got.FetchedAt.Equal(now) {
		t.Errorf("fetchedAt: got %v want %v", got.FetchedAt, now)
	}
}

func TestStore_AtReturnsNearestPrior(t *testing.T) {
	s := openTempStore(t)
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	t0 := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	for i, v := range []string{"vA", "vB", "vC"} {
		if err := s.Append(context.Background(), "nws_alerts", t0.Add(time.Duration(i)*time.Minute), v, []byte(v)); err != nil {
			t.Fatal(err)
		}
	}
	// Query at t0+1m30s: should return vB (the row at t0+1m).
	got, ok, err := s.At(context.Background(), "nws_alerts", t0.Add(90*time.Second))
	if err != nil || !ok {
		t.Fatalf("at: ok=%v err=%v", ok, err)
	}
	if got.Validator != "vB" {
		t.Errorf("got %q want vB", got.Validator)
	}

	// Query before any row: ok=false.
	_, ok, _ = s.At(context.Background(), "nws_alerts", t0.Add(-time.Hour))
	if ok {
		t.Errorf("expected miss before first row")
	}
}

func TestStore_Prune(t *testing.T) {
	s := openTempStore(t)
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	old := now.Add(-31 * 24 * time.Hour)
	recent := now.Add(-1 * time.Hour)
	if err := s.Append(context.Background(), "nws_alerts", old, "vOld", []byte("o")); err != nil {
		t.Fatal(err)
	}
	if err := s.Append(context.Background(), "nws_alerts", recent, "vNew", []byte("n")); err != nil {
		t.Fatal(err)
	}
	n, err := s.Prune(context.Background(), 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("expected 1 pruned row, got %d", n)
	}
	got, ok, _ := s.Latest(context.Background(), "nws_alerts")
	if !ok || got.Validator != "vNew" {
		t.Errorf("latest after prune: %+v", got)
	}
}

func TestStore_GzipRoundTrip(t *testing.T) {
	s := openTempStore(t)
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	big := make([]byte, 64*1024)
	for i := range big {
		big[i] = byte(i % 251)
	}
	if err := s.Append(context.Background(), "nws_alerts", time.Now(), "vBig", big); err != nil {
		t.Fatalf("append big: %v", err)
	}
	got, ok, _ := s.Latest(context.Background(), "nws_alerts")
	if !ok {
		t.Fatal("no row")
	}
	if len(got.Payload) != len(big) {
		t.Errorf("payload len mismatch: %d vs %d", len(got.Payload), len(big))
	}
}
```

- [ ] **Step 5.3: Run test, verify fail**

```bash
go test ./internal/store/...
```

Expected: fail (undefined symbols).

- [ ] **Step 5.4: Implement schema + store**

`internal/store/schema.sql`:

```sql
PRAGMA journal_mode = WAL;
PRAGMA synchronous  = NORMAL;
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS snapshots (
    source     TEXT    NOT NULL,
    fetched_at INTEGER NOT NULL, -- unix milliseconds UTC
    etag       TEXT,
    payload    BLOB    NOT NULL, -- gzipped JSON
    PRIMARY KEY (source, fetched_at)
);
```

`internal/store/store.go`:

```go
// Package store is the SQLite ring-buffer for source envelopes.
// 30-day retention; payloads stored gzipped. Pure Go (modernc.org/sqlite).
package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

type Store struct {
	db *sql.DB
}

// Open initializes a SQLite connection at path (created if missing).
// Caller must call Migrate at least once before Append/Latest/At.
func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open %q: %w", path, err)
	}
	// SQLite serializes writes; one connection avoids "database is locked" without WAL races.
	db.SetMaxOpenConns(1)
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// Migrate runs the embedded schema; idempotent.
func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, schemaSQL); err != nil {
		return fmt.Errorf("store: migrate: %w", err)
	}
	return nil
}
```

`internal/store/snapshots.go`:

```go
package store

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"time"
)

// Row is what Latest/At return.
type Row struct {
	Source    string
	FetchedAt time.Time
	Validator string
	Payload   []byte // already-decompressed JSON
}

// Append inserts (or no-ops on PK conflict) a row. Payload is gzipped on write.
func (s *Store) Append(ctx context.Context, source string, fetchedAt time.Time, validator string, payload []byte) error {
	gzipped, err := gzipBytes(payload)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO snapshots(source, fetched_at, etag, payload)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(source, fetched_at) DO NOTHING`,
		source, fetchedAt.UnixMilli(), nullableString(validator), gzipped,
	)
	if err != nil {
		return fmt.Errorf("store: append: %w", err)
	}
	return nil
}

// Latest returns the most recent row for source.
func (s *Store) Latest(ctx context.Context, source string) (Row, bool, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT source, fetched_at, COALESCE(etag, ''), payload
		 FROM snapshots WHERE source = ?
		 ORDER BY fetched_at DESC LIMIT 1`, source)
	return scanRow(row)
}

// At returns the row whose fetched_at is the largest <= t.
func (s *Store) At(ctx context.Context, source string, t time.Time) (Row, bool, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT source, fetched_at, COALESCE(etag, ''), payload
		 FROM snapshots WHERE source = ? AND fetched_at <= ?
		 ORDER BY fetched_at DESC LIMIT 1`, source, t.UnixMilli())
	return scanRow(row)
}

// Prune deletes rows older than (now - retention). Returns rows deleted.
func (s *Store) Prune(ctx context.Context, retention time.Duration) (int64, error) {
	cutoff := time.Now().Add(-retention).UnixMilli()
	res, err := s.db.ExecContext(ctx, `DELETE FROM snapshots WHERE fetched_at < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("store: prune: %w", err)
	}
	return res.RowsAffected()
}

func scanRow(row *sql.Row) (Row, bool, error) {
	var r Row
	var gzipped []byte
	var ms int64
	err := row.Scan(&r.Source, &ms, &r.Validator, &gzipped)
	if errors.Is(err, sql.ErrNoRows) {
		return Row{}, false, nil
	}
	if err != nil {
		return Row{}, false, fmt.Errorf("store: scan: %w", err)
	}
	r.FetchedAt = time.UnixMilli(ms).UTC()
	payload, err := gunzipBytes(gzipped)
	if err != nil {
		return Row{}, false, err
	}
	r.Payload = payload
	return r, true, nil
}

func gzipBytes(in []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(in); err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("store: gzip: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("store: gzip close: %w", err)
	}
	return buf.Bytes(), nil
}

func gunzipBytes(in []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(in))
	if err != nil {
		return nil, fmt.Errorf("store: gunzip: %w", err)
	}
	defer r.Close()
	return io.ReadAll(r)
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
```

- [ ] **Step 5.5: Run tests, verify pass**

```bash
go test ./internal/store/... -v -race
golangci-lint run ./internal/store/...
```

- [ ] **Step 5.6: Commit**

```bash
git add internal/store/ go.mod go.sum
git commit -m "feat(p1): add SQLite ring-buffer store (schema + Append/Latest/At/Prune, gzipped payloads)"
```

---

## Task 6: Fetcher (ticker + diff-on-write + backoff + health stats)

**Files:**
- Create: `internal/fetcher/fetcher.go`
- Create: `internal/fetcher/fetcher_test.go`

**Dependencies:** — depends on: Task 2 (Source interface), Task 4 (Cache), Task 5 (Store).

- [ ] **Step 6.1: Write the failing test**

```go
package fetcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jacaudi/cwd/internal/cache"
	"github.com/jacaudi/cwd/internal/sources"
	"github.com/jacaudi/cwd/internal/store"
)

type fakeSource struct {
	name      string
	interval  time.Duration
	hits      atomic.Int64
	respFn    func(hit int64) (sources.FetchResult, error)
}

func (f *fakeSource) Name() string            { return f.name }
func (f *fakeSource) Interval() time.Duration { return f.interval }
func (f *fakeSource) Fetch(ctx context.Context) (sources.FetchResult, error) {
	h := f.hits.Add(1)
	return f.respFn(h)
}

func newTempStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "f.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestFetcher_FirstSuccessAppendsAndBroadcasts(t *testing.T) {
	c := cache.New()
	st := newTempStore(t)
	src := &fakeSource{
		name:     "test",
		interval: 50 * time.Millisecond,
		respFn: func(_ int64) (sources.FetchResult, error) {
			return sources.FetchResult{Payload: []int{1}, Validator: "vA"}, nil
		},
	}
	f := New(src, c, st)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	sub := c.Subscribe(ctx, "test")

	go f.Run(ctx)
	select {
	case env := <-sub:
		if env.Validator != "vA" {
			t.Errorf("validator: got %q", env.Validator)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("no broadcast")
	}
	row, ok, err := st.Latest(context.Background(), "test")
	if err != nil || !ok {
		t.Fatalf("latest: ok=%v err=%v", ok, err)
	}
	if row.Validator != "vA" {
		t.Errorf("store validator: got %q", row.Validator)
	}
}

func TestFetcher_NoBroadcastOnIdenticalValidator(t *testing.T) {
	c := cache.New()
	st := newTempStore(t)
	src := &fakeSource{
		name:     "test",
		interval: 30 * time.Millisecond,
		respFn: func(_ int64) (sources.FetchResult, error) {
			return sources.FetchResult{Payload: []int{1}, Validator: "vSame"}, nil
		},
	}
	f := New(src, c, st)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	sub := c.Subscribe(ctx, "test")
	go f.Run(ctx)

	// Receive first broadcast.
	<-sub
	// Subsequent ticks should not broadcast.
	timer := time.After(150 * time.Millisecond)
	for {
		select {
		case unwanted := <-sub:
			t.Fatalf("unexpected second broadcast: %+v", unwanted)
		case <-timer:
			return
		}
	}
}

func TestFetcher_BackoffDoublesOnError(t *testing.T) {
	c := cache.New()
	st := newTempStore(t)
	src := &fakeSource{
		name:     "test",
		interval: 20 * time.Millisecond,
		respFn: func(_ int64) (sources.FetchResult, error) {
			return sources.FetchResult{}, errors.New("boom")
		},
	}
	f := New(src, c, st)
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	go f.Run(ctx)
	<-ctx.Done()
	h := f.Health()
	if h.ConsecutiveFailures < 2 {
		t.Errorf("expected ≥2 failures, got %d", h.ConsecutiveFailures)
	}
	if h.LastError == "" {
		t.Errorf("expected lastError populated")
	}
	// With doubling cap of 5x interval, max ~5*20ms=100ms; 400ms total
	// allows ~4-7 attempts. Just assert it didn't spin without bound.
	if src.hits.Load() > 25 {
		t.Errorf("too many hits, backoff not engaged: %d", src.hits.Load())
	}
}

func TestFetcher_NWSAlertsHTTPIntegration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintln(w, `{"type":"FeatureCollection","features":[]}`)
	}))
	defer srv.Close()
	src := sources.NewNWSAlerts(srv.URL, "cwd-self-host/test (test@example.com)", nil)
	c := cache.New()
	st := newTempStore(t)
	f := New(src, c, st)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	sub := c.Subscribe(ctx, "nws_alerts")
	go f.Run(ctx)
	select {
	case env := <-sub:
		alerts, ok := env.Payload.([]sources.Alert)
		if !ok {
			t.Fatalf("payload type %T", env.Payload)
		}
		if len(alerts) != 0 {
			t.Errorf("want 0 alerts, got %d", len(alerts))
		}
		// Validator must be populated by the source.
		if env.Validator == "" {
			t.Errorf("validator empty")
		}
	case <-time.After(150 * time.Millisecond):
		t.Fatal("no broadcast")
	}
	// Make sure marshalling the payload through the wire works.
	if _, err := json.Marshal(c); err == nil {
		// (we don't actually marshal Cache directly; just validate the
		// individual envelope round-trips JSON cleanly via Get)
		got, _ := c.Get("nws_alerts")
		if _, err := json.Marshal(got); err != nil {
			t.Errorf("marshal envelope: %v", err)
		}
	}
}
```

- [ ] **Step 6.2: Run test, verify fail**

```bash
go test ./internal/fetcher/...
```

Expected: fail with "undefined: New" / "undefined: Health".

- [ ] **Step 6.3: Implement `internal/fetcher/fetcher.go`**

```go
// Package fetcher runs one Source per goroutine: timer → Fetch → diff →
// cache.Set → store.Append. Exponential backoff on errors.
package fetcher

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/jacaudi/cwd/internal/cache"
	"github.com/jacaudi/cwd/internal/sources"
	"github.com/jacaudi/cwd/internal/store"
)

// Health is the per-source snapshot returned by /api/sources.
type Health struct {
	IntervalSec         int       `json:"intervalSec"`
	LastAttempt         time.Time `json:"lastAttempt"`
	LastSuccess         time.Time `json:"lastSuccess"`
	LastError           string    `json:"lastError,omitempty"`
	ETag                string    `json:"etag,omitempty"`
	AgeSec              int       `json:"ageSec"`
	ConsecutiveFailures int       `json:"consecutiveFailures"`
}

type Fetcher struct {
	src    sources.Source
	cache  *cache.Cache
	store  *store.Store
	logger *slog.Logger

	mu     sync.RWMutex
	health Health
}

func New(src sources.Source, c *cache.Cache, st *store.Store, opts ...Option) *Fetcher {
	f := &Fetcher{
		src:    src,
		cache:  c,
		store:  st,
		logger: slog.Default(),
		health: Health{IntervalSec: int(src.Interval().Seconds())},
	}
	for _, o := range opts {
		o(f)
	}
	return f
}

type Option func(*Fetcher)

func WithLogger(l *slog.Logger) Option { return func(f *Fetcher) { f.logger = l } }

// Run blocks until ctx is done. Fires immediately, then on Interval.
func (f *Fetcher) Run(ctx context.Context) {
	base := f.src.Interval()
	if base <= 0 {
		base = 30 * time.Second
	}
	maxBackoff := 5 * base
	delay := time.Duration(0) // first tick fires immediately
	timer := time.NewTimer(delay)
	defer timer.Stop()

	failures := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		ok := f.tick(ctx)
		if ok {
			failures = 0
			delay = base
		} else {
			failures++
			delay = base
			for i := 0; i < failures-1; i++ {
				delay *= 2
				if delay > maxBackoff {
					delay = maxBackoff
					break
				}
			}
		}
		f.mu.Lock()
		f.health.ConsecutiveFailures = failures
		f.mu.Unlock()
		timer.Reset(delay)
	}
}

func (f *Fetcher) tick(ctx context.Context) bool {
	now := time.Now().UTC()
	f.mu.Lock()
	f.health.LastAttempt = now
	f.mu.Unlock()

	res, err := f.src.Fetch(ctx)
	if err != nil {
		f.mu.Lock()
		f.health.LastError = truncate(err.Error(), 256)
		f.mu.Unlock()
		f.logger.Warn("fetcher.error", "source", f.src.Name(), "err", err.Error())
		return false
	}

	env := cache.Envelope{
		Source:    f.src.Name(),
		FetchedAt: now,
		Validator: res.Validator,
		Payload:   res.Payload,
	}
	prev, hadPrev := f.cache.Get(f.src.Name())
	f.cache.Set(env)
	f.mu.Lock()
	f.health.LastSuccess = now
	f.health.LastError = ""
	f.health.ETag = res.Validator
	f.mu.Unlock()

	// Append to store only on diff (matches the design's diff-on-write).
	if !hadPrev || prev.Validator != res.Validator {
		payload, err := encodePayloadJSON(res.Payload)
		if err != nil {
			f.logger.Error("fetcher.encode", "source", f.src.Name(), "err", err.Error())
			return true // success at the source layer; just couldn't persist
		}
		if err := f.store.Append(ctx, f.src.Name(), now, res.Validator, payload); err != nil {
			f.logger.Error("fetcher.store_append", "source", f.src.Name(), "err", err.Error())
		}
	}
	return true
}

// Health returns a copy of the current per-source health snapshot.
func (f *Fetcher) Health() Health {
	f.mu.RLock()
	defer f.mu.RUnlock()
	h := f.health
	if !h.LastSuccess.IsZero() {
		h.AgeSec = int(time.Since(h.LastSuccess).Seconds())
	}
	return h
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// encodePayloadJSON serializes the payload to JSON bytes for store.Append.
// Lives here (not in store) because the store is payload-agnostic.
func encodePayloadJSON(p any) ([]byte, error) {
	return jsonMarshal(p)
}
```

`internal/fetcher/json.go`:

```go
package fetcher

import "encoding/json"

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }
```

(Trivial wrapper kept so `fetcher.go` doesn't import `encoding/json` directly — easier to swap for streaming encoders later if a payload grows.)

- [ ] **Step 6.4: Run tests, verify pass**

```bash
go test ./internal/fetcher/... -v -race
golangci-lint run ./internal/fetcher/...
```

- [ ] **Step 6.5: Commit**

```bash
git add internal/fetcher/
git commit -m "feat(p1): add fetcher with diff-on-write + exponential backoff + health stats"
```

---

## Task 7: SSE hub

**Files:**
- Create: `internal/sse/event.go`
- Create: `internal/sse/hub.go`
- Create: `internal/sse/hub_test.go`

**Dependencies:** — depends on: Task 4 (Cache).

- [ ] **Step 7.1: Write the failing test**

```go
package sse

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jacaudi/cwd/internal/cache"
)

type fakeSink struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (f *fakeSink) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.buf.Write(p)
}

func (f *fakeSink) String() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.buf.String()
}

func TestHub_BroadcastsCacheUpdates(t *testing.T) {
	c := cache.New()
	h := NewHub(c, []string{"nws_alerts"})
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	go h.Run(ctx)

	sink := &fakeSink{}
	clientCtx, clientCancel := context.WithCancel(ctx)
	go h.Serve(clientCtx, sink)
	defer clientCancel()
	time.Sleep(20 * time.Millisecond) // let Serve subscribe

	c.Set(cache.Envelope{Source: "nws_alerts", FetchedAt: time.Now(), Validator: "v1", Payload: []string{"x"}})

	deadline := time.After(300 * time.Millisecond)
	for {
		select {
		case <-deadline:
			t.Fatalf("no event in sink: %q", sink.String())
		default:
			if strings.Contains(sink.String(), "event: nws_alerts.update") {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestHub_EmitsSnapshotOnConnect(t *testing.T) {
	c := cache.New()
	c.Set(cache.Envelope{Source: "nws_alerts", FetchedAt: time.Now(), Validator: "v1", Payload: []string{"x"}})
	h := NewHub(c, []string{"nws_alerts"})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	go h.Run(ctx)

	sink := &fakeSink{}
	go h.Serve(ctx, sink)

	deadline := time.After(150 * time.Millisecond)
	for {
		select {
		case <-deadline:
			t.Fatalf("no snapshot: %q", sink.String())
		default:
			if strings.Contains(sink.String(), "event: snapshot") {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestHub_PingComment(t *testing.T) {
	c := cache.New()
	h := NewHub(c, []string{"nws_alerts"}, WithPingInterval(40*time.Millisecond))
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	go h.Run(ctx)
	sink := &fakeSink{}
	go h.Serve(ctx, sink)

	deadline := time.After(180 * time.Millisecond)
	for {
		select {
		case <-deadline:
			t.Fatalf("no ping: %q", sink.String())
		default:
			if strings.Contains(sink.String(), ": ping") {
				return
			}
			time.Sleep(15 * time.Millisecond)
		}
	}
}
```

- [ ] **Step 7.2: Run test, verify fail**

```bash
go test ./internal/sse/...
```

Expected: fail with "undefined: NewHub".

- [ ] **Step 7.3: Implement `internal/sse/event.go`**

```go
package sse

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// Frame writes one SSE event to w. name == "" emits a comment ping.
func writeEvent(w io.Writer, name string, payload any) error {
	if name == "" {
		_, err := fmt.Fprint(w, ": ping\n\n")
		return err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, data)
	return err
}

// Snapshot is the wire shape for the initial frame.
// Sources is a free-form map so Phase 1 ships only nws_alerts and Phase 2 adds more
// without reshaping the type.
type Snapshot struct {
	ServerTime time.Time      `json:"serverTime"`
	Sources    map[string]any `json:"sources"`
}
```

- [ ] **Step 7.4: Implement `internal/sse/hub.go`**

```go
package sse

import (
	"context"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/jacaudi/cwd/internal/cache"
)

type Hub struct {
	cache        *cache.Cache
	sources      []string
	pingInterval time.Duration
	flusher      func(io.Writer)
}

type Option func(*Hub)

func WithPingInterval(d time.Duration) Option { return func(h *Hub) { h.pingInterval = d } }

func NewHub(c *cache.Cache, sources []string, opts ...Option) *Hub {
	h := &Hub{
		cache:        c,
		sources:      sources,
		pingInterval: 25 * time.Second,
		flusher: func(w io.Writer) {
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		},
	}
	for _, o := range opts {
		o(h)
	}
	return h
}

// Run is a no-op placeholder kept for symmetry with sources/store/fetcher.
// Each connected client runs its own Serve; no shared loop needed.
func (h *Hub) Run(ctx context.Context) { <-ctx.Done() }

// Serve writes SSE frames to w until ctx is done. Per-client.
// The caller (HTTP handler) is responsible for setting headers and Flusher
// behavior; this method writes raw frames + flushes.
func (h *Hub) Serve(ctx context.Context, w io.Writer) {
	// Initial snapshot frame.
	snap := Snapshot{
		ServerTime: time.Now().UTC(),
		Sources:    map[string]any{},
	}
	for _, name := range h.sources {
		if env, ok := h.cache.Get(name); ok {
			snap.Sources[name] = env
		}
	}
	_ = writeEvent(w, "snapshot", snap)
	h.flusher(w)

	// Subscribe to each source's cache channel.
	subs := make([]<-chan cache.Envelope, 0, len(h.sources))
	for _, name := range h.sources {
		subs = append(subs, h.cache.Subscribe(ctx, name))
	}

	ping := time.NewTicker(h.pingInterval)
	defer ping.Stop()

	// Fan-in: select over all subs + ping + ctx.
	mux := make(chan envWithName, 16)
	var wg sync.WaitGroup
	for i, ch := range subs {
		i := i
		ch := ch
		wg.Add(1)
		go func() {
			defer wg.Done()
			for env := range ch {
				select {
				case mux <- envWithName{name: h.sources[i], env: env}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-mux:
			if err := writeEvent(w, msg.name+".update", msg.env); err != nil {
				return
			}
			h.flusher(w)
		case <-ping.C:
			if err := writeEvent(w, "", nil); err != nil {
				return
			}
			h.flusher(w)
		}
	}
}

type envWithName struct {
	name string
	env  cache.Envelope
}
```

- [ ] **Step 7.5: Run tests, verify pass**

```bash
go test ./internal/sse/... -v -race
golangci-lint run ./internal/sse/...
```

- [ ] **Step 7.6: Commit**

```bash
git add internal/sse/
git commit -m "feat(p1): add SSE hub (snapshot frame + per-source updates + ping)"
```

---

## Task 8: `/api/sources` endpoint

**Files:**
- Create: `internal/api/sources.go`
- Create: `internal/api/sources_test.go`
- Modify: `internal/api/router.go` (register route — code shown in Task 13 once all four new endpoints exist; for this task, just register the single route inline alongside Phase 0 routes)

**Dependencies:** — depends on: Task 6 (Fetcher).

- [ ] **Step 8.1: Write the failing test**

```go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jacaudi/cwd/internal/fetcher"
)

type fakeHealthProvider struct {
	healths map[string]fetcher.Health
}

func (f *fakeHealthProvider) Health() map[string]fetcher.Health { return f.healths }

func TestSourcesEndpoint(t *testing.T) {
	hp := &fakeHealthProvider{
		healths: map[string]fetcher.Health{
			"nws_alerts": {
				IntervalSec: 30,
				LastAttempt: time.Now().Add(-5 * time.Second),
				LastSuccess: time.Now().Add(-5 * time.Second),
				ETag:        "sha256:abc",
				AgeSec:      5,
			},
		},
	}
	h := NewSourcesHandler(hp)
	req := httptest.NewRequest(http.MethodGet, "/api/sources", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	var got map[string]fetcher.Health
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["nws_alerts"].ETag != "sha256:abc" {
		t.Errorf("etag: got %q", got["nws_alerts"].ETag)
	}
}
```

- [ ] **Step 8.2: Run test, verify fail**

```bash
go test ./internal/api/ -run TestSourcesEndpoint
```

- [ ] **Step 8.3: Implement `internal/api/sources.go`**

```go
package api

import (
	"encoding/json"
	"net/http"

	"github.com/jacaudi/cwd/internal/fetcher"
)

// HealthProvider is what the server exposes to the API: a snapshot of all
// per-source fetcher health. Decouples this handler from the server type.
type HealthProvider interface {
	Health() map[string]fetcher.Health
}

type sourcesHandler struct {
	hp HealthProvider
}

func NewSourcesHandler(hp HealthProvider) http.Handler {
	return &sourcesHandler{hp: hp}
}

func (h *sourcesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(h.hp.Health()); err != nil {
		http.Error(w, "encode", http.StatusInternalServerError)
	}
}
```

- [ ] **Step 8.4: Wire it into `internal/api/router.go`**

Add (next to the existing Phase 0 routes):

```go
// In whatever function currently builds the *chi.Mux:
r.Method(http.MethodGet, "/api/sources", deps.SourcesHandler)
```

The router's existing dependency struct (Phase 0) must gain a `SourcesHandler http.Handler` field. Construction happens in Task 14 (server wiring).

- [ ] **Step 8.5: Run tests, verify pass**

```bash
go test ./internal/api/... -v
golangci-lint run ./internal/api/...
```

- [ ] **Step 8.6: Commit**

```bash
git add internal/api/sources.go internal/api/sources_test.go internal/api/router.go
git commit -m "feat(p1): add /api/sources health endpoint"
```

---

## Task 9: `/api/snapshot` endpoint

**Files:**
- Create: `internal/api/snapshot.go`
- Create: `internal/api/snapshot_test.go`
- Modify: `internal/api/router.go` (register route)

**Dependencies:** — depends on: Task 4 (Cache), Task 3 (Filter).

- [ ] **Step 9.1: Write the failing test**

```go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jacaudi/cwd/internal/cache"
	"github.com/jacaudi/cwd/internal/sources"
)

func TestSnapshot_EmptyCache(t *testing.T) {
	c := cache.New()
	h := NewSnapshotHandler(c, sources.NewFilter(nil, nil))
	req := httptest.NewRequest(http.MethodGet, "/api/snapshot", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	var got map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if _, ok := got["serverTime"]; !ok {
		t.Errorf("missing serverTime")
	}
	src, _ := got["sources"].(map[string]any)
	if _, has := src["nws_alerts"]; has {
		t.Errorf("expected nws_alerts omitted on empty cache, got: %+v", src)
	}
}

func TestSnapshot_AppliesRegionFilter(t *testing.T) {
	c := cache.New()
	c.Set(cache.Envelope{
		Source:    "nws_alerts",
		FetchedAt: time.Now(),
		Validator: "v1",
		Payload: []sources.Alert{
			{ID: "a1", Category: sources.CatTornado, UGCs: []string{"VAC059"}},
			{ID: "a2", Category: sources.CatTornado, UGCs: []string{"CAZ505"}},
		},
	})
	h := NewSnapshotHandler(c, sources.NewFilter([]string{"VAC059"}, nil))
	req := httptest.NewRequest(http.MethodGet, "/api/snapshot", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if !contains(body, "a1") {
		t.Errorf("a1 missing: %s", body)
	}
	if contains(body, "a2") {
		t.Errorf("a2 should be filtered out: %s", body)
	}
}

func contains(haystack, needle string) bool { return len(haystack) >= len(needle) && (indexOf(haystack, needle) >= 0) }

func indexOf(h, n string) int {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return i
		}
	}
	return -1
}
```

- [ ] **Step 9.2: Run test, verify fail**

```bash
go test ./internal/api/ -run TestSnapshot
```

- [ ] **Step 9.3: Implement `internal/api/snapshot.go`**

```go
package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jacaudi/cwd/internal/cache"
	"github.com/jacaudi/cwd/internal/sources"
)

// Envelope is the wire wrapper. Mirrors design §5.
type Envelope struct {
	Source    string    `json:"source"`
	FetchedAt time.Time `json:"fetchedAt"`
	ETag      string    `json:"etag,omitempty"`
	Payload   any       `json:"payload"`
}

// SnapshotResponse is the wire shape of /api/snapshot. Sources is a free-form
// map keyed by source name; only present sources appear (omitempty by absence).
type SnapshotResponse struct {
	ServerTime time.Time          `json:"serverTime"`
	Sources    map[string]Envelope `json:"sources"`
}

type snapshotHandler struct {
	cache  *cache.Cache
	filter sources.Filter
}

func NewSnapshotHandler(c *cache.Cache, f sources.Filter) http.Handler {
	return &snapshotHandler{cache: c, filter: f}
}

func (h *snapshotHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	resp := SnapshotResponse{
		ServerTime: time.Now().UTC(),
		Sources:    map[string]Envelope{},
	}
	if env, ok := h.cache.Get(sources.NWSAlertsName); ok {
		alerts, _ := env.Payload.([]sources.Alert)
		resp.Sources[sources.NWSAlertsName] = Envelope{
			Source:    env.Source,
			FetchedAt: env.FetchedAt,
			ETag:      env.Validator,
			Payload:   h.filter.Apply(alerts),
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "encode", http.StatusInternalServerError)
	}
}
```

- [ ] **Step 9.4: Wire route**

```go
r.Method(http.MethodGet, "/api/snapshot", deps.SnapshotHandler)
```

- [ ] **Step 9.5: Run tests, verify pass; lint**

```bash
go test ./internal/api/... -v
golangci-lint run ./internal/api/...
```

- [ ] **Step 9.6: Commit**

```bash
git add internal/api/snapshot.go internal/api/snapshot_test.go internal/api/router.go
git commit -m "feat(p1): add /api/snapshot with region filter"
```

---

## Task 10: `/api/history?at=…` endpoint

**Files:**
- Create: `internal/api/history.go`
- Create: `internal/api/history_test.go`
- Modify: `internal/api/router.go`

**Dependencies:** — depends on: Task 5 (Store), Task 3 (Filter).

- [ ] **Step 10.1: Write the failing test**

```go
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/jacaudi/cwd/internal/sources"
	"github.com/jacaudi/cwd/internal/store"
)

func newSeededStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	t0 := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	for i, v := range []string{"vA", "vB"} {
		body, _ := json.Marshal([]sources.Alert{{ID: v}})
		if err := s.Append(context.Background(), "nws_alerts", t0.Add(time.Duration(i)*time.Minute), v, body); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestHistory_NearestPrior(t *testing.T) {
	s := newSeededStore(t)
	h := NewHistoryHandler(s, sources.NewFilter(nil, nil))
	req := httptest.NewRequest(http.MethodGet, "/api/history?at=2026-05-02T12:01:30Z", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rec.Code, rec.Body.String())
	}
	var got SnapshotResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	env, ok := got.Sources["nws_alerts"]
	if !ok {
		t.Fatalf("nws_alerts missing: %s", rec.Body.String())
	}
	if env.ETag != "vB" {
		t.Errorf("expected vB (nearest-prior at 12:01:30), got %q", env.ETag)
	}
}

func TestHistory_BadAt(t *testing.T) {
	s := newSeededStore(t)
	h := NewHistoryHandler(s, sources.NewFilter(nil, nil))
	req := httptest.NewRequest(http.MethodGet, "/api/history?at=not-a-time", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHistory_BeforeAnyRow(t *testing.T) {
	s := newSeededStore(t)
	h := NewHistoryHandler(s, sources.NewFilter(nil, nil))
	req := httptest.NewRequest(http.MethodGet, "/api/history?at=2020-01-01T00:00:00Z", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	var got SnapshotResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if _, has := got.Sources["nws_alerts"]; has {
		t.Errorf("expected sources empty before first row")
	}
}
```

- [ ] **Step 10.2: Run test, verify fail**

```bash
go test ./internal/api/ -run TestHistory
```

- [ ] **Step 10.3: Implement `internal/api/history.go`**

```go
package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jacaudi/cwd/internal/sources"
	"github.com/jacaudi/cwd/internal/store"
)

type historyHandler struct {
	store  *store.Store
	filter sources.Filter
}

func NewHistoryHandler(s *store.Store, f sources.Filter) http.Handler {
	return &historyHandler{store: s, filter: f}
}

func (h *historyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	atStr := r.URL.Query().Get("at")
	if atStr == "" {
		http.Error(w, "missing 'at' query parameter", http.StatusBadRequest)
		return
	}
	at, err := time.Parse(time.RFC3339, atStr)
	if err != nil {
		http.Error(w, "invalid 'at' (want RFC3339): "+err.Error(), http.StatusBadRequest)
		return
	}
	resp := SnapshotResponse{
		ServerTime: time.Now().UTC(),
		Sources:    map[string]Envelope{},
	}
	row, ok, err := h.store.At(r.Context(), sources.NWSAlertsName, at)
	if err != nil {
		http.Error(w, "store: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if ok {
		var alerts []sources.Alert
		if err := json.Unmarshal(row.Payload, &alerts); err != nil {
			http.Error(w, "decode payload: "+err.Error(), http.StatusInternalServerError)
			return
		}
		resp.Sources[sources.NWSAlertsName] = Envelope{
			Source:    row.Source,
			FetchedAt: row.FetchedAt,
			ETag:      row.Validator,
			Payload:   h.filter.Apply(alerts),
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_ = json.NewEncoder(w).Encode(resp)
}
```

- [ ] **Step 10.4: Wire route, run tests, commit**

```go
r.Method(http.MethodGet, "/api/history", deps.HistoryHandler)
```

```bash
go test ./internal/api/... -v
golangci-lint run ./internal/api/...
git add internal/api/history.go internal/api/history_test.go internal/api/router.go
git commit -m "feat(p1): add /api/history?at= with nearest-prior lookup + region filter"
```

---

## Task 11: `/api/stream` (SSE) endpoint

**Files:**
- Create: `internal/api/stream.go`
- Create: `internal/api/stream_test.go`
- Modify: `internal/api/router.go`

**Dependencies:** — depends on: Task 4 (Cache), Task 7 (SSE Hub).

- [ ] **Step 11.1: Write the failing test**

```go
package api

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jacaudi/cwd/internal/cache"
	"github.com/jacaudi/cwd/internal/sse"
)

func TestStream_EmitsSnapshotThenUpdate(t *testing.T) {
	c := cache.New()
	hub := sse.NewHub(c, []string{"nws_alerts"}, sse.WithPingInterval(5*time.Second))
	go hub.Run(context.Background())
	h := NewStreamHandler(hub)

	req := httptest.NewRequest("GET", "/api/stream", nil)
	rec := httptest.NewRecorder()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	go func() {
		time.Sleep(30 * time.Millisecond)
		c.Set(cache.Envelope{Source: "nws_alerts", FetchedAt: time.Now(), Validator: "v1", Payload: []string{"x"}})
	}()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, "event: snapshot") {
		t.Errorf("missing snapshot frame: %s", body)
	}
	if !strings.Contains(body, "event: nws_alerts.update") {
		t.Errorf("missing update frame: %s", body)
	}
	if rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("content-type: %q", rec.Header().Get("Content-Type"))
	}
}
```

- [ ] **Step 11.2: Run test, verify fail**

```bash
go test ./internal/api/ -run TestStream
```

- [ ] **Step 11.3: Implement `internal/api/stream.go`**

```go
package api

import (
	"net/http"

	"github.com/jacaudi/cwd/internal/sse"
)

type streamHandler struct {
	hub *sse.Hub
}

func NewStreamHandler(hub *sse.Hub) http.Handler {
	return &streamHandler{hub: hub}
}

func (h *streamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering if proxied
	h.hub.Serve(r.Context(), w)
}
```

- [ ] **Step 11.4: Wire route, run tests, commit**

```go
// Note: registered OUTSIDE the per-route WriteTimeout middleware group
// (Task 13). For now, register alongside other routes.
r.Method(http.MethodGet, "/api/stream", deps.StreamHandler)
```

```bash
go test ./internal/api/... -v -race
golangci-lint run ./internal/api/...
git add internal/api/stream.go internal/api/stream_test.go internal/api/router.go
git commit -m "feat(p1): add /api/stream SSE endpoint"
```

---

## Task 12: Carryover — HEAD method on `/healthz` and `/readyz`

**Files:**
- Modify: `internal/api/healthz.go`
- Modify: `internal/api/healthz_test.go`

**Dependencies:** none.

- [ ] **Step 12.1: Update `healthz_test.go` with HEAD assertions**

Add (or replace) tests covering both methods. Phase 0 wired the GET handlers via chi's `r.Get(...)`, which does NOT match HEAD. Switch to `r.Method(http.MethodGet, ...)` + `r.Method(http.MethodHead, ...)` (or chi's `MethodFunc`).

```go
func TestHealthz_HEADReturns200(t *testing.T) {
	mux := newTestRouter(t /* see existing test helper */)
	req := httptest.NewRequest(http.MethodHead, "/healthz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("HEAD /healthz: got %d, want 200", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("HEAD response body should be empty, got %d bytes", rec.Body.Len())
	}
}

func TestReadyz_HEADReflectsState(t *testing.T) {
	mux := newTestRouter(t)
	// Before first source success, /readyz HEAD should be 503.
	req := httptest.NewRequest(http.MethodHead, "/readyz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable && rec.Code != http.StatusOK {
		t.Errorf("HEAD /readyz: got %d, want 200 or 503", rec.Code)
	}
}
```

- [ ] **Step 12.2: Run test, verify failure**

```bash
go test ./internal/api/ -run TestHealthz_HEAD -v
```

- [ ] **Step 12.3: Update `healthz.go` registration**

In whatever helper currently registers the routes, replace `r.Get("/healthz", ...)` with both methods. Pattern with chi:

```go
r.Method(http.MethodGet, "/healthz", healthzHandler)
r.Method(http.MethodHead, "/healthz", healthzHandler)

r.Method(http.MethodGet, "/readyz", readyzHandler)
r.Method(http.MethodHead, "/readyz", readyzHandler)
```

(or use a small shared helper `r.Methods(GET, HEAD, "/healthz", h)` if Phase 0 already has one).

The handler bodies do NOT change — `net/http` will skip writing the body on HEAD requests automatically.

- [ ] **Step 12.4: Run tests, lint, commit**

```bash
go test ./internal/api/... -v
golangci-lint run ./internal/api/...
git add internal/api/healthz.go internal/api/healthz_test.go
git commit -m "fix(p1): accept HEAD on /healthz and /readyz (Phase 0 carryover)"
```

---

## Task 13: Carryover — per-route `WriteTimeout` middleware policy

**Files:**
- Modify: `internal/api/router.go`
- Create: `internal/api/router_timeout_test.go`

**Dependencies:** — depends on: Task 8, Task 9, Task 10, Task 11 (all four new endpoints exist).

- [ ] **Step 13.1: Write the failing test**

```go
package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// stallHandler holds the response open beyond the WriteTimeout.
func stallHandler(d time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(d):
			_, _ = w.Write([]byte("late"))
		case <-r.Context().Done():
			return
		}
	})
}

func TestRouter_PerRouteWriteTimeoutCutsJSONRoutes(t *testing.T) {
	// Wrap the JSON-route group's middleware around a stall handler that
	// holds for 2× the timeout.
	wrapped := WithJSONTimeout(stallHandler(200 * time.Millisecond))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil).WithContext(context.Background())
	wrapped.ServeHTTP(rec, req)
	if rec.Body.String() == "late" {
		t.Errorf("expected timeout middleware to interrupt the slow handler")
	}
}

func TestRouter_StreamRouteHasNoWriteTimeout(t *testing.T) {
	// SSE handlers must not be wrapped in the JSON-timeout middleware.
	// Verify by composition: the stream handler uses NO timeout wrapper.
	if isWrappedInJSONTimeout(StreamHandlerForTest()) {
		t.Errorf("/api/stream must not be wrapped in JSON timeout middleware")
	}
}
```

(The two test helpers `WithJSONTimeout`, `StreamHandlerForTest`, and `isWrappedInJSONTimeout` are exported just for this test; alternatively keep them unexported and put the test in the same package.)

- [ ] **Step 13.2: Run test, verify fail**

```bash
go test ./internal/api/ -run TestRouter_PerRoute
```

- [ ] **Step 13.3: Implement the middleware in `router.go`**

```go
// WithJSONTimeout applies a 5s WriteTimeout via http.TimeoutHandler.
// Apply to any JSON endpoint; do NOT apply to SSE.
func WithJSONTimeout(h http.Handler) http.Handler {
	return http.TimeoutHandler(h, 5*time.Second, `{"error":"upstream timeout"}`)
}
```

In the router builder, partition routes into two groups:

```go
// JSON group — 5s write timeout via TimeoutHandler middleware.
r.Group(func(r chi.Router) {
	r.Use(jsonTimeoutMiddleware) // adapter that wraps the chi.Handler chain
	r.Method(http.MethodGet, "/api/snapshot", deps.SnapshotHandler)
	r.Method(http.MethodGet, "/api/history", deps.HistoryHandler)
	r.Method(http.MethodGet, "/api/sources", deps.SourcesHandler)
	r.Method(http.MethodGet, "/api/version", deps.VersionHandler)
	r.Method(http.MethodGet, "/api/uiconfig", deps.UIConfigHandler)
	r.Method(http.MethodGet, "/healthz", deps.HealthzHandler)
	r.Method(http.MethodHead, "/healthz", deps.HealthzHandler)
	r.Method(http.MethodGet, "/readyz", deps.ReadyzHandler)
	r.Method(http.MethodHead, "/readyz", deps.ReadyzHandler)
})

// SSE — no write timeout.
r.Method(http.MethodGet, "/api/stream", deps.StreamHandler)
```

`jsonTimeoutMiddleware`:

```go
func jsonTimeoutMiddleware(next http.Handler) http.Handler {
	return WithJSONTimeout(next)
}
```

Add a comment block above the partition explaining the policy.

- [ ] **Step 13.4: Run tests, lint, commit**

```bash
go test ./internal/api/... -v -race
golangci-lint run ./internal/api/...
git add internal/api/router.go internal/api/router_timeout_test.go
git commit -m "fix(p1): per-route WriteTimeout — 5s on JSON routes, none on SSE (Phase 0 carryover)"
```

---

## Task 14: Carryover — config env walker reaches `sources.<name>.{interval,enabled}` + WARN on env-parse failures

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Dependencies:** none.

- [ ] **Step 14.1: Write the failing tests**

```go
func TestEnvOverride_PerSource(t *testing.T) {
	t.Setenv("CWD_SOURCES_NWS_ALERTS_INTERVAL", "45s")
	t.Setenv("CWD_SOURCES_NWS_ALERTS_ENABLED", "false")
	cfg, err := Load("testdata/full.yaml") // existing fixture
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	src, ok := cfg.Sources["nws_alerts"]
	if !ok {
		t.Fatalf("nws_alerts missing")
	}
	if src.Interval != 45*time.Second {
		t.Errorf("interval: got %v, want 45s", src.Interval)
	}
	if src.Enabled {
		t.Errorf("enabled: expected false")
	}
}

func TestEnvOverride_WarnsOnBadValue(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	t.Setenv("CWD_SOURCES_NWS_ALERTS_INTERVAL", "not-a-duration")
	cfg, err := LoadWithLogger("testdata/full.yaml", logger)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("config.env_parse_failed")) {
		t.Errorf("expected WARN, got: %s", buf.String())
	}
	// Bad value must fall back to YAML — no zero/empty default.
	if cfg.Sources["nws_alerts"].Interval == 0 {
		t.Errorf("expected fallback to YAML interval, got zero")
	}
}
```

- [ ] **Step 14.2: Run, verify fail**

```bash
go test ./internal/config/... -run TestEnvOverride
```

- [ ] **Step 14.3: Extend the env walker**

Locate `applyEnvOverrides` in `internal/config/config.go`. Add a `LoadWithLogger(path string, logger *slog.Logger) (*Config, error)` constructor (and have the existing `Load(path)` call it with `slog.Default()`). Inside `applyEnvOverrides`, after the existing top-level scalar walk, add:

```go
// Per-source overrides: CWD_SOURCES_<NAME_UPPER_SNAKE>_<FIELD>
for name, src := range cfg.Sources {
	upper := strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
	intervalKey := "CWD_SOURCES_" + upper + "_INTERVAL"
	if v := os.Getenv(intervalKey); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			logger.Warn("config.env_parse_failed", "key", intervalKey, "value", v, "err", err.Error())
		} else {
			src.Interval = d
		}
	}
	enabledKey := "CWD_SOURCES_" + upper + "_ENABLED"
	if v := os.Getenv(enabledKey); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			logger.Warn("config.env_parse_failed", "key", enabledKey, "value", v, "err", err.Error())
		} else {
			src.Enabled = b
		}
	}
	cfg.Sources[name] = src
}
```

Also: scan the existing top-level scalar walker and replace each silent `_ = strconv.Parse...` ignore with a `logger.Warn("config.env_parse_failed", ...)` of the same shape. Bad values fall back to YAML (do not zero-out the field).

- [ ] **Step 14.4: Run tests, lint, commit**

```bash
go test ./internal/config/... -v
golangci-lint run ./internal/config/...
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(p1): per-source env overrides + WARN on env-parse failures (Phase 0 carryovers)"
```

---

## Task 15: Server wiring (boot order + hot-start cache + `/readyz` predicate + UA threading + prune ticker)

**Files:**
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`
- Modify: `cmd/cwd/main.go` (only if Phase 0 didn't already pass `slog.Logger` and config into server.Run)

**Dependencies:** — depends on: Task 4 (Cache), Task 5 (Store), Task 6 (Fetcher), Task 7 (SSE Hub), Task 8 (sources handler), Task 9 (snapshot), Task 10 (history), Task 11 (stream), Task 12 (HEAD), Task 13 (timeout middleware), Task 14 (config).

- [ ] **Step 15.1: Write the failing test**

```go
package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jacaudi/cwd/internal/config"
)

func TestServer_ReadyzGoesGreenAfterFirstFetch(t *testing.T) {
	// Spin up an httptest fake-NOAA returning a known body.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"type":"FeatureCollection","features":[]}`))
	}))
	defer upstream.Close()

	cfg := minimalConfigPointing(t, upstream.URL) // helper: returns config with nws_alerts URL overridden
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go srv.Run(ctx)

	// Poll /readyz until 200 or context expires.
	deadline := time.After(1500 * time.Millisecond)
	for {
		select {
		case <-deadline:
			t.Fatal("readyz never went 200")
		default:
		}
		resp, err := http.Get(srv.LocalURL() + "/readyz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = strings.Contains // silence unused import in stub
}
```

- [ ] **Step 15.2: Run test, verify fail**

- [ ] **Step 15.3: Implement server wiring**

In `internal/server/server.go`, after Phase 0's basic listener:

```go
type Server struct {
	cfg      *config.Config
	logger   *slog.Logger
	cache    *cache.Cache
	store    *store.Store
	fetchers map[string]*fetcher.Fetcher
	hub      *sse.Hub
	mux      http.Handler
	httpSrv  *http.Server
	listener net.Listener
}

func New(cfg *config.Config) (*Server, error) {
	logger := newLogger(cfg) // Phase 0 helper
	st, err := store.Open(cfg.Store.Path)
	if err != nil {
		return nil, err
	}
	if err := st.Migrate(context.Background()); err != nil {
		return nil, err
	}

	c := cache.New()

	// Build sources from config; Phase 1 ships only nws_alerts.
	srcs := map[string]sources.Source{}
	if scfg, ok := cfg.Sources["nws_alerts"]; ok && scfg.Enabled {
		ua := buildUserAgent(cfg) // "cwd-self-host/<version> (<contact>)"
		na := sources.NewNWSAlerts("https://api.weather.gov/alerts/active", ua, logger)
		na.SetInterval(scfg.Interval)
		srcs["nws_alerts"] = na
	}
	if scfg, ok := cfg.Sources["nws_alerts"]; ok && scfg.Enabled && cfg.Server.Contact == "" {
		logger.Warn("server.contact_unset", "msg", "nws_alerts enabled but server.contact empty; politeness header degrades")
	}

	// Hot-start cache from store.
	for name := range srcs {
		if row, ok, err := st.Latest(context.Background(), name); err == nil && ok {
			var alerts []sources.Alert
			if err := json.Unmarshal(row.Payload, &alerts); err == nil {
				c.Set(cache.Envelope{Source: name, FetchedAt: row.FetchedAt, Validator: row.Validator, Payload: alerts})
			}
		}
	}

	fetchers := map[string]*fetcher.Fetcher{}
	for name, src := range srcs {
		fetchers[name] = fetcher.New(src, c, st, fetcher.WithLogger(logger))
		_ = name
	}

	enabledNames := make([]string, 0, len(srcs))
	for n := range srcs {
		enabledNames = append(enabledNames, n)
	}
	hub := sse.NewHub(c, enabledNames)

	filter := sources.NewFilter(cfg.Derived.Thresholds.RegionFilter.UGCs, cfg.Derived.Thresholds.RegionFilter.WFOs)

	s := &Server{cfg: cfg, logger: logger, cache: c, store: st, fetchers: fetchers, hub: hub}
	deps := api.Deps{
		SnapshotHandler: api.NewSnapshotHandler(c, filter),
		HistoryHandler:  api.NewHistoryHandler(st, filter),
		StreamHandler:   api.NewStreamHandler(hub),
		SourcesHandler:  api.NewSourcesHandler(s),
		HealthzHandler:  api.NewHealthzHandler(),
		ReadyzHandler:   api.NewReadyzHandler(s),
		VersionHandler:  api.NewVersionHandler(),
		UIConfigHandler: api.NewUIConfigHandler(cfg),
		WebDist:         webdist.FS,
	}
	s.mux = api.BuildRouter(deps)
	s.httpSrv = &http.Server{
		Addr:        cfg.Server.Bind,
		Handler:     s.mux,
		ReadTimeout: 10 * time.Second,
		// WriteTimeout intentionally unset — per-route policy via TimeoutHandler middleware.
	}
	return s, nil
}

func (s *Server) Health() map[string]fetcher.Health {
	out := make(map[string]fetcher.Health, len(s.fetchers))
	for n, f := range s.fetchers {
		out[n] = f.Health()
	}
	return out
}

// IsReady is consumed by api.NewReadyzHandler.
func (s *Server) IsReady() bool {
	for _, f := range s.fetchers {
		if f.Health().LastSuccess.IsZero() {
			return false
		}
	}
	return true
}

func (s *Server) Run(ctx context.Context) error {
	// Spawn fetchers + hub.
	for _, f := range s.fetchers {
		go f.Run(ctx)
	}
	go s.hub.Run(ctx)

	// Prune ticker (every 6h).
	go s.runPrune(ctx)

	ln, err := net.Listen("tcp", s.cfg.Server.Bind)
	if err != nil {
		return err
	}
	s.listener = ln
	go func() {
		<-ctx.Done()
		_ = s.httpSrv.Shutdown(context.Background())
	}()
	if err := s.httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) runPrune(ctx context.Context) {
	tick := time.NewTicker(6 * time.Hour)
	defer tick.Stop()
	// Run once at boot too.
	if _, err := s.store.Prune(ctx, time.Duration(s.cfg.Store.RetentionDays)*24*time.Hour); err != nil {
		s.logger.Warn("server.prune", "err", err.Error())
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if _, err := s.store.Prune(ctx, time.Duration(s.cfg.Store.RetentionDays)*24*time.Hour); err != nil {
				s.logger.Warn("server.prune", "err", err.Error())
			}
		}
	}
}

func (s *Server) LocalURL() string { return "http://" + s.listener.Addr().String() }
```

`buildUserAgent`:

```go
func buildUserAgent(cfg *config.Config) string {
	contact := cfg.Server.Contact
	if contact == "" {
		contact = "no-contact-configured"
	}
	return fmt.Sprintf("cwd-self-host/%s (%s)", version.Version, contact)
}
```

`api.NewReadyzHandler`:

```go
type Readyer interface{ IsReady() bool }
func NewReadyzHandler(r Readyer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if r.IsReady() {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	})
}
```

- [ ] **Step 15.4: Run tests, lint, commit**

```bash
go test ./internal/server/... -v -race
go test ./...
golangci-lint run ./...
git add internal/server/ internal/api/ cmd/cwd/main.go
git commit -m "feat(p1): wire fetcher→cache→store→hub; hot-start cache; readyz reflects first-fetch"
```

---

## Task 16: Carryover — Vitest + RTL scaffold + `task web:test`

**Files:**
- Modify: `web/package.json`
- Create: `web/vitest.config.ts`
- Create: `web/src/test/setup.ts`
- Create: `web/src/test/smoke.test.ts`
- Modify: `Taskfile.yml`

**Dependencies:** none.

- [ ] **Step 16.1: Add deps**

```bash
cd web
pnpm add -D vitest @testing-library/react @testing-library/jest-dom jsdom @vitest/ui happy-dom
cd ..
```

- [ ] **Step 16.2: Create `web/vitest.config.ts`**

```ts
import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    css: false,
  },
});
```

- [ ] **Step 16.3: Create `web/src/test/setup.ts`**

```ts
import '@testing-library/jest-dom/vitest';
```

- [ ] **Step 16.4: Create the smoke test `web/src/test/smoke.test.ts`**

```ts
import { describe, it, expect } from 'vitest';

describe('vitest scaffold', () => {
  it('runs', () => {
    expect(1 + 1).toBe(2);
  });
});
```

- [ ] **Step 16.5: Add the task target**

Append to `Taskfile.yml`:

```yaml
  web:test:
    desc: Run frontend Vitest suite
    dir: web
    cmds:
      - pnpm exec vitest run
```

Add a `test:script` entry to `web/package.json`:

```json
"scripts": {
  "test": "vitest run",
  "test:watch": "vitest"
}
```

- [ ] **Step 16.6: Run, verify smoke passes**

```bash
task web:test
```

Expected: 1 passed.

- [ ] **Step 16.7: Wire into CI**

The reusable `test-web` workflow runs whatever `pnpm test` resolves to once `vitest` is on the path. Phase 0 already invokes `pnpm typecheck && pnpm build`; ensure the workflow file in `.github/workflows/pr.yml` and `.github/workflows/ci.yml` calls `pnpm test` (or `task web:test`) too. If it currently only runs typecheck+build, add the test step.

- [ ] **Step 16.8: Commit**

```bash
git add web/package.json web/pnpm-lock.yaml web/vitest.config.ts web/src/test/ Taskfile.yml .github/workflows/
git commit -m "feat(p1): add Vitest + RTL scaffold and task web:test (Phase 0 carryover)"
```

---

## Task 17: Frontend types + Zustand store + EventSource stream client

**Files:**
- Modify: `web/src/api/types.ts`
- Create: `web/src/api/stream.ts`
- Create: `web/src/store/snapshot.ts`
- Modify: `web/package.json` (add `zustand`)

**Dependencies:** — depends on: Task 16 (Vitest scaffold), Task 9 (snapshot wire shape).

- [ ] **Step 17.1: Add zustand**

```bash
cd web && pnpm add zustand && cd ..
```

- [ ] **Step 17.2: Update `web/src/api/types.ts`**

```ts
export type Severity = 'Extreme' | 'Severe' | 'Moderate' | 'Minor' | 'Unknown';

export type Category =
  | 'Tornado' | 'SevereThunderstorm' | 'FlashFlood' | 'Tropical'
  | 'HighWind' | 'RedFlag' | 'Winter' | 'ExtremeHeat' | 'ExtremeCold'
  | 'Tsunami' | 'Unknown';

export interface Alert {
  id: string;
  event: string;
  awips: string;
  headline: string;
  severity: Severity;
  sent: string;       // RFC3339
  effective: string;
  onset?: string;
  expires: string;
  ends?: string;
  areas: string[];
  ugcs?: string[];
  sames?: string[];
  wfo?: string;
  vtecEtn?: string;
  url?: string;
  category: Category;
}

export interface Envelope<T> {
  source: string;
  fetchedAt: string;
  etag?: string;
  payload: T;
}

export interface Snapshot {
  serverTime: string;
  sources: {
    nws_alerts?: Envelope<Alert[]>;
    // swpc_scales, swpc_alerts, usgs_quakes, usgs_volcanoes: Phase 2
  };
}

export interface SourceHealth {
  intervalSec: number;
  lastAttempt: string;
  lastSuccess: string;
  lastError?: string;
  etag?: string;
  ageSec: number;
  consecutiveFailures: number;
}
```

- [ ] **Step 17.3: Create `web/src/store/snapshot.ts` with a passing test**

`web/src/store/snapshot.test.ts`:

```ts
import { describe, it, expect } from 'vitest';
import { useSnapshotStore } from './snapshot';

describe('snapshot store', () => {
  it('starts empty', () => {
    const s = useSnapshotStore.getState();
    expect(s.snapshot).toBeNull();
    expect(s.connection).toBe('connecting');
  });

  it('replaces snapshot on setSnapshot', () => {
    useSnapshotStore.getState().setSnapshot({
      serverTime: '2026-05-02T12:00:00Z',
      sources: { nws_alerts: { source: 'nws_alerts', fetchedAt: '...', payload: [] } },
    });
    expect(useSnapshotStore.getState().snapshot?.sources.nws_alerts?.payload).toEqual([]);
  });

  it('updates one source on applyUpdate', () => {
    useSnapshotStore.getState().setSnapshot({
      serverTime: '2026-05-02T12:00:00Z',
      sources: { nws_alerts: { source: 'nws_alerts', fetchedAt: 'a', payload: [] } },
    });
    useSnapshotStore.getState().applyUpdate('nws_alerts', {
      source: 'nws_alerts', fetchedAt: 'b', payload: [{ id: '1' } as any],
    });
    const env = useSnapshotStore.getState().snapshot?.sources.nws_alerts;
    expect(env?.fetchedAt).toBe('b');
    expect(env?.payload.length).toBe(1);
  });
});
```

`web/src/store/snapshot.ts`:

```ts
import { create } from 'zustand';
import type { Envelope, Alert, Snapshot } from '../api/types';

type Connection = 'connecting' | 'live' | 'polling' | 'error';

interface State {
  snapshot: Snapshot | null;
  connection: Connection;
  setSnapshot: (s: Snapshot) => void;
  applyUpdate: (source: keyof Snapshot['sources'], env: Envelope<Alert[]>) => void;
  setConnection: (c: Connection) => void;
}

export const useSnapshotStore = create<State>((set) => ({
  snapshot: null,
  connection: 'connecting',
  setSnapshot: (s) => set({ snapshot: s }),
  applyUpdate: (source, env) =>
    set((prev) => {
      const base: Snapshot = prev.snapshot ?? { serverTime: env.fetchedAt, sources: {} };
      return {
        snapshot: {
          ...base,
          sources: { ...base.sources, [source]: env },
        },
      };
    }),
  setConnection: (c) => set({ connection: c }),
}));
```

- [ ] **Step 17.4: Create `web/src/api/stream.ts` with a test**

`web/src/api/stream.test.ts`:

```ts
import { describe, it, expect, vi, afterEach } from 'vitest';
import { connect } from './stream';

class FakeES {
  static last: FakeES | null = null;
  listeners: Record<string, ((e: any) => void)[]> = {};
  onerror: any = null;
  constructor(public url: string) { FakeES.last = this; }
  addEventListener(name: string, fn: (e: any) => void) {
    (this.listeners[name] ||= []).push(fn);
  }
  emit(name: string, data: any) {
    (this.listeners[name] || []).forEach(fn => fn({ data: JSON.stringify(data) }));
  }
  close() {}
}

describe('stream client', () => {
  afterEach(() => { vi.restoreAllMocks(); });

  it('fetches initial snapshot then routes SSE events', async () => {
    const fakeFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ serverTime: 't', sources: {} }),
    });
    vi.stubGlobal('fetch', fakeFetch);
    vi.stubGlobal('EventSource', FakeES as any);

    const events: any[] = [];
    await connect({ onSnapshot: e => events.push({ type: 'snap', e }), onUpdate: (n, e) => events.push({ type: 'upd', n, e }) });

    expect(fakeFetch).toHaveBeenCalledWith('/api/snapshot');
    FakeES.last!.emit('nws_alerts.update', { source: 'nws_alerts', fetchedAt: 'b', payload: [] });
    expect(events.find(x => x.type === 'upd' && x.n === 'nws_alerts')).toBeTruthy();
  });
});
```

`web/src/api/stream.ts`:

```ts
import type { Snapshot, Envelope, Alert } from './types';

export interface ConnectOpts {
  onSnapshot: (s: Snapshot) => void;
  onUpdate: (source: keyof Snapshot['sources'], env: Envelope<Alert[]>) => void;
  onError?: (e: unknown) => void;
}

export async function connect(opts: ConnectOpts): Promise<() => void> {
  // Initial paint via /api/snapshot.
  try {
    const res = await fetch('/api/snapshot');
    if (res.ok) opts.onSnapshot(await res.json());
  } catch (e) { opts.onError?.(e); }

  const es = new EventSource('/api/stream');
  es.addEventListener('snapshot', (e: MessageEvent) => {
    try { opts.onSnapshot(JSON.parse(e.data)); } catch (err) { opts.onError?.(err); }
  });
  es.addEventListener('nws_alerts.update', (e: MessageEvent) => {
    try { opts.onUpdate('nws_alerts', JSON.parse(e.data)); } catch (err) { opts.onError?.(err); }
  });
  return () => es.close();
}
```

- [ ] **Step 17.5: Run `task web:test`, lint, commit**

```bash
task web:test
git add web/package.json web/pnpm-lock.yaml web/src/api/types.ts web/src/api/stream.ts web/src/api/stream.test.ts web/src/store/
git commit -m "feat(p1): frontend types + zustand snapshot store + EventSource client"
```

---

## Task 18: Frontend `AlertBadgeBar`

**Files:**
- Create: `web/src/components/AlertBadgeBar.tsx`
- Create: `web/src/components/AlertBadgeBar.test.tsx`

**Dependencies:** — depends on: Task 16 (Vitest), Task 17 (types).

- [ ] **Step 18.1: Write the test**

```tsx
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { AlertBadgeBar } from './AlertBadgeBar';
import type { Alert } from '../api/types';

const a = (cat: Alert['category'], over: Partial<Alert> = {}): Alert => ({
  id: Math.random().toString(),
  event: cat, awips: 'XXX', headline: 'h',
  severity: 'Severe', sent: 't', effective: 't', expires: 't',
  areas: [], category: cat, ...over,
});

describe('AlertBadgeBar', () => {
  it('renders all 10 badges with zero counts when payload empty', () => {
    render(<AlertBadgeBar alerts={[]} onTsunamiClick={() => {}} />);
    for (const cat of ['Tornado','Severe Thunderstorm','Flash Flood','Tropical','High Wind','Red Flag','Winter','Extreme Heat','Extreme Cold','Tsunami']) {
      expect(screen.getByLabelText(cat)).toBeInTheDocument();
    }
  });

  it('counts alerts by category and ignores Unknown', () => {
    const alerts = [a('Tornado'), a('Tornado'), a('Tsunami'), a('Unknown')];
    render(<AlertBadgeBar alerts={alerts} onTsunamiClick={() => {}} />);
    expect(screen.getByLabelText('Tornado').textContent).toContain('2');
    expect(screen.getByLabelText('Tsunami').textContent).toContain('1');
  });
});
```

- [ ] **Step 18.2: Implement component**

```tsx
import { useMemo } from 'react';
import { Badge, Space, Tag, Tooltip } from 'antd';
import type { Alert, Category } from '../api/types';

const ORDER: { key: Exclude<Category, 'Unknown'>; label: string }[] = [
  { key: 'Tornado',            label: 'Tornado' },
  { key: 'SevereThunderstorm', label: 'Severe Thunderstorm' },
  { key: 'FlashFlood',         label: 'Flash Flood' },
  { key: 'Tropical',           label: 'Tropical' },
  { key: 'HighWind',           label: 'High Wind' },
  { key: 'RedFlag',            label: 'Red Flag' },
  { key: 'Winter',             label: 'Winter' },
  { key: 'ExtremeHeat',        label: 'Extreme Heat' },
  { key: 'ExtremeCold',        label: 'Extreme Cold' },
  { key: 'Tsunami',            label: 'Tsunami' },
];

const COLOR: Record<Exclude<Category, 'Unknown'>, string> = {
  Tornado: 'red', SevereThunderstorm: 'volcano', FlashFlood: 'cyan',
  Tropical: 'magenta', HighWind: 'gold', RedFlag: 'orange',
  Winter: 'blue', ExtremeHeat: 'red', ExtremeCold: 'geekblue',
  Tsunami: 'purple',
};

interface Props {
  alerts: Alert[];
  onTsunamiClick: () => void;
}

export function AlertBadgeBar({ alerts, onTsunamiClick }: Props) {
  const counts = useMemo(() => {
    const m: Record<string, number> = {};
    for (const a of alerts) m[a.category] = (m[a.category] ?? 0) + 1;
    return m;
  }, [alerts]);

  return (
    <Space size="small" wrap aria-label="active-alerts">
      {ORDER.map(({ key, label }) => {
        const n = counts[key] ?? 0;
        const color = n > 0 ? COLOR[key] : 'default';
        const onClick = key === 'Tsunami' ? onTsunamiClick : undefined;
        return (
          <Tooltip key={key} title={label}>
            <Badge count={n} showZero color={n > 0 ? undefined : '#999'}>
              <Tag color={color} aria-label={label} onClick={onClick} style={{ cursor: onClick && n > 0 ? 'pointer' : 'default' }}>
                {label}
              </Tag>
            </Badge>
          </Tooltip>
        );
      })}
    </Space>
  );
}
```

- [ ] **Step 18.3: Test, lint, commit**

```bash
task web:test
git add web/src/components/AlertBadgeBar.tsx web/src/components/AlertBadgeBar.test.tsx
git commit -m "feat(p1): AlertBadgeBar component (10 categories, severity color, count badges)"
```

---

## Task 19: Frontend `TsunamiPanel`

**Files:**
- Create: `web/src/components/TsunamiPanel.tsx`
- Create: `web/src/components/TsunamiPanel.test.tsx`

**Dependencies:** — depends on: Task 16, Task 17.

- [ ] **Step 19.1: Write test**

```tsx
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { TsunamiPanel } from './TsunamiPanel';
import type { Alert } from '../api/types';

const tsu = (wfo = 'NTW'): Alert => ({
  id: 'a', event: 'Tsunami Warning', awips: 'TSUWCA',
  headline: 'Tsunami Warning issued', severity: 'Extreme',
  sent: '2026-05-02T11:00:00Z', effective: '2026-05-02T11:00:00Z',
  expires: '2026-05-02T17:00:00Z',
  areas: ['Coastal Northern California'], category: 'Tsunami', wfo,
});

describe('TsunamiPanel', () => {
  it('returns null when no TSU alerts', () => {
    const { container } = render(<TsunamiPanel alerts={[]} />);
    expect(container.firstChild).toBeNull();
  });

  it('renders one item per TSU alert with NWS product link', () => {
    render(<TsunamiPanel alerts={[tsu('NTW')]} />);
    expect(screen.getByText(/Tsunami Warning issued/)).toBeInTheDocument();
    const link = screen.getByRole('link') as HTMLAnchorElement;
    expect(link.href).toContain('product.php');
    expect(link.href).toContain('issuedby=NTW');
  });

  it('uses AWIPS prefix to filter, not event text', () => {
    const a: Alert = { ...tsu(), event: 'Some Renamed Event' };
    render(<TsunamiPanel alerts={[a]} />);
    expect(screen.getByText(/Some Renamed Event|Tsunami Warning issued/)).toBeInTheDocument();
  });
});
```

- [ ] **Step 19.2: Implement**

```tsx
import { Avatar, List, Tag } from 'antd';
import { ExportOutlined } from '@ant-design/icons';
import type { Alert } from '../api/types';

interface Props {
  alerts: Alert[];
}

export function TsunamiPanel({ alerts }: Props) {
  const tsu = alerts.filter(a => a.awips.startsWith('TSU'));
  if (tsu.length === 0) return null;
  return (
    <List
      header={<strong>Tsunami</strong>}
      dataSource={tsu}
      renderItem={a => (
        <List.Item
          actions={[
            <a
              key="link"
              href={`https://forecast.weather.gov/product.php?site=NWS&product=TSU&issuedby=${a.wfo ?? ''}`}
              target="_blank"
              rel="noopener noreferrer"
            >
              <ExportOutlined /> NWS product
            </a>,
          ]}
        >
          <List.Item.Meta
            avatar={<Avatar style={{ backgroundColor: '#722ed1' }}>T</Avatar>}
            title={a.headline}
            description={
              <span>
                {a.areas.join(', ')} <Tag>{new Date(a.sent).toUTCString()}</Tag>
              </span>
            }
          />
        </List.Item>
      )}
    />
  );
}
```

- [ ] **Step 19.3: Test, lint, commit**

```bash
task web:test
git add web/src/components/TsunamiPanel.tsx web/src/components/TsunamiPanel.test.tsx
git commit -m "feat(p1): TsunamiPanel component (filter by AWIPS prefix, NWS product link)"
```

---

## Task 20: Frontend `SourceHealthIndicator`

**Files:**
- Create: `web/src/components/SourceHealthIndicator.tsx`
- Create: `web/src/components/SourceHealthIndicator.test.tsx`

**Dependencies:** — depends on: Task 16, Task 17, Task 8.

- [ ] **Step 20.1: Test**

```tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { SourceHealthIndicator } from './SourceHealthIndicator';

describe('SourceHealthIndicator', () => {
  it('renders one tag per source, color reflects health', async () => {
    const fakeFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        nws_alerts: { intervalSec: 30, lastAttempt: 't', lastSuccess: 't', ageSec: 5, consecutiveFailures: 0 },
      }),
    });
    vi.stubGlobal('fetch', fakeFetch);
    render(<SourceHealthIndicator pollMs={50} />);
    await waitFor(() => expect(screen.getByText(/nws_alerts/)).toBeInTheDocument());
  });
});
```

- [ ] **Step 20.2: Implement**

```tsx
import { useEffect, useState } from 'react';
import { Space, Tag, Tooltip } from 'antd';
import type { SourceHealth } from '../api/types';

interface Props { pollMs?: number }

export function SourceHealthIndicator({ pollMs = 15000 }: Props) {
  const [data, setData] = useState<Record<string, SourceHealth>>({});
  useEffect(() => {
    let alive = true;
    const tick = async () => {
      try {
        const r = await fetch('/api/sources');
        if (!r.ok) return;
        if (alive) setData(await r.json());
      } catch { /* swallow; tag stays last-known */ }
    };
    tick();
    const id = setInterval(tick, pollMs);
    return () => { alive = false; clearInterval(id); };
  }, [pollMs]);

  return (
    <Space size="small">
      {Object.entries(data).map(([name, h]) => {
        const color = h.consecutiveFailures > 0 ? 'red'
                    : h.ageSec > 2 * h.intervalSec ? 'gold'
                    : 'green';
        const tip = `${h.lastSuccess || 'never'} (age: ${h.ageSec}s, failures: ${h.consecutiveFailures})`;
        return (
          <Tooltip key={name} title={tip}>
            <Tag color={color}>{name}</Tag>
          </Tooltip>
        );
      })}
    </Space>
  );
}
```

- [ ] **Step 20.3: Test, lint, commit**

```bash
task web:test
git add web/src/components/SourceHealthIndicator.tsx web/src/components/SourceHealthIndicator.test.tsx
git commit -m "feat(p1): SourceHealthIndicator footer component (15s poll of /api/sources)"
```

---

## Task 21: Wire Overview page + footer

**Files:**
- Modify: `web/src/pages/Overview.tsx`
- Modify: `web/src/App.tsx` (footer slot only — Phase 0's ProLayout has `footerRender` available)
- Create: `web/src/pages/Overview.test.tsx` (smoke)

**Dependencies:** — depends on: Task 17, Task 18, Task 19, Task 20.

- [ ] **Step 21.1: Replace `<Empty>` placeholder in Overview**

```tsx
import { useEffect, useRef } from 'react';
import { Card, Space, Typography } from 'antd';
import { AlertBadgeBar } from '../components/AlertBadgeBar';
import { TsunamiPanel } from '../components/TsunamiPanel';
import { useSnapshotStore } from '../store/snapshot';
import { connect } from '../api/stream';

export function Overview() {
  const snapshot = useSnapshotStore(s => s.snapshot);
  const setSnapshot = useSnapshotStore(s => s.setSnapshot);
  const applyUpdate = useSnapshotStore(s => s.applyUpdate);
  const setConnection = useSnapshotStore(s => s.setConnection);
  const tsuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    let cancel: (() => void) | undefined;
    connect({
      onSnapshot: s => { setSnapshot(s); setConnection('live'); },
      onUpdate: (name, env) => applyUpdate(name, env),
      onError: () => setConnection('error'),
    }).then(c => { cancel = c; });
    return () => cancel?.();
  }, [setSnapshot, applyUpdate, setConnection]);

  const alerts = snapshot?.sources.nws_alerts?.payload ?? [];
  const fetchedAt = snapshot?.sources.nws_alerts?.fetchedAt;

  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Card
        title="Active Alerts"
        extra={fetchedAt ? <Typography.Text type="secondary">as of {new Date(fetchedAt).toUTCString()}</Typography.Text> : null}
      >
        <AlertBadgeBar alerts={alerts} onTsunamiClick={() => tsuRef.current?.scrollIntoView({ behavior: 'smooth' })} />
      </Card>
      <div ref={tsuRef}>
        <TsunamiPanel alerts={alerts} />
      </div>
    </Space>
  );
}

export default Overview;
```

- [ ] **Step 21.2: Mount `SourceHealthIndicator` in the ProLayout footer**

In `web/src/App.tsx`, change `footerRender` (Phase 0) to:

```tsx
footerRender={() => (
  <div style={{ padding: '8px 16px', textAlign: 'center' }}>
    <SourceHealthIndicator />
  </div>
)}
```

(Add the import.)

- [ ] **Step 21.3: Smoke test for Overview**

```tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import Overview from './Overview';

describe('Overview page', () => {
  it('renders badge bar after initial snapshot', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ serverTime: 't', sources: { nws_alerts: { source: 'nws_alerts', fetchedAt: 't', payload: [] } } }),
    }));
    class FakeES { addEventListener() {} close() {} onerror = null }
    vi.stubGlobal('EventSource', FakeES as any);
    render(<Overview />);
    await waitFor(() => expect(screen.getByLabelText('Tornado')).toBeInTheDocument());
  });
});
```

- [ ] **Step 21.4: Verify everything builds and tests pass**

```bash
task web:test
task web:typecheck
task web:build
```

- [ ] **Step 21.5: Commit**

```bash
git add web/src/pages/Overview.tsx web/src/pages/Overview.test.tsx web/src/App.tsx
git commit -m "feat(p1): wire Overview page (badge bar + tsunami panel + SSE) and footer health"
```

---

## Task 22: README updates + smoke task + final integration check

**Files:**
- Modify: `README.md`
- Modify: `Taskfile.yml` (add `smoke` target)

**Dependencies:** — depends on: all prior tasks.

- [ ] **Step 22.1: Update `README.md`**

Add a "Phase 1 status" section describing:
- What's live now: `nws_alerts` source, badge bar, tsunami panel, SSE stream, history endpoint, source health footer
- Per-source env vars: `CWD_SOURCES_NWS_ALERTS_INTERVAL=45s`, `CWD_SOURCES_NWS_ALERTS_ENABLED=false`
- The `server.contact` / User-Agent contract (link to api.weather.gov politeness rules)
- Reverse-proxy note for SSE: disable response buffering on `/api/stream`

- [ ] **Step 22.2: Add smoke target to `Taskfile.yml`**

```yaml
  smoke:
    desc: Boot the server against real api.weather.gov for 5 minutes; report /api/sources
    deps: [build]
    cmds:
      - |
        ./bin/cwd serve &
        PID=$!
        echo "PID=$PID; sleeping 60s for first fetch"
        sleep 60
        echo "---- /api/sources ----"
        curl -sS http://127.0.0.1:8765/api/sources | jq .
        echo "---- /api/snapshot (truncated) ----"
        curl -sS http://127.0.0.1:8765/api/snapshot | jq '.sources.nws_alerts.payload | length' || true
        sleep 240
        kill $PID
```

- [ ] **Step 22.3: Run the full integration check**

```bash
task build:all
task test
task lint
task web:test
task web:typecheck
task web:build
./bin/cwd serve &
PID=$!
sleep 45
curl -sS http://127.0.0.1:8765/healthz
curl -sS http://127.0.0.1:8765/readyz
curl -sS http://127.0.0.1:8765/api/sources | jq .
curl -sS http://127.0.0.1:8765/api/snapshot | jq '.sources.nws_alerts.payload | length'
kill $PID
```

Expected: `/readyz` returns 200, `/api/sources` shows `nws_alerts` with `consecutiveFailures: 0` and a recent `lastSuccess`, `/api/snapshot` returns a numeric alert count. Open `http://127.0.0.1:8765/` in a browser, confirm the badge bar populates and (if any tsunamis are active) the panel appears.

- [ ] **Step 22.4: Commit and prepare for review**

```bash
git add README.md Taskfile.yml
git commit -m "docs(p1): README phase-1 notes + smoke task"
```

---

## Self-review (writing-plans skill checklist)

**Spec coverage:** every section of the design is mapped to a task.
| Design § | Task(s) |
|---|---|
| §3 (no-ETag → content hash) | Task 2 (Validator computed in Source), Task 6 (passed through fetcher) |
| §4 (package layout) | Tasks 2-15 |
| §5 (wire types) | Task 2 (Alert), Task 9 (Envelope, SnapshotResponse), Task 17 (TS mirror) |
| §5.1 (category map) | Task 2 |
| §6.1 (boot, hot-start) | Task 15 |
| §6.2 (fetch loop) | Task 6 |
| §6.3 (cache → SSE) | Task 4 + Task 7 |
| §6.4 (HTTP routes) | Tasks 8-12 + Task 13 (timeout policy) |
| §6.5 (region filter) | Task 3 (predicate) + Tasks 9, 10, 15 (applied) |
| §6.6 (SSE wire format) | Task 7 + Task 11 |
| §7 (frontend) | Tasks 17-21 |
| §8 (Phase 0 carryovers) | Tasks 12 (HEAD), 13 (timeout), 14 (env+WARN), 16 (Vitest) |
| §9 (no new config keys) | Verified by Task 14 only mutating walker, not schema |
| §10 (testing) | Each task includes its tests; Task 22 covers end-to-end smoke |
| §11 (`/api/sources` shape) | Task 6 (Health struct) + Task 8 (handler) |
| §12 (DoD) | Task 22 |

**Placeholder scan:** searched for `TBD`, `TODO`, `implement later`, `add appropriate`, `similar to`, `etc.` — none present in the plan body. (`TODO`-style comments inside example code blocks are intentionally part of the example.)

**Type consistency:** `Envelope` (Go: `internal/api`, TS: `web/src/api/types.ts`) field names match — `source`, `fetchedAt`, `etag`, `payload`. `Health` field names match between Go (`internal/fetcher/fetcher.go`) and TS (`SourceHealth` in `web/src/api/types.ts`). The wire shape `SnapshotResponse` uses `sources` as a map; the SSE hub also uses a `Sources map[string]any` — the wire is a JSON object keyed by source name on both paths.

---

## Execution handoff

**Plan complete and saved to `docs/plans/2026-05-02-phase1-nws-alerts-implementation.md`.** Two execution options per `superpowers:writing-plans`:

1. **Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration. Aligns with this repo's `~/.claude/rules/subagent-development-workflow.md` mandate.
2. **Inline Execution** — Execute tasks in this session using `superpowers:executing-plans`, batch execution with checkpoints.

The kickoff prompt and `~/.claude/rules/plan-workflow.md` mandate option **1** (`superpowers:subagent-driven-development` is non-negotiable for multi-task plans), with the worktree, per-task review, and final independent comprehensive review bundled in. Proceeding on that path unless redirected.
