package config

import (
	"os"
	"path/filepath"
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
	if cfg.Cache.ImageMaxBytes != 524288000 {
		t.Errorf("Cache.ImageMaxBytes default = %d, want 524288000", cfg.Cache.ImageMaxBytes)
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
