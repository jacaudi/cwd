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

	"github.com/jacaudi/cwd/internal/config"
	"github.com/jacaudi/cwd/internal/server"
	"github.com/jacaudi/cwd/internal/version"
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
