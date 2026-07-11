package web

import (
	"context"
	"net/http"
	"testing"
	"waldi/internal/store"
)

func TestReaderKeyLoggedIn(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, "http://localhost:8080/post", nil)
	user := &store.User{ID: 42}
	r = r.WithContext(context.WithValue(r.Context(), currentUserKey, user))

	key, cookie := readerKeyFromRequest(r, "waldi.blog")
	if key != "user:42" {
		t.Fatalf("key = %q", key)
	}
	if cookie != nil {
		t.Fatal("expected no cookie for logged-in reader")
	}
}

func TestReaderKeyAnonymousSetsCookie(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, "http://localhost:8080/post", nil)
	r.Host = "bob.localhost:8080"

	key, cookie := readerKeyFromRequest(r, "waldi.blog")
	if len(key) < 6 || key[:5] != "anon:" {
		t.Fatalf("key = %q", key)
	}
	if cookie == nil || cookie.Name != readerCookie {
		t.Fatal("expected reader cookie")
	}
	if cookie.Value == "" {
		t.Fatal("empty cookie value")
	}
}

func TestReaderKeyReusesCookie(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, "http://localhost:8080/post", nil)
	r.AddCookie(&http.Cookie{Name: readerCookie, Value: "abc123def456ghi7"})

	key, cookie := readerKeyFromRequest(r, "waldi.blog")
	if key != "anon:abc123def456ghi7" {
		t.Fatalf("key = %q", key)
	}
	if cookie != nil {
		t.Fatal("expected no new cookie")
	}
}

func TestValidReaderToken(t *testing.T) {
	if !validReaderToken("abc123def456ghi7") {
		t.Fatal("expected valid token")
	}
	if validReaderToken("bad token!") {
		t.Fatal("expected invalid token")
	}
}
