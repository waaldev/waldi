package web

import "net/http"

func (s *Server) handleWhy(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "https://amin.waldi.blog/serendipity", http.StatusMovedPermanently)
}

func (s *Server) handleHowItWorks(w http.ResponseWriter, r *http.Request) {
	s.renderPublicInfoPage(w, r, "how_it_works.html", "how.title", "seo.how.description")
}

func (s *Server) renderPublicInfoPage(w http.ResponseWriter, r *http.Request, templateName, titleKey, descriptionKey string) {
	pd := s.newPageData(r, currentUser(r))
	pd.Inline = true
	pd.Title = pd.T(titleKey)
	pd.SEO = publicPageSEO(r, s.baseDomain, pd.Lang, r.URL.Path, titleKey, descriptionKey)
	s.withCacheHeaders(w, r, func(w http.ResponseWriter, r *http.Request) {
		s.renderer.Render(w, templateName, pd)
	})
}
