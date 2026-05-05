package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// VolcanoStateChange captures one volcano transition between two snapshots.
type VolcanoStateChange struct {
	At      time.Time // fetched_at of the snapshot in which the change first appears
	Volcano string    // observatory + " " + name (e.g. "AVO Great Sitkin")
	Prior   string    // alert level before, "" if newly elevated
	Current string    // alert level after, "" if removed from elevated list
}

// volcanoEntry mirrors the minimum fields the api/store layers need from
// internal/sources/usgs_volcanoes.go. Kept private to the store so a sources-side
// schema rename doesn't require store changes.
type volcanoEntry struct {
	Name        string `json:"name"`
	Observatory string `json:"observatory"`
	AlertLevel  string `json:"alertLevel"`
}

// VolcanoStateChanges walks adjacent snapshot pairs in [from, to] and returns
// every transition in ascending time order. Capped at maxEvents (caller passes
// 50 in production).
func (s *Store) VolcanoStateChanges(ctx context.Context, from, to time.Time, maxEvents int) ([]VolcanoStateChange, error) {
	if maxEvents <= 0 {
		return nil, fmt.Errorf("store: VolcanoStateChanges: maxEvents must be > 0, got %d", maxEvents)
	}
	rows, err := s.Range(ctx, "usgs_volcanoes", from, to)
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return nil, nil
	}

	// Decode each row's payload once.
	parsed := make([]map[string]string, len(rows)) // key → level
	for i, row := range rows {
		var entries []volcanoEntry
		if err := json.Unmarshal(row.Payload, &entries); err != nil {
			return nil, fmt.Errorf("store: VolcanoStateChanges: decode row %d: %w", i, err)
		}
		m := make(map[string]string, len(entries))
		for _, e := range entries {
			key := strings.TrimSpace(e.Observatory + " " + e.Name)
			m[key] = e.AlertLevel
		}
		parsed[i] = m
	}

	out := make([]VolcanoStateChange, 0, maxEvents)
	for i := 1; i < len(rows); i++ {
		prev, cur := parsed[i-1], parsed[i]
		// Appearances + level changes.
		for k, lvl := range cur {
			if priorLvl, had := prev[k]; !had {
				out = append(out, VolcanoStateChange{
					At: rows[i].FetchedAt, Volcano: k, Prior: "", Current: lvl,
				})
			} else if priorLvl != lvl {
				out = append(out, VolcanoStateChange{
					At: rows[i].FetchedAt, Volcano: k, Prior: priorLvl, Current: lvl,
				})
			}
			if len(out) >= maxEvents {
				return out[:maxEvents], nil
			}
		}
		// Disappearances.
		for k, lvl := range prev {
			if _, still := cur[k]; !still {
				out = append(out, VolcanoStateChange{
					At: rows[i].FetchedAt, Volcano: k, Prior: lvl, Current: "",
				})
			}
			if len(out) >= maxEvents {
				return out[:maxEvents], nil
			}
		}
	}
	return out, nil
}
