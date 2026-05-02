package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jacaudi/cwd/internal/config"
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
