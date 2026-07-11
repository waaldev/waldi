package web

import (
	"context"
	"net/http"
	"strings"
	"waldi/internal/i18n"
	"waldi/internal/store"
)

const (
	maxDisplayNameLen = 80
	maxAuthorNameLen  = 80
	maxBioLen         = 500
)

func blogSettingsViewFor(user store.User, baseDomain string) *BlogSettingsView {
	view := &BlogSettingsView{
		DisplayName: user.DisplayName,
		AuthorName:  user.AuthorName,
		Bio:         user.Bio,
		BlogLang:    user.BlogLang,
		Locale:      user.Locale,
		CNAMETarget: customDomainCNAMETarget(baseDomain),
	}
	if domain, ok := user.ActiveCustomDomain(); ok {
		view.CustomDomain = domain
		view.CustomDomainVerified = true
	} else if user.CustomDomain != nil {
		view.CustomDomain = *user.CustomDomain
		if user.CustomDomainToken != nil {
			view.ChallengeHost = challengeHostFor(*user.CustomDomain)
			view.ChallengeToken = *user.CustomDomainToken
		}
	}
	return view
}

// pagesViewFor loads a user's static pages for the Settings "Pages"
// section, ordered as they'd appear in the blog's nav.
func (s *Server) pagesViewFor(ctx context.Context, userID int64) []PageRowView {
	if s.store == nil {
		return nil
	}
	pages, err := s.store.PagesByUser(ctx, userID)
	if err != nil {
		s.logger.Error("loading pages", "err", err)
		return nil
	}
	rows := make([]PageRowView, 0, len(pages))
	for i, p := range pages {
		rows = append(rows, PageRowView{
			ID:      p.ID,
			Title:   p.Title,
			Slug:    p.Slug,
			URL:     "/" + p.Slug,
			Status:  p.Status,
			IsFirst: i == 0,
			IsLast:  i == len(pages)-1,
		})
	}
	return rows
}

func (s *Server) handleBlogSettingsForm(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	pd := s.newPageData(r, user)
	pd.Title = pd.T("blog.settings.title")
	pd.SEO = noindexSEO()
	pd.BlogSettings = blogSettingsViewFor(*user, s.baseDomain)
	if !i18n.Supported(pd.BlogSettings.Locale) {
		pd.BlogSettings.Locale, _ = resolveLocale(r, user)
	}
	pd.BlogSettings.Saved = r.URL.Query().Get("saved") == "1"
	pd.BlogSettings.Pages = s.pagesViewFor(r.Context(), user.ID)
	pd.BlogSettings.MaxPages = store.MaxPagesPerUser
	switch r.URL.Query().Get("domain") {
	case "set":
		pd.BlogSettings.DomainNotice = pd.T("blog.settings.domain.notice.set")
	case "verified":
		pd.BlogSettings.DomainNotice = pd.T("blog.settings.domain.notice.verified")
	case "removed":
		pd.BlogSettings.DomainNotice = pd.T("blog.settings.domain.notice.removed")
	}
	switch r.URL.Query().Get("pages") {
	case "limit":
		pd.BlogSettings.PagesError = pd.T("blog.settings.pages.error.limit")
	}
	switch r.URL.Query().Get("password") {
	case "saved":
		pd.BlogSettings.PasswordSaved = true
	}
	s.renderer.Render(w, "blog_settings.html", pd)
}

func (s *Server) handleBlogSettingsSave(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if s.store == nil {
		s.renderBlogSettingsError(w, r, "blog.settings.error.db")
		return
	}
	if err := r.ParseForm(); err != nil {
		s.renderBlogSettingsError(w, r, "blog.settings.error.form")
		return
	}

	displayName := strings.TrimSpace(r.FormValue("display_name"))
	authorName := strings.TrimSpace(r.FormValue("author_name"))
	bio := strings.TrimSpace(r.FormValue("bio"))
	blogLang := strings.TrimSpace(r.FormValue("blog_lang"))

	if len(displayName) > maxDisplayNameLen {
		s.renderBlogSettingsError(w, r, "blog.settings.error.display_name")
		return
	}
	if len(authorName) > maxAuthorNameLen {
		s.renderBlogSettingsError(w, r, "blog.settings.error.author_name")
		return
	}
	if len(bio) > maxBioLen {
		s.renderBlogSettingsError(w, r, "blog.settings.error.bio")
		return
	}
	if !i18n.Supported(blogLang) {
		s.renderBlogSettingsError(w, r, "blog.settings.error.lang")
		return
	}

	if err := s.store.UpdateBlogProfile(r.Context(), user.ID, displayName, authorName, bio, blogLang); err != nil {
		s.logger.Error("updating blog profile", "err", err)
		s.renderBlogSettingsError(w, r, "blog.settings.error.save")
		return
	}

	s.purgePublicCache(user.Username)
	http.Redirect(w, r, "/settings?saved=1", http.StatusSeeOther)
}

func (s *Server) renderBlogSettingsError(w http.ResponseWriter, r *http.Request, messageKey string) {
	user := currentUser(r)
	pd := s.newPageData(r, user)
	pd.Title = pd.T("blog.settings.title")
	pd.SEO = noindexSEO()
	view := &BlogSettingsView{
		DisplayName: strings.TrimSpace(r.FormValue("display_name")),
		AuthorName:  strings.TrimSpace(r.FormValue("author_name")),
		Bio:         strings.TrimSpace(r.FormValue("bio")),
		BlogLang:    strings.TrimSpace(r.FormValue("blog_lang")),
		Error:       pd.T(messageKey),
	}
	if user != nil {
		domainView := blogSettingsViewFor(*user, s.baseDomain)
		view.CustomDomain = domainView.CustomDomain
		view.CustomDomainVerified = domainView.CustomDomainVerified
		view.ChallengeHost = domainView.ChallengeHost
		view.ChallengeToken = domainView.ChallengeToken
		view.CNAMETarget = domainView.CNAMETarget
		view.Pages = s.pagesViewFor(r.Context(), user.ID)
		view.MaxPages = store.MaxPagesPerUser
	}
	pd.BlogSettings = view
	s.renderer.RenderStatus(w, http.StatusBadRequest, "blog_settings.html", pd)
}
