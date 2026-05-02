package sources

import "testing"

func mkAlert(ugcs []string, wfo string) Alert {
	return Alert{UGCs: ugcs, WFO: wfo}
}

func TestFilter_EmptyMatchesAll(t *testing.T) {
	f := NewFilter(nil, nil)
	if !f.Match(mkAlert(nil, "")) {
		t.Errorf("empty filter should match alert with no fields")
	}
	if !f.Match(mkAlert([]string{"VAC059"}, "LWX")) {
		t.Errorf("empty filter should match populated alert")
	}
}

func TestFilter_UGCMatch(t *testing.T) {
	f := NewFilter([]string{"VAC059"}, nil)
	if !f.Match(mkAlert([]string{"VAC059", "MDC031"}, "")) {
		t.Errorf("UGC list intersection should match")
	}
	if f.Match(mkAlert([]string{"CAZ505"}, "")) {
		t.Errorf("non-overlapping UGCs should not match")
	}
}

func TestFilter_WFOMatch(t *testing.T) {
	f := NewFilter(nil, []string{"LWX"})
	if !f.Match(mkAlert(nil, "LWX")) {
		t.Errorf("WFO match expected")
	}
	if f.Match(mkAlert(nil, "OAX")) {
		t.Errorf("non-matching WFO should not match")
	}
	if f.Match(mkAlert(nil, "")) {
		t.Errorf("empty WFO with non-empty filter should not match")
	}
}

func TestFilter_UnionSemantics(t *testing.T) {
	f := NewFilter([]string{"VAC059"}, []string{"OAX"})
	if !f.Match(mkAlert([]string{"VAC059"}, "")) {
		t.Errorf("UGC-only match should pass under union")
	}
	if !f.Match(mkAlert(nil, "OAX")) {
		t.Errorf("WFO-only match should pass under union")
	}
	if f.Match(mkAlert([]string{"CAZ505"}, "BOX")) {
		t.Errorf("no-match in either should fail")
	}
}

func TestFilter_Apply(t *testing.T) {
	f := NewFilter([]string{"VAC059"}, nil)
	in := []Alert{
		mkAlert([]string{"VAC059"}, ""),
		mkAlert([]string{"CAZ505"}, ""),
		mkAlert([]string{"VAC059", "MDC031"}, ""),
	}
	got := f.Apply(in)
	if len(got) != 2 {
		t.Fatalf("want 2 matching alerts, got %d", len(got))
	}
}
