// Package server owns the HTTP server lifecycle: bind, accept, graceful shutdown.
package server

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/jacaudi/cwd/internal/api"
	"github.com/jacaudi/cwd/internal/config"
)

// Run binds the configured address and serves until ctx is canceled.
// If addrCh is non-nil, the actual listen address (after :0 resolution) is sent on it.
func Run(ctx context.Context, cfg *config.Config, logger *slog.Logger, addrCh chan<- string) error {
	ready := func() bool { return true } // Phase 0: nothing to wait on; Phase 1+ wires real readiness.

	handler := api.NewRouter(api.RouterDeps{Config: cfg, Ready: ready})

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
