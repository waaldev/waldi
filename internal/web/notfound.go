package web

import (
	"net/http"
)

type notFoundRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (rw *notFoundRecorder) WriteHeader(code int) {
	if rw.wrote {
		return
	}
	rw.status = code
	if code == http.StatusNotFound {
		return
	}
	rw.wrote = true
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *notFoundRecorder) Write(b []byte) (int, error) {
	if rw.status == http.StatusNotFound && !rw.wrote {
		return len(b), nil
	}
	if !rw.wrote {
		rw.wrote = true
		if rw.status == 0 {
			rw.status = http.StatusOK
		}
	}
	return rw.ResponseWriter.Write(b)
}

func unwrapResponseWriter(w http.ResponseWriter) http.ResponseWriter {
	if rw, ok := w.(*notFoundRecorder); ok {
		return rw.ResponseWriter
	}
	return w
}

func (s *Server) renderNotFound(w http.ResponseWriter, r *http.Request) {
	viewer := currentUser(r)
	blogHost := s.isBlogHost(r.Context(), r.Host)

	var pd PageData
	if blogHost != nil && s.store != nil {
		owner, err := s.store.UserByUsername(r.Context(), blogHost.Username)
		if err == nil {
			pd = s.publicBlogPageData(r, owner, viewer)
			blogView := blogViewFromUser(owner, s.baseDomain)
			pd.Blog = &blogView
		} else {
			pd = s.newPageData(r, viewer)
		}
	} else {
		pd = s.newPageData(r, viewer)
	}

	pd.Title = pd.T("error.not_found.title")
	pd.SEO = noindexSEO()
	s.renderer.RenderStatus(unwrapResponseWriter(w), http.StatusNotFound, "404.html", pd)
}

// renderGone answers a deleted post's old URL with 410 instead of 404, so
// visitors (and crawlers) learn the address is intentionally retired rather
// than possibly mistyped.
func (s *Server) renderGone(w http.ResponseWriter, r *http.Request, username string) {
	viewer := currentUser(r)

	var pd PageData
	owner, err := s.store.UserByUsername(r.Context(), username)
	if err == nil {
		pd = s.publicBlogPageData(r, owner, viewer)
		blogView := blogViewFromUser(owner, s.baseDomain)
		pd.Blog = &blogView
	} else {
		pd = s.newPageData(r, viewer)
	}

	pd.Title = pd.T("error.gone.heading")
	pd.SEO = noindexSEO()
	pd.Gone = true
	s.renderer.RenderStatus(w, http.StatusGone, "404.html", pd)
}
