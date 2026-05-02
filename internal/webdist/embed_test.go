package webdist

import (
	"io"
	"strings"
	"testing"
)

func TestFSContainsIndexHTML(t *testing.T) {
	f, err := FS().Open("index.html")
	if err != nil {
		t.Fatalf("open index.html: %v", err)
	}
	defer f.Close()
	body, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(body), "<html") {
		t.Errorf("index.html does not contain <html, got %q", string(body)[:min(60, len(body))])
	}
}
