package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/jacaudi/cwd/internal/config"
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
