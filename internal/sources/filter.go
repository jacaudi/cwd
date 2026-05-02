package sources

// Filter is the per-operator region filter from design §6.5.
// Empty UGCs AND empty WFOs ⇒ match-all.
// Non-empty: UGC list-intersection OR WFO equality (union semantics).
type Filter struct {
	ugcs map[string]struct{}
	wfos map[string]struct{}
}

// NewFilter constructs a Filter from the given UGC and WFO allowlists.
// Empty strings are skipped; nil/empty slices yield a match-all filter.
func NewFilter(ugcs, wfos []string) Filter {
	f := Filter{ugcs: map[string]struct{}{}, wfos: map[string]struct{}{}}
	for _, u := range ugcs {
		if u != "" {
			f.ugcs[u] = struct{}{}
		}
	}
	for _, w := range wfos {
		if w != "" {
			f.wfos[w] = struct{}{}
		}
	}
	return f
}

// Match returns true iff the alert passes the filter.
func (f Filter) Match(a Alert) bool {
	if len(f.ugcs) == 0 && len(f.wfos) == 0 {
		return true
	}
	for _, u := range a.UGCs {
		if _, ok := f.ugcs[u]; ok {
			return true
		}
	}
	if a.WFO != "" {
		if _, ok := f.wfos[a.WFO]; ok {
			return true
		}
	}
	return false
}

// Apply returns a new slice of alerts that pass the filter.
// Stable order; never returns nil for non-nil input.
func (f Filter) Apply(in []Alert) []Alert {
	if len(f.ugcs) == 0 && len(f.wfos) == 0 {
		out := make([]Alert, len(in))
		copy(out, in)
		return out
	}
	out := make([]Alert, 0, len(in))
	for _, a := range in {
		if f.Match(a) {
			out = append(out, a)
		}
	}
	return out
}
