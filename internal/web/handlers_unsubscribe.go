package web

import (
	"errors"
	"net/http"
	"strings"
	"waldi/internal/store"
)

// handleUnsubscribeDigestPage shows a confirmation page for a digest
// unsubscribe link (GET, token in query). It does not itself change
// subscription state — the confirm button POSTs to the same path.
func (s *Server) handleUnsubscribeDigestPage(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	pd := s.newPageData(r, currentUser(r))
	pd.Title = pd.T("unsubscribe.title")
	pd.SEO = noindexSEO()
	pd.Auth = &AuthView{
		Mode:       "unsubscribe",
		Heading:    pd.T("unsubscribe.heading"),
		ResetToken: token,
	}

	if token == "" {
		pd.Auth.Error = pd.T("unsubscribe.invalid")
		s.renderer.Render(w, "unsubscribe.html", pd)
		return
	}

	if s.store == nil {
		pd.Auth.Error = pd.T("unsubscribe.invalid")
		s.renderer.Render(w, "unsubscribe.html", pd)
		return
	}

	user, err := s.store.UserByDigestUnsubscribeToken(r.Context(), token)
	if err == nil {
		if user.DigestUnsubscribed() {
			pd.Auth.Message = pd.T("unsubscribe.done")
		}
		s.renderer.Render(w, "unsubscribe.html", pd)
		return
	}
	if !errors.Is(err, store.ErrNotFound) {
		s.logger.Error("looking up digest unsubscribe token", "err", err)
	}

	// Not a registered user's token — check whether it belongs to an
	// anonymous captured email's digest instead.
	exists, err := s.store.EmailCaptureTokenExists(r.Context(), token)
	if err != nil {
		s.logger.Error("looking up email capture unsubscribe token", "err", err)
	}
	if !exists {
		pd.Auth.Error = pd.T("unsubscribe.invalid")
	}
	s.renderer.Render(w, "unsubscribe.html", pd)
}

// handleUnsubscribeDigest performs the unsubscribe (POST). This also serves
// one-click List-Unsubscribe-Post requests from mail clients, which submit
// here directly without visiting the confirmation page.
func (s *Server) handleUnsubscribeDigest(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		token = strings.TrimSpace(r.FormValue("token"))
	}

	pd := s.newPageData(r, currentUser(r))
	pd.Title = pd.T("unsubscribe.title")
	pd.SEO = noindexSEO()
	pd.Auth = &AuthView{
		Mode:       "unsubscribe",
		Heading:    pd.T("unsubscribe.heading"),
		ResetToken: token,
	}

	if token == "" || s.store == nil {
		pd.Auth.Error = pd.T("unsubscribe.invalid")
		s.renderer.Render(w, "unsubscribe.html", pd)
		return
	}

	if _, err := s.store.UnsubscribeFromDigestByToken(r.Context(), token); err == nil {
		pd.Auth.Message = pd.T("unsubscribe.done")
		s.renderer.Render(w, "unsubscribe.html", pd)
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		s.logger.Error("unsubscribing from digest", "err", err)
	}

	// Not a registered user's token — try it as an anonymous captured
	// email's digest unsubscribe instead.
	if err := s.store.UnsubscribeEmailCaptureByToken(r.Context(), token); err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			s.logger.Error("unsubscribing email capture", "err", err)
		}
		pd.Auth.Error = pd.T("unsubscribe.invalid")
		s.renderer.Render(w, "unsubscribe.html", pd)
		return
	}

	pd.Auth.Message = pd.T("unsubscribe.done")
	s.renderer.Render(w, "unsubscribe.html", pd)
}
