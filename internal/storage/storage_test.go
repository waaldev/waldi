package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalStoreOpenUpload(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "amin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "amin", "a.webp"), []byte("img"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "secret"), []byte("no"), 0o644); err != nil {
		t.Fatal(err)
	}
	l := LocalStore{RootDir: root}

	rc, err := l.OpenUpload(context.Background(), "amin", "/static/uploads/amin/a.webp")
	if err != nil {
		t.Fatalf("OpenUpload: %v", err)
	}
	b, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(b) != "img" {
		t.Fatalf("got %q", b)
	}

	for _, u := range []string{
		"/static/uploads/other/a.webp",
		"/static/uploads/amin/../secret",
		"/static/uploads/amin/..",
		"/static/uploads/amin/missing.webp",
		"https://example.com/a.webp",
	} {
		if _, err := l.OpenUpload(context.Background(), "amin", u); !errors.Is(err, ErrNotFound) {
			t.Fatalf("%s: expected ErrNotFound, got %v", u, err)
		}
	}
}
