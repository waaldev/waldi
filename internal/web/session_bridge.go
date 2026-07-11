package web

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const bridgeTTL = 5 * time.Minute

var errInvalidBridge = errors.New("invalid bridge token")

func bridgeSigningKey(baseDomain string) []byte {
	sum := sha256.Sum256([]byte("waldi-bridge:" + strings.ToLower(strings.TrimSpace(baseDomain))))
	return sum[:]
}

func (s *Server) createBridgeToken(sessionToken, targetHost string) (string, error) {
	targetHost = hostWithoutPort(strings.ToLower(strings.TrimSpace(targetHost)))
	if sessionToken == "" || targetHost == "" {
		return "", errInvalidBridge
	}
	exp := time.Now().Add(bridgeTTL).Unix()
	payload := sessionToken + "|" + strconv.FormatInt(exp, 10) + "|" + targetHost
	mac := hmac.New(sha256.New, bridgeSigningKey(s.baseDomain))
	_, _ = mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + sig, nil
}

func (s *Server) parseBridgeToken(token, expectedHost string) (sessionToken string, err error) {
	expectedHost = hostWithoutPort(strings.ToLower(strings.TrimSpace(expectedHost)))
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return "", errInvalidBridge
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", errInvalidBridge
	}
	fields := strings.Split(string(raw), "|")
	if len(fields) != 3 {
		return "", errInvalidBridge
	}
	exp, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return "", errInvalidBridge
	}
	if fields[2] != expectedHost {
		return "", errInvalidBridge
	}
	mac := hmac.New(sha256.New, bridgeSigningKey(s.baseDomain))
	_, _ = mac.Write(raw)
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(parts[1]), []byte(expectedSig)) {
		return "", errInvalidBridge
	}
	return fields[0], nil
}

func (s *Server) handleAuthBridge(w http.ResponseWriter, r *http.Request) {
	dest, err := s.bridgeContinueURL(r, r.URL.Query().Get("return"))
	if errors.Is(err, errBridgeUnauthorized) {
		redirect(w, r, appLoginURL(r, s.baseDomain, r.URL.Query().Get("return")))
		return
	}
	if err != nil {
		if errors.Is(err, errInvalidBridgeReturn) {
			http.Error(w, "bad return url", http.StatusBadRequest)
			return
		}
		s.logger.Error("creating bridge url", "err", err)
		http.Error(w, "bridge failed", http.StatusInternalServerError)
		return
	}
	redirect(w, r, dest)
}

func (s *Server) handleAPIAuthBridge(w http.ResponseWriter, r *http.Request) {
	if origin := r.Header.Get("Origin"); origin != "" && s.allowedOrigin(r.Context(), origin) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Vary", "Origin")
	}
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	dest, err := s.bridgeContinueURL(r, r.URL.Query().Get("return"))
	if errors.Is(err, errBridgeUnauthorized) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"continue":""}`))
		return
	}
	if err != nil {
		if errors.Is(err, errInvalidBridgeReturn) {
			http.Error(w, "bad return url", http.StatusBadRequest)
			return
		}
		s.logger.Error("creating bridge url", "err", err)
		http.Error(w, "bridge failed", http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"continue": dest})
}

var (
	errBridgeUnauthorized  = errors.New("bridge unauthorized")
	errInvalidBridgeReturn = errors.New("invalid bridge return")
)

func (s *Server) bridgeContinueURL(r *http.Request, returnRaw string) (string, error) {
	sessionToken, ok := s.sessionTokenFromRequest(r)
	if !ok {
		return "", errBridgeUnauthorized
	}

	returnURL, ok := s.validBridgeReturn(r.Context(), returnRaw)
	if !ok {
		return "", errInvalidBridgeReturn
	}

	token, err := s.createBridgeToken(sessionToken, returnURL.Host)
	if err != nil {
		return "", err
	}

	dest := requestScheme(r) + "://" + returnURL.Host + requestPort(r) + "/auth/continue"
	q := url.Values{}
	q.Set("token", token)
	ret := returnURL.Path
	if returnURL.RawQuery != "" {
		ret += "?" + returnURL.RawQuery
	}
	q.Set("return", ret)
	return dest + "?" + q.Encode(), nil
}

// sessionTokenFromRequest resolves the caller's real session token for the
// bridge handoff. The waldi_session cookie (SameSite=Lax) covers same-site
// requests, already validated by withSession. Cross-site requests from a
// custom domain never carry it, so they fall back to the waldi_bridge probe
// cookie (SameSite=None), which is resolved to the underlying session via
// the store rather than trusted directly.
func (s *Server) sessionTokenFromRequest(r *http.Request) (string, bool) {
	if currentUser(r) != nil {
		if cookie, err := r.Cookie(sessionCookie); err == nil && cookie.Value != "" {
			return cookie.Value, true
		}
	}

	cookie, err := r.Cookie(bridgeCookie)
	if err != nil || cookie.Value == "" || s.store == nil {
		return "", false
	}
	_, sessionToken, err := s.store.UserAndSessionByBridgeToken(r.Context(), cookie.Value)
	if err != nil {
		return "", false
	}
	return sessionToken, true
}

func (s *Server) handleAuthContinue(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		http.NotFound(w, r)
		return
	}

	sessionToken, err := s.parseBridgeToken(token, r.Host)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if s.store == nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}

	user, err := s.store.UserBySessionToken(r.Context(), sessionToken)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	expires := time.Now().Add(sessionTTL)
	domain := sessionCookieDomain(r.Host, s.baseDomain)
	setSessionCookie(w, sessionToken, expires, domain, requestScheme(r) == "https")

	dest := strings.TrimSpace(r.URL.Query().Get("return"))
	if dest == "" || dest[0] != '/' {
		dest = "/"
	}
	_ = user
	redirect(w, r, dest)
}

func (s *Server) handleAPIMe(w http.ResponseWriter, r *http.Request) {
	if origin := r.Header.Get("Origin"); origin != "" && s.allowedOrigin(r.Context(), origin) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Vary", "Origin")
	}
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	username := s.usernameFromRequest(r)
	if username == "" {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"username":""}`))
		return
	}
	_, _ = fmt.Fprintf(w, `{"username":%q}`, username)
}

func (s *Server) usernameFromRequest(r *http.Request) string {
	if user := currentUser(r); user != nil {
		return user.Username
	}
	cookie, err := r.Cookie(bridgeCookie)
	if err != nil || cookie.Value == "" || s.store == nil {
		return ""
	}
	user, _, err := s.store.UserAndSessionByBridgeToken(r.Context(), cookie.Value)
	if err != nil {
		return ""
	}
	return user.Username
}

func (s *Server) allowedOrigin(ctx context.Context, origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := hostWithoutPort(strings.ToLower(u.Host))
	if host == hostWithoutPort(strings.ToLower(s.baseDomain)) {
		return true
	}
	if s.isBlogHost(ctx, host) != nil {
		return true
	}
	return isLocalDevHost(host)
}

func (s *Server) validBridgeReturn(ctx context.Context, raw string) (*url.URL, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, false
	}
	host := hostWithoutPort(strings.ToLower(u.Host))
	if host == hostWithoutPort(strings.ToLower(s.baseDomain)) {
		return u, true
	}
	if s.isBlogHost(ctx, host) != nil {
		return u, true
	}
	if isLocalDevHost(host) {
		return u, true
	}
	return nil, false
}

func pageURL(r *http.Request) string {
	return requestScheme(r) + "://" + r.Host + r.URL.RequestURI()
}

func appLoginURL(r *http.Request, baseDomain, returnTo string) string {
	login := appBaseURL(r, baseDomain) + "/login"
	if returnTo = strings.TrimSpace(returnTo); returnTo != "" {
		login += "?next=" + url.QueryEscape(returnTo)
	}
	return login
}

func authBridgeURL(r *http.Request, baseDomain string) string {
	return appBaseURL(r, baseDomain) + "/auth/bridge?return=" + url.QueryEscape(pageURL(r))
}
