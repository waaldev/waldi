package web

import (
	"net/http"
	"net/url"
	"testing"
)

func TestRedirectSameHost(t *testing.T) {
	r, _ := http.NewRequest(http.MethodPost, "http://bob.localhost:8080/follow/alice", nil)
	r.Host = "bob.localhost:8080"
	r.Header.Set("Referer", "http://bob.localhost:8080/hello-world")

	got := redirectSameHost(r)
	if got != "/hello-world" {
		t.Fatalf("got %q, want /hello-world", got)
	}
}

func TestRedirectSameHostRejectsCrossHostReferer(t *testing.T) {
	r, _ := http.NewRequest(http.MethodPost, "http://bob.localhost:8080/follow/alice", nil)
	r.Host = "bob.localhost:8080"
	r.Header.Set("Referer", "http://localhost:8080/")

	got := redirectSameHost(r)
	if got != "/" {
		t.Fatalf("got %q, want /", got)
	}
}

func TestLoginURLOnBlogSubdomain(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, "http://bob.localhost:8080/", nil)
	r.Host = "bob.localhost:8080"

	s := &Server{baseDomain: "waldi.blog", customDomains: newCustomDomainCache()}
	got := s.loginURL(r, "waldi.blog", "http://bob.localhost:8080/hello")
	if got != "/login?next=http%3A%2F%2Fbob.localhost%3A8080%2Fhello" {
		t.Fatalf("got %q", got)
	}
}

func TestIsCrossHostRedirectOnLocalhost(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, "http://localhost:8080/auth/bridge", nil)
	r.Host = "localhost:8080"

	if !isCrossHostRedirect(r, "http://bob.localhost:8080/auth/continue?token=abc") {
		t.Fatal("expected cross-host redirect")
	}
	if isCrossHostRedirect(r, "/login") {
		t.Fatal("relative path is not cross-host")
	}
	if isCrossHostRedirect(r, "http://localhost:8080/") {
		t.Fatal("same host is not cross-host")
	}
}

func TestLoginURLUsesPageURL(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, "http://bob.localhost:8080/post", nil)
	r.Host = "bob.localhost:8080"
	r.URL.Path = "/post"

	s := &Server{baseDomain: "waldi.blog", customDomains: newCustomDomainCache()}
	got := s.loginURL(r, "waldi.blog", pageURL(r))
	want := "/login?next=" + url.QueryEscape("http://bob.localhost:8080/post")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
