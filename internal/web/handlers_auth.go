package web

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"waldi/internal/mail"
	"waldi/internal/store"

	"github.com/jackc/pgx/v5/pgconn"
)

func (s *Server) handleSignupForm(w http.ResponseWriter, r *http.Request) {
	pd := s.newPageData(r, currentUser(r))
	pd.SEO = noindexSEO()

	pd.Title = pd.T("auth.signup.title")
	pd.Auth = &AuthView{
		Mode:        "signup",
		Heading:     pd.T("auth.signup.heading"),
		SubmitLabel: pd.T("auth.signup.submit"),
		InviteCode:  strings.TrimSpace(r.URL.Query().Get("invite")),
	}
	s.renderer.Render(w, "auth.html", pd)
}

func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		s.renderAuthError(w, r, "signup", "auth.error.db")
		return
	}
	if err := r.ParseForm(); err != nil {
		s.renderAuthError(w, r, "signup", "auth.error.form")
		return
	}

	username := normalizeUsername(r.FormValue("username"))
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	password := r.FormValue("password")
	blogName := strings.TrimSpace(r.FormValue("blog_name"))
	blogDescription := strings.TrimSpace(r.FormValue("blog_description"))

	if !validBlogUsername(username) {
		s.renderAuthError(w, r, "signup", "auth.error.username")
		return
	}
	if !validEmail(email) {
		s.renderAuthError(w, r, "signup", "auth.error.email")
		return
	}
	if len(password) < 8 {
		s.renderAuthError(w, r, "signup", "auth.error.password_len")
		return
	}
	if blogName == "" || len(blogName) > maxDisplayNameLen {
		s.renderAuthError(w, r, "signup", "auth.error.blog_name")
		return
	}
	if len(blogDescription) > maxBioLen {
		s.renderAuthError(w, r, "signup", "auth.error.blog_description")
		return
	}

	hash, err := hashPassword(password)
	if err != nil {
		s.logger.Error("hashing password", "err", err)
		s.renderAuthError(w, r, "signup", "auth.error.signup_failed")
		return
	}

	verifyToken, err := newAuthToken(verifyTokenBytes)
	if err != nil {
		s.logger.Error("creating verify token", "err", err)
		s.renderAuthError(w, r, "signup", "auth.error.signup_failed")
		return
	}

	lang, _ := resolveLocale(r, nil)
	var user store.User
	inviteCode := strings.TrimSpace(r.FormValue("invite"))
	if inviteCode != "" {
		user, err = s.store.CreateUserAndRedeemInvitation(r.Context(), inviteCode, username, email, hash, lang, verifyToken, blogName, blogDescription)
		if errors.Is(err, store.ErrInviteInvalid) {
			s.renderAuthError(w, r, "signup", "auth.error.invite_invalid")
			return
		}
	} else {
		user, err = s.store.CreateUser(r.Context(), username, email, hash, lang, verifyToken, blogName, blogDescription)
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			s.renderAuthError(w, r, "signup", "auth.error.duplicate")
			return
		}
		s.logger.Error("creating user", "err", err)
		s.renderAuthError(w, r, "signup", "auth.error.signup_failed")
		return
	}

	if err := s.startSession(w, r, user.ID); err != nil {
		s.logger.Error("starting session", "err", err)
		s.renderAuthError(w, r, "signup", "auth.error.autologin")
		return
	}
	if err := s.store.AdoptEmailCaptureFollows(r.Context(), user.ID, user.Email); err != nil {
		s.logger.Error("adopting email capture follows", "err", err)
	}
	if err := s.store.DeleteEmailCapturesByEmail(r.Context(), user.Email); err != nil {
		s.logger.Error("cleaning up email captures", "err", err)
	}
	s.queueVerificationEmail(user, appBaseURL(r, s.baseDomain), verifyToken, false)
	s.notifySignup(r, user)
	http.Redirect(w, r, "/verify-email", http.StatusSeeOther)
}

func (s *Server) handleLoginForm(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user != nil {
		redirect(w, r, safeNextURL(r.URL.Query().Get("next"), appBaseURL(r, s.baseDomain)))
		return
	}
	pd := s.newPageData(r, user)
	pd.Title = pd.T("auth.login.title")
	pd.SEO = noindexSEO()
	pd.Auth = &AuthView{
		Mode:        "login",
		Heading:     pd.T("auth.login.heading"),
		SubmitLabel: pd.T("auth.login.submit"),
		NextURL:     r.URL.Query().Get("next"),
	}
	s.renderer.Render(w, "auth.html", pd)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		s.renderAuthError(w, r, "login", "auth.error.db")
		return
	}
	if err := r.ParseForm(); err != nil {
		s.renderAuthError(w, r, "login", "auth.error.form")
		return
	}

	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	password := r.FormValue("password")

	user, err := s.store.UserByEmail(r.Context(), email)
	if errors.Is(err, store.ErrNotFound) || !checkPassword(user.PasswordHash, password) {
		s.renderAuthError(w, r, "login", "auth.error.credentials")
		return
	}
	if err != nil {
		s.logger.Error("loading login user", "err", err)
		s.renderAuthError(w, r, "login", "auth.error.login_failed")
		return
	}

	if err := s.startSession(w, r, user.ID); err != nil {
		s.logger.Error("starting session", "err", err)
		s.renderAuthError(w, r, "login", "auth.error.login_failed")
		return
	}

	dest := safeNextURL(r.FormValue("next"), "/")
	if dest == "/" && !user.EmailVerified() {
		dest = "/verify-email"
	}
	redirect(w, r, dest)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if s.store != nil {
		if cookie, err := r.Cookie(sessionCookie); err == nil && cookie.Value != "" {
			if err := s.store.DeleteSession(r.Context(), cookie.Value); err != nil {
				s.logger.Error("deleting session", "err", err)
			}
		}
	}
	domain := sessionCookieDomain(r.Host, s.baseDomain)
	secure := requestScheme(r) == "https"
	clearSessionCookie(w, domain, secure)
	clearBridgeCookie(w, domain, secure)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleVerifyEmailPage(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token != "" && s.store != nil {
		verified, err := s.store.VerifyEmailByToken(r.Context(), token)
		if err == nil {
			if err := s.startSession(w, r, verified.ID); err != nil {
				s.logger.Error("starting session after verify", "err", err)
			}
			http.Redirect(w, r, "/verify-email?verified=1", http.StatusSeeOther)
			return
		}
		if !errors.Is(err, store.ErrNotFound) {
			s.logger.Error("verifying email", "err", err)
		}
	}

	user := currentUser(r)
	if user == nil {
		if token != "" {
			http.Redirect(w, r, "/login?next="+url.QueryEscape("/verify-email"), http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if user.EmailVerified() && r.URL.Query().Get("verified") != "1" {
		http.Redirect(w, r, "/write", http.StatusSeeOther)
		return
	}

	pd := s.newPageData(r, user)
	pd.Title = pd.T("auth.verify.title")
	pd.SEO = noindexSEO()
	pd.Auth = &AuthView{
		Mode:    "verify",
		Heading: pd.T("auth.verify.heading"),
	}
	switch {
	case r.URL.Query().Get("verified") == "1":
		pd.Auth.Message = pd.T("auth.verify.done")
	case r.URL.Query().Get("sent") == "1":
		pd.Auth.Message = pd.T("auth.verify.resent")
	case !s.canResendVerification(user):
		pd.Auth.Message = pd.T("auth.verify.wait")
	}
	pd.Auth.CanResend = s.canResendVerification(user)
	s.renderer.Render(w, "verify_email.html", pd)
}

func (s *Server) handleResendVerification(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if user.EmailVerified() {
		http.Redirect(w, r, "/write", http.StatusSeeOther)
		return
	}
	if !s.canResendVerification(user) {
		http.Redirect(w, r, "/verify-email", http.StatusSeeOther)
		return
	}
	s.queueVerificationEmail(*user, appBaseURL(r, s.baseDomain), "", true)
	http.Redirect(w, r, "/verify-email?sent=1", http.StatusSeeOther)
}

func (s *Server) handleForgotPasswordForm(w http.ResponseWriter, r *http.Request) {
	pd := s.newPageData(r, currentUser(r))
	pd.Title = pd.T("auth.forgot.title")
	pd.SEO = noindexSEO()
	pd.Auth = &AuthView{
		Mode:        "forgot",
		Heading:     pd.T("auth.forgot.heading"),
		SubmitLabel: pd.T("auth.forgot.submit"),
	}
	if r.URL.Query().Get("sent") == "1" {
		pd.Auth.Message = pd.T("auth.forgot.sent")
	}
	s.renderer.Render(w, "auth.html", pd)
}

func (s *Server) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		s.renderAuthError(w, r, "forgot", "auth.error.db")
		return
	}
	if err := r.ParseForm(); err != nil {
		s.renderAuthError(w, r, "forgot", "auth.error.form")
		return
	}

	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	user, err := s.store.UserByEmail(r.Context(), email)
	if err == nil {
		token, err := newAuthToken(resetTokenBytes)
		if err != nil {
			s.logger.Error("creating reset token", "err", err)
		} else {
			expires := time.Now().Add(time.Hour)
			if err := s.store.SetPasswordResetToken(r.Context(), user.ID, token, expires); err != nil {
				s.logger.Error("setting reset token", "err", err)
			} else if s.mailer != nil {
				resetURL := passwordResetURL(r, s.baseDomain, token)
				subject, plain, htmlBody := mail.PasswordResetEmail(user.Locale, resetURL)
				if err := s.mailer.SendHTML(r.Context(), user.Email, subject, plain, htmlBody, mail.BrandName(user.Locale)); err != nil {
					s.logger.Error("sending reset email", "err", err)
				}
			}
		}
	} else if !errors.Is(err, store.ErrNotFound) {
		s.logger.Error("loading user for reset", "err", err)
	}

	http.Redirect(w, r, "/forgot-password?sent=1", http.StatusSeeOther)
}

func (s *Server) handleResetPasswordForm(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		http.NotFound(w, r)
		return
	}
	pd := s.newPageData(r, currentUser(r))
	pd.Title = pd.T("auth.reset.title")
	pd.SEO = noindexSEO()
	pd.Auth = &AuthView{
		Mode:        "reset",
		Heading:     pd.T("auth.reset.heading"),
		SubmitLabel: pd.T("auth.reset.submit"),
		ResetToken:  token,
	}
	s.renderer.Render(w, "auth.html", pd)
}

func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		s.renderAuthError(w, r, "reset", "auth.error.db")
		return
	}
	if err := r.ParseForm(); err != nil {
		s.renderAuthError(w, r, "reset", "auth.error.form")
		return
	}

	token := strings.TrimSpace(r.FormValue("token"))
	password := r.FormValue("password")
	if token == "" {
		s.renderAuthError(w, r, "reset", "auth.error.reset_invalid")
		return
	}
	if len(password) < 8 {
		s.renderAuthError(w, r, "reset", "auth.error.password_len")
		return
	}

	hash, err := hashPassword(password)
	if err != nil {
		s.logger.Error("hashing password", "err", err)
		s.renderAuthError(w, r, "reset", "auth.error.reset_failed")
		return
	}

	user, err := s.store.ResetPasswordByToken(r.Context(), token, hash)
	if errors.Is(err, store.ErrNotFound) {
		s.renderAuthError(w, r, "reset", "auth.error.reset_invalid")
		return
	}
	if err != nil {
		s.logger.Error("resetting password", "err", err)
		s.renderAuthError(w, r, "reset", "auth.error.reset_failed")
		return
	}

	if err := s.startSession(w, r, user.ID); err != nil {
		s.logger.Error("starting session after reset", "err", err)
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) startSession(w http.ResponseWriter, r *http.Request, userID int64) error {
	token, err := newSessionToken()
	if err != nil {
		return err
	}
	bridgeToken, err := newSessionToken()
	if err != nil {
		return err
	}
	expires := time.Now().Add(sessionTTL)
	if err := s.store.CreateSession(r.Context(), token, bridgeToken, userID, expires); err != nil {
		return err
	}
	domain := sessionCookieDomain(r.Host, s.baseDomain)
	secure := requestScheme(r) == "https"
	setSessionCookie(w, token, expires, domain, secure)
	setBridgeCookie(w, bridgeToken, expires, domain, secure)
	return nil
}

func (s *Server) renderAuthError(w http.ResponseWriter, r *http.Request, mode, messageKey string) {
	pd := s.newPageData(r, currentUser(r))
	heading := pd.T("auth.login.heading")
	submit := pd.T("auth.login.submit")
	title := pd.T("auth.login.title")
	switch mode {
	case "signup":
		heading = pd.T("auth.signup.heading")
		submit = pd.T("auth.signup.submit")
		title = pd.T("auth.signup.title")
	case "forgot":
		heading = pd.T("auth.forgot.heading")
		submit = pd.T("auth.forgot.submit")
		title = pd.T("auth.forgot.title")
	case "reset":
		heading = pd.T("auth.reset.heading")
		submit = pd.T("auth.reset.submit")
		title = pd.T("auth.reset.title")
	}
	pd.Title = title
	pd.SEO = noindexSEO()
	pd.Auth = &AuthView{
		Mode:        mode,
		Heading:     heading,
		SubmitLabel: submit,
		Error:       pd.T(messageKey),
		ResetToken:  strings.TrimSpace(r.FormValue("token")),
		InviteCode:  strings.TrimSpace(r.FormValue("invite")),
	}
	s.renderer.RenderStatus(w, http.StatusBadRequest, "auth.html", pd)
}

func (s *Server) handleSessionBridge(w http.ResponseWriter, r *http.Request) {
	if !isLocalDevHost(r.Host) {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")

	username := ""
	if user := currentUser(r); user != nil {
		username = user.Username
	}

	origin := appBaseURL(r, s.baseDomain)
	_, _ = w.Write([]byte(`<!doctype html><script>parent.postMessage({username:`))
	_, _ = w.Write([]byte(strconv.Quote(username)))
	_, _ = w.Write([]byte(`}, ` + strconv.Quote(origin) + `)</script>`))
}
