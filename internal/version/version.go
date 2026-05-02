// Package version exposes build-time identifiers, set via -ldflags.
package version

// These are overridden at build time, e.g.:
//
//	go build -ldflags="-X github.com/jacaudi/cwd/internal/version.Version=v0.1.0 \
//	  -X github.com/jacaudi/cwd/internal/version.Commit=$(git rev-parse --short HEAD) \
//	  -X github.com/jacaudi/cwd/internal/version.Date=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
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
