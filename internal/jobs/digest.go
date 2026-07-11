package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"waldi/internal/i18n"
	"waldi/internal/mail"
	"waldi/internal/store"
)

type DigestJob struct {
	Store   *store.Store
	Logger  *slog.Logger
	Mailer  mail.Mailer
	Now     func() time.Time
	BaseURL string // app base URL, used to build unsubscribe links
}

func (j DigestJob) Run(ctx context.Context) error {
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

	sentAt := now()
	since := sentAt.Add(-24 * time.Hour)
	users, err := j.Store.UsersWithDigestActivity(ctx, since, 1000)
	if err != nil {
		return err
	}

	baseURL := strings.TrimSuffix(j.BaseURL, "/")

	for _, user := range users {
		if !digestEligible(user) {
			continue
		}

		sent, err := j.Store.DigestSentToday(ctx, user.ID, "writer", sentAt)
		if err != nil {
			return err
		}
		if sent {
			continue
		}

		stats, err := j.Store.PostStatsForUser(ctx, user.ID, since, 10)
		if err != nil {
			return err
		}
		if len(stats) == 0 {
			continue
		}

		sentences := make([]string, 0, len(stats))
		for _, stat := range stats {
			sentences = append(sentences, i18n.T(user.Locale, "digest.line", stat.PostTitle, DigestSentence(user.Locale, stat)))
		}

		token, err := unsubscribeToken(ctx, j.Store, user)
		if err != nil {
			logger.Error("resolving digest unsubscribe token", "username", user.Username, "err", err)
			continue
		}
		unsubscribeURL := baseURL + "/unsubscribe/digest?token=" + token

		subject, plain, htmlBody := mail.DigestEmail(user.Locale, sentences, unsubscribeURL)
		headers := map[string]string{
			"List-Unsubscribe":      fmt.Sprintf("<%s>", unsubscribeURL),
			"List-Unsubscribe-Post": "List-Unsubscribe=One-Click",
			"Precedence":            "bulk",
		}
		if err := mailer.SendBulk(ctx, user.Email, subject, plain, htmlBody, mail.BrandName(user.Locale), headers); err != nil {
			logger.Error("sending digest", "username", user.Username, "err", err)
			continue
		}

		if err := j.Store.RecordDigestSent(ctx, user.ID, "writer", sentAt); err != nil {
			return err
		}
	}
	return nil
}

func digestEligible(user store.User) bool {
	return user.Email != "" && user.EmailVerified() && !user.DigestUnsubscribed()
}

// DigestSentence narrates a post's last-24-hours activity as a short list of
// clauses, e.g. "142 people read this. 89 finished. 6 followed you because
// of it. 2 wrote to you." Clauses with nothing to report are omitted so the
// sentence only ever grows to match real activity.
func DigestSentence(lang string, stat store.PostStats) string {
	var clauses []string
	if stat.Readers > 0 {
		clauses = append(clauses, i18n.T(lang, "digest.clause.readers", stat.Readers))
	}
	if stat.Completed > 0 {
		clauses = append(clauses, i18n.T(lang, "digest.clause.completed", stat.Completed))
	}
	if stat.Follows > 0 {
		clauses = append(clauses, i18n.T(lang, "digest.clause.follows", stat.Follows))
	}
	if stat.Letters > 0 {
		clauses = append(clauses, i18n.T(lang, "digest.clause.letters", stat.Letters))
	}
	return strings.Join(clauses, " ")
}
