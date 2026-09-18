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
