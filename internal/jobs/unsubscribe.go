package jobs

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"waldi/internal/store"
)

func newUnsubscribeToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// unsubscribeToken returns the user's persistent digest-unsubscribe token,
// generating and persisting one if it doesn't exist yet. Shared by every
// digest job (writer activity digest, reader digest) since both link to the
// same /unsubscribe/digest opt-out.
func unsubscribeToken(ctx context.Context, st *store.Store, user store.User) (string, error) {
	if user.DigestUnsubscribeToken != nil {
		return *user.DigestUnsubscribeToken, nil
	}
	generated, err := newUnsubscribeToken()
	if err != nil {
		return "", err
	}
	return st.SetDigestUnsubscribeToken(ctx, user.ID, generated)
}
