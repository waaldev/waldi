package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"waldi/internal/i18n"
	"waldi/internal/mail"
	"waldi/internal/store"
)

const readerDigestFolloweePostLimit = 10

// ReaderDigestJob emails registered, verified readers a morning letter:
// new posts from writers they follow, plus their assigned wildcard stranger
// for the day. It shares the writer-activity digest's unsubscribe opt-out
// (one "daily digest" preference per user, not two).
type ReaderDigestJob struct {
	Store      *store.Store
	Logger     *slog.Logger
	Mailer     mail.Mailer
	Now        func() time.Time
	BaseURL    string // app base URL, used to build unsubscribe links
	BaseDomain string // blog subdomain base, used to build post links
}

// postURL builds a post's public blog URL from outside any HTTP request
// context (this job runs as a CLI cron, not a web handler) - mirrors
// telegrambot.Bot.postURL's same simplification.
func postURL(baseDomain, username, slug string) string {
	return "https://" + username + "." + baseDomain + "/" + slug
}

func (j ReaderDigestJob) Run(ctx context.Context) error {
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
	day := BeginningOfDay(sentAt)
	baseURL := strings.TrimSuffix(j.BaseURL, "/")

	users, err := j.Store.VerifiedSubscribedUsers(ctx, 1000)
	if err != nil {
		return err
	}

	for _, user := range users {
		sent, err := j.Store.DigestSentToday(ctx, user.ID, "reader", sentAt)
		if err != nil {
			return err
		}
		if sent {
			continue
		}

		followeePosts, err := j.Store.FeedPosts(ctx, user.ID, since, readerDigestFolloweePostLimit)
		if err != nil {
			return err
		}

		wildcard, wcErr := j.Store.AssignedWildcard(ctx, user.ID, day)
		if wcErr != nil && !errors.Is(wcErr, store.ErrNotFound) {
			return wcErr
		}
		hasWildcard := wcErr == nil

		if len(followeePosts) == 0 && !hasWildcard {
			continue
		}

		followeeLines := make([]mail.DigestLine, 0, len(followeePosts))
		for _, post := range followeePosts {
			followeeLines = append(followeeLines, mail.DigestLine{
				Text: i18n.T(user.Locale, "reader_digest.followee_line", authorLabel(post), post.Title),
				URL:  postURL(j.BaseDomain, post.Username, post.Slug),
			})
		}

		var wildcardLine *mail.DigestLine
		if hasWildcard {
			wildcardLine = &mail.DigestLine{
				Text: i18n.T(user.Locale, "reader_digest.wildcard_line", wildcard.Title, authorLabel(wildcard)),
				URL:  postURL(j.BaseDomain, wildcard.Username, wildcard.Slug),
			}
		}

		token, err := unsubscribeToken(ctx, j.Store, user)
		if err != nil {
			logger.Error("resolving reader digest unsubscribe token", "username", user.Username, "err", err)
			continue
		}
		unsubscribeURL := baseURL + "/unsubscribe/digest?token=" + token

		subject, plain, htmlBody := mail.ReaderDigestEmail(user.Locale, followeeLines, wildcardLine, unsubscribeURL)
		headers := map[string]string{
			"List-Unsubscribe":      fmt.Sprintf("<%s>", unsubscribeURL),
			"List-Unsubscribe-Post": "List-Unsubscribe=One-Click",
			"Precedence":            "bulk",
		}
		if err := mailer.SendBulk(ctx, user.Email, subject, plain, htmlBody, mail.BrandName(user.Locale), headers); err != nil {
			logger.Error("sending reader digest", "username", user.Username, "err", err)
			continue
		}

		if err := j.Store.RecordDigestSent(ctx, user.ID, "reader", sentAt); err != nil {
			return err
		}
	}

	return j.runForCapturedEmails(ctx, mailer, logger, sentAt, since, baseURL)
}

// runForCapturedEmails mails the reader digest to anonymous, not-yet-signed-up
// captured emails too: new posts from the blogs each address subscribed to,
// plus a wildcard stranger post (see EmailCaptureWildcard). Once an address
// signs up its captures are deleted (see DeleteEmailCapturesByEmail), so this
// only ever reaches people without an account yet.
func (j ReaderDigestJob) runForCapturedEmails(ctx context.Context, mailer mail.Mailer, logger *slog.Logger, sentAt, since time.Time, baseURL string) error {
	addresses, err := j.Store.EmailCaptureAddresses(ctx, 0)
	if err != nil {
		return err
	}
	posts, err := j.Store.EmailCaptureFolloweePosts(ctx, since, 0)
	if err != nil {
		return err
	}
	postsByEmail := make(map[string][]store.EmailCaptureDigestPost, len(posts))
	for _, post := range posts {
		postsByEmail[post.Email] = append(postsByEmail[post.Email], post)
	}

	for _, addr := range addresses {
		sent, err := j.Store.EmailCaptureDigestSentToday(ctx, addr.Email, sentAt)
		if err != nil {
			return err
		}
		if sent {
			continue
		}

		lang := i18n.ReaderLang(addr.Lang)
		followeePosts := postsByEmail[addr.Email]
		lines := make([]mail.DigestLine, 0, len(followeePosts))
		for _, post := range followeePosts {
			lines = append(lines, mail.DigestLine{
				Text: i18n.T(lang, "reader_digest.followee_line", post.AuthorLabel, post.PostTitle),
				URL:  postURL(j.BaseDomain, post.Username, post.Slug),
			})
		}

		wildcard, wcErr := j.Store.EmailCaptureWildcard(ctx, addr.Email, lang)
		if wcErr != nil && !errors.Is(wcErr, store.ErrNotFound) {
			return wcErr
		}
		var wildcardLine *mail.DigestLine
		if wcErr == nil {
			wildcardLine = &mail.DigestLine{
				Text: i18n.T(lang, "reader_digest.wildcard_line", wildcard.Title, authorLabel(wildcard)),
				URL:  postURL(j.BaseDomain, wildcard.Username, wildcard.Slug),
			}
		}

		if len(lines) == 0 && wildcardLine == nil {
			continue
		}

		token, err := newUnsubscribeToken()
		if err != nil {
			logger.Error("generating email capture unsubscribe token", "email", addr.Email, "err", err)
			continue
		}
		token, err = j.Store.EmailCaptureUnsubscribeToken(ctx, addr.Email, token)
		if err != nil {
			logger.Error("resolving email capture unsubscribe token", "email", addr.Email, "err", err)
			continue
		}
		unsubscribeURL := baseURL + "/unsubscribe/digest?token=" + token

		subject, plain, htmlBody := mail.ReaderDigestEmail(lang, lines, wildcardLine, unsubscribeURL)
		headers := map[string]string{
			"List-Unsubscribe":      fmt.Sprintf("<%s>", unsubscribeURL),
			"List-Unsubscribe-Post": "List-Unsubscribe=One-Click",
			"Precedence":            "bulk",
		}
		if err := mailer.SendBulk(ctx, addr.Email, subject, plain, htmlBody, mail.BrandName(lang), headers); err != nil {
			logger.Error("sending email capture digest", "email", addr.Email, "err", err)
			continue
		}

		if err := j.Store.RecordEmailCaptureDigestSent(ctx, addr.Email, sentAt); err != nil {
			return err
		}
	}
	return nil
}

// authorLabel returns the best available display name for a post's author.
func authorLabel(post store.Post) string {
	if post.AuthorName != "" {
		return post.AuthorName
	}
	if post.DisplayName != "" {
		return post.DisplayName
	}
	return post.Username
}
