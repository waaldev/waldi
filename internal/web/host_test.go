package web

import (
	"net/http"
	"testing"
	"time"
	"waldi/internal/store"
)

func TestBlogFromHost(t *testing.T) {
	tests := []struct {
		name       string
		host       string
		baseDomain string
		want       string
	}{
		{
			name:       "production subdomain",
			host:       "sara.waldi.blog",
			baseDomain: "waldi.blog",
			want:       "sara",
		},
		{
			name:       "production subdomain with port",
			host:       "sara.waldi.blog:8080",
			baseDomain: "waldi.blog",
			want:       "sara",
		},
		{
			name:       "localhost dev subdomain",
			host:       "sara.localhost:8080",
			baseDomain: "waldi.blog",
			want:       "sara",
		},
		{
			name:       "waldi test dev subdomain",
			host:       "sara.waldi.test:8080",
			baseDomain: "waldi.blog",
			want:       "sara",
		},
		{
			name:       "apex is app",
			host:       "waldi.blog",
			baseDomain: "waldi.blog",
			want:       "",
		},
		{
			name:       "reserved subdomain",
			host:       "www.waldi.blog",
			baseDomain: "waldi.blog",
			want:       "",
		},
		{
			name:       "invalid username",
			host:       "sa_ra.waldi.blog",
			baseDomain: "waldi.blog",
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BlogFromHost(tt.host, tt.baseDomain)
			if tt.want == "" {
				if got != nil {
					t.Fatalf("got blog %q, want nil", got.Username)
				}
				return
			}
			if got == nil {
				t.Fatalf("got nil, want %q", tt.want)
			}
			if got.Username != tt.want {
				t.Fatalf("got %q, want %q", got.Username, tt.want)
			}
		})
	}
}

func TestPublicBlogURL(t *testing.T) {
	tests := []struct {
		name       string
		host       string
		baseDomain string
		username   string
		path       string
		want       string
	}{
		{
			name:       "local dev blog home",
			host:       "localhost:8080",
			baseDomain: "waldi.blog",
			username:   "sara",
			path:       "/",
			want:       "http://sara.localhost:8080/",
		},
		{
			name:       "local dev post",
			host:       "localhost:8080",
			baseDomain: "waldi.blog",
			username:   "sara",
			path:       "/hello-world",
			want:       "http://sara.localhost:8080/hello-world",
		},
		{
			name:       "production blog",
			host:       "waldi.blog",
			baseDomain: "waldi.blog",
			username:   "sara",
			path:       "/",
			want:       "http://sara.waldi.blog/",
		},
		{
			name:       "waldi test domain",
			host:       "waldi.test:8080",
			baseDomain: "waldi.blog",
			username:   "sara",
			path:       "/",
			want:       "http://sara.waldi.test:8080/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &http.Request{Host: tt.host}
			got := PublicBlogURL(r, tt.baseDomain, tt.username, tt.path)
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPublicBlogURLForOwner(t *testing.T) {
	verifiedAt := time.Now()
	domain := "blog.example.com"

	t.Run("verified custom domain is canonical", func(t *testing.T) {
		r := &http.Request{Host: "waldi.blog"}
		owner := store.User{Username: "sara", CustomDomain: &domain, CustomDomainVerifiedAt: &verifiedAt}
		got := PublicBlogURLForOwner(r, "waldi.blog", owner, "/hello-world")
		want := "http://blog.example.com/hello-world"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("unverified custom domain falls back to subdomain", func(t *testing.T) {
		r := &http.Request{Host: "waldi.blog"}
		owner := store.User{Username: "sara", CustomDomain: &domain}
		got := PublicBlogURLForOwner(r, "waldi.blog", owner, "/")
		want := "http://sara.waldi.blog/"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("no custom domain falls back to subdomain", func(t *testing.T) {
		r := &http.Request{Host: "localhost:8080"}
		owner := store.User{Username: "sara"}
		got := PublicBlogURLForOwner(r, "waldi.blog", owner, "/")
		want := "http://sara.localhost:8080/"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
}

func TestSessionCookieDomainForCustomDomain(t *testing.T) {
	got := sessionCookieDomain("blog.example.com", "waldi.blog")
	if got != "" {
		t.Fatalf("got %q, want host-only (empty) cookie domain", got)
	}
}
