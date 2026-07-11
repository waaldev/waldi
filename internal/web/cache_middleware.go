package web

import (
	"bytes"
	"net/http"
	"waldi/internal/i18n"
)

const (
	publicCacheControl  = "public, max-age=86400, stale-while-revalidate=604800"
	privateCacheControl = "private, max-age=0, must-revalidate"
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
	w.Header().Set("Cache-Control", cacheControlForLocale(r))
	w.Header().Set("Vary", "Cookie, CF-IPCountry")
	w.WriteHeader(capture.status)
	_, _ = capture.buf.WriteTo(w)
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
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		next.ServeHTTP(w, r)
	})
}
