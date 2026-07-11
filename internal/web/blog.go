package web

import (
	"net/http"
	"strings"
	"waldi/internal/i18n"
	"waldi/internal/store"
)

func appBaseURL(r *http.Request, baseDomain string) string {
	scheme := requestScheme(r)
	port := requestPort(r)
	if isLocalDevHost(r.Host) {
		return scheme + "://" + localBlogDomain(r.Host) + port
	}
	return scheme + "://" + baseDomain + port
}

func publicAuthorName(user store.User) string {
	if name := strings.TrimSpace(user.AuthorName); name != "" {
		return name
	}
	return publicDisplayName(user)
}

func publicDisplayName(user store.User) string {
	if name := strings.TrimSpace(user.DisplayName); name != "" {
		return name
	}
	return user.Username
}

// writerLabel picks one public writer label: author name, else blog name, else username.
func writerLabel(authorName, displayName, username string) string {
	if name := strings.TrimSpace(authorName); name != "" {
		return name
	}
	if name := strings.TrimSpace(displayName); name != "" {
		return name
	}
	return strings.TrimSpace(username)
}

func writerLabelFromUser(user store.User) string {
	return writerLabel(user.AuthorName, user.DisplayName, user.Username)
}

func blogPageLang(owner store.User) string {
	if i18n.Supported(owner.BlogLang) {
		return owner.BlogLang
	}
	if i18n.Supported(owner.Locale) {
		return owner.Locale
	}
	return i18n.Default
}

func (s *Server) publicBlogPageData(r *http.Request, owner store.User, viewer *store.User) PageData {
	lang := blogPageLang(owner)
	blog := s.isBlogHost(r.Context(), r.Host)
	onCustomDomain := blog != nil && blog.Custom
	return PageData{
		Lang:             lang,
		Dir:              i18n.Dir(lang),
		CurrentUser:      s.userView(r, viewer),
		AppBaseURL:       appBaseURL(r, s.baseDomain),
		PageURL:          pageURL(r),
		LoginURL:         s.loginURL(r, s.baseDomain, pageURL(r)),
		BridgeSession:    onCustomDomain && viewer == nil,
		DevSessionBridge: isLocalDevHost(r.Host),
	}
}

func blogViewFromUser(owner store.User, baseDomain string) BlogView {
	title := owner.Username + "." + baseDomain
	if domain, ok := owner.ActiveCustomDomain(); ok {
		title = domain
	}
	return BlogView{
		Username:    owner.Username,
		DisplayName: publicDisplayName(owner),
		AuthorName:  publicAuthorName(owner),
		WriterLabel: writerLabelFromUser(owner),
		Bio:         strings.TrimSpace(owner.Bio),
		Lang:        blogPageLang(owner),
		Title:       title,
		Empty:       true,
	}
}
