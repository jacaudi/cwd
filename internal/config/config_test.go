package config

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestLoadMinimalAppliesDefaults(t *testing.T) {
	cfg, err := Load("testdata/minimal.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.Bind != "127.0.0.1:8765" {
		t.Errorf("Server.Bind default = %q, want 127.0.0.1:8765", cfg.Server.Bind)
	}
	if cfg.Server.LogLevel != "info" {
		t.Errorf("Server.LogLevel default = %q, want info", cfg.Server.LogLevel)
	}
	if cfg.Server.LogFormat != "json" {
		t.Errorf("Server.LogFormat default = %q, want json", cfg.Server.LogFormat)
	}
	if cfg.Store.RetentionDays != 30 {
		t.Errorf("Store.RetentionDays default = %d, want 30", cfg.Store.RetentionDays)
	}
	if cfg.Images.DiskMaxBytes != 524288000 {
		t.Errorf("Images.DiskMaxBytes default = %d, want 524288000", cfg.Images.DiskMaxBytes)
	}
	if cfg.UI.DefaultTheme != "dark" {
		t.Errorf("UI.DefaultTheme default = %q, want dark", cfg.UI.DefaultTheme)
	}
	if cfg.UI.DefaultLanding != "/" {
		t.Errorf("UI.DefaultLanding default = %q, want /", cfg.UI.DefaultLanding)
	}
	if got := cfg.Sources["nws_alerts"].Interval; got != 30*time.Second {
		t.Errorf("Sources.nws_alerts.Interval default = %s, want 30s", got)
	}
}

func TestLoadFullParsesAllFields(t *testing.T) {
	cfg, err := Load("testdata/full.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.Bind != "0.0.0.0:9000" {
		t.Errorf("Server.Bind = %q", cfg.Server.Bind)
	}
	if cfg.Server.LogLevel != "debug" {
		t.Errorf("Server.LogLevel = %q", cfg.Server.LogLevel)
	}
	if cfg.Store.RetentionDays != 7 {
		t.Errorf("Store.RetentionDays = %d", cfg.Store.RetentionDays)
	}
	if got := cfg.Sources["usgs_volcanoes"].Interval; got != 5*time.Minute {
		t.Errorf("Sources.usgs_volcanoes.Interval = %s", got)
	}
}

func TestEnvOverridesYAML(t *testing.T) {
	t.Setenv("CWD_SERVER_BIND", "127.0.0.1:9999")
	t.Setenv("CWD_UI_DEFAULT_THEME", "light")
	cfg, err := Load("testdata/minimal.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.Bind != "127.0.0.1:9999" {
		t.Errorf("env override Server.Bind = %q", cfg.Server.Bind)
	}
	if cfg.UI.DefaultTheme != "light" {
		t.Errorf("env override UI.DefaultTheme = %q", cfg.UI.DefaultTheme)
	}
}

func TestValidateRejectsBadLogLevel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte("server:\n  contact: x\n  log_level: nonsense\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected error on bad log_level")
	}
}

func TestValidateRejectsBadTheme(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte("server:\n  contact: x\nui:\n  default_theme: rainbow\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected error on bad ui.default_theme")
	}
}

func TestMissingContactWarnsButLoads(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nocontact.yaml")
	if err := os.WriteFile(path, []byte("server: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.Contact != "" {
		t.Errorf("Server.Contact = %q, want empty", cfg.Server.Contact)
	}
	if !cfg.MissingContact() {
		t.Error("MissingContact() = false, want true")
	}
}

func TestPartialSourceOverridePreservesEnabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "partial.yaml")
	if err := os.WriteFile(path, []byte("server:\n  contact: x\nsources:\n  nws_alerts:\n    interval: 15s\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.Sources["nws_alerts"].Interval; got != 15*time.Second {
		t.Errorf("nws_alerts.Interval = %s, want 15s", got)
	}
	if !cfg.Sources["nws_alerts"].IsEnabled() {
		t.Error("nws_alerts: partial override silently disabled the source")
	}
}

func TestExplicitlyDisabledSourceStaysDisabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "off.yaml")
	if err := os.WriteFile(path, []byte("server:\n  contact: x\nsources:\n  nws_alerts:\n    enabled: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Sources["nws_alerts"].IsEnabled() {
		t.Error("nws_alerts: explicit disable was not honored")
	}
}

func TestXDGPathsHandleTrailingSlash(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/tmp/with-slash/")
	t.Setenv("XDG_CACHE_HOME", "/tmp/cache-with-slash/")
	cfg, err := Load("") // defaults-only path
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	wantStore := filepath.Join("/tmp/with-slash", "cwd/cwd.db")
	if cfg.Store.Path != wantStore {
		t.Errorf("Store.Path = %q, want %q", cfg.Store.Path, wantStore)
	}
	// Images.CacheDir uses XDG_STATE_HOME (the proxy is durable cache, not throwaway).
	wantImages := filepath.Join("/tmp/with-slash", "cwd/images")
	if cfg.Images.CacheDir != wantImages {
		t.Errorf("Images.CacheDir = %q, want %q", cfg.Images.CacheDir, wantImages)
	}
}

func TestEnvOverride_PerSource(t *testing.T) {
	t.Setenv("CWD_SOURCES_NWS_ALERTS_INTERVAL", "45s")
	t.Setenv("CWD_SOURCES_NWS_ALERTS_ENABLED", "false")
	cfg, err := Load("testdata/full.yaml")
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
	if src.IsEnabled() {
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
	if cfg.Sources["nws_alerts"].Interval == 0 {
		t.Errorf("expected fallback to YAML interval, got zero")
	}
}

func TestLoadWithLogger_NilLoggerFallsBack(t *testing.T) {
	cfg, err := LoadWithLogger("testdata/full.yaml", nil)
	if err != nil {
		t.Fatalf("LoadWithLogger(nil logger): %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
}

func TestValidateSWPCProducts_DropsMalformedAndWARNs(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	cfg := &Config{
		Server: ServerConfig{Bind: "127.0.0.1:0", LogLevel: "info", LogFormat: "json"},
		UI:     UIConfig{DefaultTheme: "dark", DefaultLanding: "/", EnableHistory: true},
		Store:  StoreConfig{Path: "/tmp/x.db", RetentionDays: 30},
		Derived: DerivedConfig{Thresholds: ThresholdsConfig{
			SWPCAlertWindowHours: 24,
			SWPCAlertProducts:    []string{"K08A", "lowercase", "K05A;DROP", "P12A", "WARK04W"},
		}},
	}
	validateSWPCProducts(cfg, logger)
	got := cfg.Derived.Thresholds.SWPCAlertProducts
	want := []string{"K08A", "P12A", "WARK04W"}
	if !sameStrings(got, want) {
		t.Errorf("kept = %v, want %v", got, want)
	}
	if !bytes.Contains(buf.Bytes(), []byte("config.swpc_alert_product_invalid")) {
		t.Errorf("expected WARN, got: %s", buf.String())
	}
}

func TestValidateSWPCWindow_ClampsAndWARNs(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	cfg := &Config{Derived: DerivedConfig{Thresholds: ThresholdsConfig{SWPCAlertWindowHours: -3}}}
	validateSWPCWindow(cfg, logger)
	if cfg.Derived.Thresholds.SWPCAlertWindowHours != 24 {
		t.Errorf("clamp failed: got %d", cfg.Derived.Thresholds.SWPCAlertWindowHours)
	}
	if !bytes.Contains(buf.Bytes(), []byte("config.swpc_alert_window_clamped")) {
		t.Errorf("expected WARN, got: %s", buf.String())
	}
}

func TestValidateSourceIntervalFloors_WARNUnder10s(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	enabled := true
	cfg := &Config{Sources: map[string]SourceConfig{
		"nws_alerts":  {Interval: 5 * time.Second, Enabled: &enabled},
		"swpc_scales": {Interval: 60 * time.Second, Enabled: &enabled},
	}}
	validateSourceIntervalFloors(cfg, logger)
	if !bytes.Contains(buf.Bytes(), []byte("config.source_interval_too_low")) {
		t.Errorf("expected WARN for 5s interval, got: %s", buf.String())
	}
	if bytes.Count(buf.Bytes(), []byte("config.source_interval_too_low")) != 1 {
		t.Errorf("WARN should fire once (only the 5s source), got: %s", buf.String())
	}
	// Values must NOT be clamped — operator override stands.
	if cfg.Sources["nws_alerts"].Interval != 5*time.Second {
		t.Errorf("interval mutated: %v", cfg.Sources["nws_alerts"].Interval)
	}
}

func sameStrings(a, b []string) bool {
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

func TestDefaults_ImagesBlockShape(t *testing.T) {
	cfg := defaults()
	if cfg.Images.DiskMaxBytes <= 0 {
		t.Errorf("Images.DiskMaxBytes default must be > 0")
	}
	if cfg.Images.HotMaxBytes < 0 {
		t.Errorf("Images.HotMaxBytes default must be >= 0")
	}
	if cfg.Images.HotMaxEntries <= 0 {
		t.Errorf("Images.HotMaxEntries default must be > 0")
	}
	if len(cfg.Images.Prewarm) == 0 {
		t.Errorf("Images.Prewarm default must be non-empty")
	}
	wantPrewarm := []string{"spc.day1otlk", "spc.day2otlk", "spc.day3otlk", "nhc.atl_7d"}
	if got := cfg.Images.Prewarm; !slices.Equal(got, wantPrewarm) {
		t.Errorf("Images.Prewarm = %v, want %v", got, wantPrewarm)
	}
}

func TestValidate_RejectsNonPositiveDiskMaxBytes(t *testing.T) {
	cfg := defaults()
	cfg.Images.DiskMaxBytes = 0
	if err := validate(cfg); err == nil {
		t.Error("expected error for DiskMaxBytes <= 0")
	}
}

func TestValidate_AllowsZeroHotMaxBytes_DisablesHotTier(t *testing.T) {
	cfg := defaults()
	cfg.Images.HotMaxBytes = 0
	if err := validate(cfg); err != nil {
		t.Errorf("expected zero HotMaxBytes accepted (disables tier), got %v", err)
	}
}

func TestValidateImageIntervals_ClampsBelowFloor(t *testing.T) {
	cfg := defaults()
	cfg.Images.ImageIntervals = map[string]time.Duration{
		"spc.day1otlk": 5 * time.Second, // below 60s floor
		"nhc.atl_7d":   2 * time.Minute, // OK
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	validateImageIntervals(cfg, logger)
	if got := cfg.Images.ImageIntervals["spc.day1otlk"]; got != 60*time.Second {
		t.Errorf("clamped value = %s, want 60s", got)
	}
	if got := cfg.Images.ImageIntervals["nhc.atl_7d"]; got != 2*time.Minute {
		t.Errorf("untouched value changed to %s", got)
	}
}

func TestValidateImagesPrewarm_DropsUnknownKey(t *testing.T) {
	cfg := defaults()
	cfg.Images.Prewarm = []string{"spc.day1otlk", "bogus.never_existed", "nhc.atl_7d"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	validateImagesPrewarm(cfg, logger)
	want := []string{"spc.day1otlk", "nhc.atl_7d"}
	if !slices.Equal(cfg.Images.Prewarm, want) {
		t.Errorf("Prewarm = %v, want %v", cfg.Images.Prewarm, want)
	}
}

func TestEnvOverride_ImagesPrewarm_ReplacesList(t *testing.T) {
	t.Setenv("CWD_IMAGES_PREWARM", "spc.day1otlk,nhc.epac_7d,wpc.heatrisk_day1")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := defaults()
	applyEnvOverrides(cfg, logger)
	want := []string{"spc.day1otlk", "nhc.epac_7d", "wpc.heatrisk_day1"}
	if !slices.Equal(cfg.Images.Prewarm, want) {
		t.Errorf("after env override Prewarm = %v, want %v", cfg.Images.Prewarm, want)
	}
}

func TestEnvOverride_ImageIntervals_DoubleUnderscoreSeparator(t *testing.T) {
	// CWD_IMAGES_IMAGE_INTERVALS_SPC__DAY1OTLK_FIRE → spc.day1otlk_fire
	t.Setenv("CWD_IMAGES_IMAGE_INTERVALS_SPC__DAY1OTLK_FIRE", "120s")
	t.Setenv("CWD_IMAGES_IMAGE_INTERVALS_NHC__ATL_7D", "1h")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := defaults()
	applyEnvOverrides(cfg, logger)
	if got := cfg.Images.ImageIntervals["spc.day1otlk_fire"]; got != 120*time.Second {
		t.Errorf("spc.day1otlk_fire = %s, want 120s", got)
	}
	if got := cfg.Images.ImageIntervals["nhc.atl_7d"]; got != time.Hour {
		t.Errorf("nhc.atl_7d = %s, want 1h", got)
	}
}

func TestEnvOverride_ImageIntervals_BadDurationLoggedNotApplied(t *testing.T) {
	t.Setenv("CWD_IMAGES_IMAGE_INTERVALS_SPC__DAY1OTLK", "not-a-duration")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := defaults()
	applyEnvOverrides(cfg, logger)
	if _, ok := cfg.Images.ImageIntervals["spc.day1otlk"]; ok {
		t.Error("bad duration should not have been applied")
	}
}

func TestRegionFilter_DropsInvalid(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	cfg := &Config{
		Derived: DerivedConfig{
			Thresholds: ThresholdsConfig{
				RegionFilter: RegionFilter{
					UGCs: []string{"VAC059", "INVALID", "CAZ505", "ab1234"},
					WFOs: []string{"LWX", "FOO99", "OAX"},
				},
			},
		},
	}
	validateRegionFilter(cfg, logger)
	if !slices.Equal(cfg.Derived.Thresholds.RegionFilter.UGCs, []string{"VAC059", "CAZ505"}) {
		t.Errorf("UGCs after validate: %v", cfg.Derived.Thresholds.RegionFilter.UGCs)
	}
	if !slices.Equal(cfg.Derived.Thresholds.RegionFilter.WFOs, []string{"LWX", "OAX"}) {
		t.Errorf("WFOs after validate: %v", cfg.Derived.Thresholds.RegionFilter.WFOs)
	}
	s := buf.String()
	if !strings.Contains(s, "INVALID") || !strings.Contains(s, "FOO99") || !strings.Contains(s, "ab1234") {
		t.Errorf("warn log missing entries: %s", s)
	}
}
