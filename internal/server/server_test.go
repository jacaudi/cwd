package server

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jacaudi/cwd/internal/config"
	"github.com/jacaudi/cwd/internal/imageproxy"
)

func TestRunStartsAndShutsDownGracefully(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Bind:      "127.0.0.1:0", // pick any free port
			LogLevel:  "info",
			LogFormat: "text",
		},
		UI: config.UIConfig{DefaultTheme: "dark", DefaultLanding: "/", EnableHistory: true},
		Store: config.StoreConfig{
			Path:          filepath.Join(t.TempDir(), "cwd.db"),
			RetentionDays: 30,
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	addrCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, cfg, logger, addrCh)
	}()

	addr := <-addrCh
	resp, err := http.Get("http://" + addr + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("/healthz status = %d, want 200", resp.StatusCode)
	}

	cancel()
	if err := <-errCh; err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, http.ErrServerClosed) {
		t.Errorf("Run error = %v", err)
	}
}

func TestRun_AllFiveSourcesGoLiveAndHotStart(t *testing.T) {
	nwsBody := []byte(`{"type":"FeatureCollection","features":[]}`)
	swpcScalesBody := []byte(`{"1":{"DateStamp":"2026-05-03","R":{"MinorProb":"5","MajorProb":"0"},"S":{"Prob":"0"},"G":{"Scale":"0","Text":"none"}},"2":{"DateStamp":"2026-05-04","R":{"MinorProb":"5","MajorProb":"0"},"S":{"Prob":"0"},"G":{"Scale":"0","Text":"none"}},"3":{"DateStamp":"2026-05-05","R":{"MinorProb":"5","MajorProb":"0"},"S":{"Prob":"0"},"G":{"Scale":"0","Text":"none"}}}`)
	swpcAlertsBody := []byte(`[]`)
	quakesBody := []byte(`{"type":"FeatureCollection","features":[]}`)
	volcsBody := []byte(`[]`)

	makeServer := func(body []byte, contentType string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", contentType)
			w.Header().Set("ETag", `"e1"`)
			_, _ = w.Write(body)
		}))
	}
	nwsSrv := makeServer(nwsBody, "application/geo+json")
	defer nwsSrv.Close()
	scalesSrv := makeServer(swpcScalesBody, "application/json")
	defer scalesSrv.Close()
	alertsSrv := makeServer(swpcAlertsBody, "application/json")
	defer alertsSrv.Close()
	quakesSrv := makeServer(quakesBody, "application/geo+json")
	defer quakesSrv.Close()
	volcsSrv := makeServer(volcsBody, "application/json")
	defer volcsSrv.Close()

	t.Setenv("CWD_NWS_ALERTS_URL", nwsSrv.URL)
	t.Setenv("CWD_SWPC_SCALES_URL", scalesSrv.URL)
	t.Setenv("CWD_SWPC_ALERTS_URL", alertsSrv.URL)
	t.Setenv("CWD_USGS_QUAKES_URL", quakesSrv.URL)
	t.Setenv("CWD_USGS_VOLCANOES_URL", volcsSrv.URL)

	enabled := true
	cfg := &config.Config{
		Server: config.ServerConfig{Bind: "127.0.0.1:0", LogLevel: "info", LogFormat: "text", Contact: "test@example.com"},
		UI:     config.UIConfig{DefaultTheme: "dark", DefaultLanding: "/", EnableHistory: true},
		Store:  config.StoreConfig{Path: filepath.Join(t.TempDir(), "cwd.db"), RetentionDays: 30},
		Sources: map[string]config.SourceConfig{
			"nws_alerts":     {Interval: 80 * time.Millisecond, Enabled: &enabled},
			"swpc_scales":    {Interval: 80 * time.Millisecond, Enabled: &enabled},
			"swpc_alerts":    {Interval: 80 * time.Millisecond, Enabled: &enabled},
			"usgs_quakes":    {Interval: 80 * time.Millisecond, Enabled: &enabled},
			"usgs_volcanoes": {Interval: 80 * time.Millisecond, Enabled: &enabled},
		},
		Derived: config.DerivedConfig{Thresholds: config.ThresholdsConfig{
			SWPCAlertWindowHours: 24,
			SWPCAlertProducts:    []string{"K08A", "K09A", "P12A", "P13A"},
		}},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	addrCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() { errCh <- Run(ctx, cfg, logger, addrCh) }()
	addr := <-addrCh

	deadline := time.After(3 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("/readyz never went 200")
		default:
		}
		resp, err := http.Get("http://" + addr + "/readyz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				goto done
			}
		}
		time.Sleep(40 * time.Millisecond)
	}
done:
	resp, err := http.Get("http://" + addr + "/api/sources")
	if err != nil {
		t.Fatalf("GET /api/sources: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	for _, name := range []string{"nws_alerts", "swpc_scales", "swpc_alerts", "usgs_quakes", "usgs_volcanoes"} {
		if !strings.Contains(string(body), name) {
			t.Errorf("/api/sources missing %s: %s", name, body)
		}
	}
	cancel()
	if err := <-errCh; err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, http.ErrServerClosed) {
		t.Errorf("Run error = %v", err)
	}
}

func TestRun_ImageProxy_LiveOnSourcesAndImgRoute(t *testing.T) {
	imgBody := []byte("PNGBYTES")
	imgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("ETag", `"img1"`)
		_, _ = w.Write(imgBody)
	}))
	defer imgSrv.Close()

	// Phase 2 sources: stub each so /readyz can go green within the test budget.
	stub := func(body []byte, ct string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", ct)
			w.Header().Set("ETag", `"e"`)
			_, _ = w.Write(body)
		}))
	}
	nws := stub([]byte(`{"type":"FeatureCollection","features":[]}`), "application/geo+json")
	defer nws.Close()
	scales := stub([]byte(`{"1":{"DateStamp":"2026-05-03","R":{"MinorProb":"5","MajorProb":"0"},"S":{"Prob":"0"},"G":{"Scale":"0","Text":"none"}},"2":{"DateStamp":"2026-05-04","R":{"MinorProb":"5","MajorProb":"0"},"S":{"Prob":"0"},"G":{"Scale":"0","Text":"none"}},"3":{"DateStamp":"2026-05-05","R":{"MinorProb":"5","MajorProb":"0"},"S":{"Prob":"0"},"G":{"Scale":"0","Text":"none"}}}`), "application/json")
	defer scales.Close()
	swpcAlerts := stub([]byte(`[]`), "application/json")
	defer swpcAlerts.Close()
	quakes := stub([]byte(`{"type":"FeatureCollection","features":[]}`), "application/geo+json")
	defer quakes.Close()
	volcs := stub([]byte(`[]`), "application/json")
	defer volcs.Close()

	t.Setenv("CWD_NWS_ALERTS_URL", nws.URL)
	t.Setenv("CWD_SWPC_SCALES_URL", scales.URL)
	t.Setenv("CWD_SWPC_ALERTS_URL", swpcAlerts.URL)
	t.Setenv("CWD_USGS_QUAKES_URL", quakes.URL)
	t.Setenv("CWD_USGS_VOLCANOES_URL", volcs.URL)
	// Override the SPC Day 1 registry URL to the test image server.
	t.Setenv("CWD_IMAGE_URL_SPC_DAY1OTLK", imgSrv.URL)

	enabled := true
	cacheDir := filepath.Join(t.TempDir(), "imgs")
	cfg := &config.Config{
		Server: config.ServerConfig{Bind: "127.0.0.1:0", LogLevel: "info", LogFormat: "text", Contact: "test@example.com"},
		UI:     config.UIConfig{DefaultTheme: "dark", DefaultLanding: "/", EnableHistory: true},
		Store:  config.StoreConfig{Path: filepath.Join(t.TempDir(), "cwd.db"), RetentionDays: 30},
		Sources: map[string]config.SourceConfig{
			"nws_alerts":     {Interval: 80 * time.Millisecond, Enabled: &enabled},
			"swpc_scales":    {Interval: 80 * time.Millisecond, Enabled: &enabled},
			"swpc_alerts":    {Interval: 80 * time.Millisecond, Enabled: &enabled},
			"usgs_quakes":    {Interval: 80 * time.Millisecond, Enabled: &enabled},
			"usgs_volcanoes": {Interval: 80 * time.Millisecond, Enabled: &enabled},
		},
		Images: config.ImagesConfig{
			CacheDir:       cacheDir,
			DiskMaxBytes:   1 << 20,
			HotMaxBytes:    1 << 20,
			HotMaxEntries:  16,
			ImageIntervals: map[string]time.Duration{"spc.day1otlk": 60 * time.Second},
			Prewarm:        []string{"spc.day1otlk"},
		},
		Derived: config.DerivedConfig{Thresholds: config.ThresholdsConfig{
			SWPCAlertWindowHours: 24,
			SWPCAlertProducts:    []string{"K08A", "K09A", "P12A", "P13A"},
		}},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	addrCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() { errCh <- Run(ctx, cfg, logger, addrCh) }()
	addr := <-addrCh

	// Wait for /readyz.
	deadline := time.After(4 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("/readyz never went 200")
		default:
		}
		resp, err := http.Get("http://" + addr + "/readyz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				goto ready
			}
		}
		time.Sleep(40 * time.Millisecond)
	}
ready:

	// /api/sources includes image:spc.day1otlk after prewarm tick.
	{
		var body []byte
		for i := 0; i < 30; i++ {
			resp, err := http.Get("http://" + addr + "/api/sources")
			if err == nil {
				body, _ = io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				if strings.Contains(string(body), `"image:spc.day1otlk"`) {
					break
				}
			}
			time.Sleep(50 * time.Millisecond)
		}
		if !strings.Contains(string(body), `"image:spc.day1otlk"`) {
			t.Errorf("/api/sources missing image:spc.day1otlk\nbody=%s", body)
		}
	}

	// /img/spc/day1otlk serves the test bytes.
	{
		resp, err := http.Get("http://" + addr + "/img/spc/day1otlk")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("/img status = %d", resp.StatusCode)
		}
		got, _ := io.ReadAll(resp.Body)
		if !bytes.Equal(got, imgBody) {
			t.Errorf("/img body mismatch")
		}
	}

	cancel()
	if err := <-errCh; err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, http.ErrServerClosed) {
		t.Errorf("Run = %v", err)
	}
}

// TestImagesRegistryConfigSync guards against silent drift between
// imageproxy.Registry and config.knownImageKeys (the unexported set used by
// validateImagesPrewarm). If a new image is added to the registry but the
// config-side mirror isn't updated, validateImagesPrewarm would WARN-and-drop
// the operator's prewarm entry at boot. This test verifies every registry key
// survives Validate via the CWD_IMAGES_PREWARM env override.
func TestImagesRegistryConfigSync(t *testing.T) {
	keys := make([]string, 0, len(imageproxy.Registry))
	for _, img := range imageproxy.Registry {
		keys = append(keys, img.Key())
	}
	t.Setenv("CWD_IMAGES_PREWARM", strings.Join(keys, ","))

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg, err := config.LoadWithLogger("", logger)
	if err != nil {
		t.Fatalf("config.LoadWithLogger: %v", err)
	}
	if got, want := len(cfg.Images.Prewarm), len(keys); got != want {
		t.Fatalf("prewarm len after Validate = %d, want %d (registry has %d keys; some were dropped — config.knownImageKeys is out of sync with imageproxy.Registry)\ngot:  %v\nwant: %v", got, want, len(imageproxy.Registry), cfg.Images.Prewarm, keys)
	}
	survived := make(map[string]struct{}, len(cfg.Images.Prewarm))
	for _, k := range cfg.Images.Prewarm {
		survived[k] = struct{}{}
	}
	for _, k := range keys {
		if _, ok := survived[k]; !ok {
			t.Errorf("registry key %q was dropped by validateImagesPrewarm — config.knownImageKeys is missing it", k)
		}
	}
}

func TestRunReadyzGoesGreenAfterFirstFetch(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"type":"FeatureCollection","features":[]}`))
	}))
	defer upstream.Close()

	enabled := true
	cfg := &config.Config{
		Server: config.ServerConfig{Bind: "127.0.0.1:0", LogLevel: "info", LogFormat: "text", Contact: "test@example.com"},
		UI:     config.UIConfig{DefaultTheme: "dark", DefaultLanding: "/", EnableHistory: true},
		Store:  config.StoreConfig{Path: filepath.Join(t.TempDir(), "cwd.db"), RetentionDays: 30},
		Sources: map[string]config.SourceConfig{
			"nws_alerts": {Interval: 80 * time.Millisecond, Enabled: &enabled},
		},
	}

	// Override the upstream URL via env var; see nwsAlertsURL in server.go.
	t.Setenv("CWD_NWS_ALERTS_URL", upstream.URL)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	addrCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() { errCh <- Run(ctx, cfg, logger, addrCh) }()

	addr := <-addrCh

	// Poll /readyz until 200 or deadline.
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("/readyz never went 200 within deadline")
		default:
		}
		resp, err := http.Get("http://" + addr + "/readyz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				goto done
			}
		}
		time.Sleep(40 * time.Millisecond)
	}
done:
	cancel()
	if err := <-errCh; err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, http.ErrServerClosed) {
		t.Errorf("Run error = %v", err)
	}
}
