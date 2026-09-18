package web

import (
	"context"
	"net/http"
	"strings"
)

type forcedLocaleKey struct{}

const (
	defaultPublicLang = "en"
	prefixedLang      = "fa"
)

var localizedLangs = []string{defaultPublicLang, prefixedLang}

var localizedPaths = []string{"/how-it-works", "/explore", "/write/invite"}

type LangAlternate struct {
	Lang string
	URL  string
}

func withForcedLocale(ctx context.Context, lang string) context.Context {
	return context.WithValue(ctx, forcedLocaleKey{}, lang)
}

func forcedLocale(r *http.Request) string {
	lang, _ := r.Context().Value(forcedLocaleKey{}).(string)
	return lang
}

func localePrefix(lang string) string {
	if lang == prefixedLang {
		return "/" + prefixedLang
	}
	return ""
}

func localizedPath(lang, path string) string {
	prefix := localePrefix(lang)
	if prefix == "" {
		return path
	}
	if path == "/" {
		return prefix
	}
	return prefix + path
}

func unlocalizedPath(path string) string {
	prefix := localePrefix(prefixedLang)
	if path == prefix {
		return "/"
	}
	if rest, ok := strings.CutPrefix(path, prefix+"/"); ok {
		return "/" + rest
	}
	return path
}

func langAlternates(r *http.Request, baseDomain, path string) []LangAlternate {
	base := strings.TrimSuffix(appBaseURL(r, baseDomain), "/")
	path = unlocalizedPath(path)
	alternates := make([]LangAlternate, 0, len(localizedLangs)+1)
	for _, lang := range localizedLangs {
		alternates = append(alternates, LangAlternate{Lang: lang, URL: base + localizedPath(lang, path)})
	}
	return append(alternates, LangAlternate{Lang: "x-default", URL: base + localizedPath(defaultPublicLang, path)})
}

func (s *Server) routeLocalizedPages() {
	prefix := localePrefix(prefixedLang)
	s.mux.HandleFunc("GET "+prefix, s.localized(prefixedLang, s.handleLocalizedHome))
	s.mux.HandleFunc("GET "+prefix+"/how-it-works", s.localized(prefixedLang, s.handleHowItWorks))
	s.mux.HandleFunc("GET "+prefix+"/explore", s.localized(prefixedLang, s.handleExplore))
	s.mux.HandleFunc("GET "+prefix+"/write/invite", s.localized(prefixedLang, s.handleWriteInviteForm))
}

func (s *Server) localized(lang string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == localePrefix(lang) && s.serveBlogPostAt(w, r, lang) {
			return
		}
		if s.isBlogHost(r.Context(), r.Host) != nil {
			http.NotFound(w, r)
			return
		}
		next(w, r.WithContext(withForcedLocale(r.Context(), lang)))
	}
}

func (s *Server) defaultLocalized(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if currentUser(r) != nil || s.isBlogHost(r.Context(), r.Host) != nil {
			next(w, r)
			return
		}
		next(w, r.WithContext(withForcedLocale(r.Context(), defaultPublicLang)))
	}
}

func (s *Server) handleLocalizedHome(w http.ResponseWriter, r *http.Request) {
	if currentUser(r) != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.handleHome(w, r)
}

func otherLang(lang string) string {
	if lang == prefixedLang {
		return defaultPublicLang
	}
	return prefixedLang
}
