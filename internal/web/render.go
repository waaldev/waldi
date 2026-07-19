package web

import (
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"waldi/internal/i18n"
	"waldi/internal/store"
)

type Renderer struct {
	templates *template.Template
}

// inlineAssets are small static files whose content is embedded directly
// into public-page HTML (via the {{siteX}} template funcs below) so a cold
// visit to a public page needs only one request. Read once at startup from
// the same fs.FS the templates are parsed from.
type inlineAssets struct {
	mainCSS  template.CSS
	fontsCSS template.CSS
	themeJS  template.JS
	postJS   template.JS
}

func loadInlineAssets(files fs.FS) (inlineAssets, error) {
	read := func(name string) (string, error) {
		b, err := fs.ReadFile(files, name)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	mainCSS, err := read("web/static/css/main.css")
	if err != nil {
		return inlineAssets{}, err
	}
	mainCSS = minifyCSS(mainCSS)
	fontsCSS, err := read("web/static/css/fonts.css")
	if err != nil {
		return inlineAssets{}, err
	}
	fontsCSS = minifyCSS(fontsCSS)
	themeJS, err := read("web/static/js/theme.js")
	if err != nil {
		return inlineAssets{}, err
	}
	postJS, err := read("web/static/js/post.js")
	if err != nil {
		return inlineAssets{}, err
	}
	return inlineAssets{
		mainCSS:  template.CSS(mainCSS),
		fontsCSS: template.CSS(fontsCSS),
		themeJS:  template.JS(themeJS),
		postJS:   template.JS(postJS),
	}, nil
}

type PageData struct {
	Title            string
	Lang             string
	Dir              string
	CurrentUser      *UserView
	Blog             *BlogView
	BlogSettings     *BlogSettingsView
	ImportBlogir     *ImportBlogirView
	Auth             *AuthView
	Write            *WriteView
	WriteInvite      *WriteInviteView
	Post             *PostView
	Feed             *FeedView
	Inbox            *InboxView
	SEO              *SEOView
	AppBaseURL       string
	PageURL          string
	LoginURL         string
	BaseDomain       string
	BridgeSession    bool
	DevSessionBridge bool
	NavActive        string
	Gone             bool
	// Inline marks a public, chrome-less page render (blog pages,
	// unsubscribe/resume links) where CSS/JS is embedded directly into the
	// HTML so the page needs only one request to load. App pages leave this
	// false and keep cacheable external <link>/<script src> assets, since a
	// logged-in session navigates many pages and benefits from disk-cache
	// reuse of one shared main.css/theme.js.
	Inline bool
}

type UserView struct {
	Username string
	Name     string
	Email    string
	BlogURL  string
	CanWrite bool
}

type AuthView struct {
	Mode        string
	Heading     string
	SubmitLabel string
	Error       string
	Message     string
	CanResend   bool
	ResetToken  string
	NextURL     string
	InviteCode  string
}

type BlogView struct {
	Username    string
	DisplayName string
	AuthorName  string
	WriterLabel string
	Bio         string
	Lang        string
	Title       string
	PublicURL   string
	Posts       []PostView
	Archives    []ArchiveYear
	Empty       bool
	CanFollow   bool
	Following   bool
	OlderURL    string
	IsOwner     bool
	Pages       []PageNavView
}

// PageNavView is a published static page as it appears in a blog's nav.
type PageNavView struct {
	Title string
	URL   string
}

// PageRowView is one of the owner's static pages as listed in Settings.
type PageRowView struct {
	ID      int64
	Title   string
	Slug    string
	URL     string
	Status  string
	IsFirst bool
	IsLast  bool
}

type ArchiveYear struct {
	Year  string
	Posts []PostView
}

type BlogSettingsView struct {
	DisplayName string
	AuthorName  string
	Bio         string
	BlogLang    string
	Locale      string
	Error       string
	Saved       bool

	CustomDomain         string
	CustomDomainVerified bool
	ChallengeHost        string
	ChallengeToken       string
	CNAMETarget          string
	DomainError          string
	DomainNotice         string

	Pages         []PageRowView
	PagesError    string
	MaxPages      int
	PasswordError string
	PasswordSaved bool
}

// ImportBlogirView backs the hidden /settings/import-blogir page used to
// restore a blog.ir export by hand (not linked from any nav).
type ImportBlogirView struct {
	Error    string
	Imported int
	Skipped  int
	Failed   []ImportBlogirFailureView
	Done     bool
}

type ImportBlogirFailureView struct {
	Title string
	Slug  string
	Err   string
}

type WriteView struct {
	Drafts            []PostView
	Published         []PostView
	PublishedOlderURL string
	Post              *PostView
	Error             string
	Saved             bool
}

type WriteInviteView struct {
	Error   string
	Message string
}

type PostView struct {
	ID               int64
	UserID           int64
	Username         string
	WriterLabel      string
	BlogURL          string
	Title            string
	Slug             string
	HTML             template.HTML
	DocJSON          template.JS
	Status           string
	Lang             string
	Dir              string
	PublishedAt      string
	PublishedAtISO   string
	PublishedAtInput string
	ModifiedAtISO    string
	FeedDate         string
	PublishedYear    string
	Excerpt          string
	WriteMeta        string
	URL              string
	Following        bool
	CanFollow        bool
	CanSendLetters   bool
	ImpressionID     int64
	DateError        bool
	Subscribed       bool
	LetterSent       bool
}

type FeedView struct {
	Days     []FeedDay
	Sample   []PostView
	Wildcard *PostView
	Empty    bool
}

type FeedDay struct {
	Label string
	Posts []PostView
}

type InboxView struct {
	Letters []LetterView
	Letter  *LetterView
	Stats   []StatsView
	Empty   bool
	Error   string
}

type LetterView struct {
	ID              int64
	PostID          int64
	PostTitle       string
	PostSlug        string
	FromUsername    string
	FromWriterLabel string
	FromBlogURL     string
	Body            string
	CreatedAt       string
	Read            bool
}

type StatsView struct {
	PostTitle string
	Sentence  string
}

// minifyCSS strips comments and non-essential whitespace before embedding
// CSS in public-page HTML. It only drops whitespace immediately adjacent to
// { } ; : , — never whitespace between two other tokens (which would
// collapse a descendant-combinator space, e.g. ".a .b", into something
// meaningless) — and never touches the contents of quoted strings (e.g.
// content: " •"), so it's safe on hand-authored CSS without needing a real
// parser.
func minifyCSS(css string) string {
	var out strings.Builder
	out.Grow(len(css))
	runes := []rune(css)
	n := len(runes)
	pendingSpace := false
	afterPunct := true // true at start so leading whitespace is dropped
	for i := 0; i < n; i++ {
		c := runes[i]
		switch {
		case c == '/' && i+1 < n && runes[i+1] == '*':
			i += 2
			for i < n && (runes[i] != '*' || i+1 >= n || runes[i+1] != '/') {
				i++
			}
			i++ // skip trailing '/'
			if !afterPunct {
				pendingSpace = true
			}
		case c == '"' || c == '\'':
			if pendingSpace {
				out.WriteByte(' ')
				pendingSpace = false
			}
			quote := c
			out.WriteRune(c)
			i++
			for i < n {
				out.WriteRune(runes[i])
				if runes[i] == '\\' && i+1 < n {
					i++
					out.WriteRune(runes[i])
				} else if runes[i] == quote {
					break
				}
				i++
			}
			afterPunct = false
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			if !afterPunct {
				pendingSpace = true
			}
		case c == '{' || c == '}' || c == ';' || c == ':' || c == ',':
			pendingSpace = false
			out.WriteRune(c)
			afterPunct = true
		default:
			if pendingSpace {
				out.WriteByte(' ')
				pendingSpace = false
			}
			out.WriteRune(c)
			afterPunct = false
		}
	}
	return strings.TrimSpace(out.String())
}

func NewRenderer(files fs.FS) (*Renderer, error) {
	assets, err := loadInlineAssets(files)
	if err != nil {
		return nil, err
	}
	funcs := template.FuncMap{
		"siteMainCSS":  func() template.CSS { return assets.mainCSS },
		"siteFontsCSS": func() template.CSS { return assets.fontsCSS },
		"siteThemeJS":  func() template.JS { return assets.themeJS },
		"sitePostJS":   func() template.JS { return assets.postJS },
	}
	tmpl, err := template.New("").Funcs(funcs).ParseFS(files, "web/templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Renderer{templates: tmpl}, nil
}

func (r *Renderer) Render(w http.ResponseWriter, name string, data PageData) {
	r.RenderStatus(w, http.StatusOK, name, data)
}

func (r *Renderer) RenderStatus(w http.ResponseWriter, status int, name string, data PageData) {
	if data.Lang == "" {
		data.Lang = i18n.Default
	}
	if data.Dir == "" {
		data.Dir = i18n.Dir(data.Lang)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := r.templates.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, i18n.T(data.Lang, "error.render"), http.StatusInternalServerError)
	}
}

// T translates a catalog key into the page's resolved language, for use
// from templates as {{.T "some.key" args...}}.
func (p PageData) T(key string, args ...any) string {
	return i18n.T(p.Lang, key, args...)
}

// newPageData starts a PageData with the request's resolved locale and
// current-user view already filled in.
func (s *Server) newPageData(r *http.Request, user *store.User) PageData {
	lang, dir := resolveLocale(r, user)
	return PageData{
		Lang:        lang,
		Dir:         dir,
		CurrentUser: s.userView(r, user),
		BaseDomain:  s.baseDomain,
		NavActive:   navActiveForPath(r.URL.Path),
	}
}

// navActiveForPath derives which app-bar nav link (if any) is "current"
// from the request path, so every handler that calls newPageData gets it
// for free instead of setting it individually.
func navActiveForPath(path string) string {
	switch {
	case path == "/write" || strings.HasPrefix(path, "/write/"):
		return "write"
	case path == "/inbox" || strings.HasPrefix(path, "/inbox/"):
		return "inbox"
	case path == "/you" || strings.HasPrefix(path, "/you/"):
		return "you"
	default:
		return ""
	}
}

// ExampleBlogHost formats username.baseDomain for signup hints.
func (p PageData) ExampleBlogHost(username string) string {
	username = strings.TrimSpace(username)
	if username == "" {
		username = "username"
	}
	base := strings.TrimSpace(p.BaseDomain)
	if base == "" {
		base = "waldi.blog"
	}
	return username + "." + base
}

func (s *Server) userView(r *http.Request, user *store.User) *UserView {
	if user == nil {
		return nil
	}
	return &UserView{
		Username: user.Username,
		Name:     user.Username,
		Email:    user.Email,
		BlogURL:  PublicBlogURLForOwner(r, s.baseDomain, *user, "/"),
		CanWrite: user.CanWrite,
	}
}
