package web

import (
	"net/http"
	"waldi/internal/storage"
	"waldi/internal/store"
)

const minPasswordLen = 8

// handleChangePassword lets a signed-in user change their password from
// Settings, without going through the email-based reset flow.
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if s.store == nil {
		s.renderSettingsPasswordError(w, r, user, "blog.settings.error.db")
		return
	}
	if err := r.ParseForm(); err != nil {
		s.renderSettingsPasswordError(w, r, user, "blog.settings.error.form")
		return
	}

	current := r.FormValue("current_password")
	next := r.FormValue("new_password")

	if !checkPassword(user.PasswordHash, current) {
		s.renderSettingsPasswordError(w, r, user, "blog.settings.password.error.current")
		return
	}
	if len(next) < minPasswordLen {
		s.renderSettingsPasswordError(w, r, user, "blog.settings.password.error.length")
		return
	}

	hash, err := hashPassword(next)
	if err != nil {
		s.logger.Error("hashing password", "err", err)
		s.renderSettingsPasswordError(w, r, user, "blog.settings.error.save")
		return
	}
	if err := s.store.UpdatePasswordHash(r.Context(), user.ID, hash); err != nil {
		s.logger.Error("updating password", "err", err)
		s.renderSettingsPasswordError(w, r, user, "blog.settings.error.save")
		return
	}

	http.Redirect(w, r, "/settings?password=saved", http.StatusSeeOther)
}

func (s *Server) renderSettingsPasswordError(w http.ResponseWriter, r *http.Request, user *store.User, messageKey string) {
	pd := s.newPageData(r, user)
	pd.Title = pd.T("blog.settings.title")
	pd.SEO = noindexSEO()
	view := blogSettingsViewFor(*user, s.baseDomain)
	view.Pages = s.pagesViewFor(r.Context(), user.ID)
	view.MaxPages = store.MaxPagesPerUser
	view.PasswordError = pd.T(messageKey)
	pd.BlogSettings = view
	s.renderer.RenderStatus(w, http.StatusBadRequest, "blog_settings.html", pd)
}

// handleDeleteAccount permanently deletes the signed-in user's account
// after confirming their password. Posts, sessions, follows, and letters
// are removed along with the users row by the cascading foreign keys.
func (s *Server) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if s.store == nil {
		s.renderSettingsDeleteError(w, r, user, "blog.settings.error.db")
		return
	}
	if err := r.ParseForm(); err != nil {
		s.renderSettingsDeleteError(w, r, user, "blog.settings.error.form")
		return
	}

	if !checkPassword(user.PasswordHash, r.FormValue("current_password")) {
		s.renderSettingsDeleteError(w, r, user, "blog.settings.delete.error.password")
		return
	}

	if err := s.store.DeleteUser(r.Context(), user.ID); err != nil {
		s.logger.Error("deleting account", "user_id", user.ID, "err", err)
		s.renderSettingsDeleteError(w, r, user, "blog.settings.error.save")
		return
	}

	domain := sessionCookieDomain(r.Host, s.baseDomain)
	secure := requestScheme(r) == "https"
	clearSessionCookie(w, domain, secure)
	clearBridgeCookie(w, domain, secure)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) renderSettingsDeleteError(w http.ResponseWriter, r *http.Request, user *store.User, messageKey string) {
	pd := s.newPageData(r, user)
	pd.Title = pd.T("blog.settings.title")
	pd.SEO = noindexSEO()
	view := blogSettingsViewFor(*user, s.baseDomain)
	view.Pages = s.pagesViewFor(r.Context(), user.ID)
	view.MaxPages = store.MaxPagesPerUser
	view.DeleteError = pd.T(messageKey)
	pd.BlogSettings = view
	s.renderer.RenderStatus(w, http.StatusBadRequest, "blog_settings.html", pd)
}

// handleExportPosts streams the user's blog as a zip archive: a posts.json
// dump plus a self-contained static copy of the blog that opens offline, so
// writers can always take their words with them.
func (s *Server) handleExportPosts(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if s.store == nil {
		http.Error(w, "unavailable", http.StatusInternalServerError)
		return
	}

	posts, err := s.store.AllPostsByUser(r.Context(), user.ID)
	if err != nil {
		s.logger.Error("exporting posts", "err", err)
		http.Error(w, "export failed", http.StatusInternalServerError)
		return
	}
	pages, err := s.store.PublishedPagesByUsername(r.Context(), user.Username)
	if err != nil {
		s.logger.Error("exporting pages", "err", err)
		http.Error(w, "export failed", http.StatusInternalServerError)
		return
	}

	exp := blogExport{
		renderer:   s.renderer,
		appBaseURL: appBaseURL(r, s.baseDomain),
		owner:      *user,
		posts:      posts,
		pages:      pages,
		logger:     s.logger,
	}
	if images, ok := s.images.(storage.UploadReader); ok {
		exp.images = images
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+user.Username+`-export.zip"`)
	if err := exp.write(r.Context(), w); err != nil {
		s.logger.Error("writing export", "err", err)
	}
}
