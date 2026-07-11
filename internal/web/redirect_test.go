package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"waldi/internal/store"
)

func TestIsPublicBlogRedirectPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "/", want: true},
		{path: "/hello-world", want: true},
		{path: "/feed.xml", want: true},
		{path: "/sitemap.xml", want: true},
		{path: "/robots.txt", want: true},
		{path: "/login", want: false},
		{path: "/settings", want: false},
		{path: "/write", want: false},
		{path: "/write/42", want: false},
		{path: "/auth/bridge", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := isPublicBlogRedirectPath(tt.path); got != tt.want {
				t.Fatalf("isPublicBlogRedirectPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestSubdomainToCustomDomainRedirect(t *testing.T) {
	verifiedAt := time.Now()
	domain := "blog.example.com"
	owner := store.User{
		Username:               "sara",
		CustomDomain:           &domain,
		CustomDomainVerifiedAt: &verifiedAt,
	}

	t.Run("subdomain post redirects to custom domain", func(t *testing.T) {
		r, _ := http.NewRequest(http.MethodGet, "http://sara.waldi.blog/my-post?src=feed", nil)
		r.Host = "sara.waldi.blog"
		got := subdomainToCustomDomainRedirect(r, "waldi.blog", owner)
		want := "http://blog.example.com/my-post?src=feed"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("custom domain host does not redirect", func(t *testing.T) {
		r, _ := http.NewRequest(http.MethodGet, "http://blog.example.com/my-post", nil)
		r.Host = "blog.example.com"
		got := subdomainToCustomDomainRedirect(r, "waldi.blog", owner)
		if got != "" {
			t.Fatalf("got %q, want empty", got)
		}
	})

	t.Run("unverified domain does not redirect", func(t *testing.T) {
		r, _ := http.NewRequest(http.MethodGet, "http://sara.waldi.blog/", nil)
		r.Host = "sara.waldi.blog"
		unverified := store.User{Username: "sara", CustomDomain: &domain}
		got := subdomainToCustomDomainRedirect(r, "waldi.blog", unverified)
		if got != "" {
			t.Fatalf("got %q, want empty", got)
		}
	})

	t.Run("app path does not redirect", func(t *testing.T) {
		r, _ := http.NewRequest(http.MethodGet, "http://sara.waldi.blog/settings", nil)
		r.Host = "sara.waldi.blog"
		got := subdomainToCustomDomainRedirect(r, "waldi.blog", owner)
		if got != "" {
			t.Fatalf("got %q, want empty", got)
		}
	})
}

func TestPermanentRedirectUses301ForProduction(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, "http://sara.waldi.blog/", nil)
	r.Host = "sara.waldi.blog"
	w := httptest.NewRecorder()

	permanentRedirect(w, r, "https://blog.example.com/")

	if w.Code != http.StatusMovedPermanently {
		t.Fatalf("status %d, want 301", w.Code)
	}
	if got := w.Header().Get("Location"); got != "https://blog.example.com/" {
		t.Fatalf("location %q", got)
	}
}

func TestRenderCrossHostRedirect(t *testing.T) {
	w := httptest.NewRecorder()
	renderCrossHostRedirect(w, "http://bob.localhost:8080/auth/continue?token=abc")

	if w.Code != 200 {
		t.Fatalf("status %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "bob.localhost:8080/auth/continue") {
		t.Fatalf("body missing dest: %s", body)
	}
	if strings.Contains(w.Header().Get("Location"), "bob.localhost") {
		t.Fatal("should not use Location header")
	}
}

func TestRedirectUsesHTMLForCrossHost(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, "http://localhost:8080/auth/bridge", nil)
	r.Host = "localhost:8080"
	w := httptest.NewRecorder()

	redirect(w, r, "http://bob.localhost:8080/auth/continue?token=abc")

	if w.Code != 200 {
		t.Fatalf("status %d, want HTML 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "location.replace") {
		t.Fatal("expected JS redirect")
	}
}

func TestRedirectUsesHTTPForSameHost(t *testing.T) {
	r, _ := http.NewRequest(http.MethodPost, "http://bob.localhost:8080/follow/alice", nil)
	r.Host = "bob.localhost:8080"
	w := httptest.NewRecorder()

	redirect(w, r, "/hello")

	if w.Code != http.StatusSeeOther {
		t.Fatalf("status %d", w.Code)
	}
	if got := w.Header().Get("Location"); got != "/hello" {
		t.Fatalf("location %q", got)
	}
}
