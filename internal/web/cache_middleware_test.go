package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCacheHeadersSeparateBrowserAndCDNLifetimes(t *testing.T) {
	s := &Server{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	s.withCacheHeaders(w, r, renderCachedTestPage)

	if got := w.Header().Get("Cache-Control"); got != publicCacheControl {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := w.Header().Get("ETag"); got == "" {
		t.Fatal("missing ETag")
	}
}

func TestCacheHeadersReturnNotModified(t *testing.T) {
	s := &Server{}
	firstRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	first := httptest.NewRecorder()
	s.withCacheHeaders(first, firstRequest, renderCachedTestPage)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("If-None-Match", first.Header().Get("ETag"))
	w := httptest.NewRecorder()
	s.withCacheHeaders(w, r, renderCachedTestPage)

	if w.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotModified)
	}
	if w.Body.Len() != 0 {
		t.Fatalf("304 response has %d body bytes", w.Body.Len())
	}
}

func renderCachedTestPage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte("<p>cached</p>"))
}
