package web

import (
	"errors"
	"net/http"
	"strings"
	"waldi/internal/store"
)

// handleResumeDigest resumes a paused digest subscription (GET, one-click —
// this is a re-permission ask, not an unsubscribe, so there's no confirm
// step). Also lifts a pause for users who click through a stale
// digest-unsubscribe link before ever pausing, which is harmless.
func (s *Server) handleResumeDigest(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	pd := s.newPageData(r, currentUser(r))
	pd.Inline = true
	pd.Title = pd.T("resume.title")
	pd.SEO = noindexSEO()
	pd.Auth = &AuthView{
		Mode:    "resume",
		Heading: pd.T("resume.heading"),
	}

	if token == "" || s.store == nil {
		pd.Auth.Error = pd.T("resume.invalid")
		s.renderer.Render(w, "resume.html", pd)
		return
	}

	if _, err := s.store.ResumeDigestByToken(r.Context(), token); err == nil {
		pd.Auth.Message = pd.T("resume.done")
	} else {
		if !errors.Is(err, store.ErrNotFound) {
			s.logger.Error("resuming digest", "err", err)
		}
		pd.Auth.Error = pd.T("resume.invalid")
	}
	s.renderer.Render(w, "resume.html", pd)
}
