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
			defer res.Body.Close()
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
