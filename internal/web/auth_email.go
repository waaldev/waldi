package web

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"
	"waldi/internal/mail"
	"waldi/internal/store"
)

const (
	verifyTokenBytes  = 32
	resetTokenBytes   = 32
	verifyResendWait  = 2 * time.Minute
	verifyMailTimeout = 20 * time.Second
)

func newAuthToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf[:]), nil
}

func (s *Server) queueVerificationEmail(user store.User, baseURL, token string, rotateToken bool) {
	if s.store == nil {
		return
	}
	if s.mailer == nil {
		s.logger.Warn("verification email not sent: mailer not configured", "user_id", user.ID, "email", user.Email)
		return
	}

	baseURL = strings.TrimSuffix(strings.TrimSpace(baseURL), "/")
	userID := user.ID
	email := user.Email
	locale := user.Locale

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), verifyMailTimeout)
		defer cancel()

		sendToken := strings.TrimSpace(token)
		if rotateToken || sendToken == "" {
			t, err := newAuthToken(verifyTokenBytes)
			if err != nil {
				s.logger.Error("creating verify token", "err", err, "user_id", userID)
				return
			}
			sendToken = t
			if err := s.store.SetEmailVerifyToken(ctx, userID, sendToken); err != nil {
				s.logger.Error("setting verify token", "err", err, "user_id", userID)
				return
			}
		}

		verifyURL := baseURL + "/verify-email?token=" + sendToken
		if !mail.Configured(s.mailer) {
			s.logger.Info("verification link (SMTP not configured)", "user_id", userID, "email", email, "url", verifyURL)
			return
		}

		subject, plain, htmlBody := mail.VerificationEmail(locale, verifyURL)
		if err := s.mailer.SendHTML(ctx, email, subject, plain, htmlBody, mail.BrandName(locale)); err != nil {
			s.logger.Error("sending verification email", "err", err, "user_id", userID, "email", email)
		}
	}()
}

func (s *Server) requireVerified(w http.ResponseWriter, r *http.Request, user *store.User) bool {
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return false
	}
	if user.EmailVerified() {
		return true
	}
	http.Redirect(w, r, "/verify-email", http.StatusSeeOther)
	return false
}

func (s *Server) canResendVerification(user *store.User) bool {
	if user == nil || user.EmailVerified() {
		return false
	}
	if user.EmailVerifySentAt == nil {
		return true
	}
	return time.Since(*user.EmailVerifySentAt) >= verifyResendWait
}

func passwordResetURL(r *http.Request, baseDomain, token string) string {
	return fmt.Sprintf("%s/reset-password?token=%s", appBaseURL(r, baseDomain), token)
}
