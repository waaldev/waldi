package web

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"waldi/internal/store"
)

const maxPageTitleLen = 80

// handleCreatePage adds a new static page (an "About"/"Now"-style page,
// stored as a post with type=page) and sends the owner straight into the
// normal Tiptap editor to write it.
func (s *Server) handleCreatePage(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !s.requireVerified(w, r, user) {
		return
	}
	if s.store == nil {
		s.renderSettingsPagesError(w, r, user, "blog.settings.error.db")
		return
	}
	if err := r.ParseForm(); err != nil {
		s.renderSettingsPagesError(w, r, user, "blog.settings.error.form")
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" || len(title) > maxPageTitleLen {
		s.renderSettingsPagesError(w, r, user, "blog.settings.pages.error.title")
		return
	}

	existing, err := s.store.PagesByUser(r.Context(), user.ID)
	if err != nil {
		s.logger.Error("counting pages", "err", err)
		s.renderSettingsPagesError(w, r, user, "blog.settings.error.save")
		return
	}
	if len(existing) >= store.MaxPagesPerUser {
		s.renderSettingsPagesError(w, r, user, "blog.settings.pages.error.limit")
		return
	}

	slug, errKey := s.resolvePageSlug(r.Context(), user.ID, r.FormValue("slug"), title, 0)
	if errKey != "" {
		s.renderSettingsPagesError(w, r, user, errKey)
		return
	}

	raw := defaultDocJSON()
	html, words, err := renderPostDoc(raw)
	if err != nil {
		s.logger.Error("rendering default page doc", "err", err)
		s.renderSettingsPagesError(w, r, user, "blog.settings.error.save")
		return
	}

	page, err := s.store.CreatePageDraft(r.Context(), user.ID, title, slug, raw, html, words)
	if err != nil {
		s.logger.Error("creating page", "err", err)
		s.renderSettingsPagesError(w, r, user, "blog.settings.error.save")
		return
	}

	http.Redirect(w, r, "/write/"+strconv.FormatInt(page.ID, 10), http.StatusSeeOther)
}

// handleMovePage swaps a page with its neighbor above or below.
func (s *Server) handleMovePage(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireVerified(w, r, user) {
		return
	}
	if s.store == nil {
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	pages, err := s.store.PagesByUser(r.Context(), user.ID)
	if err != nil {
		s.logger.Error("loading pages", "err", err)
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}

	index := -1
	for i, p := range pages {
		if p.ID == id {
			index = i
			break
		}
	}
	if index == -1 {
		http.NotFound(w, r)
		return
	}

	var neighbor int
	switch r.FormValue("direction") {
	case "up":
		neighbor = index - 1
	case "down":
		neighbor = index + 1
	default:
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	if neighbor < 0 || neighbor >= len(pages) {
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}

	if err := s.store.SwapPagePositions(r.Context(), user.ID, pages[index].ID, pages[neighbor].ID); err != nil {
		s.logger.Error("reordering page", "err", err)
	} else {
		s.purgePublicCache(user.Username)
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

// resolvePageSlug turns a user-supplied slug (or, if blank, the page
// title) into a final slug for postID (0 for a brand-new page), rejecting
// reserved words and slugs already taken by another of the user's posts.
// Returns an i18n error key on failure, or "" on success.
func (s *Server) resolvePageSlug(ctx context.Context, userID int64, requested, title string, postID int64) (string, string) {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		base := slugFromTitle(title)
		if reservedPageSlugs[base] {
			base += "-page"
		}
		slug, err := s.store.UniqueSlug(ctx, userID, base, postID)
		if err != nil {
			s.logger.Error("finding unique page slug", "err", err)
			return "", "blog.settings.error.save"
		}
		return slug, ""
	}

	slug := slugFromTitle(requested)
	if slug == "" {
		return "", "blog.settings.pages.error.slug"
	}
	if reservedPageSlugs[slug] {
		return "", "blog.settings.pages.error.slug_reserved"
	}
	available, err := s.store.SlugAvailable(ctx, userID, slug, postID)
	if err != nil {
		s.logger.Error("checking page slug", "err", err)
		return "", "blog.settings.error.save"
	}
	if !available {
		return "", "blog.settings.pages.error.slug_taken"
	}
	return slug, ""
}

// handleRenamePage lets the owner set a page's slug directly, rather than
// leaving it derived from the title (unlike ordinary posts).
func (s *Server) handleRenamePage(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireVerified(w, r, user) {
		return
	}
	if s.store == nil {
		s.renderSettingsPagesError(w, r, user, "blog.settings.error.db")
		return
	}
	if err := r.ParseForm(); err != nil {
		s.renderSettingsPagesError(w, r, user, "blog.settings.error.form")
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	requested := strings.TrimSpace(r.FormValue("slug"))
	if requested == "" {
		s.renderSettingsPagesError(w, r, user, "blog.settings.pages.error.slug")
		return
	}
	slug, errKey := s.resolvePageSlug(r.Context(), user.ID, requested, "", id)
	if errKey != "" {
		s.renderSettingsPagesError(w, r, user, errKey)
		return
	}

	if err := s.store.RenamePageSlug(r.Context(), user.ID, id, slug); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		s.logger.Error("renaming page", "err", err)
		s.renderSettingsPagesError(w, r, user, "blog.settings.error.save")
		return
	}

	s.purgePublicCache(user.Username)
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (s *Server) handleDeletePage(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireVerified(w, r, user) {
		return
	}
	if s.store == nil {
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := s.store.DeletePage(r.Context(), user.ID, id); err != nil && !errors.Is(err, store.ErrNotFound) {
		s.logger.Error("deleting page", "err", err)
	} else {
		s.purgePublicCache(user.Username)
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

// renderSettingsPagesError re-renders Settings with an error scoped to the
// Pages section, keeping the rest of the page (profile fields, domain
// panel) populated as it would normally be.
func (s *Server) renderSettingsPagesError(w http.ResponseWriter, r *http.Request, user *store.User, messageKey string) {
	pd := s.newPageData(r, user)
	pd.Title = pd.T("blog.settings.title")
	pd.SEO = noindexSEO()
	view := blogSettingsViewFor(*user, s.baseDomain)
	view.Pages = s.pagesViewFor(r.Context(), user.ID)
	view.MaxPages = store.MaxPagesPerUser
	view.PagesError = pd.T(messageKey)
	pd.BlogSettings = view
	s.renderer.RenderStatus(w, http.StatusBadRequest, "blog_settings.html", pd)
}
