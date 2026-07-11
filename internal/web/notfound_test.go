package web

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func testServer(t *testing.T) *Server {
	t.Helper()
	root := moduleRoot(t)
	renderer, err := NewRenderer(os.DirFS(root))
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	return &Server{
		baseDomain:    "waldi.blog",
		logger:        testLogger(t),
		renderer:      renderer,
		customDomains: newCustomDomainCache(),
	}
}

func testLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestNotFoundRecorderSwallowsDefault404(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &notFoundRecorder{ResponseWriter: rec}
	http.NotFound(rw, httptest.NewRequest(http.MethodGet, "/missing", nil))

	if rw.status != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rw.status)
	}
	if rw.wrote {
		t.Fatal("expected default 404 body to be swallowed")
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %q", rec.Body.String())
	}
}

func TestRenderNotFoundPlatform(t *testing.T) {
	s := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/no-such-page", nil)
	req.Host = "waldi.blog"
	req.Header.Set("CF-IPCountry", "US")
	rec := httptest.NewRecorder()

	s.renderNotFound(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	body := rec.Body.String()
	for _, part := range []string{"no page here", "Go home", "waldi"} {
		if !strings.Contains(body, part) {
			t.Fatalf("body missing %q: %s", part, body)
		}
	}
}

func TestServeHTTPUnmatchedRouteRenders404Page(t *testing.T) {
	s, err := NewServer(Config{BaseDomain: "waldi.blog", Logger: testLogger(t)})
	if err != nil {
		root := moduleRoot(t)
		renderer, rerr := NewRenderer(os.DirFS(root))
		if rerr != nil {
			t.Fatalf("NewRenderer: %v", rerr)
		}
		s = &Server{
			baseDomain:    "waldi.blog",
			logger:        testLogger(t),
			renderer:      renderer,
			customDomains: newCustomDomainCache(),
			mux:           http.NewServeMux(),
		}
		s.routes()
	}
	req := httptest.NewRequest(http.MethodGet, "/unknown-post-slug", nil)
	req.Host = "waldi.blog"
	req.Header.Set("CF-IPCountry", "US")
	rec := httptest.NewRecorder()

	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "no page here") {
		t.Fatalf("expected styled 404 page, got: %s", rec.Body.String())
	}
}
