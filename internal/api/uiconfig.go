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
