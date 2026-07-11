package web

import (
	"errors"
	"net"
	"net/http"
	"waldi/internal/store"
)

func (s *Server) handleCaddyAsk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !requestFromPrivateNetwork(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	domain := normalizeCustomDomain(r.URL.Query().Get("domain"))
	if domain == "" {
		http.Error(w, "missing domain", http.StatusBadRequest)
		return
	}
	if reservedCustomDomain(domain, s.baseDomain) {
		http.Error(w, "reserved", http.StatusForbidden)
		return
	}
	if s.store == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}

	if _, err := s.store.UserByCustomDomain(r.Context(), domain); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "not found", http.StatusForbidden)
			return
		}
		s.logger.Error("caddy ask lookup", "domain", domain, "err", err)
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func requestFromPrivateNetwork(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
}
