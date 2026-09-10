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

func TestPublicResourceCacheHeadersReturnNotModified(t *testing.T) {
	body := []byte("<rss></rss>")
	firstRequest := httptest.NewRequest(http.MethodGet, "/feed.xml", nil)
	first := httptest.NewRecorder()
	writePublicResource(first, firstRequest, "application/rss+xml; charset=utf-8", body)

	r := httptest.NewRequest(http.MethodGet, "/feed.xml", nil)
	r.Header.Set("If-None-Match", first.Header().Get("ETag"))
	w := httptest.NewRecorder()
	writePublicResource(w, r, "application/rss+xml; charset=utf-8", body)

	if w.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotModified)
	}
	if got := w.Header().Get("Cache-Control"); got != publicCacheControl {
		t.Fatalf("Cache-Control = %q, want %q", got, publicCacheControl)
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

	handler := s.withSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.withCacheHeaders(w, r, renderCachedTestPage)
	}))
	handler.ServeHTTP(w, r)

	if got := w.Header().Get("Cache-Control"); got != privateSessionCacheControl {
		t.Fatalf("Cache-Control = %q, want %q", got, privateSessionCacheControl)
	}
}

func TestPublicBlogCacheOverridesSignedInPrivatePolicy(t *testing.T) {
	s := &Server{}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: "session-token"})
	w := httptest.NewRecorder()

	handler := s.withSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.withPublicBlogCacheHeaders(w, r, renderCachedTestPage)
	}))
	handler.ServeHTTP(w, r)

	if got := w.Header().Get("Cache-Control"); got != publicCacheControl {
		t.Fatalf("Cache-Control = %q, want %q", got, publicCacheControl)
	}
}

func renderCachedTestPage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte("<p>cached</p>"))
}

func TestStaticCacheRequiresVersionForImmutable(t *testing.T) {
	handler := staticCacheControl(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, tt := range []struct {
		url  string
		want string
	}{
		{url: "/static/favicon.png", want: staticRevalidateCacheControl},
		{url: "/static/favicon.png?v=abc", want: "public, max-age=31536000, immutable"},
		{url: "/static/uploads/user/image.webp", want: "public, max-age=31536000, immutable"},
	} {
		r := httptest.NewRequest(http.MethodGet, tt.url, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if got := w.Header().Get("Cache-Control"); got != tt.want {
			t.Errorf("%s: Cache-Control = %q, want %q", tt.url, got, tt.want)
		}
	}
}
