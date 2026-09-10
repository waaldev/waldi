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

func TestPublicBlogCacheHeadersIncludeSignedInReaders(t *testing.T) {
	s := &Server{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: "session-token"})
	r.AddCookie(&http.Cookie{Name: localeCookie, Value: "en"})
	w := httptest.NewRecorder()

	s.withPublicBlogCacheHeaders(w, r, renderCachedTestPage)

	if got := w.Header().Get("Cache-Control"); got != publicCacheControl {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := w.Header().Get("Vary"); got != "" {
		t.Fatalf("Vary = %q, want empty", got)
	}
}

func TestPublicBlogCacheHeadersStripCookies(t *testing.T) {
	s := &Server{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	w.Header().Add("Set-Cookie", "stale=; Max-Age=0")

	s.withPublicBlogCacheHeaders(w, r, func(w http.ResponseWriter, _ *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "reader", Value: "token"})
		_, _ = w.Write([]byte("<p>cached</p>"))
	})

	if got := w.Header().Values("Set-Cookie"); len(got) != 0 {
		t.Fatalf("Set-Cookie = %q, want none", got)
	}
}

func TestAppCacheHeadersBypassSignedInReaders(t *testing.T) {
	s := &Server{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: "session-token"})
	w := httptest.NewRecorder()

	s.withCacheHeaders(w, r, renderCachedTestPage)

	if got := w.Header().Get("Cache-Control"); got != "" {
		t.Fatalf("Cache-Control = %q, want empty", got)
	}
}

func renderCachedTestPage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte("<p>cached</p>"))
}
