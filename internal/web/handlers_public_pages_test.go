package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublicInformationPages(t *testing.T) {
	s := testServer(t)
	s.mux = http.NewServeMux()
	s.routes()

	tests := []struct {
		path string
		want string
	}{
		{path: "/how-it-works", want: "A few things about Waldi"},
		{path: "/write/invite", want: "Would you like to write here?"},
		{path: "/explore", want: "Recent writing and the blogs on Waldi."},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "https://waldi.blog"+tt.path, nil)
			req.Header.Set("Accept-Language", "en")
			rec := httptest.NewRecorder()
			s.ServeHTTP(rec, req)

			res := rec.Result()
			defer func() { _ = res.Body.Close() }()
			if res.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
			}
			body, err := io.ReadAll(res.Body)
			if err != nil {
				t.Fatalf("ReadAll: %v", err)
			}
			if !strings.Contains(string(body), tt.want) {
				t.Fatalf("body does not contain %q", tt.want)
			}
		})
	}
}

func TestLandingExplainsReaderAndWriterPaths(t *testing.T) {
	s := testServer(t)
	s.mux = http.NewServeMux()
	s.routes()
	req := httptest.NewRequest(http.MethodGet, "https://waldi.blog/", nil)
	req.Header.Set("Accept-Language", "en")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	body := rec.Body.String()
	for _, want := range []string{"Join Waldi", "A quiet place to write, and to be read.", "Writing is invite-only for now.", "/how-it-works", "/write/invite", "https://amin.waldi.blog/serendipity"} {
		if !strings.Contains(body, want) {
			t.Errorf("landing body does not contain %q", want)
		}
	}
}

func TestExploreLanguageFilter(t *testing.T) {
	s := testServer(t)
	s.mux = http.NewServeMux()
	s.routes()

	req := httptest.NewRequest(http.MethodGet, "https://waldi.blog/explore?lang=fa", nil)
	req.Header.Set("Accept-Language", "en")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	body := rec.Body.String()
	for _, want := range []string{`href="/explore"`, `href="/explore?lang=en"`, `href="/explore?lang=fa" lang="fa" aria-current="page"`, `<link rel="canonical" href="https://waldi.blog/explore">`} {
		if !strings.Contains(body, want) {
			t.Errorf("explore body does not contain %q", want)
		}
	}

	req = httptest.NewRequest(http.MethodGet, "https://waldi.blog/explore?lang=xx", nil)
	rec = httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "/explore" {
		t.Fatalf("status %d location %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestLocalizedPublicPages(t *testing.T) {
	s := testServer(t)
	s.mux = http.NewServeMux()
	s.routes()

	tests := []struct {
		path      string
		acceptTag string
		want      []string
	}{
		{
			path:      "/fa/how-it-works",
			acceptTag: "en",
			want: []string{
				`<html lang="fa" dir="rtl">`,
				`<link rel="canonical" href="https://waldi.blog/fa/how-it-works">`,
				`<link rel="alternate" hreflang="en" href="https://waldi.blog/how-it-works">`,
				`<link rel="alternate" hreflang="fa" href="https://waldi.blog/fa/how-it-works">`,
				`<link rel="alternate" hreflang="x-default" href="https://waldi.blog/how-it-works">`,
				`href="/fa/explore"`,
				`<a class="lang-toggle" href="/how-it-works" hreflang="en">`,
			},
		},
		{
			path:      "/",
			acceptTag: "fa",
			want: []string{
				`<html lang="en" dir="ltr">`,
				`<link rel="canonical" href="https://waldi.blog/">`,
				`<link rel="alternate" hreflang="fa" href="https://waldi.blog/fa">`,
				`<link rel="alternate" hreflang="x-default" href="https://waldi.blog/">`,
				`href="/write/invite"`,
				`<a class="lang-toggle" href="/fa" hreflang="fa">`,
			},
		},
		{
			path:      "/explore",
			acceptTag: "fa",
			want:      []string{`<html lang="en" dir="ltr">`, `href="/explore?lang=fa"`},
		},
		{
			path:      "/fa/write/invite",
			acceptTag: "en",
			want:      []string{`<html lang="fa" dir="rtl">`, `<link rel="canonical" href="https://waldi.blog/fa/write/invite">`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "https://waldi.blog"+tt.path, nil)
			req.Header.Set("Accept-Language", tt.acceptTag)
			req.Header.Set("CF-IPCountry", "IR")
			rec := httptest.NewRecorder()
			s.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d", rec.Code)
			}
			body := rec.Body.String()
			for _, want := range tt.want {
				if !strings.Contains(body, want) {
					t.Errorf("body does not contain %q", want)
				}
			}
			if strings.Contains(body, "locale.js") {
				t.Error("localized page should not load locale.js")
			}
			if got := rec.Header().Get("Vary"); got != "" {
				t.Errorf("Vary = %q, want none", got)
			}
		})
	}
}

func TestEnglishIsDefaultEvenWithPersianCookie(t *testing.T) {
	s := testServer(t)
	s.mux = http.NewServeMux()
	s.routes()
	req := httptest.NewRequest(http.MethodGet, "https://waldi.blog/how-it-works", nil)
	req.AddCookie(&http.Cookie{Name: localeCookie, Value: "fa"})
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), `<html lang="en" dir="ltr">`) {
		t.Fatal("unprefixed public page is not English")
	}
}

func TestUnknownNestedPathsAreNotFound(t *testing.T) {
	s := testServer(t)
	s.mux = http.NewServeMux()
	s.routes()
	for _, path := range []string{"/en/how-it-works", "/foo/bar", "/fa/unknown"} {
		req := httptest.NewRequest(http.MethodGet, "https://waldi.blog"+path, nil)
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s status = %d, want 404", path, rec.Code)
		}
	}
}
