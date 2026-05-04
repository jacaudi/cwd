package imageproxy

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestRegistry_NoDuplicateKeys(t *testing.T) {
	seen := map[string]struct{}{}
	for _, img := range Registry {
		k := img.Key()
		if _, dup := seen[k]; dup {
			t.Errorf("duplicate registry key: %q", k)
		}
		seen[k] = struct{}{}
	}
}

func TestRegistry_ExpectedKeyCount(t *testing.T) {
	const want = 20
	if got := len(Registry); got != want {
		t.Errorf("Registry size = %d, want %d", got, want)
	}
}

func TestRegistry_EveryEntryWellFormed(t *testing.T) {
	allowedMIME := map[string]struct{}{
		"image/png": {}, "image/gif": {}, "image/jpeg": {},
	}
	for _, img := range Registry {
		t.Run(img.Key(), func(t *testing.T) {
			if img.Source == "" {
				t.Errorf("Source empty")
			}
			if img.Name == "" {
				t.Errorf("Name empty")
			}
			if strings.ContainsAny(img.Source, "./") {
				t.Errorf("Source %q must not contain '.' or '/'", img.Source)
			}
			if strings.ContainsAny(img.Name, "./") {
				t.Errorf("Name %q must not contain '.' or '/'", img.Name)
			}
			u, err := url.Parse(img.URL)
			if err != nil {
				t.Errorf("URL parse: %v", err)
			} else if u.Scheme != "https" {
				t.Errorf("URL scheme = %q, want https", u.Scheme)
			}
			if _, ok := allowedMIME[img.MIME]; !ok {
				t.Errorf("MIME %q not in {image/png,image/gif,image/jpeg}", img.MIME)
			}
			if img.DefaultInterval < 60*time.Second {
				t.Errorf("DefaultInterval = %s, must be >= 60s (politeness floor)", img.DefaultInterval)
			}
			if strings.TrimSpace(img.Description) == "" {
				t.Errorf("Description empty")
			}
		})
	}
}

func TestImage_KeyAndPath(t *testing.T) {
	img := Image{Source: "spc", Name: "day1otlk"}
	if got := img.Key(); got != "spc.day1otlk" {
		t.Errorf("Key() = %q, want spc.day1otlk", got)
	}
	if got := img.Path(); got != "/img/spc/day1otlk" {
		t.Errorf("Path() = %q, want /img/spc/day1otlk", got)
	}
}

func TestRegistry_ByKey_LookupHits(t *testing.T) {
	want := []string{
		"spc.day1otlk", "spc.day2otlk", "spc.day3otlk",
		"spc.day1otlk_fire", "spc.day2otlk_fire", "spc.day38otlk_fire",
		"wpc.ero_day1", "wpc.ero_day2", "wpc.ero_day3",
		"wpc.wssi_day1", "wpc.wssi_day2", "wpc.wssi_day3",
		"wpc.heatrisk_day1", "wpc.heatrisk_day2", "wpc.heatrisk_day3",
		"nhc.atl_7d", "nhc.epac_7d", "nhc.cpac_7d",
		"navy.jtwc_abpw",
		"nwc.fho_national",
	}
	byKey := ByKey()
	for _, k := range want {
		if _, ok := byKey[k]; !ok {
			t.Errorf("ByKey() missing %q", k)
		}
	}
	if got := len(byKey); got != len(want) {
		t.Errorf("ByKey() size = %d, want %d", got, len(want))
	}
}

func TestKey_Constants_MatchRegistry(t *testing.T) {
	pairs := []struct {
		name  string
		value string
	}{
		{"KeySPCDay1Otlk", KeySPCDay1Otlk},
		{"KeySPCDay2Otlk", KeySPCDay2Otlk},
		{"KeySPCDay3Otlk", KeySPCDay3Otlk},
		{"KeyNHCAtl7D", KeyNHCAtl7D},
	}
	byKey := ByKey()
	for _, p := range pairs {
		if _, ok := byKey[p.value]; !ok {
			t.Errorf("%s = %q not in registry", p.name, p.value)
		}
	}
}
