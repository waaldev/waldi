package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"waldi/internal/i18n"
	"waldi/internal/store"

	"github.com/jackc/pgx/v5/pgconn"
)

func (s *Server) handleWrite(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !s.requireVerified(w, r, user) {
		return
	}
	if !user.CanWrite {
		http.Redirect(w, r, "/write/invite", http.StatusSeeOther)
		return
	}
	if s.store == nil {
		s.renderWriteError(w, r, "write.error.db")
		return
	}

	drafts, err := s.store.DraftsByUser(r.Context(), user.ID, writePageSize)
	if err != nil {
		s.logger.Error("listing user drafts", "err", err)
		s.renderWriteError(w, r, "write.error.list")
		return
	}

	cursor, err := parsePageCursor(r)
	if err != nil {
		s.renderWriteError(w, r, "error.bad_cursor")
		return
	}
	rawPublished, err := s.store.PublishedPostsByUser(r.Context(), user.ID, publishedPageSize+1, cursor)
	if err != nil {
		s.logger.Error("listing user published posts", "err", err)
		s.renderWriteError(w, r, "write.error.list")
		return
	}
	published, hasMore := trimPage(rawPublished, publishedPageSize)
	engagement, err := s.store.PostEngagementByUser(r.Context(), user.ID)
	if err != nil {
		s.logger.Error("loading post engagement", "err", err)
		s.renderWriteError(w, r, "write.error.list")
		return
	}

	lang, _ := resolveLocale(r, user)
	now := time.Now()
	pd := s.newPageData(r, user)
	pd.Title = pd.T("write.title")
	pd.SEO = noindexSEO()
	pd.Write = &WriteView{
		Drafts:            writeDraftViews(drafts, lang, now),
		Published:         writePublishedViews(published, engagement, lang),
		PublishedOlderURL: publishedOlderURL("/write", published, hasMore),
	}
	s.renderer.Render(w, "write.html", pd)
}

func (s *Server) handleCreateDraft(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !s.requireVerified(w, r, user) {
		return
	}
	if s.store == nil {
		s.renderWriteError(w, r, "write.error.db")
		return
	}

	pd := s.newPageData(r, user)
	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		title = pd.T("write.default_title")
	}
	raw := defaultDocJSON()
	html, words, err := renderPostDoc(raw)
	if err != nil {
		s.logger.Error("rendering default doc", "err", err)
		s.renderWriteError(w, r, "write.error.create_failed")
		return
	}

	slug, err := s.store.UniqueSlug(r.Context(), user.ID, slugFromTitle(title), 0)
	if err != nil {
		s.logger.Error("creating draft slug", "err", err)
		s.renderWriteError(w, r, "write.error.create_failed")
		return
	}

	draft, err := s.store.CreateDraft(r.Context(), user.ID, title, slug, raw, html, words)
	if err != nil {
		s.logger.Error("creating draft", "err", err)
		s.renderWriteError(w, r, "write.error.create_failed")
		return
	}
	http.Redirect(w, r, "/write/"+strconv.FormatInt(draft.ID, 10), http.StatusSeeOther)
}

func (s *Server) handleEditDraft(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !s.requireVerified(w, r, user) {
		return
	}
	if s.store == nil {
		s.renderWriteError(w, r, "write.error.db")
		return
	}

	id, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}
	p, err := s.store.PostByIDForUser(r.Context(), id, user.ID)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.logger.Error("loading draft", "err", err)
		s.renderWriteError(w, r, "write.error.load_failed")
		return
	}

	view := postView(p)
	pd := s.newPageData(r, user)
	pd.Title = p.Title
	pd.SEO = noindexSEO()
	pd.Write = &WriteView{Post: &view}
	s.renderer.Render(w, "editor.html", pd)
}

func (s *Server) handleAutosaveDraft(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if s.store == nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}

	id, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	lang, _ := resolveLocale(r, user)
	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		title = i18n.T(lang, "write.default_title")
	}
	raw := json.RawMessage(r.FormValue("doc"))
	html, words, err := renderPostDoc(raw)
	if err != nil {
		http.Error(w, "invalid document", http.StatusBadRequest)
		return
	}

	existing, err := s.store.PostByIDForUser(r.Context(), id, user.ID)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.logger.Error("loading draft for autosave", "err", err)
		http.Error(w, "save failed", http.StatusInternalServerError)
		return
	}

	slug := existing.Slug
	if existing.Status != "published" {
		slug, err = s.store.UniqueSlug(r.Context(), user.ID, slugFromTitle(title), id)
		if err != nil {
			s.logger.Error("creating draft slug", "err", err)
			http.Error(w, "save failed", http.StatusInternalServerError)
			return
		}
	}

	p, err := s.store.UpdateDraft(r.Context(), id, user.ID, title, slug, raw, html, words)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if isUniqueViolation(err) {
		http.Error(w, "duplicate slug", http.StatusConflict)
		return
	}
	if err != nil {
		s.logger.Error("autosaving draft", "err", err)
		http.Error(w, "save failed", http.StatusInternalServerError)
		return
	}

	if p.Status == "published" {
		s.purgePublicCache(user.Username)
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write([]byte(`{"ok":true,"slug":"` + p.Slug + `"}`))
}

func (s *Server) handlePublishPost(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if s.store == nil {
		s.renderWriteError(w, r, "write.error.db")
		return
	}

	id, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}
	p, err := s.store.PublishPost(r.Context(), id, user.ID)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.logger.Error("publishing post", "err", err)
		s.renderWriteError(w, r, "write.error.publish_failed")
		return
	}
	s.purgePublicCache(user.Username)
	s.notifyPublish(r, *user, p)
	http.Redirect(w, r, "/write/"+strconv.FormatInt(p.ID, 10), http.StatusSeeOther)
}

func (s *Server) handleUnpublishPost(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if s.store == nil {
		s.renderWriteError(w, r, "write.error.db")
		return
	}

	id, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}
	_, err := s.store.UnpublishPost(r.Context(), id, user.ID)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.logger.Error("unpublishing post", "err", err)
		s.renderWriteError(w, r, "write.error.unpublish_failed")
		return
	}
	s.purgePublicCache(user.Username)
	http.Redirect(w, r, "/write/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func (s *Server) handleDeletePost(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if s.store == nil {
		s.renderWriteError(w, r, "write.error.db")
		return
	}

	id, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}
	err := s.store.DeletePost(r.Context(), id, user.ID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		s.logger.Error("deleting post", "err", err)
		s.renderWriteError(w, r, "write.error.delete_failed")
		return
	}
	s.purgePublicCache(user.Username)
	http.Redirect(w, r, "/write", http.StatusSeeOther)
}

// handleUpdatePublishedAt changes the displayed publish date of one of the
// current user's own published posts. The form (rendered on /you/{slug})
// submits the post's own slug alongside the id so we can redirect back to
// the in-app owner view regardless of outcome.
func (s *Server) handleUpdatePublishedAt(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if s.store == nil {
		s.renderWriteError(w, r, "write.error.db")
		return
	}

	id, ok := parseID(w, r.PathValue("id"))
	if !ok {
		return
	}
	slug := r.FormValue("slug")

	t, err := parsePublishedAtInput(r.FormValue("published_at"), r.FormValue("lang"))
	if err != nil || t.After(time.Now()) {
		http.Redirect(w, r, "/you/"+slug+"?date_error=1", http.StatusSeeOther)
		return
	}

	_, err = s.store.UpdatePublishedAt(r.Context(), id, user.ID, t)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.logger.Error("updating published_at", "err", err)
		s.renderWriteError(w, r, "write.error.date_failed")
		return
	}
	s.purgePublicCache(user.Username)
	http.Redirect(w, r, "/you/"+slug, http.StatusSeeOther)
}

func (s *Server) renderWriteError(w http.ResponseWriter, r *http.Request, messageKey string) {
	pd := s.newPageData(r, currentUser(r))
	pd.Title = pd.T("write.title")
	pd.SEO = noindexSEO()
	pd.Write = &WriteView{Error: pd.T(messageKey)}
	s.renderer.RenderStatus(w, http.StatusBadRequest, "write.html", pd)
}

func parseID(w http.ResponseWriter, raw string) (int64, bool) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 1 {
		http.Error(w, "bad id", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
