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
