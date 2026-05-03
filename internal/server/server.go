// Package server owns the HTTP server lifecycle: bind, accept, graceful shutdown.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/jacaudi/cwd/internal/api"
	"github.com/jacaudi/cwd/internal/cache"
	"github.com/jacaudi/cwd/internal/config"
	"github.com/jacaudi/cwd/internal/fetcher"
	"github.com/jacaudi/cwd/internal/sources"
	"github.com/jacaudi/cwd/internal/sse"
	"github.com/jacaudi/cwd/internal/store"
	"github.com/jacaudi/cwd/internal/version"
)

// nwsAlertsURL returns the upstream URL for the NWS active alerts API.
// CWD_NWS_ALERTS_URL overrides the default for tests; never promote this to a
// user-facing config field.
var nwsAlertsURL = func() string {
	if v := os.Getenv("CWD_NWS_ALERTS_URL"); v != "" {
		return v
	}
	return "https://api.weather.gov/alerts/active"
}

// swpcScalesURL returns the upstream URL for the SWPC NOAA scales product.
// CWD_SWPC_SCALES_URL overrides the default for tests; never promote this
// to a user-facing config field.
var swpcScalesURL = func() string {
	if v := os.Getenv("CWD_SWPC_SCALES_URL"); v != "" {
		return v
	}
	return "https://services.swpc.noaa.gov/products/noaa-scales.json"
}

// swpcAlertsURL returns the upstream URL for the SWPC alerts product.
// CWD_SWPC_ALERTS_URL overrides the default for tests; never promote this
// to a user-facing config field.
var swpcAlertsURL = func() string {
	if v := os.Getenv("CWD_SWPC_ALERTS_URL"); v != "" {
		return v
	}
	return "https://services.swpc.noaa.gov/products/alerts.json"
}

// usgsQuakesURL returns the upstream URL for the USGS significant-day quake
// feed. CWD_USGS_QUAKES_URL overrides the default for tests; never promote
// this to a user-facing config field.
var usgsQuakesURL = func() string {
	if v := os.Getenv("CWD_USGS_QUAKES_URL"); v != "" {
		return v
	}
	return "https://earthquake.usgs.gov/earthquakes/feed/v1.0/summary/significant_day.geojson"
}

// usgsVolcanoesURL returns the upstream URL for the USGS elevated-volcano
// feed. CWD_USGS_VOLCANOES_URL overrides the default for tests; never
// promote this to a user-facing config field.
var usgsVolcanoesURL = func() string {
	if v := os.Getenv("CWD_USGS_VOLCANOES_URL"); v != "" {
		return v
	}
	return "https://volcanoes.usgs.gov/hans-public/api/volcano/getElevatedVolcanoes"
}

// healthFn is an adapter that turns a func into an api.HealthProvider.
type healthFn func() map[string]fetcher.Health

func (h healthFn) Health() map[string]fetcher.Health { return h() }

// Run binds the configured address and serves until ctx is canceled.
// If addrCh is non-nil, the actual listen address (after :0 resolution) is sent on it.
func Run(ctx context.Context, cfg *config.Config, logger *slog.Logger, addrCh chan<- string) error {
	// 1. Open + migrate the store.
	st, err := store.Open(cfg.Store.Path)
	if err != nil {
		return fmt.Errorf("server: open store: %w", err)
	}
	defer func() {
		if cerr := st.Close(); cerr != nil {
			logger.Warn("server.store_close", "err", cerr.Error())
		}
	}()
	if err := st.Migrate(ctx); err != nil {
		return fmt.Errorf("server: migrate: %w", err)
	}

	// 2. Build cache.
	c := cache.New()

	// 3. Build the User-Agent string.
	contact := cfg.Server.Contact
	if contact == "" {
		contact = "no-contact-configured"
	}
	userAgent := "cwd-self-host/" + version.Version + " (" + contact + ")"

	// 4. Build sources from config.
	srcs := map[string]sources.Source{}
	if scfg, ok := cfg.Sources["nws_alerts"]; ok && scfg.IsEnabled() {
		na := sources.NewNWSAlerts(nwsAlertsURL(), userAgent, logger)
		if scfg.Interval > 0 {
			na.SetInterval(scfg.Interval)
		}
		srcs[sources.NWSAlertsName] = na
	}
	if scfg, ok := cfg.Sources["swpc_scales"]; ok && scfg.IsEnabled() {
		s := sources.NewSWPCScales(swpcScalesURL(), userAgent, logger)
		if scfg.Interval > 0 {
			s.SetInterval(scfg.Interval)
		}
		srcs[sources.SWPCScalesName] = s
	}
	if scfg, ok := cfg.Sources["swpc_alerts"]; ok && scfg.IsEnabled() {
		window := time.Duration(cfg.Derived.Thresholds.SWPCAlertWindowHours) * time.Hour
		s := sources.NewSWPCAlerts(swpcAlertsURL(), userAgent, cfg.Derived.Thresholds.SWPCAlertProducts, window, logger)
		if scfg.Interval > 0 {
			s.SetInterval(scfg.Interval)
		}
		srcs[sources.SWPCAlertsName] = s
	}
	if scfg, ok := cfg.Sources["usgs_quakes"]; ok && scfg.IsEnabled() {
		s := sources.NewUSGSQuakes(usgsQuakesURL(), userAgent, logger)
		if scfg.Interval > 0 {
			s.SetInterval(scfg.Interval)
		}
		srcs[sources.USGSQuakesName] = s
	}
	if scfg, ok := cfg.Sources["usgs_volcanoes"]; ok && scfg.IsEnabled() {
		s := sources.NewUSGSVolcanoes(usgsVolcanoesURL(), userAgent, logger)
		if scfg.Interval > 0 {
			s.SetInterval(scfg.Interval)
		}
		srcs[sources.USGSVolcanoesName] = s
	}

	// 5. Hot-start cache from store for each enabled source.
	for name := range srcs {
		row, ok, err := st.Latest(ctx, name)
		if err != nil {
			logger.Warn("server.hotstart", "source", name, "err", err.Error())
			continue
		}
		if !ok {
			continue
		}
		payload, err := hotStartDecode(name, row.Payload)
		if err != nil {
			logger.Warn("server.hotstart_decode", "source", name, "err", err.Error())
			continue
		}
		c.Set(cache.Envelope{Source: name, FetchedAt: row.FetchedAt, Validator: row.Validator, Payload: payload})
	}

	// 6. Build fetchers.
	fetchers := map[string]*fetcher.Fetcher{}
	for name, src := range srcs {
		fetchers[name] = fetcher.New(src, c, st, fetcher.WithLogger(logger))
	}

	// 7. Build region filter.
	filter := sources.NewFilter(
		cfg.Derived.Thresholds.RegionFilter.UGCs,
		cfg.Derived.Thresholds.RegionFilter.WFOs,
	)

	// 8. Build SSE hub (with filter so SSE emit applies the same predicate as
	// /api/snapshot and /api/history — see design §2 Q3, §6.5, §11).
	enabledNames := make([]string, 0, len(srcs))
	for n := range srcs {
		enabledNames = append(enabledNames, n)
	}
	hub := sse.NewHub(c, enabledNames, sse.WithFilter(filter))

	// 9. Build /readyz closure — green only if all enabled sources have at least one success.
	// Empty fetchers map (no sources enabled) → ready immediately.
	ready := func() bool {
		for _, f := range fetchers {
			if f.Health().LastSuccess.IsZero() {
				return false
			}
		}
		return true
	}

	// 10. Build HealthProvider for /api/sources.
	healthProvider := healthFn(func() map[string]fetcher.Health {
		out := make(map[string]fetcher.Health, len(fetchers))
		for n, f := range fetchers {
			out[n] = f.Health()
		}
		return out
	})

	// 11. Build router with all deps wired.
	handler := api.NewRouter(api.RouterDeps{
		Config:          cfg,
		Ready:           ready,
		SourcesHandler:  api.NewSourcesHandler(healthProvider),
		SnapshotHandler: api.NewSnapshotHandler(c, filter),
		HistoryHandler:  api.NewHistoryHandler(st, filter),
		StreamHandler:   api.NewStreamHandler(hub),
	})

	ln, err := net.Listen("tcp", cfg.Server.Bind)
	if err != nil {
		return err
	}
	if addrCh != nil {
		addrCh <- ln.Addr().String()
	}

	// 12. Spawn fetchers + hub + prune ticker before serving.
	for _, f := range fetchers {
		go f.Run(ctx)
	}
	go hub.Run(ctx)
	go runPrune(ctx, st, cfg, logger)

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

	// 13. Wait for shutdown or serve error.
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

// runPrune fires immediately and then every 6 hours, deleting rows older than the
// configured retention window.
func runPrune(ctx context.Context, st *store.Store, cfg *config.Config, logger *slog.Logger) {
	retention := time.Duration(cfg.Store.RetentionDays) * 24 * time.Hour
	prune := func() {
		if _, err := st.Prune(ctx, retention); err != nil {
			logger.Warn("server.prune", "err", err.Error())
		}
	}
	prune()
	tick := time.NewTicker(6 * time.Hour)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			prune()
		}
	}
}
