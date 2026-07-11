package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"waldi/internal/mail"
	"waldi/internal/store"
)

const reactivationInactiveAfter = 30 * 24 * time.Hour

// ReactivationJob pauses digest email for accounts that have gone quiet for
// ~30 days and sends a one-time re-permission email asking whether they
// still want it. Digests stay paused until the user clicks the resume link
// (or otherwise becomes active again) — nobody keeps getting daily mail
// they've stopped opening.
type ReactivationJob struct {
	Store   *store.Store
	Logger  *slog.Logger
	Mailer  mail.Mailer
	Now     func() time.Time
	BaseURL string
}

func (j ReactivationJob) Run(ctx context.Context) error {
	if j.Store == nil {
		return fmt.Errorf("store is required")
	}
	logger := j.Logger
	if logger == nil {
		logger = slog.Default()
	}
	mailer := j.Mailer
	if mailer == nil {
		mailer = mail.NoopMailer{Logger: logger}
	}
	now := time.Now
	if j.Now != nil {
		now = j.Now
	}

	cutoff := now().Add(-reactivationInactiveAfter)
	baseURL := strings.TrimSuffix(j.BaseURL, "/")

	users, err := j.Store.UsersInactiveForDigest(ctx, cutoff, 1000)
	if err != nil {
		return err
	}

	for _, user := range users {
		token, err := unsubscribeToken(ctx, j.Store, user)
		if err != nil {
			logger.Error("resolving reactivation token", "username", user.Username, "err", err)
			continue
		}
		resumeURL := baseURL + "/resume-digest?token=" + token

		subject, plain, htmlBody := mail.ReactivationEmail(user.Locale, resumeURL)
		if err := mailer.SendBulk(ctx, user.Email, subject, plain, htmlBody, mail.BrandName(user.Locale), nil); err != nil {
			logger.Error("sending reactivation email", "username", user.Username, "err", err)
			continue
		}

		if err := j.Store.PauseDigest(ctx, user.ID, now()); err != nil {
			return err
		}
	}
	return nil
}
