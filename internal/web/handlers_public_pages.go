package web

import (
	"net/http"
	"waldi/internal/i18n"
	"waldi/internal/store"
)

const (
	explorePostLimit   = 30
	exploreWriterLimit = 200
)

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

func (s *Server) serveBlogPostAt(w http.ResponseWriter, r *http.Request, slug string) bool {
	blog := s.isBlogHost(r.Context(), r.Host)
	if blog == nil {
		return false
	}
	s.withPublicBlogCacheHeaders(w, r, func(w http.ResponseWriter, r *http.Request) {
		s.servePublicPost(w, r, blog.Username, slug)
	})
	return true
}

type ExploreView struct {
	Posts   []PostView
	Writers []ExploreWriterView
}

type ExploreWriterView struct {
	Name      string
	Bio       string
	URL       string
	Lang      string
	Dir       string
	PostCount int
}

func (s *Server) handleExplore(w http.ResponseWriter, r *http.Request) {
	if s.serveBlogPostAt(w, r, "explore") {
		return
	}
	pd := s.newPageData(r, currentUser(r))
	pd.Inline = true
	pd.Title = pd.T("explore.title")
	pd.SEO = publicPageSEO(r, s.baseDomain, pd.Lang, r.URL.Path, "explore.title", "seo.explore.description")
	explore, err := s.loadExplore(r)
	if err != nil {
		s.logger.Error("loading explore", "err", err)
		http.Error(w, "explore unavailable", http.StatusInternalServerError)
		return
	}
	pd.Explore = explore
	s.withCacheHeaders(w, r, func(w http.ResponseWriter, r *http.Request) {
		s.renderer.Render(w, "explore.html", pd)
	})
}

func (s *Server) loadExplore(r *http.Request) (*ExploreView, error) {
	view := &ExploreView{}
	if s.store == nil {
		return view, nil
	}
	writers, err := s.store.ExploreWriters(r.Context(), exploreWriterLimit)
	if err != nil {
		return nil, err
	}
	posts, err := s.store.ExplorePosts(r.Context(), explorePostLimit)
	if err != nil {
		return nil, err
	}

	owners := make(map[string]store.User, len(writers))
	for _, wr := range writers {
		owners[wr.User.Username] = wr.User
		lang := postLang(blogPageLang(wr.User))
		view.Writers = append(view.Writers, ExploreWriterView{
			Name:      publicDisplayName(wr.User),
			Bio:       metaDescription(wr.User.Bio),
			URL:       PublicBlogURLForOwner(r, s.baseDomain, wr.User, "/"),
			Lang:      lang,
			Dir:       i18n.Dir(lang),
			PostCount: wr.PostCount,
		})
	}
	for _, p := range posts {
		owner, ok := owners[p.Username]
		if !ok {
			owner = store.User{Username: p.Username}
		}
		pv := postView(p)
		pv.URL = PublicBlogURLForOwner(r, s.baseDomain, owner, "/"+p.Slug)
		pv.BlogURL = PublicBlogURLForOwner(r, s.baseDomain, owner, "/")
		pv.Excerpt = postExcerpt(p.HTML, 200)
		view.Posts = append(view.Posts, pv)
	}
	return view, nil
}
