package web

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleCaddyAsk(t *testing.T) {
	s := &Server{baseDomain: "waldi.blog", logger: slog.Default()}

	t.Run("rejects public client", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/internal/caddy-ask?domain=blog.example.com", nil)
		req.RemoteAddr = "203.0.113.10:12345"
		w := httptest.NewRecorder()

		s.handleCaddyAsk(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
		}
	})

	t.Run("rejects reserved domain", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/internal/caddy-ask?domain=sara.waldi.blog", nil)
		req.RemoteAddr = "172.18.0.2:12345"
		w := httptest.NewRecorder()

		s.handleCaddyAsk(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
		}
	})

	t.Run("rejects missing domain", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/internal/caddy-ask", nil)
		req.RemoteAddr = "172.18.0.2:12345"
		w := httptest.NewRecorder()

		s.handleCaddyAsk(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("rejects without store", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/internal/caddy-ask?domain=blog.example.com", nil)
		req.RemoteAddr = "172.18.0.2:12345"
		w := httptest.NewRecorder()

		s.handleCaddyAsk(w, req)

		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
		}
	})
}

func TestRequestFromPrivateNetwork(t *testing.T) {
	tests := []struct {
		addr string
		want bool
	}{
		{"172.18.0.2:12345", true},
		{"127.0.0.1:8080", true},
		{"10.0.0.5:443", true},
		{"203.0.113.10:12345", false},
	}

	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.addr
			if got := requestFromPrivateNetwork(req); got != tt.want {
				t.Fatalf("requestFromPrivateNetwork(%q) = %v, want %v", tt.addr, got, tt.want)
			}
		})
	}
}
