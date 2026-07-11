package web

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRandomUploadName(t *testing.T) {
	name, err := randomUploadName(".webp")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(name) != name {
		t.Fatalf("upload name contains path separators: %q", name)
	}
	if !strings.HasSuffix(name, ".webp") {
		t.Fatalf("upload name = %q, want .webp suffix", name)
	}
}
