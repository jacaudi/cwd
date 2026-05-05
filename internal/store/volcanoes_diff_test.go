package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// payload helper — match the shape internal/sources/usgs_volcanoes uses.
type vEntry struct {
	Name        string `json:"name"`
	Observatory string `json:"observatory"`
	AlertLevel  string `json:"alertLevel"`
}

func vPayload(t *testing.T, entries ...vEntry) []byte {
	t.Helper()
	b, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestStore_VolcanoStateChanges_DetectsAppearance(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
	_ = s.Append(ctx, "usgs_volcanoes", t0, "v1", vPayload(t))
	_ = s.Append(ctx, "usgs_volcanoes", t0.Add(time.Minute), "v2", vPayload(t,
		vEntry{Name: "Great Sitkin", Observatory: "AVO", AlertLevel: "WATCH"}))

	changes, err := s.VolcanoStateChanges(ctx, t0.Add(-time.Hour), t0.Add(time.Hour), 50)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(changes); got != 1 {
		t.Fatalf("len(changes) = %d, want 1", got)
	}
	c := changes[0]
	if c.Volcano != "AVO Great Sitkin" {
		t.Errorf("Volcano = %q, want %q", c.Volcano, "AVO Great Sitkin")
	}
	if c.Prior != "" {
		t.Errorf("Prior = %q, want empty (newly appeared)", c.Prior)
	}
	if c.Current != "WATCH" {
		t.Errorf("Current = %q, want WATCH", c.Current)
	}
}

func TestStore_VolcanoStateChanges_DetectsLevelChange(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
	_ = s.Append(ctx, "usgs_volcanoes", t0, "v1", vPayload(t,
		vEntry{Name: "Kilauea", Observatory: "HVO", AlertLevel: "ADVISORY"}))
	_ = s.Append(ctx, "usgs_volcanoes", t0.Add(time.Minute), "v2", vPayload(t,
		vEntry{Name: "Kilauea", Observatory: "HVO", AlertLevel: "WATCH"}))

	changes, err := s.VolcanoStateChanges(ctx, t0.Add(-time.Hour), t0.Add(time.Hour), 50)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(changes); got != 1 {
		t.Fatalf("len(changes) = %d, want 1", got)
	}
	c := changes[0]
	if c.Prior != "ADVISORY" || c.Current != "WATCH" {
		t.Errorf("change = %s→%s, want ADVISORY→WATCH", c.Prior, c.Current)
	}
}

func TestStore_VolcanoStateChanges_DetectsDisappearance(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
	_ = s.Append(ctx, "usgs_volcanoes", t0, "v1", vPayload(t,
		vEntry{Name: "Kilauea", Observatory: "HVO", AlertLevel: "WATCH"}))
	_ = s.Append(ctx, "usgs_volcanoes", t0.Add(time.Minute), "v2", vPayload(t))

	changes, err := s.VolcanoStateChanges(ctx, t0.Add(-time.Hour), t0.Add(time.Hour), 50)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(changes); got != 1 {
		t.Fatalf("len(changes) = %d, want 1", got)
	}
	c := changes[0]
	if c.Prior != "WATCH" || c.Current != "" {
		t.Errorf("change = %s→%s, want WATCH→<empty>", c.Prior, c.Current)
	}
}

func TestStore_VolcanoStateChanges_NoChangeNoEvent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
	v := vEntry{Name: "Kilauea", Observatory: "HVO", AlertLevel: "WATCH"}
	_ = s.Append(ctx, "usgs_volcanoes", t0, "v1", vPayload(t, v))
	_ = s.Append(ctx, "usgs_volcanoes", t0.Add(time.Minute), "v2", vPayload(t, v))

	changes, err := s.VolcanoStateChanges(ctx, t0.Add(-time.Hour), t0.Add(time.Hour), 50)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(changes); got != 0 {
		t.Errorf("len(changes) = %d, want 0", got)
	}
}

func TestStore_VolcanoStateChanges_HonorsMaxEvents(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
	// 5 distinct volcanoes appear in succession.
	for i := 0; i < 5; i++ {
		entries := make([]vEntry, 0, i+1)
		for j := 0; j <= i; j++ {
			entries = append(entries, vEntry{
				Name: "V" + string(rune('A'+j)), Observatory: "X", AlertLevel: "ADVISORY",
			})
		}
		_ = s.Append(ctx, "usgs_volcanoes", t0.Add(time.Duration(i)*time.Minute),
			"v"+string(rune('a'+i)), vPayload(t, entries...))
	}

	changes, err := s.VolcanoStateChanges(ctx, t0.Add(-time.Hour), t0.Add(time.Hour), 3)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(changes); got != 3 {
		t.Errorf("len(changes) = %d, want 3 (cap respected)", got)
	}
}

func TestStore_VolcanoStateChanges_AscendingOrder(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
	_ = s.Append(ctx, "usgs_volcanoes", t0, "v1", vPayload(t))
	_ = s.Append(ctx, "usgs_volcanoes", t0.Add(time.Minute), "v2", vPayload(t,
		vEntry{Name: "A", Observatory: "X", AlertLevel: "WATCH"}))
	_ = s.Append(ctx, "usgs_volcanoes", t0.Add(2*time.Minute), "v3", vPayload(t,
		vEntry{Name: "A", Observatory: "X", AlertLevel: "WATCH"},
		vEntry{Name: "B", Observatory: "X", AlertLevel: "WARNING"}))

	changes, err := s.VolcanoStateChanges(ctx, t0.Add(-time.Hour), t0.Add(time.Hour), 50)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(changes); got != 2 {
		t.Fatalf("len(changes) = %d, want 2", got)
	}
	if !changes[0].At.Before(changes[1].At) {
		t.Errorf("changes not in ascending order: %s then %s", changes[0].At, changes[1].At)
	}
}
