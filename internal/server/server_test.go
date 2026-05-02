package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
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
