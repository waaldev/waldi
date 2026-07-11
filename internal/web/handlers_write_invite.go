package web

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"waldi/internal/store"
)

const writeRequestNoteMaxRunes = 2000

func (s *Server) handleWriteInviteForm(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login?next="+url.QueryEscape("/write/invite"), http.StatusSeeOther)
		return
	}
	if !s.requireVerified(w, r, user) {
		return
	}
	if user.CanWrite {
		http.Redirect(w, r, "/write", http.StatusSeeOther)
		return
	}

	pd := s.newPageData(r, user)
	pd.Title = pd.T("write.invite.title")
	pd.SEO = noindexSEO()
	pd.WriteInvite = &WriteInviteView{}
	s.renderer.Render(w, "write_invite.html", pd)
}

func (s *Server) handleRedeemWriteInvite(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login?next="+url.QueryEscape("/write/invite"), http.StatusSeeOther)
		return
	}
	if !s.requireVerified(w, r, user) {
		return
	}
	if s.store == nil {
		s.renderWriteInviteError(w, r, user, "write.error.db")
		return
	}
	if err := r.ParseForm(); err != nil {
		s.renderWriteInviteError(w, r, user, "auth.error.form")
		return
	}

	code := strings.TrimSpace(r.FormValue("code"))
	if code == "" {
		s.renderWriteInviteError(w, r, user, "write.invite.error.code_invalid")
		return
	}

	if err := s.store.RedeemInvitationForUser(r.Context(), code, user.ID); err != nil {
		if errors.Is(err, store.ErrInviteInvalid) {
			s.renderWriteInviteError(w, r, user, "write.invite.error.code_invalid")
			return
		}
		s.logger.Error("redeeming write invite", "err", err)
		s.renderWriteInviteError(w, r, user, "write.error.db")
		return
	}

	http.Redirect(w, r, "/write", http.StatusSeeOther)
}

func (s *Server) handleWriteRequest(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login?next="+url.QueryEscape("/write/invite"), http.StatusSeeOther)
		return
	}
	if !s.requireVerified(w, r, user) {
		return
	}
	if s.store == nil {
		s.renderWriteInviteError(w, r, user, "write.error.db")
		return
	}
	if err := r.ParseForm(); err != nil {
		s.renderWriteInviteError(w, r, user, "auth.error.form")
		return
	}

	blogLink := strings.TrimSpace(r.FormValue("blog_link"))
	note := strings.TrimSpace(r.FormValue("note"))
	if note == "" {
		s.renderWriteInviteError(w, r, user, "write.invite.error.note_required")
		return
	}
	if runes := []rune(note); len(runes) > writeRequestNoteMaxRunes {
		note = string(runes[:writeRequestNoteMaxRunes])
	}

	if _, err := s.store.CreateWriteRequest(r.Context(), user.ID, blogLink, note); err != nil {
		s.logger.Error("creating write request", "err", err)
		s.renderWriteInviteError(w, r, user, "write.error.db")
		return
	}
	s.notifyWriteRequest(*user, blogLink, note)

	pd := s.newPageData(r, user)
	pd.Title = pd.T("write.invite.title")
	pd.SEO = noindexSEO()
	pd.WriteInvite = &WriteInviteView{Message: pd.T("write.invite.request.sent")}
	s.renderer.Render(w, "write_invite.html", pd)
}

func (s *Server) renderWriteInviteError(w http.ResponseWriter, r *http.Request, user *store.User, messageKey string) {
	pd := s.newPageData(r, user)
	pd.Title = pd.T("write.invite.title")
	pd.SEO = noindexSEO()
	pd.WriteInvite = &WriteInviteView{Error: pd.T(messageKey)}
	s.renderer.RenderStatus(w, http.StatusBadRequest, "write_invite.html", pd)
}
