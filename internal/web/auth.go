package web

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"
	"waldi/internal/store"

	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const (
	currentUserKey contextKey = "currentUser"
	sessionCookie             = "waldi_session"
	bridgeCookie              = "waldi_bridge"
	sessionTTL                = 30 * 24 * time.Hour
)

func currentUser(r *http.Request) *store.User {
	user, _ := r.Context().Value(currentUserKey).(*store.User)
	return user
}

func (s *Server) withSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.store == nil {
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie(sessionCookie)
		if err != nil || cookie.Value == "" {
			next.ServeHTTP(w, r)
			return
		}

		user, err := s.store.UserBySessionToken(r.Context(), cookie.Value)
		if err != nil {
			if !errors.Is(err, store.ErrNotFound) {
				s.logger.Error("loading session user", "err", err)
			}
			clearSessionCookie(w, sessionCookieDomain(r.Host, s.baseDomain), requestScheme(r) == "https")
			next.ServeHTTP(w, r)
			return
		}

		ctx := context.WithValue(r.Context(), currentUserKey, &user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func checkPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func newSessionToken() (string, error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf[:]), nil
}

func normalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}
