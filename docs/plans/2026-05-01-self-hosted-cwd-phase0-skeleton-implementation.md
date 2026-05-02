# Self-Hosted CWD — Phase 0 (Skeleton) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up the empty-but-runnable single-binary skeleton: a Go HTTP server that loads YAML+env config, serves an embedded React+AntD-Pro SPA shell with dark default + 5 routed placeholder pages, exposes `healthz`/`readyz`/`version`/`uiconfig`, and ships with CI + Makefile + lint config. No upstream sources, no cache, no SSE — those are Phases 1–6.

**Architecture:** Three concurrent subsystems are designed (fetchers, cache+store+hub, HTTP API) but Phase 0 only ships the third's skeleton. The frontend uses Vite + React + TS + AntD + AntD Pro Components, built into `internal/webdist/dist/` and embedded via `//go:embed`. The backend wires `cmd/cwd → config → server → chi router → embedded SPA`. Pure Go end-to-end (no CGO).

**Tech Stack:**
- Backend: Go 1.24, `chi` router, `gopkg.in/yaml.v3`, `log/slog` stdlib
- Frontend: Node 22 LTS, pnpm, Vite 5, React 18, TypeScript 5, AntD 5, `@ant-design/pro-components`, `react-router-dom` 6
- Tooling: `golangci-lint` (vet/staticcheck/govet/errcheck), GitHub Actions CI, Makefile

> **For Claude:** REQUIRED EXECUTION WORKFLOW (follow in order):
> 1. `superpowers:using-git-worktrees` — Isolate work in a dedicated worktree
> 2. `superpowers:subagent-driven-development` — Dispatch a fresh subagent per task
> 3. `superpowers:test-driven-development` — All subagents use TDD
> 4. `superpowers:verification-before-completion` — Verify all tests pass per task
> 5. `superpowers:requesting-code-review` — Code review after each task (built in)
> 6. After all tasks: comprehensive code review on full diff from branch point (automatic)
> 7. `superpowers:finishing-a-development-branch` — Complete the branch
>
> Skills carry their own model and effort settings. Do not override them.

---

## Pre-flight decisions (resolve before Task 1)

The design (§13) listed these as deferred to scaffold time. The plan defaults are stated; confirm or override in Task 1:

| Item | Default | Override how |
|---|---|---|
| Go module path | `github.com/acaudill/cwd` | Edit `go mod init` argument in Task 2 |
| License | MIT | Replace `LICENSE` content in Task 1 |
| CI provider | GitHub Actions | Swap `.github/workflows/` for `.gitlab-ci.yml` in Task 16 |
| Repo name on remote | `cwd` (singular, the binary name) | Adjust module path + README accordingly |
| Recon artifacts location | Move `cwd-*.html/.js/.txt/.md/.png` from repo root → `docs/recon/artifacts/` | Done in Task 1 |

If the operator picks GitLab, swap `.github/workflows/ci.yml` for the equivalent `.gitlab-ci.yml`; the script bodies are identical.

---

## File map

Files created in this phase (✱ = test file). Anything not listed is out of scope for Phase 0.

```
LICENSE                                      MIT
README.md                                    minimal: build/run/config
.gitignore                                   Go + Node + Vite + IDE + repo cache
.golangci.yml                                lint config
Makefile                                     build, test, lint, run, build-web, build-all, clean

go.mod, go.sum                               module = github.com/acaudill/cwd

cmd/cwd/main.go                              entry: flags → config.Load → server.Run

internal/config/config.go                    YAML+env loading, defaults, validation
internal/config/config_test.go              ✱
internal/config/testdata/minimal.yaml        fixture
internal/config/testdata/full.yaml           fixture

internal/version/version.go                  build-time vars (version, commit, date)

internal/api/router.go                       chi mux + middleware (logger, recoverer)
internal/api/healthz.go                      GET /healthz, /readyz
internal/api/version.go                      GET /api/version
internal/api/uiconfig.go                     GET /api/uiconfig
internal/api/router_test.go                 ✱
internal/api/healthz_test.go                ✱
internal/api/version_test.go                ✱
internal/api/uiconfig_test.go               ✱

internal/server/server.go                    lifecycle: bind, graceful shutdown, signals
internal/server/server_test.go              ✱

internal/webdist/embed.go                    //go:embed dist/*
internal/webdist/dist/.gitkeep               so embed compiles before first pnpm build
internal/webdist/dist/index.html             placeholder so embed compiles before first frontend build

web/.gitignore                               node_modules, dist
web/package.json                             pinned versions
web/pnpm-lock.yaml                           generated
web/tsconfig.json
web/tsconfig.node.json                       for vite.config.ts
web/vite.config.ts                           outDir: ../internal/webdist/dist
web/index.html                               Vite entry HTML
web/src/main.tsx                             ReactDOM.createRoot
web/src/App.tsx                              ConfigProvider + ProLayout + Routes
web/src/theme.ts                             ConfigProvider tokens, dark/light/auto
web/src/api/client.ts                        fetch wrapper
web/src/api/types.ts                         Version, UIConfig
web/src/pages/Overview.tsx                   <Empty> placeholder
web/src/pages/Hazards.tsx                    <Empty> placeholder
web/src/pages/SpaceWeather.tsx               <Empty> placeholder
web/src/pages/Events.tsx                     <Empty> placeholder
web/src/pages/History.tsx                    <Empty> placeholder
web/src/components/SettingsDrawer.tsx        Drawer; theme switcher only in Phase 0

.github/workflows/ci.yml                     lint, test, build (Go + frontend)
docs/recon/artifacts/                        moved recon files (cwd-*.{html,js,txt,md,png})
```

---

## Tasks

### Task 1: Repo bootstrap (`git init`, .gitignore, README skeleton, LICENSE, recon move)

**Files:**
- Create: `LICENSE`, `README.md`, `.gitignore`
- Move: `cwd-fullpage.png`, `cwd-get_count_alerts.js`, `cwd-get_cwd_info.js`, `cwd-index.html`, `cwd-network.txt`, `cwd-snapshot.md`, `.playwright-mcp/` → `docs/recon/artifacts/`

- [ ] **Step 1.1: Confirm pre-flight decisions** with the operator (module path, license, CI provider). If they override, update the plan steps below in-place before continuing.

- [ ] **Step 1.2: Move recon artifacts out of repo root**

```bash
mkdir -p docs/recon/artifacts
mv cwd-fullpage.png cwd-get_count_alerts.js cwd-get_cwd_info.js cwd-index.html cwd-network.txt cwd-snapshot.md docs/recon/artifacts/
mv .playwright-mcp docs/recon/artifacts/
```

- [ ] **Step 1.3: Write `.gitignore`**

```gitignore
# Binaries
/cwd
/dist/

# Go
*.exe
*.test
*.out
/bin/
/coverage.out
/coverage.html

# Node / Vite
web/node_modules/
web/dist/
web/.pnpm-store/

# Embedded SPA build output (regenerated; never commit)
internal/webdist/dist/*
!internal/webdist/dist/.gitkeep

# IDE / OS
.DS_Store
.idea/
.vscode/
*.swp

# Local config & state
*.local.yaml
config.local.yaml
```

- [ ] **Step 1.4: Write `LICENSE` (MIT)**

```text
MIT License

Copyright (c) 2026 Adam Caudill

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

- [ ] **Step 1.5: Write `README.md` skeleton**

```markdown
# cwd — Self-Hosted Critical Weather Day Status

A self-hostable replacement for [https://www.nco.ncep.noaa.gov/status/cwd/](https://www.nco.ncep.noaa.gov/status/cwd/).

**Status:** Phase 0 (skeleton). No live data yet — see `docs/plans/`.

## Quick start

```bash
make build-all     # builds frontend + backend, produces ./cwd
./cwd serve        # binds 127.0.0.1:8765 by default
open http://127.0.0.1:8765
```

## Configuration

Default config path: `$XDG_CONFIG_HOME/cwd/config.yaml` (or `--config <path>`, or `$CWD_CONFIG`). See `docs/plans/2026-05-01-self-hosted-cwd-design.md` §8 for the full schema.

## Development

```bash
make test          # backend unit tests
make lint          # golangci-lint
make run           # backend only (no frontend rebuild)
make build-web     # frontend only → internal/webdist/dist/
```

## Documentation

- Design: `docs/plans/2026-05-01-self-hosted-cwd-design.md`
- Recon: `docs/recon/2026-05-01-ncep-cwd-status-recon.md`
- License: MIT
```

- [ ] **Step 1.6: `git init` and first commit**

```bash
git init -b main
git add LICENSE README.md .gitignore docs/
git status                                 # verify recon files are staged under docs/recon/
git commit -m "chore: bootstrap repo with license, readme, gitignore, and recon artifacts"
```

Expected: clean working tree after commit. `git log --oneline` shows one commit.

---

### Task 2: Go module + .golangci.yml + dist/.gitkeep

**Files:**
- Create: `go.mod` (via `go mod init`), `.golangci.yml`, `internal/webdist/dist/.gitkeep`, `internal/webdist/dist/index.html`

- [ ] **Step 2.1: Initialize Go module**

```bash
go mod init github.com/acaudill/cwd
```

Expected: `go.mod` created with `module github.com/acaudill/cwd` and `go 1.24`.

- [ ] **Step 2.2: Add chi + yaml.v3 deps (no code yet — just lock the versions)**

```bash
go get github.com/go-chi/chi/v5@v5.1.0
go get gopkg.in/yaml.v3@v3.0.1
go mod tidy
```

Expected: `go.sum` populated with chi + yaml.

- [ ] **Step 2.3: Write `.golangci.yml`**

```yaml
run:
  timeout: 5m
  go: "1.24"

linters:
  disable-all: true
  enable:
    - errcheck
    - govet
    - ineffassign
    - staticcheck
    - unused
    - gofmt
    - goimports
    - misspell
    - revive

linters-settings:
  revive:
    rules:
      - name: var-naming
      - name: exported
      - name: unused-parameter
        disabled: true

issues:
  exclude-dirs:
    - internal/webdist/dist
  exclude-rules:
    - path: _test\.go
      linters:
        - errcheck
```

- [ ] **Step 2.4: Create the embed placeholder so `//go:embed` compiles before any frontend build**

```bash
mkdir -p internal/webdist/dist
touch internal/webdist/dist/.gitkeep
```

```html
<!-- internal/webdist/dist/index.html -->
<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <title>cwd — placeholder</title>
  </head>
  <body>
    <p>This is the embed placeholder. Run <code>make build-web</code> to build the real SPA.</p>
  </body>
</html>
```

- [ ] **Step 2.5: Verify lint runs (no Go files yet, should report no issues)**

```bash
golangci-lint run ./...
```

Expected: exit 0, no findings.

- [ ] **Step 2.6: Commit**

```bash
git add go.mod go.sum .golangci.yml internal/webdist/dist/.gitkeep internal/webdist/dist/index.html
git commit -m "chore: init go module and lint config; add embed placeholder"
```

---

### Task 3: Config package (TDD)

**Files:**
- Create: `internal/config/config.go`, `internal/config/config_test.go`, `internal/config/testdata/minimal.yaml`, `internal/config/testdata/full.yaml`

- [ ] **Step 3.1: Write fixtures**

`internal/config/testdata/minimal.yaml`:
```yaml
server:
  contact: "ops@example.com"
```

`internal/config/testdata/full.yaml`:
```yaml
server:
  bind: "0.0.0.0:9000"
  contact: "ops@example.com"
  log_level: "debug"
  log_format: "text"

store:
  path: "/tmp/cwd.db"
  retention_days: 7

cache:
  image_dir: "/tmp/cwd-img"
  image_max_bytes: 1048576

sources:
  nws_alerts:     { interval: "30s", enabled: true }
  swpc_scales:    { interval: "60s", enabled: true }
  swpc_alerts:    { interval: "60s", enabled: true }
  usgs_quakes:    { interval: "60s", enabled: true }
  usgs_volcanoes: { interval: "5m",  enabled: true }

images:
  default_mode: "lazy"
  prewarm: []

derived:
  thresholds:
    swpc_alert_window_hours: 24
    swpc_alert_products: [K08A, K09A, P12A, P13A]
    region_filter:
      ugcs: []
      wfos: []

ui:
  default_theme: "dark"
  default_landing: "/"
  enable_history: true
```

- [ ] **Step 3.2: Write the failing tests**

`internal/config/config_test.go`:
```go
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
```

- [ ] **Step 3.3: Run tests to confirm they fail**

```bash
go test ./internal/config/...
```

Expected: build failure (`Load`, `Config` types not defined).

- [ ] **Step 3.4: Implement `internal/config/config.go`**

```go
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

type Config struct {
	Server   ServerConfig            `yaml:"server"`
	Store    StoreConfig             `yaml:"store"`
	Cache    CacheConfig             `yaml:"cache"`
	Sources  map[string]SourceConfig `yaml:"sources"`
	Images   ImagesConfig            `yaml:"images"`
	Derived  DerivedConfig           `yaml:"derived"`
	UI       UIConfig                `yaml:"ui"`
}

type ServerConfig struct {
	Bind      string `yaml:"bind"`
	Contact   string `yaml:"contact"`
	LogLevel  string `yaml:"log_level"`
	LogFormat string `yaml:"log_format"`
}

type StoreConfig struct {
	Path          string `yaml:"path"`
	RetentionDays int    `yaml:"retention_days"`
}

type CacheConfig struct {
	ImageDir      string `yaml:"image_dir"`
	ImageMaxBytes int64  `yaml:"image_max_bytes"`
}

type SourceConfig struct {
	Interval time.Duration `yaml:"interval"`
	Enabled  bool          `yaml:"enabled"`
}

type ImagesConfig struct {
	DefaultMode string   `yaml:"default_mode"`
	Prewarm     []string `yaml:"prewarm"`
}

type DerivedConfig struct {
	Thresholds ThresholdsConfig `yaml:"thresholds"`
}

type ThresholdsConfig struct {
	SWPCAlertWindowHours int          `yaml:"swpc_alert_window_hours"`
	SWPCAlertProducts    []string     `yaml:"swpc_alert_products"`
	RegionFilter         RegionFilter `yaml:"region_filter"`
}

type RegionFilter struct {
	UGCs []string `yaml:"ugcs"`
	WFOs []string `yaml:"wfos"`
}

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
```

- [ ] **Step 3.5: Run tests to confirm they pass**

```bash
go test ./internal/config/... -v
```

Expected: all 6 tests PASS.

- [ ] **Step 3.6: Commit**

```bash
git add internal/config/
git commit -m "feat(config): YAML+env config with defaults, validation, and dark default theme"
```

---

### Task 4: Version package

**Files:**
- Create: `internal/version/version.go`

- [ ] **Step 4.1: Write `internal/version/version.go`**

```go
// Package version exposes build-time identifiers, set via -ldflags.
package version

// These are overridden at build time, e.g.:
//   go build -ldflags="-X github.com/acaudill/cwd/internal/version.Version=v0.1.0 \
//     -X github.com/acaudill/cwd/internal/version.Commit=$(git rev-parse --short HEAD) \
//     -X github.com/acaudill/cwd/internal/version.Date=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

// Info is the JSON shape returned by GET /api/version.
type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
	Go      string `json:"go"`
}
```

- [ ] **Step 4.2: Compile-check**

```bash
go build ./internal/version/...
```

Expected: exit 0.

- [ ] **Step 4.3: Commit**

```bash
git add internal/version/
git commit -m "feat(version): build-time version/commit/date identifiers"
```

---

### Task 5: Webdist embed package

**Files:**
- Create: `internal/webdist/embed.go`

- [ ] **Step 5.1: Write `internal/webdist/embed.go`**

```go
// Package webdist embeds the built React+AntD-Pro SPA at compile time.
// The dist/ directory is populated by `pnpm build` from web/.
// A placeholder index.html is committed so the package compiles before any
// frontend build has run (e.g. on a fresh clone before `make build-web`).
package webdist

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embedded embed.FS

// FS returns the SPA filesystem rooted at "dist".
// Use with http.FileServer(http.FS(webdist.FS())) or with the SPA fallback handler.
func FS() fs.FS {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		// Programmer error — the embed directive guarantees dist/ exists.
		panic("webdist: embedded dist subtree missing: " + err.Error())
	}
	return sub
}
```

- [ ] **Step 5.2: Compile-check**

```bash
go build ./internal/webdist/...
```

Expected: exit 0 (the placeholder `dist/index.html` from Task 2 satisfies the embed directive).

- [ ] **Step 5.3: Commit**

```bash
git add internal/webdist/embed.go
git commit -m "feat(webdist): embed SPA build via //go:embed"
```

---

### Task 6: API endpoints (healthz, readyz, version) (TDD)

**Files:**
- Create: `internal/api/healthz.go`, `internal/api/version.go`, `internal/api/healthz_test.go`, `internal/api/version_test.go`

- [ ] **Step 6.1: Write the failing tests**

`internal/api/healthz_test.go`:
```go
package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthzAlwaysOK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	Healthz()(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if rr.Body.String() != "ok\n" {
		t.Errorf("body = %q, want \"ok\\n\"", rr.Body.String())
	}
}

func TestReadyzReportsReadiness(t *testing.T) {
	ready := false
	h := Readyz(func() bool { return ready })

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("not ready: status = %d, want 503", rr.Code)
	}

	ready = true
	rr = httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("ready: status = %d, want 200", rr.Code)
	}
}
```

`internal/api/version_test.go`:
```go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/acaudill/cwd/internal/version"
)

func TestVersionReturnsBuildInfo(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	rr := httptest.NewRecorder()
	Version()(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var got version.Info
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Version != version.Version {
		t.Errorf("Version = %q, want %q", got.Version, version.Version)
	}
	if got.Go != runtime.Version() {
		t.Errorf("Go = %q, want %q", got.Go, runtime.Version())
	}
}
```

- [ ] **Step 6.2: Run tests to confirm they fail**

```bash
go test ./internal/api/...
```

Expected: build failure (`Healthz`, `Readyz`, `Version` not defined).

- [ ] **Step 6.3: Implement `internal/api/healthz.go`**

```go
package api

import (
	"io"
	"net/http"
)

// Healthz returns a handler that always responds 200 OK — proves the process is up.
func Healthz() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, "ok\n")
	}
}

// Readyz returns a handler that responds 200 only when ready() reports true.
// Phase 1+ wires ready() to "all enabled sources have produced at least one snapshot".
// In Phase 0 the operator passes a no-op true predicate.
func Readyz(ready func() bool) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if !ready() {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, "not ready\n")
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, "ready\n")
	}
}
```

- [ ] **Step 6.4: Implement `internal/api/version.go`**

```go
package api

import (
	"encoding/json"
	"net/http"
	"runtime"

	"github.com/acaudill/cwd/internal/version"
)

// Version returns a handler that emits the build-time identifiers as JSON.
func Version() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		info := version.Info{
			Version: version.Version,
			Commit:  version.Commit,
			Date:    version.Date,
			Go:      runtime.Version(),
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(info)
	}
}
```

- [ ] **Step 6.5: Run tests to confirm they pass**

```bash
go test ./internal/api/... -v
```

Expected: 3 tests PASS.

- [ ] **Step 6.6: Commit**

```bash
git add internal/api/healthz.go internal/api/version.go internal/api/healthz_test.go internal/api/version_test.go
git commit -m "feat(api): healthz, readyz, version endpoints"
```

---

### Task 7: API uiconfig endpoint (TDD)

**Files:**
- Create: `internal/api/uiconfig.go`, `internal/api/uiconfig_test.go`

- [ ] **Step 7.1: Write the failing test**

`internal/api/uiconfig_test.go`:
```go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/acaudill/cwd/internal/config"
)

func TestUIConfigSubsetsTheConfig(t *testing.T) {
	cfg := &config.Config{
		UI: config.UIConfig{
			DefaultTheme:   "dark",
			DefaultLanding: "/",
			EnableHistory:  true,
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/uiconfig", nil)
	rr := httptest.NewRecorder()
	UIConfig(cfg)(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["defaultTheme"] != "dark" {
		t.Errorf("defaultTheme = %v, want dark", got["defaultTheme"])
	}
	if got["defaultLanding"] != "/" {
		t.Errorf("defaultLanding = %v, want /", got["defaultLanding"])
	}
	if got["enableHistory"] != true {
		t.Errorf("enableHistory = %v, want true", got["enableHistory"])
	}
	// must NOT leak server config like contact / log_level
	if _, ok := got["contact"]; ok {
		t.Error("uiconfig should not expose server.contact")
	}
}
```

- [ ] **Step 7.2: Run test to confirm it fails**

```bash
go test ./internal/api/... -run UIConfig
```

Expected: build failure (`UIConfig` not defined).

- [ ] **Step 7.3: Implement `internal/api/uiconfig.go`**

```go
package api

import (
	"encoding/json"
	"net/http"

	"github.com/acaudill/cwd/internal/config"
)

// UIConfig returns the subset of the server's config that's safe to expose to the SPA.
// This is the source of truth for client-side defaults (theme, landing page, feature flags).
// Server-side knobs (bind, contact, log level, intervals, retention) are NOT exposed.
func UIConfig(cfg *config.Config) http.HandlerFunc {
	type payload struct {
		DefaultTheme   string `json:"defaultTheme"`
		DefaultLanding string `json:"defaultLanding"`
		EnableHistory  bool   `json:"enableHistory"`
	}
	return func(w http.ResponseWriter, _ *http.Request) {
		p := payload{
			DefaultTheme:   cfg.UI.DefaultTheme,
			DefaultLanding: cfg.UI.DefaultLanding,
			EnableHistory:  cfg.UI.EnableHistory,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(p)
	}
}
```

- [ ] **Step 7.4: Run test to confirm it passes**

```bash
go test ./internal/api/... -run UIConfig -v
```

Expected: PASS.

- [ ] **Step 7.5: Commit**

```bash
git add internal/api/uiconfig.go internal/api/uiconfig_test.go
git commit -m "feat(api): uiconfig endpoint serves SPA-safe config subset"
```

---

### Task 8: API router (TDD)

**Files:**
- Create: `internal/api/router.go`, `internal/api/router_test.go`

- [ ] **Step 8.1: Write the failing tests**

`internal/api/router_test.go`:
```go
package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/acaudill/cwd/internal/config"
)

func TestRouterServesAllEndpoints(t *testing.T) {
	cfg := &config.Config{UI: config.UIConfig{DefaultTheme: "dark", DefaultLanding: "/", EnableHistory: true}}
	h := NewRouter(cfg, func() bool { return true })

	cases := []struct {
		path string
		want int
	}{
		{"/healthz", 200},
		{"/readyz", 200},
		{"/api/version", 200},
		{"/api/uiconfig", 200},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != tc.want {
			t.Errorf("GET %s: status = %d, want %d", tc.path, rr.Code, tc.want)
		}
	}
}

func TestRouterServesEmbeddedSPAOnUnknownPath(t *testing.T) {
	cfg := &config.Config{UI: config.UIConfig{DefaultTheme: "dark"}}
	h := NewRouter(cfg, func() bool { return true })

	req := httptest.NewRequest(http.MethodGet, "/some/spa/route", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	body, _ := io.ReadAll(rr.Body)
	if !strings.Contains(string(body), "<html") {
		t.Errorf("body should contain <html, got %q", string(body)[:min(80, len(body))])
	}
}

func TestRouterDoesNotIntercept404OnAPI(t *testing.T) {
	cfg := &config.Config{UI: config.UIConfig{DefaultTheme: "dark"}}
	h := NewRouter(cfg, func() bool { return true })

	req := httptest.NewRequest(http.MethodGet, "/api/does-not-exist", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

func min(a, b int) int { if a < b { return a }; return b }
```

- [ ] **Step 8.2: Run tests to confirm they fail**

```bash
go test ./internal/api/... -run Router
```

Expected: `NewRouter` not defined.

- [ ] **Step 8.3: Implement `internal/api/router.go`**

```go
package api

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	chimid "github.com/go-chi/chi/v5/middleware"

	"github.com/acaudill/cwd/internal/config"
	"github.com/acaudill/cwd/internal/webdist"
)

// NewRouter assembles the Phase 0 HTTP surface:
//   - /healthz, /readyz             — liveness + readiness probes
//   - /api/version, /api/uiconfig   — JSON endpoints
//   - /                             — embedded SPA (with SPA-fallback for client-side routes)
//
// /api/* paths that don't match return 404 (no SPA fallback for the API namespace).
func NewRouter(cfg *config.Config, ready func() bool) http.Handler {
	r := chi.NewRouter()
	r.Use(chimid.RequestID)
	r.Use(chimid.RealIP)
	r.Use(chimid.Recoverer)

	r.Get("/healthz", Healthz())
	r.Get("/readyz", Readyz(ready))

	r.Route("/api", func(r chi.Router) {
		r.Get("/version", Version())
		r.Get("/uiconfig", UIConfig(cfg))
	})

	r.NotFound(spaFallback())
	return r
}

// spaFallback serves the embedded SPA's index.html for any unmatched non-/api path,
// and serves static SPA assets for paths that exist in the embedded FS.
// /api/* paths return a plain 404.
func spaFallback() http.HandlerFunc {
	fileServer := http.FileServer(http.FS(webdist.FS()))
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		// If the path doesn't exist as a static asset, rewrite to "/" so the SPA loads index.html
		// and lets react-router handle the route.
		if !assetExists(r.URL.Path) {
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, r2)
			return
		}
		fileServer.ServeHTTP(w, r)
	}
}

func assetExists(p string) bool {
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		return true
	}
	f, err := webdist.FS().Open(p)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}
```

- [ ] **Step 8.4: Run tests to confirm they pass**

```bash
go test ./internal/api/... -v
```

Expected: all router tests PASS along with the earlier endpoint tests.

- [ ] **Step 8.5: Lint clean**

```bash
golangci-lint run ./internal/api/...
```

Expected: exit 0.

- [ ] **Step 8.6: Commit**

```bash
git add internal/api/router.go internal/api/router_test.go
git commit -m "feat(api): chi router with SPA fallback and /api 404 isolation"
```

---

### Task 9: Server lifecycle (TDD)

**Files:**
- Create: `internal/server/server.go`, `internal/server/server_test.go`

- [ ] **Step 9.1: Write the failing test**

`internal/server/server_test.go`:
```go
package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/acaudill/cwd/internal/config"
)

func TestRunStartsAndShutsDownGracefully(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Bind:      "127.0.0.1:0", // pick any free port
			LogLevel:  "info",
			LogFormat: "text",
		},
		UI: config.UIConfig{DefaultTheme: "dark", DefaultLanding: "/", EnableHistory: true},
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
	if err := <-errCh; err != nil && err != context.Canceled && err != http.ErrServerClosed {
		t.Errorf("Run error = %v", err)
	}
}
```

- [ ] **Step 9.2: Run test to confirm it fails**

```bash
go test ./internal/server/...
```

Expected: `Run` not defined.

- [ ] **Step 9.3: Implement `internal/server/server.go`**

```go
// Package server owns the HTTP server lifecycle: bind, accept, graceful shutdown.
package server

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/acaudill/cwd/internal/api"
	"github.com/acaudill/cwd/internal/config"
)

// Run binds the configured address and serves until ctx is canceled.
// If addrCh is non-nil, the actual listen address (after :0 resolution) is sent on it.
func Run(ctx context.Context, cfg *config.Config, logger *slog.Logger, addrCh chan<- string) error {
	ready := func() bool { return true } // Phase 0: nothing to wait on; Phase 1+ wires real readiness.

	handler := api.NewRouter(cfg, ready)

	ln, err := net.Listen("tcp", cfg.Server.Bind)
	if err != nil {
		return err
	}
	if addrCh != nil {
		addrCh <- ln.Addr().String()
	}

	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	logger.Info("server listening", "addr", ln.Addr().String())

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- srv.Serve(ln)
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown requested")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return nil
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
```

- [ ] **Step 9.4: Run test to confirm it passes**

```bash
go test ./internal/server/... -v
```

Expected: PASS.

- [ ] **Step 9.5: Lint clean**

```bash
golangci-lint run ./internal/server/...
```

Expected: exit 0.

- [ ] **Step 9.6: Commit**

```bash
git add internal/server/
git commit -m "feat(server): http server lifecycle with graceful shutdown"
```

---

### Task 10: cmd/cwd entry point

**Files:**
- Create: `cmd/cwd/main.go`

- [ ] **Step 10.1: Write `cmd/cwd/main.go`**

```go
// Command cwd is the self-hosted Critical Weather Day Status server.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/acaudill/cwd/internal/config"
	"github.com/acaudill/cwd/internal/server"
	"github.com/acaudill/cwd/internal/version"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "serve":
		if err := runServe(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "cwd serve: %v\n", err)
			os.Exit(1)
		}
	case "version", "-v", "--version":
		fmt.Printf("cwd %s (commit %s, built %s)\n", version.Version, version.Commit, version.Date)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	configPath := fs.String("config", "", "path to config.yaml (default: $CWD_CONFIG, then $XDG_CONFIG_HOME/cwd/config.yaml)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	path := resolveConfigPath(*configPath)
	cfg, err := config.Load(path)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := newLogger(cfg)
	if cfg.MissingContact() {
		logger.Warn("server.contact is unset; NWS API requests will use a placeholder UA. Set CWD_SERVER_CONTACT or server.contact in config.yaml.")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	return server.Run(ctx, cfg, logger, nil)
}

func resolveConfigPath(flagPath string) string {
	if flagPath != "" {
		return flagPath
	}
	if v := os.Getenv("CWD_CONFIG"); v != "" {
		return v
	}
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return v + "/cwd/config.yaml"
	}
	if home, err := os.UserHomeDir(); err == nil {
		p := home + "/.config/cwd/config.yaml"
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "" // Load("") returns defaults-only
}

func newLogger(cfg *config.Config) *slog.Logger {
	var level slog.Level
	switch cfg.Server.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if cfg.Server.LogFormat == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.New(handler)
}

func usage() {
	fmt.Fprintln(os.Stderr, `cwd — self-hosted Critical Weather Day Status

USAGE:
  cwd serve [--config <path>]   Start the HTTP server
  cwd version                   Print version info
  cwd help                      Print this message

CONFIG RESOLUTION ORDER:
  1. --config flag
  2. $CWD_CONFIG
  3. $XDG_CONFIG_HOME/cwd/config.yaml
  4. ~/.config/cwd/config.yaml
  5. defaults only (no file)`)
}
```

- [ ] **Step 10.2: Compile-check**

```bash
go build -o cwd ./cmd/cwd
```

Expected: produces `./cwd` binary.

- [ ] **Step 10.3: Smoke-test the help output**

```bash
./cwd help
./cwd version
```

Expected: help text printed; version prints `cwd dev (commit unknown, built unknown)`.

- [ ] **Step 10.4: Smoke-test serve (background, hit healthz, kill)**

```bash
./cwd serve &
SERVER_PID=$!
sleep 1
curl -sS -o /dev/null -w "%{http_code}\n" http://127.0.0.1:8765/healthz
curl -sS http://127.0.0.1:8765/api/version
curl -sS http://127.0.0.1:8765/api/uiconfig
kill $SERVER_PID
wait $SERVER_PID 2>/dev/null
```

Expected: `200`, valid JSON for version + uiconfig, server exits cleanly.

- [ ] **Step 10.5: Commit**

```bash
git add cmd/cwd/main.go
rm -f ./cwd
git commit -m "feat(cmd): cwd serve|version|help entry point"
```

---

### Task 11: Backend Makefile targets

**Files:**
- Create: `Makefile`

- [ ] **Step 11.1: Write `Makefile`** (only backend + meta targets in this task; frontend targets added in Task 14)

```makefile
# cwd — self-hosted Critical Weather Day Status
# Default target prints help.

GO          ?= go
BIN_DIR     ?= bin
BIN          := $(BIN_DIR)/cwd
PKG          := github.com/acaudill/cwd

VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE        := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS     := -X $(PKG)/internal/version.Version=$(VERSION) \
               -X $(PKG)/internal/version.Commit=$(COMMIT) \
               -X $(PKG)/internal/version.Date=$(DATE)

.PHONY: help build test lint run clean

help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the cwd binary (assumes web/dist already populated)
	@mkdir -p $(BIN_DIR)
	$(GO) build -ldflags="$(LDFLAGS)" -o $(BIN) ./cmd/cwd

test: ## Run backend unit tests
	$(GO) test ./...

lint: ## Run golangci-lint
	golangci-lint run ./...

run: ## Run cwd serve with current code (no rebuild of frontend)
	$(GO) run ./cmd/cwd serve

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR)
```

- [ ] **Step 11.2: Verify each target works**

```bash
make help
make test
make lint
make build
ls -la bin/cwd
```

Expected: `make help` prints target list; `make test` PASS; `make lint` exit 0; `make build` produces `bin/cwd`.

- [ ] **Step 11.3: Commit**

```bash
git add Makefile
git commit -m "build: backend Makefile targets (build, test, lint, run, clean)"
```

---

### Task 12: Frontend bootstrap (Vite + React + TS + AntD + AntD Pro)

**Files:**
- Create: `web/package.json`, `web/tsconfig.json`, `web/tsconfig.node.json`, `web/vite.config.ts`, `web/index.html`, `web/.gitignore`

- [ ] **Step 12.1: Write `web/.gitignore`**

```gitignore
node_modules
dist
.pnpm-store
```

- [ ] **Step 12.2: Write `web/package.json` (pinned versions)**

```json
{
  "name": "cwd-web",
  "private": true,
  "version": "0.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build",
    "preview": "vite preview",
    "typecheck": "tsc --noEmit"
  },
  "dependencies": {
    "@ant-design/icons": "5.5.1",
    "@ant-design/pro-components": "2.7.18",
    "antd": "5.21.4",
    "react": "18.3.1",
    "react-dom": "18.3.1",
    "react-router-dom": "6.27.0",
    "zustand": "5.0.0"
  },
  "devDependencies": {
    "@types/react": "18.3.11",
    "@types/react-dom": "18.3.1",
    "@vitejs/plugin-react-swc": "3.7.1",
    "typescript": "5.6.3",
    "vite": "5.4.10"
  },
  "packageManager": "pnpm@9.12.2",
  "engines": {
    "node": ">=22"
  }
}
```

- [ ] **Step 12.3: Write `web/tsconfig.json`**

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "useDefineForClassFields": true,
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "Bundler",
    "allowImportingTsExtensions": true,
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,
    "baseUrl": ".",
    "paths": { "@/*": ["src/*"] }
  },
  "include": ["src"],
  "references": [{ "path": "./tsconfig.node.json" }]
}
```

- [ ] **Step 12.4: Write `web/tsconfig.node.json`**

```json
{
  "compilerOptions": {
    "composite": true,
    "skipLibCheck": true,
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "allowSyntheticDefaultImports": true,
    "strict": true
  },
  "include": ["vite.config.ts"]
}
```

- [ ] **Step 12.5: Write `web/vite.config.ts`**

```ts
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react-swc";
import path from "node:path";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: { "@": path.resolve(__dirname, "./src") },
  },
  build: {
    // Embed target — Go side reads from internal/webdist/dist/
    outDir: path.resolve(__dirname, "../internal/webdist/dist"),
    emptyOutDir: true,
    sourcemap: true,
    chunkSizeWarningLimit: 1500, // AntD Pro is chonky; expected
  },
  server: {
    port: 5173,
    proxy: {
      "/api":     "http://127.0.0.1:8765",
      "/healthz": "http://127.0.0.1:8765",
      "/readyz":  "http://127.0.0.1:8765",
    },
  },
});
```

- [ ] **Step 12.6: Write `web/index.html`**

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0, viewport-fit=cover" />
    <meta name="color-scheme" content="dark light" />
    <title>cwd — Critical Weather Day Status</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

- [ ] **Step 12.7: Install dependencies (pnpm)**

```bash
cd web
pnpm install
```

Expected: `pnpm-lock.yaml` generated, `node_modules/` populated.

- [ ] **Step 12.8: Commit pnpm-lock**

```bash
cd ..
git add web/.gitignore web/package.json web/pnpm-lock.yaml web/tsconfig.json web/tsconfig.node.json web/vite.config.ts web/index.html
git commit -m "feat(web): bootstrap vite+react+ts+antd+antd-pro toolchain"
```

---

### Task 13: Frontend theme + ConfigProvider with dark default

**Files:**
- Create: `web/src/theme.ts`, `web/src/api/types.ts`, `web/src/api/client.ts`, `web/src/main.tsx`

- [ ] **Step 13.1: Write `web/src/api/types.ts`**

```ts
export type ThemeMode = "dark" | "light" | "auto";

export interface UIConfig {
  defaultTheme: ThemeMode;
  defaultLanding: string;
  enableHistory: boolean;
}

export interface VersionInfo {
  version: string;
  commit: string;
  date: string;
  go: string;
}
```

- [ ] **Step 13.2: Write `web/src/api/client.ts`**

```ts
import type { UIConfig, VersionInfo } from "./types";

async function getJSON<T>(path: string): Promise<T> {
  const res = await fetch(path, { headers: { Accept: "application/json" } });
  if (!res.ok) throw new Error(`${path}: ${res.status} ${res.statusText}`);
  return (await res.json()) as T;
}

export const api = {
  uiconfig: () => getJSON<UIConfig>("/api/uiconfig"),
  version:  () => getJSON<VersionInfo>("/api/version"),
};
```

- [ ] **Step 13.3: Write `web/src/theme.ts`**

```ts
import { theme as antdTheme, type ThemeConfig } from "antd";
import type { ThemeMode } from "./api/types";

const tokenOverrides: ThemeConfig["token"] = {
  colorPrimary: "#0a4f8a",
  borderRadius: 6,
  fontFamilyCode:
    "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace",
};

/**
 * resolveAlgorithm picks the AntD algorithm for the current theme mode.
 * "auto" honours `prefers-color-scheme`.
 */
export function resolveAlgorithm(mode: ThemeMode): typeof antdTheme.darkAlgorithm {
  if (mode === "dark") return antdTheme.darkAlgorithm;
  if (mode === "light") return antdTheme.defaultAlgorithm;
  // auto
  if (typeof window !== "undefined" && window.matchMedia?.("(prefers-color-scheme: dark)").matches) {
    return antdTheme.darkAlgorithm;
  }
  return antdTheme.defaultAlgorithm;
}

export function buildThemeConfig(mode: ThemeMode): ThemeConfig {
  return {
    algorithm: resolveAlgorithm(mode),
    token: tokenOverrides,
  };
}

const STORAGE_KEY = "cwd.themeMode";

export function loadStoredTheme(): ThemeMode | null {
  try {
    const v = localStorage.getItem(STORAGE_KEY);
    if (v === "dark" || v === "light" || v === "auto") return v;
  } catch {
    /* ignore */
  }
  return null;
}

export function persistTheme(mode: ThemeMode): void {
  try {
    localStorage.setItem(STORAGE_KEY, mode);
  } catch {
    /* ignore */
  }
}
```

- [ ] **Step 13.4: Write `web/src/main.tsx`**

```tsx
import React from "react";
import ReactDOM from "react-dom/client";
import { App as AntdApp, ConfigProvider } from "antd";
import { BrowserRouter } from "react-router-dom";
import App from "./App";
import { buildThemeConfig, loadStoredTheme } from "./theme";

// Server-default-theme is fetched async by App (from /api/uiconfig). Until then,
// honour any stored choice or fall back to "dark" (matches design §6.1 default).
const initialMode = loadStoredTheme() ?? "dark";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <ConfigProvider theme={buildThemeConfig(initialMode)}>
      <AntdApp>
        <BrowserRouter>
          <App initialThemeMode={initialMode} />
        </BrowserRouter>
      </AntdApp>
    </ConfigProvider>
  </React.StrictMode>,
);
```

- [ ] **Step 13.5: Compile-check (App.tsx is added in Task 14, so this step ends with a known type error if run alone — defer the build to Task 14)**

Skip a build run here; Task 14 introduces `App.tsx` and we run `pnpm build` at the end of Task 14.

- [ ] **Step 13.6: Commit**

```bash
git add web/src/theme.ts web/src/api/types.ts web/src/api/client.ts web/src/main.tsx
git commit -m "feat(web): theme + api client + main entry with dark default"
```

---

### Task 14: Frontend ProLayout + routes + Settings drawer + first build

**Files:**
- Create: `web/src/App.tsx`, `web/src/components/SettingsDrawer.tsx`, `web/src/pages/Overview.tsx`, `web/src/pages/Hazards.tsx`, `web/src/pages/SpaceWeather.tsx`, `web/src/pages/Events.tsx`, `web/src/pages/History.tsx`

- [ ] **Step 14.1: Write five placeholder pages**

`web/src/pages/Overview.tsx`:
```tsx
import { Empty, Typography } from "antd";

export default function Overview() {
  return (
    <div style={{ padding: 24 }}>
      <Typography.Title level={3}>Overview</Typography.Title>
      <Empty description="Live status, alert badges, and 3-day outlook arrive in Phase 1." />
    </div>
  );
}
```

`web/src/pages/Hazards.tsx`:
```tsx
import { Empty, Typography } from "antd";

export default function Hazards() {
  return (
    <div style={{ padding: 24 }}>
      <Typography.Title level={3}>Hazards</Typography.Title>
      <Empty description="Severe storms, wildfire, rainfall, winter, heat, tropical, flooding maps arrive in Phase 3." />
    </div>
  );
}
```

`web/src/pages/SpaceWeather.tsx`:
```tsx
import { Empty, Typography } from "antd";

export default function SpaceWeather() {
  return (
    <div style={{ padding: 24 }}>
      <Typography.Title level={3}>Space Weather</Typography.Title>
      <Empty description="SWPC 3-day G/S/R forecast arrives in Phase 2." />
    </div>
  );
}
```

`web/src/pages/Events.tsx`:
```tsx
import { Empty, Typography } from "antd";

export default function Events() {
  return (
    <div style={{ padding: 24 }}>
      <Typography.Title level={3}>Events</Typography.Title>
      <Empty description="Tsunamis, earthquakes, and volcano alerts arrive in Phase 2." />
    </div>
  );
}
```

`web/src/pages/History.tsx`:
```tsx
import { Empty, Typography } from "antd";

export default function History() {
  return (
    <div style={{ padding: 24 }}>
      <Typography.Title level={3}>History</Typography.Title>
      <Empty description="Time-slider replay over the SQLite ring buffer arrives in Phase 4." />
    </div>
  );
}
```

- [ ] **Step 14.2: Write `web/src/components/SettingsDrawer.tsx`**

```tsx
import { Drawer, Form, Radio } from "antd";
import type { ThemeMode } from "../api/types";

interface Props {
  open: boolean;
  onClose: () => void;
  themeMode: ThemeMode;
  onThemeChange: (mode: ThemeMode) => void;
}

export default function SettingsDrawer({ open, onClose, themeMode, onThemeChange }: Props) {
  return (
    <Drawer title="Settings" open={open} onClose={onClose} placement="right" width={360}>
      <Form layout="vertical">
        <Form.Item label="Theme" tooltip="Server default applies on first load; your choice persists locally.">
          <Radio.Group
            value={themeMode}
            onChange={(e) => onThemeChange(e.target.value as ThemeMode)}
            optionType="button"
            buttonStyle="solid"
            options={[
              { label: "Dark",  value: "dark"  },
              { label: "Light", value: "light" },
              { label: "Auto",  value: "auto"  },
            ]}
          />
        </Form.Item>
        {/* Server-side knobs (intervals, thresholds, prewarm) are read-only via /api/uiconfig
            in v1 per design §6.2; richer Settings UI lands in Phase 6+. */}
      </Form>
    </Drawer>
  );
}
```

- [ ] **Step 14.3: Write `web/src/App.tsx`**

```tsx
import { useEffect, useMemo, useState } from "react";
import { ConfigProvider, Space, Tag } from "antd";
import { ProLayout } from "@ant-design/pro-components";
import { Route, Routes, useNavigate, useLocation } from "react-router-dom";
import {
  AppstoreOutlined,
  CloudOutlined,
  ExperimentOutlined,
  FireOutlined,
  HistoryOutlined,
  SettingOutlined,
  ThunderboltOutlined,
} from "@ant-design/icons";

import { api } from "./api/client";
import type { ThemeMode, UIConfig, VersionInfo } from "./api/types";
import { buildThemeConfig, persistTheme } from "./theme";
import SettingsDrawer from "./components/SettingsDrawer";

import Overview from "./pages/Overview";
import Hazards from "./pages/Hazards";
import SpaceWeather from "./pages/SpaceWeather";
import Events from "./pages/Events";
import History from "./pages/History";

interface AppProps {
  initialThemeMode: ThemeMode;
}

const route = {
  path: "/",
  routes: [
    { path: "/",        name: "Overview",      icon: <AppstoreOutlined /> },
    { path: "/hazards", name: "Hazards",       icon: <ThunderboltOutlined /> },
    { path: "/space",   name: "Space Weather", icon: <ExperimentOutlined /> },
    { path: "/events",  name: "Events",        icon: <FireOutlined /> },
    { path: "/history", name: "History",       icon: <HistoryOutlined /> },
  ],
};

export default function App({ initialThemeMode }: AppProps) {
  const navigate = useNavigate();
  const location = useLocation();

  const [themeMode, setThemeMode] = useState<ThemeMode>(initialThemeMode);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [serverVersion, setServerVersion] = useState<VersionInfo | null>(null);
  const [uiConfig, setUIConfig] = useState<UIConfig | null>(null);

  // First-paint: load server config + version. If user has no stored choice and the server
  // says a different default, adopt it. (loadStoredTheme already ran in main.tsx, so we
  // only adopt when the user hasn't explicitly chosen.)
  useEffect(() => {
    let cancelled = false;
    Promise.allSettled([api.uiconfig(), api.version()]).then(([cfg, ver]) => {
      if (cancelled) return;
      if (cfg.status === "fulfilled") {
        setUIConfig(cfg.value);
        const stored = localStorage.getItem("cwd.themeMode");
        if (!stored && cfg.value.defaultTheme !== themeMode) {
          setThemeMode(cfg.value.defaultTheme);
        }
      }
      if (ver.status === "fulfilled") setServerVersion(ver.value);
    });
    return () => { cancelled = true; };
    // themeMode intentionally not in deps: this only runs once on mount.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const themeConfig = useMemo(() => buildThemeConfig(themeMode), [themeMode]);

  const onThemeChange = (mode: ThemeMode) => {
    setThemeMode(mode);
    persistTheme(mode);
  };

  return (
    <ConfigProvider theme={themeConfig}>
      <ProLayout
        title="cwd"
        logo={false}
        layout="mix"
        fixSiderbar
        location={{ pathname: location.pathname }}
        route={route}
        menuItemRender={(item, dom) => (
          <a onClick={() => item.path && navigate(item.path)}>{dom}</a>
        )}
        rightContentRender={() => (
          <Space>
            <Tag color="default">Phase 0 — skeleton</Tag>
            <a aria-label="Settings" onClick={() => setSettingsOpen(true)}>
              <SettingOutlined />
            </a>
          </Space>
        )}
        footerRender={() => (
          <div style={{ textAlign: "center", padding: 8, opacity: 0.65 }}>
            cwd {serverVersion?.version ?? "…"} ·{" "}
            UI default: <code>{uiConfig?.defaultTheme ?? "…"}</code> ·{" "}
            <a href="/api/version">version</a> · <a href="/api/uiconfig">uiconfig</a>
          </div>
        )}
      >
        <Routes>
          <Route path="/"        element={<Overview />} />
          <Route path="/hazards" element={<Hazards />} />
          <Route path="/space"   element={<SpaceWeather />} />
          <Route path="/events"  element={<Events />} />
          <Route path="/history" element={<History />} />
        </Routes>

        <SettingsDrawer
          open={settingsOpen}
          onClose={() => setSettingsOpen(false)}
          themeMode={themeMode}
          onThemeChange={onThemeChange}
        />
      </ProLayout>
    </ConfigProvider>
  );
}
```

- [ ] **Step 14.4: First frontend build**

```bash
cd web
pnpm build
```

Expected: TypeScript passes (`tsc`), Vite emits to `../internal/webdist/dist/`. Verify:

```bash
ls ../internal/webdist/dist/
# should include index.html and assets/
```

- [ ] **Step 14.5: Commit**

```bash
cd ..
git add web/src/App.tsx web/src/components/ web/src/pages/
# Built artifacts under internal/webdist/dist/ are gitignored except .gitkeep.
git status
git commit -m "feat(web): ProLayout + 5 routed placeholder pages + settings drawer"
```

---

### Task 15: Frontend Makefile targets + verify embed pipeline end-to-end

**Files:**
- Modify: `Makefile`

- [ ] **Step 15.1: Append frontend targets to `Makefile`**

```makefile

# ---- Frontend / embed pipeline ----

WEB_DIR     := web
PNPM        ?= pnpm

.PHONY: web-install build-web build-all dev-web

web-install: ## Install frontend dependencies (pnpm install)
	cd $(WEB_DIR) && $(PNPM) install --frozen-lockfile

build-web: ## Build the SPA into internal/webdist/dist/
	cd $(WEB_DIR) && $(PNPM) build

build-all: build-web build ## Build frontend then backend (single-binary release shape)

dev-web: ## Run Vite dev server (proxies /api to localhost:8765)
	cd $(WEB_DIR) && $(PNPM) dev
```

- [ ] **Step 15.2: Run the full pipeline cleanly**

```bash
make clean
rm -rf internal/webdist/dist/*
touch internal/webdist/dist/.gitkeep   # restore, gitignored
make build-all
ls -lh bin/cwd
```

Expected: clean build → `bin/cwd` exists, ~15+ MB.

- [ ] **Step 15.3: End-to-end smoke**

```bash
./bin/cwd serve &
SERVER_PID=$!
sleep 1
curl -sS -o /dev/null -w "/healthz=%{http_code}\n"   http://127.0.0.1:8765/healthz
curl -sS -o /dev/null -w "/readyz=%{http_code}\n"    http://127.0.0.1:8765/readyz
curl -sS -o /dev/null -w "/api/version=%{http_code}\n" http://127.0.0.1:8765/api/version
curl -sS -o /dev/null -w "/api/uiconfig=%{http_code}\n" http://127.0.0.1:8765/api/uiconfig
curl -sS -o /tmp/cwd-spa.html -w "/=%{http_code} bytes=%{size_download}\n" http://127.0.0.1:8765/
grep -q "<div id=\"root\"></div>" /tmp/cwd-spa.html && echo "SPA root div: OK"
kill $SERVER_PID
wait $SERVER_PID 2>/dev/null
rm -f /tmp/cwd-spa.html
```

Expected: all five responses are 200; SPA HTML contains the root div.

- [ ] **Step 15.4: Commit**

```bash
git add Makefile
git commit -m "build: frontend Makefile targets (web-install, build-web, build-all, dev-web)"
```

---

### Task 16: GitHub Actions CI (lint + test + build)

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 16.1: Write `.github/workflows/ci.yml`**

```yaml
name: ci

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

permissions:
  contents: read

jobs:
  backend:
    name: Go (lint + test + build)
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: "1.24"
          cache: true

      - name: Lint
        uses: golangci/golangci-lint-action@v6
        with:
          version: v1.60.3
          args: --timeout=5m

      - name: Test
        run: go test -race -coverprofile=coverage.out ./...

      - name: Build (with embed placeholder; frontend tested in `web` job)
        run: go build -o /tmp/cwd ./cmd/cwd

  web:
    name: Web (typecheck + build)
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Node
        uses: actions/setup-node@v4
        with:
          node-version: "22"

      - uses: pnpm/action-setup@v4
        with:
          version: 9.12.2

      - name: Install
        working-directory: web
        run: pnpm install --frozen-lockfile

      - name: Typecheck
        working-directory: web
        run: pnpm typecheck

      - name: Build
        working-directory: web
        run: pnpm build

      - name: Verify embed output exists
        run: test -f internal/webdist/dist/index.html

  build-all:
    name: Single-binary build (web + go)
    runs-on: ubuntu-latest
    needs: [backend, web]
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: "1.24"
          cache: true

      - name: Set up Node
        uses: actions/setup-node@v4
        with:
          node-version: "22"

      - uses: pnpm/action-setup@v4
        with:
          version: 9.12.2

      - name: Install web deps
        working-directory: web
        run: pnpm install --frozen-lockfile

      - name: Build web → embed dir
        working-directory: web
        run: pnpm build

      - name: Build cwd binary
        run: make build

      - name: Smoke
        run: |
          ./bin/cwd serve &
          PID=$!
          sleep 1
          curl -sSf http://127.0.0.1:8765/healthz
          curl -sSf http://127.0.0.1:8765/api/version
          curl -sSf http://127.0.0.1:8765/api/uiconfig
          kill $PID
          wait $PID 2>/dev/null || true
```

- [ ] **Step 16.2: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: github actions for lint, test, frontend build, single-binary smoke"
```

> **GitLab CI alternative:** if the operator chose GitLab in pre-flight, swap the file for `.gitlab-ci.yml` with three stages (`backend`, `web`, `build-all`) running the same shell commands as the GitHub workflow above. The semantics are identical; the YAML format differs.

---

### Task 17: Final verification + README touch-up

**Files:**
- Modify: `README.md` (optional — only if anything in the quick-start drifted from reality)

- [ ] **Step 17.1: Run the full verification matrix locally**

```bash
make clean
rm -rf internal/webdist/dist/*
touch internal/webdist/dist/.gitkeep
make web-install
make lint
make test
make build-all
./bin/cwd version
./bin/cwd serve &
SERVER_PID=$!
sleep 1
for path in /healthz /readyz /api/version /api/uiconfig /; do
  printf "%-15s " "$path"
  curl -sS -o /dev/null -w "%{http_code}\n" "http://127.0.0.1:8765$path"
done
kill $SERVER_PID
wait $SERVER_PID 2>/dev/null
```

Expected output (paths and 200s):

```
/healthz        200
/readyz         200
/api/version    200
/api/uiconfig   200
/               200
```

- [ ] **Step 17.2: Open `http://127.0.0.1:8765` in a real browser (manual)**

Using the existing Playwright MCP install or just a local browser:
- ProLayout renders with sidebar, header, footer.
- Sidebar lists Overview / Hazards / Space Weather / Events / History.
- Default theme is dark.
- Settings gear opens a Drawer with a 3-button theme switcher (Dark / Light / Auto).
- Footer shows version (`cwd dev`) and `UI default: dark`.
- Switching theme persists across reload (localStorage).
- Clicking each sidebar item swaps in an `Empty` placeholder — no console errors.

- [ ] **Step 17.3: README touch-up if anything diverged**

If the actual flags, paths, or output differ from the README quick-start, edit `README.md` to match reality and commit:

```bash
git add README.md
git commit -m "docs: README quick-start aligned with shipped Phase 0 behavior"
```

If the README is already accurate, skip the commit.

- [ ] **Step 17.4: Tag & summary commit**

No `git tag` until Phase 6 (release artifacts), but mark Phase 0 done with a final commit if anything trailing remains. Otherwise `git log --oneline` shows the full Phase 0 history; subagent-driven-development will move into the comprehensive code review automatically.

---

## Self-review

**1. Spec coverage** (mapped to design sections):

| Design section | Phase 0 covers | Task |
|---|---|---|
| §3 Architecture (HTTP API surface only — no fetchers/cache/store/SSE/imgproxy yet) | ✅ | 6, 7, 8, 9, 10 |
| §4 Package layout | ✅ skeleton (no `sources/`, `fetcher/`, `cache/`, `store/`, `imgproxy/`, `sse/` yet — those come in later phases) | 5, 6, 7, 8, 9, 10, 12, 13, 14 |
| §5.4 HTTP API: `/healthz`, `/readyz`, `/api/version`, `/api/uiconfig`, `/` (SPA) | ✅ | 6, 7, 8 |
| §5.4 HTTP API: `/api/snapshot`, `/api/history*`, `/api/stream`, `/img/*`, `/api/sources` | deferred (Phases 1–4) | — |
| §6.1 ProLayout, ConfigProvider, dark default | ✅ | 13, 14 |
| §6.2 Five page routes + Settings drawer | ✅ skeleton (placeholder pages) | 14 |
| §6.3 Reusable components | deferred (Phases 1–5 — they need real data) | — |
| §7 PWA | deferred (Phase 5) | — |
| §8 Configuration | ✅ full schema parsed + defaulted + env-overridden + validated | 3 |
| §9 Observability: slog, /api/sources, /healthz, /readyz, /api/version | ✅ slog + healthz + readyz + version (no /api/sources yet — needs sources package) | 3, 6, 10 |
| §10 Testing | ✅ unit-test pattern established for Go side; frontend Vitest deferred until there's component logic worth testing | 3, 6, 7, 8, 9 |
| §11 Phase 0 demo: `cwd serve` → AntD Pro shell | ✅ end-to-end verified | 15, 17 |
| §12 Risks: User-Agent contact warning, embed.FS+frontend ordering, AntD pin | ✅ all addressed | 3, 5, 10, 12 |
| §13 Open items: module path, license, CI, AntD pin | ✅ defaults chosen, override path documented | Pre-flight + 1 + 2 + 12 + 16 |

No gaps for Phase 0 scope. All deferred items are explicitly out of Phase 0 per design §11.

**2. Placeholder scan:** No "TBD"/"TODO"/"implement later"/"add appropriate X". Every code step shows the actual code. Every command step shows the exact command and expected output.

**3. Type/symbol consistency:**
- Module path used consistently as `github.com/acaudill/cwd` across `go mod init`, all imports in tests, Makefile `PKG`, ldflags, CI.
- `config.Config` field names (`Server.Bind`, `UI.DefaultTheme`, etc.) match between definition (Task 3), tests (Task 3), `UIConfig` handler (Task 7), `NewRouter` signature (Task 8), `server.Run` signature (Task 9), and `cmd/cwd/main.go` (Task 10).
- `version.Info` shape matches between `internal/version/version.go` (Task 4), `Version()` handler (Task 6), and the frontend `VersionInfo` interface (Task 13).
- UIConfig JSON keys (`defaultTheme`, `defaultLanding`, `enableHistory`) match between Go handler (Task 7), Go test (Task 7), and frontend type (Task 13).
- Theme storage key `"cwd.themeMode"` matches between `theme.ts` (Task 13) and `App.tsx` (Task 14).
- Frontend route paths (`/`, `/hazards`, `/space`, `/events`, `/history`) match between `App.tsx` route table and `<Routes>` (Task 14).

No drift detected.

---

## Execution handoff

Plan complete and saved to `docs/plans/2026-05-01-self-hosted-cwd-phase0-skeleton-implementation.md`. Two execution options:

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration. Matches the user's `rules/plan-workflow.md` and `rules/subagent-development-workflow.md`.

**2. Inline Execution** — I execute tasks in this session using `superpowers:executing-plans`, batch execution with checkpoints for review.

**Which approach?**
