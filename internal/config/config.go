// Package config loads, validates, and defaults the cwd YAML+env configuration.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	ugcRE = regexp.MustCompile(`^[A-Z]{2}[CZ]\d{3}$`)
	wfoRE = regexp.MustCompile(`^[A-Z]{3}$`)
	// SWPC product code regex. Accepts the documented allowlist namespace from
	// internal/sources/swpc_alerts.go: K-series (K04A...K09A...K0n[AW]),
	// P-series (P10A...P15A), WARK*/WATA*/RWAR/X-series, SUM*. The plan's
	// first-draft regex ^[A-Z]{3,8}[0-9A-Z]?$ rejected all defaults() codes
	// (K08A starts with one letter, not three) and was corrected before
	// implementation. Total length 3-8: leading uppercase letter + 2-7
	// uppercase-or-digit chars.
	swpcProductRE = regexp.MustCompile(`^[A-Z][A-Z0-9]{2,7}$`)
)

// Config is the top-level configuration struct.
type Config struct {
	Server  ServerConfig            `yaml:"server"`
	Store   StoreConfig             `yaml:"store"`
	Sources map[string]SourceConfig `yaml:"sources"`
	Images  ImagesConfig            `yaml:"images"`
	Derived DerivedConfig           `yaml:"derived"`
	UI      UIConfig                `yaml:"ui"`
}

// ServerConfig holds HTTP server and logging settings.
type ServerConfig struct {
	Bind      string `yaml:"bind"`
	Contact   string `yaml:"contact"`
	LogLevel  string `yaml:"log_level"`
	LogFormat string `yaml:"log_format"`
}

// StoreConfig holds SQLite database settings.
type StoreConfig struct {
	Path          string `yaml:"path"`
	RetentionDays int    `yaml:"retention_days"`
}

// SourceConfig holds per-source polling settings.
type SourceConfig struct {
	Interval time.Duration `yaml:"interval"`
	Enabled  *bool         `yaml:"enabled"`
}

// IsEnabled reports whether this source is enabled.
// A nil Enabled pointer means "not explicitly set" and defaults to true.
func (s SourceConfig) IsEnabled() bool { return s.Enabled == nil || *s.Enabled }

// boolPtr returns a pointer to b, useful for setting *bool fields in struct literals.
func boolPtr(b bool) *bool { return &b }

// ImagesConfig holds image proxy settings. See design §8.
type ImagesConfig struct {
	// CacheDir is the disk-store root. Created with 0700 if missing. Boot
	// fails if the directory is not writable.
	CacheDir string `yaml:"cache_dir"`
	// DiskMaxBytes caps the on-disk store. Must be > 0.
	DiskMaxBytes int64 `yaml:"disk_max_bytes"`
	// HotMaxBytes caps the in-memory hot tier. 0 disables the tier (disk-only).
	HotMaxBytes int64 `yaml:"hot_max_bytes"`
	// HotMaxEntries caps the hot tier by entry count.
	HotMaxEntries int `yaml:"hot_max_entries"`
	// ImageIntervals overrides the per-image cadence by dot key (e.g.
	// "spc.day1otlk": 90s). Values below 60s are clamped + logged.
	ImageIntervals map[string]time.Duration `yaml:"image_intervals"`
	// Prewarm lists registry keys that should be polled in the background.
	// Unknown keys are dropped at boot with a WARN.
	Prewarm []string `yaml:"prewarm"`
}

// knownImageKeys returns the set of registry keys recognized by validateImagesPrewarm.
//
// IMPORTANT: This list MUST stay in sync with imageproxy.Registry's keys (see
// internal/imageproxy/registry.go). It is intentionally NOT imported from the
// imageproxy package because Phase 3 Task 4 will introduce an
// imageproxy → config dependency, which would create an import cycle if this
// package referenced imageproxy. When adding/removing a registry image, update
// both this list AND the imageproxy.Registry.
func knownImageKeys() map[string]struct{} {
	return map[string]struct{}{
		"spc.day1otlk":       {},
		"spc.day2otlk":       {},
		"spc.day3otlk":       {},
		"spc.day1otlk_fire":  {},
		"spc.day2otlk_fire":  {},
		"spc.day38otlk_fire": {},
		"wpc.ero_day1":       {},
		"wpc.ero_day2":       {},
		"wpc.ero_day3":       {},
		"wpc.wssi_day1":      {},
		"wpc.wssi_day2":      {},
		"wpc.wssi_day3":      {},
		"wpc.heatrisk_day1":  {},
		"wpc.heatrisk_day2":  {},
		"wpc.heatrisk_day3":  {},
		"nhc.atl_7d":         {},
		"nhc.epac_7d":        {},
		"nhc.cpac_7d":        {},
		"navy.jtwc_abpw":     {},
		"nwc.fho_national":   {},
	}
}

// DerivedConfig holds computed/threshold settings.
type DerivedConfig struct {
	Thresholds ThresholdsConfig `yaml:"thresholds"`
}

// ThresholdsConfig holds alert threshold settings.
type ThresholdsConfig struct {
	SWPCAlertWindowHours int          `yaml:"swpc_alert_window_hours"`
	SWPCAlertProducts    []string     `yaml:"swpc_alert_products"`
	RegionFilter         RegionFilter `yaml:"region_filter"`
}

// RegionFilter holds geographic filter settings.
type RegionFilter struct {
	UGCs []string `yaml:"ugcs"`
	WFOs []string `yaml:"wfos"`
}

// UIConfig holds UI display settings.
type UIConfig struct {
	DefaultTheme   string `yaml:"default_theme"`
	DefaultLanding string `yaml:"default_landing"`
	EnableHistory  bool   `yaml:"enable_history"`
}

// LoadWithLogger reads the YAML at path, applies defaults, applies CWD_* env overrides, and validates.
// Parse failures on env overrides are logged as WARN via logger; bad values fall back to the YAML default.
// If logger is nil, slog.Default() is used.
func LoadWithLogger(path string, logger *slog.Logger) (*Config, error) {
	if logger == nil {
		logger = slog.Default()
	}

	cfg := defaults()

	if path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read config %q: %w", path, err)
		}
		if err := yaml.Unmarshal(raw, cfg); err != nil {
			return nil, fmt.Errorf("parse config %q: %w", path, err)
		}
	}

	applyEnvOverrides(cfg, logger)

	// Re-apply defaults on a per-source basis so partial source maps still get sane defaults.
	mergeSourceDefaults(cfg)

	validateRegionFilter(cfg, logger)
	validateSWPCProducts(cfg, logger)
	validateSWPCWindow(cfg, logger)
	validateSourceIntervalFloors(cfg, logger)
	validateImageIntervals(cfg, logger)
	validateImagesPrewarm(cfg, logger)

	if err := validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// validateRegionFilter drops entries that don't match the format and warns.
// Must run BEFORE the filter is constructed in server.Run so bad entries
// never reach sources.NewFilter.
func validateRegionFilter(cfg *Config, logger *slog.Logger) {
	rf := &cfg.Derived.Thresholds.RegionFilter
	rf.UGCs = filterByRegex(rf.UGCs, ugcRE, "ugc", logger)
	rf.WFOs = filterByRegex(rf.WFOs, wfoRE, "wfo", logger)
}

// validateSWPCProducts drops entries that don't match the SWPC product code
// shape and WARNs. Mirrors validateRegionFilter's pattern.
func validateSWPCProducts(cfg *Config, logger *slog.Logger) {
	in := cfg.Derived.Thresholds.SWPCAlertProducts
	out := make([]string, 0, len(in))
	for _, p := range in {
		if swpcProductRE.MatchString(p) {
			out = append(out, p)
		} else {
			logger.Warn("config.swpc_alert_product_invalid", "value", p, "pattern", swpcProductRE.String())
		}
	}
	cfg.Derived.Thresholds.SWPCAlertProducts = out
}

// validateSWPCWindow clamps non-positive window-hours to 24 and WARNs.
func validateSWPCWindow(cfg *Config, logger *slog.Logger) {
	if cfg.Derived.Thresholds.SWPCAlertWindowHours <= 0 {
		logger.Warn("config.swpc_alert_window_clamped",
			"value", cfg.Derived.Thresholds.SWPCAlertWindowHours,
			"clamped_to", 24)
		cfg.Derived.Thresholds.SWPCAlertWindowHours = 24
	}
}

// validateSourceIntervalFloors WARNs (does not modify) for any enabled source
// whose interval is under 10s — protects upstreams from over-polling without
// overriding an operator's explicit choice.
func validateSourceIntervalFloors(cfg *Config, logger *slog.Logger) {
	const floor = 10 * time.Second
	for name, src := range cfg.Sources {
		if !src.IsEnabled() {
			continue
		}
		if src.Interval > 0 && src.Interval < floor {
			logger.Warn("config.source_interval_too_low",
				"source", name,
				"interval", src.Interval.String(),
				"floor", floor.String())
		}
	}
}

// validateImageIntervals clamps per-image intervals below 60s + WARNs.
func validateImageIntervals(cfg *Config, logger *slog.Logger) {
	const floor = 60 * time.Second
	for k, d := range cfg.Images.ImageIntervals {
		if d > 0 && d < floor {
			logger.Warn("config.image_interval_clamped",
				"key", k, "value", d.String(), "floor", floor.String())
			cfg.Images.ImageIntervals[k] = floor
		}
	}
}

// validateImagesPrewarm drops prewarm entries that don't appear in the
// known image-key set and WARNs. Mirrors validateRegionFilter /
// validateSWPCProducts. See knownImageKeys for the import-cycle rationale.
func validateImagesPrewarm(cfg *Config, logger *slog.Logger) {
	known := knownImageKeys()
	in := cfg.Images.Prewarm
	out := make([]string, 0, len(in))
	for _, k := range in {
		if _, ok := known[k]; ok {
			out = append(out, k)
		} else {
			logger.Warn("config.images_prewarm_unknown_key", "key", k)
		}
	}
	cfg.Images.Prewarm = out
}

func filterByRegex(in []string, re *regexp.Regexp, kind string, logger *slog.Logger) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if re.MatchString(s) {
			out = append(out, s)
		} else {
			logger.Warn("config.region_filter_invalid", "kind", kind, "value", s, "pattern", re.String())
		}
	}
	return out
}

// Load reads the YAML at path, applies defaults, applies CWD_* env overrides, and validates.
// It uses slog.Default() for logging env-parse warnings.
func Load(path string) (*Config, error) {
	return LoadWithLogger(path, slog.Default())
}

// MissingContact reports whether the operator left server.contact empty.
// Callers should log a loud warning so NWS API politeness rules aren't accidentally violated.
func (c *Config) MissingContact() bool { return strings.TrimSpace(c.Server.Contact) == "" }

func defaults() *Config {
	return &Config{
		Server: ServerConfig{
			Bind:      "127.0.0.1:8765",
			LogLevel:  "info",
			LogFormat: "json",
		},
		Store: StoreConfig{
			Path:          xdgState("cwd/cwd.db"),
			RetentionDays: 30,
		},
		Sources: map[string]SourceConfig{
			"nws_alerts":     {Interval: 30 * time.Second, Enabled: boolPtr(true)},
			"swpc_scales":    {Interval: 60 * time.Second, Enabled: boolPtr(true)},
			"swpc_alerts":    {Interval: 60 * time.Second, Enabled: boolPtr(true)},
			"usgs_quakes":    {Interval: 60 * time.Second, Enabled: boolPtr(true)},
			"usgs_volcanoes": {Interval: 5 * time.Minute, Enabled: boolPtr(true)},
		},
		Images: ImagesConfig{
			CacheDir:       xdgState("cwd/images"),
			DiskMaxBytes:   524_288_000, // 500 MiB
			HotMaxBytes:    67_108_864,  // 64 MiB
			HotMaxEntries:  256,
			ImageIntervals: map[string]time.Duration{},
			// Default prewarm list — must stay in sync with
			// imageproxy.DefaultPrewarmKeys(). Hardcoded as a literal here to
			// avoid an import cycle: Phase 3 Task 4 will introduce
			// imageproxy → config, so config cannot import imageproxy.
			Prewarm: []string{"spc.day1otlk", "spc.day2otlk", "spc.day3otlk", "nhc.atl_7d"},
		},
		Derived: DerivedConfig{
			Thresholds: ThresholdsConfig{
				SWPCAlertWindowHours: 24,
				SWPCAlertProducts:    []string{"K08A", "K09A", "P12A", "P13A"},
			},
		},
		UI: UIConfig{
			DefaultTheme:   "dark",
			DefaultLanding: "/",
			EnableHistory:  true,
		},
	}
}

func mergeSourceDefaults(cfg *Config) {
	defs := defaults().Sources
	if cfg.Sources == nil {
		cfg.Sources = defs
		return
	}
	for name, d := range defs {
		s, ok := cfg.Sources[name]
		if !ok {
			cfg.Sources[name] = d
			continue
		}
		if s.Interval == 0 {
			s.Interval = d.Interval
		}
		if s.Enabled == nil {
			s.Enabled = d.Enabled
		}
		cfg.Sources[name] = s
	}
}

// applyEnvOverrides walks CWD_* env variables and writes them into matching string/int/bool fields.
// Naming convention: CWD_<SECTION>_<FIELD>, e.g. CWD_SERVER_BIND, CWD_UI_DEFAULT_THEME.
// Only top-level scalar fields under Server, Store, Images, UI are supported via reflection.
// Per-source overrides use CWD_SOURCES_<UPPER_SOURCE_NAME>_INTERVAL and CWD_SOURCES_<UPPER_SOURCE_NAME>_ENABLED.
// Slice + nested-map fields under Images are handled explicitly:
//   - CWD_IMAGES_PREWARM (comma-separated registry keys; replaces the entire list).
//   - CWD_IMAGES_IMAGE_INTERVALS_<KEY> (duration; "__" in <KEY> is the dot
//     separator, e.g. CWD_IMAGES_IMAGE_INTERVALS_SPC__DAY1OTLK_FIRE → spc.day1otlk_fire).
//
// Parse failures are logged as WARN via logger; bad values fall back to the current (YAML-supplied) value.
func applyEnvOverrides(cfg *Config, logger *slog.Logger) {
	apply := func(prefix string, v reflect.Value) {
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			tag := t.Field(i).Tag.Get("yaml")
			if tag == "" {
				continue
			}
			envName := "CWD_" + strings.ToUpper(prefix+"_"+tag)
			val, ok := os.LookupEnv(envName)
			if !ok {
				continue
			}
			f := v.Field(i)
			switch f.Kind() {
			case reflect.String:
				f.SetString(val)
			case reflect.Int, reflect.Int64:
				n, err := strconv.ParseInt(val, 10, 64)
				if err != nil {
					logger.Warn("config.env_parse_failed", "key", envName, "value", val, "err", err.Error())
				} else {
					f.SetInt(n)
				}
			case reflect.Bool:
				b, err := strconv.ParseBool(val)
				if err != nil {
					logger.Warn("config.env_parse_failed", "key", envName, "value", val, "err", err.Error())
				} else {
					f.SetBool(b)
				}
			}
		}
	}
	apply("server", reflect.ValueOf(&cfg.Server).Elem())
	apply("store", reflect.ValueOf(&cfg.Store).Elem())
	apply("images", reflect.ValueOf(&cfg.Images).Elem())
	apply("ui", reflect.ValueOf(&cfg.UI).Elem())

	// Images.Prewarm — comma-separated registry keys; replaces the entire list.
	if v, ok := os.LookupEnv("CWD_IMAGES_PREWARM"); ok {
		parts := strings.Split(v, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if t := strings.TrimSpace(p); t != "" {
				out = append(out, t)
			}
		}
		cfg.Images.Prewarm = out
	}

	// Images.ImageIntervals — CWD_IMAGES_IMAGE_INTERVALS_<KEY> with "__" as
	// the dot separator. Example: CWD_IMAGES_IMAGE_INTERVALS_SPC__DAY1OTLK_FIRE=90s
	// → cfg.Images.ImageIntervals["spc.day1otlk_fire"] = 90s.
	const intervalsPrefix = "CWD_IMAGES_IMAGE_INTERVALS_"
	if cfg.Images.ImageIntervals == nil {
		cfg.Images.ImageIntervals = map[string]time.Duration{}
	}
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, intervalsPrefix) {
			continue
		}
		eq := strings.IndexByte(kv, '=')
		if eq < 0 {
			continue
		}
		name := kv[:eq]
		val := kv[eq+1:]
		rest := name[len(intervalsPrefix):]
		// Convert UPPER+__ to lower+. (single-underscore stays in segment).
		key := strings.ToLower(strings.ReplaceAll(rest, "__", "."))
		d, perr := time.ParseDuration(val)
		if perr != nil {
			logger.Warn("config.env_parse_failed", "key", name, "value", val, "err", perr.Error())
			continue
		}
		cfg.Images.ImageIntervals[key] = d
	}

	// Per-source overrides: CWD_SOURCES_<UPPER_SOURCE_NAME>_INTERVAL and _ENABLED.
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
				src.Enabled = boolPtr(b)
			}
		}

		cfg.Sources[name] = src
	}
}

func validate(cfg *Config) error {
	switch cfg.Server.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("server.log_level must be one of debug|info|warn|error, got %q", cfg.Server.LogLevel)
	}
	switch cfg.Server.LogFormat {
	case "json", "text":
	default:
		return fmt.Errorf("server.log_format must be json|text, got %q", cfg.Server.LogFormat)
	}
	switch cfg.UI.DefaultTheme {
	case "dark", "light", "auto":
	default:
		return fmt.Errorf("ui.default_theme must be dark|light|auto, got %q", cfg.UI.DefaultTheme)
	}
	if cfg.Store.RetentionDays <= 0 {
		return errors.New("store.retention_days must be > 0")
	}
	if cfg.Images.DiskMaxBytes <= 0 {
		return errors.New("images.disk_max_bytes must be > 0")
	}
	if cfg.Images.HotMaxBytes < 0 {
		return errors.New("images.hot_max_bytes must be >= 0 (0 disables hot tier)")
	}
	if cfg.Images.HotMaxEntries <= 0 {
		return errors.New("images.hot_max_entries must be > 0")
	}
	if cfg.Images.CacheDir == "" {
		return errors.New("images.cache_dir must be set")
	}
	return nil
}

func xdgState(rel string) string {
	if v := os.Getenv("XDG_STATE_HOME"); v != "" {
		return filepath.Join(v, rel)
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local/state", rel)
	}
	return "./" + rel
}
