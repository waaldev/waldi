package store

import (
	"context"
	"fmt"
	"time"
)

type PostEngagement struct {
	Readers int
	Letters int
}

func (s *Store) PostEngagementByUser(ctx context.Context, userID int64) (map[int64]PostEngagement, error) {
	rows, err := s.pool.Query(ctx, `
		select p.id,
		       count(distinct i.reader_key) filter (
		         where i.reader_key <> ('user:' || p.user_id::text)
		       )::int as readers,
		       count(distinct l.id)::int as letters
		from posts p
		left join impressions i on i.post_id = p.id
		left join letters l on l.post_id = p.id
		where p.user_id = $1 and p.status = 'published' and p.type = 'post'
		group by p.id
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("loading post engagement: %w", err)
	}
	defer rows.Close()

	out := make(map[int64]PostEngagement)
	for rows.Next() {
		var postID int64
		var e PostEngagement
		if err := rows.Scan(&postID, &e.Readers, &e.Letters); err != nil {
			return nil, fmt.Errorf("scanning post engagement: %w", err)
		}
		out[postID] = e
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating post engagement: %w", err)
	}
	return out, nil
}

type PostStats struct {
	PostID      int64
	PostTitle   string
	Readers     int
	Completed   int
	Follows     int
	Letters     int
	PublishedAt *time.Time
}

func (s *Store) PostStatsForUser(ctx context.Context, userID int64, since time.Time, limit int) ([]PostStats, error) {
	rows, err := s.pool.Query(ctx, `
		select p.id,
		       p.title,
		       count(distinct i.reader_key) filter (
		         where i.reader_key <> ('user:' || p.user_id::text)
		           and greatest(i.created_at, coalesce(r.updated_at, i.created_at)) >= $2
		       )::int as readers,
		       count(distinct i.reader_key) filter (
		         where i.reader_key <> ('user:' || p.user_id::text)
		           and r.completed
		           and greatest(i.created_at, coalesce(r.updated_at, i.created_at)) >= $2
		       )::int as completed,
		       count(distinct f.follower_id)::int as follows,
		       count(distinct l.id)::int as letters,
		       p.published_at
		from posts p
		left join impressions i on i.post_id = p.id
		left join readings r on r.impression_id = i.id
		left join follows f on f.source_post_id = p.id and f.created_at >= $2
		left join letters l on l.post_id = p.id and l.created_at >= $2
		where p.user_id = $1 and p.status = 'published'
		group by p.id, p.title, p.published_at, p.user_id
		having count(distinct i.reader_key) filter (
		         where i.reader_key <> ('user:' || p.user_id::text)
		           and greatest(i.created_at, coalesce(r.updated_at, i.created_at)) >= $2
		       ) > 0
		    or count(distinct f.follower_id) > 0
		    or count(distinct l.id) > 0
		order by max(coalesce(i.created_at, r.updated_at, f.created_at, l.created_at, p.published_at)) desc
		limit $3
	`, userID, since, limit)
	if err != nil {
		return nil, fmt.Errorf("loading post stats: %w", err)
	}
	defer rows.Close()

	var stats []PostStats
	for rows.Next() {
		var stat PostStats
		if err := rows.Scan(
			&stat.PostID,
			&stat.PostTitle,
			&stat.Readers,
			&stat.Completed,
			&stat.Follows,
			&stat.Letters,
			publishedAtStatDest(&stat),
		); err != nil {
			return nil, fmt.Errorf("scanning post stats: %w", err)
		}
		stats = append(stats, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating post stats: %w", err)
	}
	return stats, nil
}

func (s *Store) UsersWithDigestActivity(ctx context.Context, since time.Time, limit int) ([]User, error) {
	rows, err := s.pool.Query(ctx, `
		select distinct u.id, u.username, u.email, u.password_hash, u.locale,
		       u.display_name, u.author_name, u.bio, u.blog_lang,
		       u.email_verified_at, u.email_verify_token, u.email_verify_sent_at,
		       u.password_reset_token, u.password_reset_expires_at,
		       u.created_at, u.custom_domain, u.custom_domain_token, u.custom_domain_verified_at,
		       u.digest_unsubscribe_token, u.digest_unsubscribed_at, u.can_write,
		       u.last_active_at, u.digest_paused_at
		from users u
		join posts p on p.user_id = u.id
		left join impressions i on i.post_id = p.id
			and greatest(i.created_at, coalesce((
				select max(r.updated_at) from readings r where r.impression_id = i.id
			), i.created_at)) >= $1
		left join follows f on f.source_post_id = p.id and f.created_at >= $1
		left join letters l on l.post_id = p.id and l.created_at >= $1
		where (i.id is not null or f.follower_id is not null or l.id is not null)
		  and u.email_verified_at is not null
		  and u.email <> ''
		  and u.digest_unsubscribed_at is null
		  and u.digest_paused_at is null
		order by u.id
		limit $2
	`, since, limit)
	if err != nil {
		return nil, fmt.Errorf("loading digest users: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(scanUser(&user)...); err != nil {
			return nil, fmt.Errorf("scanning digest user: %w", err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating digest users: %w", err)
	}
	return users, nil
}

func (s *Store) DigestSentToday(ctx context.Context, userID int64, kind string, day time.Time) (bool, error) {
	start := beginningOfDay(day)
	end := start.Add(24 * time.Hour)

	var sent bool
	if err := s.pool.QueryRow(ctx, `
		select exists(
			select 1 from digests
			where user_id = $1 and kind = $2 and sent_at >= $3 and sent_at < $4
		)
	`, userID, kind, start, end).Scan(&sent); err != nil {
		return false, fmt.Errorf("checking digest idempotency: %w", err)
	}
	return sent, nil
}

func (s *Store) RecordDigestSent(ctx context.Context, userID int64, kind string, sentAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		insert into digests (user_id, kind, sent_at)
		values ($1, $2, $3)
		on conflict do nothing
	`, userID, kind, sentAt)
	if err != nil {
		return fmt.Errorf("recording digest: %w", err)
	}
	return nil
}

// VerifiedSubscribedUsers returns accounts eligible to receive the reader
// digest: verified email, not opted out of digest emails. Every candidate
// may or may not have anything to report on a given day - callers skip
// sending when there's nothing to say.
func (s *Store) VerifiedSubscribedUsers(ctx context.Context, limit int) ([]User, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := s.pool.Query(ctx, `
		select u.id, u.username, u.email, u.password_hash, u.locale,
		       u.display_name, u.author_name, u.bio, u.blog_lang,
		       u.email_verified_at, u.email_verify_token, u.email_verify_sent_at,
		       u.password_reset_token, u.password_reset_expires_at,
		       u.created_at, u.custom_domain, u.custom_domain_token, u.custom_domain_verified_at,
		       u.digest_unsubscribe_token, u.digest_unsubscribed_at, u.can_write,
		       u.last_active_at, u.digest_paused_at
		from users u
		where u.email_verified_at is not null
		  and u.email <> ''
		  and u.digest_unsubscribed_at is null
		  and u.digest_paused_at is null
		order by u.id
		limit $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("loading verified subscribed users: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(scanUser(&user)...); err != nil {
			return nil, fmt.Errorf("scanning user: %w", err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating users: %w", err)
	}
	return users, nil
}

func beginningOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

type postStatsPublishedAtScanner struct {
	stat *PostStats
}

func publishedAtStatDest(stat *PostStats) any {
	return postStatsPublishedAtScanner{stat: stat}
}

func (s postStatsPublishedAtScanner) Scan(value any) error {
	scanner := publishedAtScanner{post: &Post{}}
	if err := scanner.Scan(value); err != nil {
		return err
	}
	if scanner.post.PublishedAt != nil {
		published := *scanner.post.PublishedAt
		s.stat.PublishedAt = &published
	}
	return nil
}
