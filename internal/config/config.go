// Package config loads, validates, and defaults the cwd YAML+env configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration struct.
type Config struct {
	Server  ServerConfig            `yaml:"server"`
	Store   StoreConfig             `yaml:"store"`
	Cache   CacheConfig             `yaml:"cache"`
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

// CacheConfig holds image cache settings.
type CacheConfig struct {
	ImageDir      string `yaml:"image_dir"`
	ImageMaxBytes int64  `yaml:"image_max_bytes"`
}

// SourceConfig holds per-source polling settings.
type SourceConfig struct {
	Interval time.Duration `yaml:"interval"`
	Enabled  bool          `yaml:"enabled"`
}

// ImagesConfig holds image loading settings.
type ImagesConfig struct {
	DefaultMode string   `yaml:"default_mode"`
	Prewarm     []string `yaml:"prewarm"`
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

// Load reads the YAML at path, applies defaults, applies CWD_* env overrides, and validates.
func Load(path string) (*Config, error) {
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

	applyEnvOverrides(cfg)

	// Re-apply defaults on a per-source basis so partial source maps still get sane defaults.
	mergeSourceDefaults(cfg)

	if err := validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
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
		Cache: CacheConfig{
			ImageDir:      xdgCache("cwd/img"),
			ImageMaxBytes: 524_288_000,
		},
		Sources: map[string]SourceConfig{
			"nws_alerts":     {Interval: 30 * time.Second, Enabled: true},
			"swpc_scales":    {Interval: 60 * time.Second, Enabled: true},
			"swpc_alerts":    {Interval: 60 * time.Second, Enabled: true},
			"usgs_quakes":    {Interval: 60 * time.Second, Enabled: true},
			"usgs_volcanoes": {Interval: 5 * time.Minute, Enabled: true},
		},
		Images: ImagesConfig{DefaultMode: "lazy"},
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
		cfg.Sources[name] = s
	}
}

// applyEnvOverrides walks CWD_* env variables and writes them into matching string/int/bool fields.
// Naming convention: CWD_<SECTION>_<FIELD>, e.g. CWD_SERVER_BIND, CWD_UI_DEFAULT_THEME.
// Only top-level scalar fields under Server, Store, Cache, Images, UI are supported.
func applyEnvOverrides(cfg *Config) {
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
				if n, err := strconv.ParseInt(val, 10, 64); err == nil {
					f.SetInt(n)
				}
			case reflect.Bool:
				if b, err := strconv.ParseBool(val); err == nil {
					f.SetBool(b)
				}
			}
		}
	}
	apply("server", reflect.ValueOf(&cfg.Server).Elem())
	apply("store", reflect.ValueOf(&cfg.Store).Elem())
	apply("cache", reflect.ValueOf(&cfg.Cache).Elem())
	apply("images", reflect.ValueOf(&cfg.Images).Elem())
	apply("ui", reflect.ValueOf(&cfg.UI).Elem())
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
	switch cfg.Images.DefaultMode {
	case "lazy", "prewarm":
	default:
		return fmt.Errorf("images.default_mode must be lazy|prewarm, got %q", cfg.Images.DefaultMode)
	}
	if cfg.Store.RetentionDays <= 0 {
		return errors.New("store.retention_days must be > 0")
	}
	return nil
}

func xdgState(rel string) string {
	if v := os.Getenv("XDG_STATE_HOME"); v != "" {
		return v + "/" + rel
	}
	if home, err := os.UserHomeDir(); err == nil {
		return home + "/.local/state/" + rel
	}
	return "./" + rel
}

func xdgCache(rel string) string {
	if v := os.Getenv("XDG_CACHE_HOME"); v != "" {
		return v + "/" + rel
	}
	if home, err := os.UserHomeDir(); err == nil {
		return home + "/.cache/" + rel
	}
	return "./" + rel
}
