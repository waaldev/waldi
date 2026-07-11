package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// CreateEmailCapture stores an anonymous visitor's email address against the
// blog they subscribed from, ignoring duplicates. The same address may be
// captured against several different blogs (one row per email+blog) so that
// signing up later can follow all of them — see AdoptEmailCaptureFollows.
func (s *Store) CreateEmailCapture(ctx context.Context, email, sourceUsername string, sourcePostID *int64) error {
	_, err := s.pool.Exec(ctx, `
		insert into email_captures (email, source_username, source_post_id)
		values ($1, $2, $3)
		on conflict (email, source_username) do nothing
	`, email, sourceUsername, sourcePostID)
	if err != nil {
		return fmt.Errorf("creating email capture: %w", err)
	}
	return nil
}

// AdoptEmailCaptureFollows follows, on behalf of a newly created account,
// every blog that email address had anonymously subscribed to before
// signing up. Self-follows (signing up under the username you'd captured
// from) are skipped; already-existing follows are left untouched.
func (s *Store) AdoptEmailCaptureFollows(ctx context.Context, userID int64, email string) error {
	_, err := s.pool.Exec(ctx, `
		insert into follows (follower_id, followee_id, source_post_id)
		select $1, u.id, ec.source_post_id
		from email_captures ec
		join users u on u.username = ec.source_username
		where ec.email = $2 and u.id <> $1
		on conflict (follower_id, followee_id) do nothing
	`, userID, email)
	if err != nil {
		return fmt.Errorf("adopting email capture follows: %w", err)
	}
	return nil
}

// DeleteEmailCapturesByEmail removes an email's capture rows and any digest
// bookkeeping for it — called once that address becomes a real account, so
// it stops being treated as anonymous.
func (s *Store) DeleteEmailCapturesByEmail(ctx context.Context, email string) error {
	_, err := s.pool.Exec(ctx, `delete from email_captures where email = $1`, email)
	if err != nil {
		return fmt.Errorf("deleting email captures: %w", err)
	}
	_, err = s.pool.Exec(ctx, `delete from email_capture_digests where email = $1`, email)
	if err != nil {
		return fmt.Errorf("deleting email capture digest record: %w", err)
	}
	return nil
}

// EmailCaptureDigestPost is one new post to report to an anonymous captured
// email, from a blog they subscribed to before signing up.
type EmailCaptureDigestPost struct {
	Email       string
	BlogLang    string
	AuthorLabel string
	PostTitle   string
	Username    string
	Slug        string
}

// EmailCaptureFolloweePosts returns, for every still-anonymous captured
// email, posts published since `since` by the blogs that address subscribed
// to — the anonymous-capture equivalent of a registered reader's followee
// feed. Ordered by email so callers can group consecutive rows.
func (s *Store) EmailCaptureFolloweePosts(ctx context.Context, since time.Time, limit int) ([]EmailCaptureDigestPost, error) {
	if limit <= 0 {
		limit = 5000
	}
	rows, err := s.pool.Query(ctx, `
		select ec.email, u.blog_lang, coalesce(nullif(u.author_name, ''), nullif(u.display_name, ''), u.username), p.title, u.username, p.slug
		from email_captures ec
		join users u on u.username = ec.source_username
		join posts p on p.user_id = u.id
		where p.status = 'published' and p.type = 'post' and p.published_at >= $1
		order by ec.email, p.published_at desc
		limit $2
	`, since, limit)
	if err != nil {
		return nil, fmt.Errorf("loading email capture followee posts: %w", err)
	}
	defer rows.Close()

	var posts []EmailCaptureDigestPost
	for rows.Next() {
		var p EmailCaptureDigestPost
		if err := rows.Scan(&p.Email, &p.BlogLang, &p.AuthorLabel, &p.PostTitle, &p.Username, &p.Slug); err != nil {
			return nil, fmt.Errorf("scanning email capture followee post: %w", err)
		}
		posts = append(posts, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating email capture followee posts: %w", err)
	}
	return posts, nil
}

// EmailCaptureAddress is one still-anonymous captured email, with a
// representative blog language (from its most recently captured source)
// standing in for the account locale it doesn't have.
type EmailCaptureAddress struct {
	Email string
	Lang  string
}

// EmailCaptureAddresses lists every distinct still-anonymous captured
// email — the full candidate pool for the reader digest's anonymous pass,
// independent of whether they have any fresh followee posts today (someone
// with nothing new from their subscribed blogs can still get a wildcard).
func (s *Store) EmailCaptureAddresses(ctx context.Context, limit int) ([]EmailCaptureAddress, error) {
	if limit <= 0 {
		limit = 5000
	}
	rows, err := s.pool.Query(ctx, `
		select distinct on (ec.email) ec.email, u.blog_lang
		from email_captures ec
		join users u on u.username = ec.source_username
		order by ec.email, ec.created_at desc
		limit $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("listing email capture addresses: %w", err)
	}
	defer rows.Close()

	var addresses []EmailCaptureAddress
	for rows.Next() {
		var a EmailCaptureAddress
		if err := rows.Scan(&a.Email, &a.Lang); err != nil {
			return nil, fmt.Errorf("scanning email capture address: %w", err)
		}
		addresses = append(addresses, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating email capture addresses: %w", err)
	}
	return addresses, nil
}

// EmailCaptureWildcard picks a "today's stranger" post for an anonymous
// captured email's digest: an admin-curated wildcard_pool post if one is
// eligible, otherwise a random published post — in both cases excluding
// blogs that email already subscribed to. Anonymous addresses have no
// account to track prior wildcard history against, so this doesn't attempt
// the impression-based dedup registered readers get; a straight random pick
// is the honest equivalent here.
func (s *Store) EmailCaptureWildcard(ctx context.Context, email, readerLang string) (Post, error) {
	var p Post
	err := s.pool.QueryRow(ctx, `
		select p.id, p.user_id, u.username, u.author_name, u.display_name, p.title, p.slug, p.doc, p.html, p.status, p.type, p.page_position,
		       p.word_count, p.published_at, p.created_at, p.updated_at, u.blog_lang
		from wildcard_pool wp
		join posts p on p.id = wp.post_id
		join users u on u.id = p.user_id
		where p.status = 'published'
		  and u.blog_lang = $2
		  and not exists (
		    select 1 from email_captures ec
		    where ec.email = $1 and ec.source_username = u.username
		  )
		order by random()
		limit 1
	`, email, readerLang).Scan(postWithUserScanFields(&p)...)
	if err == nil {
		return p, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Post{}, fmt.Errorf("finding email capture wildcard pool candidate: %w", err)
	}

	err = s.pool.QueryRow(ctx, `
		select p.id, p.user_id, u.username, u.author_name, u.display_name, p.title, p.slug, p.doc, p.html, p.status, p.type, p.page_position,
		       p.word_count, p.published_at, p.created_at, p.updated_at, u.blog_lang
		from posts p
		join users u on u.id = p.user_id
		where p.status = 'published'
		  and p.type = 'post'
		  and p.word_count >= 50
		  and lower(p.title) !~ '(^|[^a-z])test([^a-z]|$)'
		  and lower(p.slug) !~ '(^|[^a-z])test([^a-z]|$)'
		  and u.blog_lang = $2
		  and not exists (
		    select 1 from email_captures ec
		    where ec.email = $1 and ec.source_username = u.username
		  )
		order by random()
		limit 1
	`, email, readerLang).Scan(postWithUserScanFields(&p)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return Post{}, ErrNotFound
	}
	if err != nil {
		return Post{}, fmt.Errorf("finding email capture wildcard candidate: %w", err)
	}
	return p, nil
}

// EmailCaptureDigestSentToday reports whether the given anonymous captured
// email already received a digest today.
func (s *Store) EmailCaptureDigestSentToday(ctx context.Context, email string, day time.Time) (bool, error) {
	start := beginningOfDay(day)
	end := start.Add(24 * time.Hour)
	var sent bool
	if err := s.pool.QueryRow(ctx, `
		select exists(
			select 1 from email_capture_digests
			where email = $1 and sent_at >= $2 and sent_at < $3
		)
	`, email, start, end).Scan(&sent); err != nil {
		return false, fmt.Errorf("checking email capture digest idempotency: %w", err)
	}
	return sent, nil
}

// EmailCaptureUnsubscribeToken returns the persistent unsubscribe token for
// an anonymous captured email, generating one on first use.
func (s *Store) EmailCaptureUnsubscribeToken(ctx context.Context, email, generatedToken string) (string, error) {
	var resolved string
	err := s.pool.QueryRow(ctx, `
		insert into email_capture_digests (email, unsubscribe_token)
		values ($1, $2)
		on conflict (email) do update set email = excluded.email
		returning unsubscribe_token
	`, email, generatedToken).Scan(&resolved)
	if err != nil {
		return "", fmt.Errorf("resolving email capture unsubscribe token: %w", err)
	}
	return resolved, nil
}

// RecordEmailCaptureDigestSent marks an anonymous captured email as having
// received today's digest.
func (s *Store) RecordEmailCaptureDigestSent(ctx context.Context, email string, sentAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		update email_capture_digests set sent_at = $2 where email = $1
	`, email, sentAt)
	if err != nil {
		return fmt.Errorf("recording email capture digest sent: %w", err)
	}
	return nil
}

// EmailCaptureTokenExists reports whether an unsubscribe token belongs to an
// anonymous captured email (used by the shared /unsubscribe/digest page to
// validate the link before showing the confirm button).
func (s *Store) EmailCaptureTokenExists(ctx context.Context, token string) (bool, error) {
	var exists bool
	if err := s.pool.QueryRow(ctx, `
		select exists(select 1 from email_capture_digests where unsubscribe_token = $1)
	`, token).Scan(&exists); err != nil {
		return false, fmt.Errorf("checking email capture unsubscribe token: %w", err)
	}
	return exists, nil
}

// UnsubscribeEmailCaptureByToken removes all capture data for the email
// matching an unsubscribe token — both the follow-adoption source rows and
// the digest bookkeeping — so the address is fully forgotten.
func (s *Store) UnsubscribeEmailCaptureByToken(ctx context.Context, token string) error {
	var email string
	err := s.pool.QueryRow(ctx, `
		delete from email_capture_digests where unsubscribe_token = $1
		returning email
	`, token).Scan(&email)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("unsubscribing email capture: %w", err)
	}
	if _, err := s.pool.Exec(ctx, `delete from email_captures where email = $1`, email); err != nil {
		return fmt.Errorf("deleting email captures on unsubscribe: %w", err)
	}
	return nil
}
