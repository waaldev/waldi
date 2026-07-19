package web

import (
	"errors"
	"html/template"
	"net/http"
	"time"
	"waldi/internal/i18n"
	"waldi/internal/jobs"
	"waldi/internal/store"
)

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	blog := s.isBlogHost(r.Context(), r.Host)
	if blog != nil {
		s.withCacheHeaders(w, r, func(w http.ResponseWriter, r *http.Request) {
			s.renderPublicProfile(w, r, blog.Username)
		})
		return
	}

	user := currentUser(r)
	pd := s.newPageData(r, user)
	if user == nil {
		pd.Inline = true
	}
	feed := &FeedView{Empty: true}
	if user != nil && s.store != nil {
		since := time.Now().Add(-feedWindow)
		posts, err := s.store.FeedPosts(r.Context(), user.ID, since, feedHardLimit)
		if err != nil {
			s.logger.Error("loading feed", "err", err)
			http.Error(w, pd.T("error.home"), http.StatusInternalServerError)
			return
		}
		views := s.postViewsWithURLs(r, posts)
		feed.Empty = len(views) == 0
		feed.Days = buildFeedDays(views, pd.Lang)

		day := today()
		floor, err := s.store.WildcardImpressionFloor(r.Context())
		if err != nil {
			s.logger.Error("loading wildcard impression floor", "err", err)
			floor = jobs.DefaultWildcardImpressionFloor
		}
		if err := jobs.EnsureUserWildcard(r.Context(), s.store, user.ID, user.Locale, day, floor); err != nil {
			s.logger.Error("ensuring wildcard", "err", err)
		}

		wildcard, err := s.store.AssignedWildcard(r.Context(), user.ID, day)
		if err == nil {
			view := postView(wildcard)
			view.URL = PublicBlogURL(r, s.baseDomain, wildcard.Username, "/"+wildcard.Slug+"?src=wildcard")
			view.BlogURL = PublicBlogURL(r, s.baseDomain, wildcard.Username, "/")
			view.Excerpt = postExcerpt(wildcard.HTML, 200)
			feed.Wildcard = &view
		} else if !errors.Is(err, store.ErrNotFound) {
			s.logger.Error("loading wildcard", "err", err)
		}
	}

	pd.Title = pd.T("home.title")
	if user == nil {
		pd.SEO = landingSEO(r, s.baseDomain, pd.Lang)
	} else {
		pd.SEO = noindexSEO()
	}
	pd.Feed = feed
	if user == nil {
		s.withCacheHeaders(w, r, func(w http.ResponseWriter, r *http.Request) {
			s.renderer.Render(w, "home.html", pd)
		})
		return
	}
	s.renderer.Render(w, "home.html", pd)
}

// handleReadRandom sends the visitor to a random published post in their
// locale's language, so "just read" lands on real writing instead of a form.
// Falls back to the default language, then the landing page, when nothing
// matches.
func (s *Server) handleReadRandom(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if s.store == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	lang, _ := resolveLocale(r, currentUser(r))
	p, err := s.store.RandomPublishedPost(r.Context(), lang)
	if errors.Is(err, store.ErrNotFound) && lang != i18n.Default {
		p, err = s.store.RandomPublishedPost(r.Context(), i18n.Default)
	}
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			s.logger.Error("picking random post", "err", err)
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, PublicBlogURL(r, s.baseDomain, p.Username, "/"+p.Slug), http.StatusSeeOther)
}

// handleYou renders the owner's in-app blog preview: the same content a
// guest would see on the public blog, but with the full app header and
// owner-only affordances (Settings/Drafts), distinct from the minimal
// public view served on the actual username.baseDomain host.
func (s *Server) handleYou(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if s.store == nil {
		http.Error(w, "unavailable", http.StatusInternalServerError)
		return
	}

	blogView, err := s.buildBlogView(r, *user, user, true)
	if errors.Is(err, errBadCursor) {
		pd := s.newPageData(r, user)
		http.Error(w, pd.T("error.bad_cursor"), http.StatusBadRequest)
		return
	}
	if err != nil {
		s.logger.Error("loading blog preview", "err", err)
		pd := s.newPageData(r, user)
		http.Error(w, pd.T("error.profile"), http.StatusInternalServerError)
		return
	}

	pd := s.newPageData(r, user)
	pd.Title = pd.T("profile.title", blogView.DisplayName)
	pd.SEO = noindexSEO()
	pd.Blog = &blogView
	s.renderer.Render(w, "you.html", pd)
}

// handleYouPost renders the owner's in-app view of one of their own
// published posts: the same body a public visitor sees, but with the app
// header and owner-only affordances (edit/change date/delete) instead of
// follow/letter actions. This keeps the public post.html free of any
// viewer-identity branching.
func (s *Server) handleYouPost(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if s.store == nil {
		http.Error(w, "unavailable", http.StatusInternalServerError)
		return
	}

	slug := r.PathValue("slug")
	p, err := s.store.PublishedPostByUsernameAndSlug(r.Context(), user.Username, slug)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.logger.Error("loading owner post", "err", err)
		pd := s.newPageData(r, user)
		http.Error(w, pd.T("error.post"), http.StatusInternalServerError)
		return
	}

	view := postView(p)
	view.URL = PublicBlogURLForOwner(r, s.baseDomain, *user, "/"+p.Slug)
	view.DateError = r.URL.Query().Get("date_error") == "1"

	pd := s.newPageData(r, user)
	pd.Title = p.Title
	pd.SEO = noindexSEO()
	pd.Blog = &BlogView{
		Username:    user.Username,
		DisplayName: publicDisplayName(*user),
		AuthorName:  publicAuthorName(*user),
		WriterLabel: writerLabelFromUser(*user),
		PublicURL:   PublicBlogURLForOwner(r, s.baseDomain, *user, "/"),
	}
	pd.Post = &view
	s.renderer.Render(w, "you-post.html", pd)
}

func (s *Server) handleSkipWildcard(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if s.store == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	day := today()
	if err := s.store.SkipWildcard(r.Context(), user.ID, day); err != nil {
		s.logger.Error("skipping wildcard", "err", err)
	} else {
		floor, err := s.store.WildcardImpressionFloor(r.Context())
		if err != nil {
			s.logger.Error("loading wildcard impression floor", "err", err)
			floor = jobs.DefaultWildcardImpressionFloor
		}
		if err := jobs.EnsureUserWildcard(r.Context(), s.store, user.ID, user.Locale, day, floor); err != nil {
			s.logger.Error("rerolling wildcard", "err", err)
		}
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handlePublicPost(w http.ResponseWriter, r *http.Request) {
	blog := s.isBlogHost(r.Context(), r.Host)
	if blog == nil {
		http.NotFound(w, r)
		return
	}
	s.withCacheHeaders(w, r, func(w http.ResponseWriter, r *http.Request) {
		s.servePublicPost(w, r, blog.Username, r.PathValue("slug"))
	})
}

func (s *Server) servePublicPost(w http.ResponseWriter, r *http.Request, username, slug string) {
	if s.store == nil {
		http.NotFound(w, r)
		return
	}

	p, err := s.store.PublishedPostByUsernameAndSlug(r.Context(), username, slug)
	if errors.Is(err, store.ErrNotFound) {
		gone, goneErr := s.store.WasPostDeleted(r.Context(), username, slug)
		if goneErr != nil {
			s.logger.Error("checking deleted post", "err", goneErr)
		}
		if gone {
			s.renderGone(w, r, username)
			return
		}
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.logger.Error("loading public post", "err", err)
		lang, _ := resolveLocale(r, currentUser(r))
		http.Error(w, i18n.T(lang, "error.post"), http.StatusInternalServerError)
		return
	}

	view := postView(p)
	view.BlogURL = "/"
	view.ImpressionID = s.createPostImpression(w, r, p.ID, p.UserID)
	view.Subscribed = r.URL.Query().Get("subscribed") == "1"
	view.LetterSent = r.URL.Query().Get("letter") == "sent"
	user := currentUser(r)
	if user != nil {
		view.CanFollow = user.ID != p.UserID
		if view.CanFollow {
			following, err := s.store.IsFollowing(r.Context(), user.ID, p.UserID)
			if err != nil {
				s.logger.Error("loading follow state", "err", err)
			}
			view.Following = following

			completed, err := s.store.CompletedReadingsCount(r.Context(), user.ID)
			if err != nil {
				s.logger.Error("checking completed readings", "err", err)
			}
			view.CanSendLetters = completed >= minCompletedReadingsForLetters
		}
	}

	owner, err := s.store.UserByUsername(r.Context(), username)
	if err != nil {
		s.logger.Error("loading post author", "err", err)
		lang, _ := resolveLocale(r, user)
		http.Error(w, i18n.T(lang, "error.post"), http.StatusInternalServerError)
		return
	}

	pd := s.publicBlogPageData(r, owner, user)
	pd.Title = p.Title
	pd.SEO = postSEO(r, s.baseDomain, owner, p)
	blogTitle := username + "." + s.baseDomain
	if domain, ok := owner.ActiveCustomDomain(); ok {
		blogTitle = domain
	}
	pd.Blog = &BlogView{
		Username:    owner.Username,
		DisplayName: publicDisplayName(owner),
		AuthorName:  publicAuthorName(owner),
		WriterLabel: writerLabelFromUser(owner),
		Title:       blogTitle,
	}
	pd.Post = &view
	s.renderer.Render(w, "post.html", pd)
}

func (s *Server) renderPublicProfile(w http.ResponseWriter, r *http.Request, username string) {
	viewer := currentUser(r)
	if s.store == nil {
		http.NotFound(w, r)
		return
	}

	owner, err := s.store.UserByUsername(r.Context(), username)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.logger.Error("loading public profile user", "err", err)
		pd := s.newPageData(r, viewer)
		http.Error(w, pd.T("error.profile"), http.StatusInternalServerError)
		return
	}

	blogView, err := s.buildBlogView(r, owner, viewer, false)
	if errors.Is(err, errBadCursor) {
		pd := s.publicBlogPageData(r, owner, viewer)
		http.Error(w, pd.T("error.bad_cursor"), http.StatusBadRequest)
		return
	}
	if err != nil {
		s.logger.Error("loading public profile", "err", err)
		pd := s.publicBlogPageData(r, owner, viewer)
		http.Error(w, pd.T("error.profile"), http.StatusInternalServerError)
		return
	}

	pd := s.publicBlogPageData(r, owner, viewer)
	pd.Title = pd.T("profile.title", blogView.DisplayName)
	pd.SEO = blogSEO(r, s.baseDomain, owner)
	pd.Blog = &blogView
	s.renderer.Render(w, "profile.html", pd)
}

var errBadCursor = errors.New("bad page cursor")

// buildBlogView assembles the BlogView shared by the public blog profile
// page (guests, on username.baseDomain) and the owner's in-app "You"
// preview at /you — same posts/archives/pages; the owner template adds
// settings/drafts affordances instead of follow.
func (s *Server) buildBlogView(r *http.Request, owner store.User, viewer *store.User, isOwner bool) (BlogView, error) {
	blogView := blogViewFromUser(owner, s.baseDomain)
	blogView.IsOwner = isOwner
	blogView.PublicURL = PublicBlogURLForOwner(r, s.baseDomain, owner, "/")

	pageSize := publishedPageSize
	cursor, err := parsePageCursor(r)
	if err != nil {
		return BlogView{}, errBadCursor
	}

	rawPosts, err := s.store.PublishedPostsByUsername(r.Context(), owner.Username, pageSize+1, cursor)
	if err != nil {
		return BlogView{}, err
	}
	posts, hasMore := trimPage(rawPosts, pageSize)
	blogView.Posts = postViewsForBlog(posts, blogView.Lang)
	for i := range blogView.Posts {
		if isOwner {
			blogView.Posts[i].URL = "/you/" + blogView.Posts[i].Slug
		} else {
			blogView.Posts[i].URL = PublicBlogURLForOwner(r, s.baseDomain, owner, "/"+blogView.Posts[i].Slug)
		}
	}
	blogView.Archives = buildArchives(blogView.Posts)
	blogView.Empty = len(posts) == 0 && !cursor.Active()
	olderPath := "/"
	if isOwner {
		olderPath = "/you"
	}
	blogView.OlderURL = publishedOlderURL(olderPath, posts, hasMore)

	pages, err := s.store.PublishedPagesByUsername(r.Context(), owner.Username)
	if err != nil {
		return BlogView{}, err
	}
	for _, p := range pages {
		blogView.Pages = append(blogView.Pages, PageNavView{
			Title: p.Title,
			URL:   PublicBlogURLForOwner(r, s.baseDomain, owner, "/"+p.Slug),
		})
	}

	if !isOwner && viewer != nil && viewer.ID != owner.ID {
		blogView.CanFollow = true
		following, err := s.store.IsFollowing(r.Context(), viewer.ID, owner.ID)
		if err != nil {
			s.logger.Error("loading follow state", "err", err)
		}
		blogView.Following = following
	}

	return blogView, nil
}

func postViews(posts []store.Post) []PostView {
	views := make([]PostView, 0, len(posts))
	for _, p := range posts {
		views = append(views, postView(p))
	}
	return views
}

func (s *Server) postViewsWithURLs(r *http.Request, posts []store.Post) []PostView {
	views := make([]PostView, 0, len(posts))
	for _, p := range posts {
		views = append(views, s.postViewWithURL(r, p))
	}
	return views
}

func (s *Server) postViewWithURL(r *http.Request, p store.Post) PostView {
	view := postView(p)
	view.URL = PublicBlogURL(r, s.baseDomain, p.Username, "/"+p.Slug+"?src=feed")
	view.BlogURL = PublicBlogURL(r, s.baseDomain, p.Username, "/")
	view.Excerpt = postExcerpt(p.HTML, 200)
	return view
}

func postView(p store.Post) PostView {
	published := ""
	publishedISO := ""
	modifiedISO := p.UpdatedAt.UTC().Format(time.RFC3339)
	lang := postLang(p.BlogLang)
	publishedInput := ""
	if p.PublishedAt != nil {
		published = formatArticleDate(*p.PublishedAt, lang)
		publishedISO = p.PublishedAt.UTC().Format(time.RFC3339)
		publishedInput = formatDate(*p.PublishedAt, lang)
	}
	html := p.HTML
	return PostView{
		ID:               p.ID,
		UserID:           p.UserID,
		Username:         p.Username,
		WriterLabel:      writerLabel(p.AuthorName, p.DisplayName, p.Username),
		Title:            p.Title,
		Slug:             p.Slug,
		HTML:             template.HTML(html),
		DocJSON:          template.JS(p.Doc),
		Status:           p.Status,
		Lang:             lang,
		Dir:              i18n.Dir(lang),
		PublishedAt:      published,
		PublishedAtISO:   publishedISO,
		PublishedAtInput: publishedInput,
		ModifiedAtISO:    modifiedISO,
		URL:              "/" + p.Slug,
	}
}

func (s *Server) createPostImpression(w http.ResponseWriter, r *http.Request, postID, authorID int64) int64 {
	if s.store == nil {
		return 0
	}
	if user := currentUser(r); user != nil && user.ID == authorID {
		return 0
	}

	readerKey, cookie := readerKeyFromRequest(r, s.baseDomain)
	if cookie != nil {
		http.SetCookie(w, cookie)
	}

	var userID *int64
	if user := currentUser(r); user != nil {
		id := user.ID
		userID = &id
	}

	source := impressionSourceFromRequest(r)
	id, err := s.store.EnsureImpression(r.Context(), postID, readerKey, userID, source)
	if err != nil {
		s.logger.Error("creating impression", "post_id", postID, "source", source, "err", err)
		return 0
	}
	return id
}

func impressionSourceFromRequest(r *http.Request) store.ImpressionSource {
	switch r.URL.Query().Get("src") {
	case "feed":
		return store.ImpressionFeed
	case "wildcard":
		return store.ImpressionWildcard
	default:
		return store.ImpressionDirect
	}
}

func today() time.Time {
	now := time.Now()
	y, m, d := now.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, now.Location())
}
