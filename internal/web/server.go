package web

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"waldi/internal/cdn"
	"waldi/internal/mail"
	"waldi/internal/storage"
	"waldi/internal/store"
)

// Notifier delivers admin-facing notifications (e.g. the Telegram admin bot).
type Notifier interface {
	Notify(ctx context.Context, text string)
}

type Config struct {
	BaseDomain string
	DevMode    bool
	Logger     *slog.Logger
	Store      *store.Store
	Mailer     mail.Mailer
	Images     storage.ImageStore
	S3Media    *storage.S3Store
	CDNPurger  cdn.Purger
	Notifier   Notifier
}

type Server struct {
	baseDomain    string
	logger        *slog.Logger
	renderer      *Renderer
	mux           *http.ServeMux
	store         *store.Store
	mailer        mail.Mailer
	images        storage.ImageStore
	s3media       *storage.S3Store
	cdnPurger     cdn.Purger
	notifier      Notifier
	customDomains *customDomainCache
}

func NewServer(cfg Config) (*Server, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	renderer, err := NewRenderer(os.DirFS("."))
	if err != nil {
		return nil, err
	}

	s := &Server{
		baseDomain:    cfg.BaseDomain,
		logger:        cfg.Logger,
		renderer:      renderer,
		mux:           http.NewServeMux(),
		store:         cfg.Store,
		mailer:        cfg.Mailer,
		images:        cfg.Images,
		s3media:       cfg.S3Media,
		cdnPurger:     cfg.CDNPurger,
		notifier:      cfg.Notifier,
		customDomains: newCustomDomainCache(),
	}
	s.routes()
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.maybeRedirectSubdomainToCustomDomain(w, r) {
		return
	}
	rec := &notFoundRecorder{ResponseWriter: w}
	s.withSession(s.mux).ServeHTTP(rec, r)
	if rec.status == http.StatusNotFound && !rec.wrote {
		s.renderNotFound(rec.ResponseWriter, r)
	}
}

func (s *Server) routes() {
	static := staticCacheControl(http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	s.mux.Handle("GET /static/", static)
	if s.s3media != nil && s.s3media.NeedsProxy() {
		s.mux.HandleFunc("GET /media/{key...}", s.handleMedia)
	}
	s.mux.HandleFunc("GET /internal/caddy-ask", s.handleCaddyAsk)
	s.mux.HandleFunc("GET /robots.txt", s.handleRobots)
	s.mux.HandleFunc("GET /feed.xml", s.handleBlogFeed)
	s.mux.HandleFunc("GET /sitemap.xml", s.handleBlogSitemapOrApp)
	s.mux.HandleFunc("GET /", s.handleHome)
	s.mux.HandleFunc("POST /lang/{code}", s.handleSetLocale)
	s.mux.HandleFunc("GET /read", s.handleReadRandom)
	s.mux.HandleFunc("GET /signup", s.handleSignupForm)
	s.mux.HandleFunc("POST /signup", s.handleSignup)
	s.mux.HandleFunc("GET /login", s.handleLoginForm)
	s.mux.HandleFunc("POST /login", s.handleLogin)
	s.mux.HandleFunc("POST /logout", s.handleLogout)
	s.mux.HandleFunc("GET /verify-email", s.handleVerifyEmailPage)
	s.mux.HandleFunc("POST /verify-email/resend", s.handleResendVerification)
	s.mux.HandleFunc("GET /forgot-password", s.handleForgotPasswordForm)
	s.mux.HandleFunc("POST /forgot-password", s.handleForgotPassword)
	s.mux.HandleFunc("GET /reset-password", s.handleResetPasswordForm)
	s.mux.HandleFunc("POST /reset-password", s.handleResetPassword)
	s.mux.HandleFunc("GET /unsubscribe/digest", s.handleUnsubscribeDigestPage)
	s.mux.HandleFunc("POST /unsubscribe/digest", s.handleUnsubscribeDigest)
	s.mux.HandleFunc("GET /resume-digest", s.handleResumeDigest)
	s.mux.HandleFunc("GET /api/me", s.handleAPIMe)
	s.mux.HandleFunc("OPTIONS /api/me", s.handleAPIMe)
	s.mux.HandleFunc("GET /api/auth/bridge", s.handleAPIAuthBridge)
	s.mux.HandleFunc("OPTIONS /api/auth/bridge", s.handleAPIAuthBridge)
	s.mux.HandleFunc("GET /auth/bridge", s.handleAuthBridge)
	s.mux.HandleFunc("GET /auth/continue", s.handleAuthContinue)
	s.mux.HandleFunc("GET /api/session-bridge", s.handleSessionBridge)
	s.mux.HandleFunc("GET /you", s.handleYou)
	s.mux.HandleFunc("GET /you/{slug}", s.handleYouPost)
	s.mux.HandleFunc("GET /settings", s.handleBlogSettingsForm)
	s.mux.HandleFunc("POST /settings", s.handleBlogSettingsSave)
	s.mux.HandleFunc("POST /settings/domain", s.handleSetCustomDomain)
	s.mux.HandleFunc("POST /settings/domain/verify", s.handleVerifyCustomDomain)
	s.mux.HandleFunc("POST /settings/domain/remove", s.handleRemoveCustomDomain)
	s.mux.HandleFunc("POST /settings/password", s.handleChangePassword)
	s.mux.HandleFunc("POST /settings/locale", s.handleSettingsLocale)
	s.mux.HandleFunc("GET /settings/export", s.handleExportPosts)
	s.mux.HandleFunc("GET /settings/import-blogir", s.handleImportBlogirForm)
	s.mux.HandleFunc("POST /settings/import-blogir", s.handleImportBlogir)
	s.mux.HandleFunc("POST /settings/pages", s.handleCreatePage)
	s.mux.HandleFunc("POST /settings/pages/{id}/move", s.handleMovePage)
	s.mux.HandleFunc("POST /settings/pages/{id}/slug", s.handleRenamePage)
	s.mux.HandleFunc("POST /settings/pages/{id}/delete", s.handleDeletePage)
	s.mux.HandleFunc("GET /write", s.handleWrite)
	s.mux.HandleFunc("GET /write/invite", s.handleWriteInviteForm)
	s.mux.HandleFunc("POST /write/invite/redeem", s.handleRedeemWriteInvite)
	s.mux.HandleFunc("POST /write/invite/request", s.handleWriteRequest)
	s.mux.HandleFunc("POST /write/drafts", s.handleCreateDraft)
	s.mux.HandleFunc("GET /write/{id}", s.handleEditDraft)
	s.mux.HandleFunc("POST /api/drafts/{id}", s.handleAutosaveDraft)
	s.mux.HandleFunc("POST /api/uploads/images", s.handleImageUpload)
	s.mux.HandleFunc("POST /api/events/readings", s.handleReadingEvent)
	s.mux.HandleFunc("POST /write/{id}/publish", s.handlePublishPost)
	s.mux.HandleFunc("POST /write/{id}/draft", s.handleUnpublishPost)
	s.mux.HandleFunc("POST /write/{id}/delete", s.handleDeletePost)
	s.mux.HandleFunc("POST /write/{id}/published-at", s.handleUpdatePublishedAt)
	s.mux.HandleFunc("POST /subscribe", s.handleSubscribe)
	s.mux.HandleFunc("POST /follow/{username}", s.handleFollow)
	s.mux.HandleFunc("POST /unfollow/{username}", s.handleUnfollow)
	s.mux.HandleFunc("POST /wildcard/skip", s.handleSkipWildcard)
	s.mux.HandleFunc("GET /inbox", s.handleInbox)
	s.mux.HandleFunc("GET /inbox/{id}", s.handleLetter)
	s.mux.HandleFunc("POST /letters", s.handleCreateLetter)
	s.mux.HandleFunc("GET /{slug}", s.handlePublicPost)
}
