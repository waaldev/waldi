package web

import (
	"context"
	"net/http"
	"strings"
	"time"
	"waldi/internal/i18n"
)

func (s *Server) handleSetLocale(w http.ResponseWriter, r *http.Request) {
	lang := r.PathValue("code")
	if !i18n.Supported(lang) {
		http.NotFound(w, r)
		return
	}

	setLocaleCookie(w, r, s.baseDomain, lang)

	// locale.js posts here in the background to silently correct the
	// server's CF-IPCountry guess against the visitor's real OS timezone
	// (common for Iranian/Afghan visitors on a VPN, whose IP country isn't
	// Iran/Afghanistan). It only needs the cookie set for the next
	// request — no DB write, no cache purge, no redirect.
	if r.URL.Query().Get("auto") == "1" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if user := currentUser(r); user != nil && s.store != nil {
		if err := s.store.UpdateUserLocale(r.Context(), user.ID, lang); err != nil {
			s.logger.Error("updating user locale", "err", err)
		}
	}

	dest := redirectBack(r)
	s.purgeLocalePage(r, dest)
	redirect(w, r, dest)
}

func (s *Server) handleSettingsLocale(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}

	lang := strings.TrimSpace(r.FormValue("locale"))
	if !i18n.Supported(lang) {
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}

	setLocaleCookie(w, r, s.baseDomain, lang)

	if s.store != nil {
		if err := s.store.UpdateUserLocale(r.Context(), user.ID, lang); err != nil {
			s.logger.Error("updating user locale", "err", err)
		}
	}

	s.purgeLocalePage(r, "/settings")
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (s *Server) purgeLocalePage(r *http.Request, dest string) {
	if s.cdnPurger == nil {
		return
	}

	purgeURL := requestScheme(r) + "://" + r.Host + dest
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := s.cdnPurger.PurgeURLs(ctx, []string{purgeURL}); err != nil {
			s.logger.Error("purging cdn cache after locale change", "err", err, "url", purgeURL)
		}
	}()
}
