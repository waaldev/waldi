package web

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	signupMinFillTime = 3 * time.Second
	signupFormMaxAge  = 24 * time.Hour
)

var signupFormKey = func() []byte {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic(err)
	}
	return key
}()

func signupFormSignature(ts string) string {
	mac := hmac.New(sha256.New, signupFormKey)
	mac.Write([]byte(ts))
	return hex.EncodeToString(mac.Sum(nil))
}

func signupFormToken(now time.Time) string {
	ts := strconv.FormatInt(now.Unix(), 10)
	return ts + "." + signupFormSignature(ts)
}

func signupFormTimely(token string, now time.Time) bool {
	ts, sig, ok := strings.Cut(token, ".")
	if !ok || !hmac.Equal([]byte(sig), []byte(signupFormSignature(ts))) {
		return false
	}
	unix, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return false
	}
	elapsed := now.Sub(time.Unix(unix, 0))
	return elapsed >= signupMinFillTime && elapsed <= signupFormMaxAge
}

func signupTrapFilled(r *http.Request) bool {
	return r.FormValue("website") != ""
}
