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
