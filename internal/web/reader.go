package web

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

const readerCookie = "waldi_reader"

var readerTokenPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`)

func readerKeyFromRequest(r *http.Request, baseDomain string) (string, *http.Cookie) {
	if user := currentUser(r); user != nil {
		return fmt.Sprintf("user:%d", user.ID), nil
	}

	if c, err := r.Cookie(readerCookie); err == nil {
		if token := strings.TrimSpace(c.Value); validReaderToken(token) {
			return "anon:" + token, nil
		}
	}

	token, err := newReaderToken()
	if err != nil {
		token = "aaaaaaaaaaaaaaaa"
	}
	cookie := &http.Cookie{
		Name:     readerCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   365 * 24 * 60 * 60,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestScheme(r) == "https",
	}
	if domain := sessionCookieDomain(r.Host, baseDomain); domain != "" {
		cookie.Domain = domain
	}
	return "anon:" + token, cookie
}

func newReaderToken() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf[:]), nil
}

func validReaderToken(token string) bool {
	return readerTokenPattern.MatchString(token)
}
