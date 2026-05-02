package sources

import (
	"context"
	"time"
)

// FetchResult holds the output of a single Source.Fetch call.
// Validator is a content-derived string (e.g. "sha256:<hex>") used for
// change detection — equal Validators mean the payload has not changed.
type FetchResult struct {
	Payload   any
	Validator string
}

// Source is the interface implemented by all polling data sources.
type Source interface {
	Name() string
	Interval() time.Duration
	Fetch(ctx context.Context) (FetchResult, error)
}
