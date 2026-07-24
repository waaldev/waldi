package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIAuthBridgeRequiresSession(t *testing.T) {
	s := &Server{baseDomain: "waldi.blog"}
	r, _ := http.NewRequest(http.MethodGet, "http://localhost:8080/api/auth/bridge?return=http://bob.localhost:8080/", nil)
	r.Host = "localhost:8080"
	w := httptest.NewRecorder()

	s.handleAPIAuthBridge(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", w.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["continue"] != "" {
		t.Fatalf("expected empty continue, got %q", body["continue"])
	}
}

func TestAPIAuthBridgeCORS(t *testing.T) {
	s := &Server{baseDomain: "waldi.blog"}
	r, _ := http.NewRequest(http.MethodOptions, "http://localhost:8080/api/auth/bridge", nil)
	r.Host = "localhost:8080"
	r.Header.Set("Origin", "http://bob.localhost:8080")
	w := httptest.NewRecorder()

	s.handleAPIAuthBridge(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://bob.localhost:8080" {
		t.Fatalf("origin %q", got)
	}
}

func TestAllowedOriginAcceptsVerifiedCustomDomain(t *testing.T) {
	s := &Server{baseDomain: "waldi.blog", customDomains: newCustomDomainCache()}
	s.customDomains.store("blog.example.com", "sara", customDomainPositiveTTL)

	if !s.allowedOrigin(t.Context(), "https://blog.example.com") {
		t.Fatal("expected verified custom domain origin to be allowed")
	}
}

func TestAllowedOriginRejectsUnknownCustomDomain(t *testing.T) {
	s := &Server{baseDomain: "waldi.blog", customDomains: newCustomDomainCache()}
	s.customDomains.store("blog.example.com", "", customDomainNegativeTTL)

	if s.allowedOrigin(t.Context(), "https://blog.example.com") {
		t.Fatal("expected unverified custom domain origin to be rejected")
	}
}

func TestValidBridgeReturnAcceptsVerifiedCustomDomain(t *testing.T) {
	s := &Server{baseDomain: "waldi.blog", customDomains: newCustomDomainCache()}
	s.customDomains.store("blog.example.com", "sara", customDomainPositiveTTL)

	_, ok := s.validBridgeReturn(t.Context(), "https://blog.example.com/hello")
	if !ok {
		t.Fatal("expected verified custom domain return url to be valid")
	}
}

func TestBridgeContinueURLRejectsBadReturn(t *testing.T) {
	s := &Server{baseDomain: "waldi.blog"}
	r, _ := http.NewRequest(http.MethodGet, "http://localhost:8080/api/auth/bridge", nil)
	r.Host = "localhost:8080"
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: "tok"})
	// Without session middleware currentUser is nil - still an expected rejection.
	_, err := s.bridgeContinueURL(r, "not-a-url")
	if err == nil {
		t.Fatal("expected error")
	}
}
