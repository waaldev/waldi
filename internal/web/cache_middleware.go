package web

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"waldi/internal/i18n"
)

const (
	publicCacheControl           = "public, max-age=0, must-revalidate, s-maxage=2592000"
	privateCacheControl          = "private, max-age=0, must-revalidate"
	privateSessionCacheControl   = "private, no-store"
	staticRevalidateCacheControl = "public, max-age=86400, must-revalidate"
)

type cacheCapture struct {
	http.ResponseWriter
	status int
	buf    bytes.Buffer
	header http.Header
}

func newCacheCapture(w http.ResponseWriter) *cacheCapture {
	return &cacheCapture{
		ResponseWriter: w,
		status:         http.StatusOK,
		header:         make(http.Header),
	}
}

func (c *cacheCapture) Header() http.Header {
	return c.header
}

func (c *cacheCapture) WriteHeader(status int) {
	c.status = status
}

func (c *cacheCapture) Write(b []byte) (int, error) {
	return c.buf.Write(b)
}

// withCacheHeaders tags anonymous, successful HTML responses with a public
// Cache-Control (and Vary) header so the CDN can cache and revalidate them.
// It does not retain a copy of the response itself; Cloudflare is the only
// cache layer.
func (s *Server) withCacheHeaders(w http.ResponseWriter, r *http.Request, next func(http.ResponseWriter, *http.Request)) {
	if r.Method != http.MethodGet || hasSessionCookie(r) {
		next(w, r)
		return
	}
	s.captureCacheableHTML(w, r, next, cacheControlForLocale(r), "Cookie, CF-IPCountry", false)
}

// withPublicBlogCacheHeaders caches the identity-neutral public blog shell
// equally for anonymous and signed-in readers. Reader-specific controls are
// loaded separately from no-store endpoints.
func (s *Server) withPublicBlogCacheHeaders(w http.ResponseWriter, r *http.Request, next func(http.ResponseWriter, *http.Request)) {
	if r.Method != http.MethodGet {
		next(w, r)
		return
	}
	s.captureCacheableHTML(w, r, next, publicCacheControl, "", true)
}

func (s *Server) captureCacheableHTML(w http.ResponseWriter, r *http.Request, next func(http.ResponseWriter, *http.Request), cacheControl, vary string, stripCookies bool) {
	capture := newCacheCapture(w)
	next(capture, r)

	if capture.status != http.StatusOK || !isHTMLResponse(capture.header) || capture.buf.Len() == 0 {
		flushCapture(w, capture)
		return
	}

	for k, vals := range capture.header {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	if stripCookies {
		w.Header().Del("Set-Cookie")
	}
	w.Header().Set("Cache-Control", cacheControl)
	if vary != "" {
		w.Header().Set("Vary", vary)
	}
	etag := etagForBody(capture.buf.Bytes())
	w.Header().Set("ETag", etag)
	if etagMatches(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.WriteHeader(capture.status)
	_, _ = capture.buf.WriteTo(w)
}

func etagForBody(body []byte) string {
	sum := sha256.Sum256(body)
	return `"` + hex.EncodeToString(sum[:8]) + `"`
}

func etagMatches(header, etag string) bool {
	for value := range strings.SplitSeq(header, ",") {
		value = strings.TrimSpace(value)
		if value == "*" || value == etag {
			return true
		}
	}
	return false
}

func writePublicResource(w http.ResponseWriter, r *http.Request, contentType string, body []byte) {
	w.Header().Del("Set-Cookie")
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", publicCacheControl)
	etag := etagForBody(body)
	w.Header().Set("ETag", etag)
	if etagMatches(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	_, _ = w.Write(body)
}

func hasSessionCookie(r *http.Request) bool {
	c, err := r.Cookie(sessionCookie)
	return err == nil && c.Value != ""
}

func cacheControlForLocale(r *http.Request) string {
	if c, err := r.Cookie(localeCookie); err == nil && i18n.Supported(c.Value) {
		return privateCacheControl
	}
	return publicCacheControl
}

func isHTMLResponse(h http.Header) bool {
	ct := h.Get("Content-Type")
	return ct == "" || ct == "text/html; charset=utf-8"
}

func flushCapture(w http.ResponseWriter, c *cacheCapture) {
	for k, vals := range c.header {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(c.status)
	_, _ = c.buf.WriteTo(w)
}

func staticCacheControl(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cacheControl := staticRevalidateCacheControl
		if r.URL.Query().Get("v") != "" || strings.HasPrefix(r.URL.Path, "/static/uploads/") {
			cacheControl = "public, max-age=31536000, immutable"
		}
		w.Header().Set("Cache-Control", cacheControl)
		next.ServeHTTP(w, r)
	})
}
