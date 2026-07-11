package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type Letter struct {
	ID              int64
	PostID          int64
	PostTitle       string
	PostSlug        string
	FromUserID      int64
	FromUsername    string
	FromAuthorName  string
	FromDisplayName string
	ToUserID        int64
	Body            string
	CreatedAt       time.Time
	ReadAt          *time.Time
}

func (s *Store) CreateLetter(ctx context.Context, postID, fromUserID, toUserID int64, body string) (Letter, error) {
	var letter Letter
	err := s.pool.QueryRow(ctx, `
		insert into letters (post_id, from_user, to_user, body)
		values ($1, $2, $3, $4)
		returning id, post_id, from_user, to_user, body, created_at, read_at
	`, postID, fromUserID, toUserID, body).Scan(letterScanFields(&letter)...)
	if err != nil {
		return Letter{}, fmt.Errorf("creating letter: %w", err)
	}
	return letter, nil
}

// CompletedReadingsCount counts posts (excluding the reader's own) a user
// has completed reading — the "you write to a writer after reading them"
// qualifying signal for sending a letter.
func (s *Store) CompletedReadingsCount(ctx context.Context, userID int64) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `
		select count(*)
		from impressions i
		join readings r on r.impression_id = i.id
		join posts p on p.id = i.post_id
		where i.user_id = $1 and r.completed and p.user_id <> $1
	`, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting completed readings: %w", err)
	}
	return count, nil
}

// LettersSentSince counts letters a user has sent since a given time — used
// to enforce a daily letter rate limit.
func (s *Store) LettersSentSince(ctx context.Context, userID int64, since time.Time) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `
		select count(*) from letters where from_user = $1 and created_at >= $2
	`, userID, since).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting recent letters: %w", err)
	}
	return count, nil
}

func (s *Store) LettersForUser(ctx context.Context, userID int64, since time.Time, limit int) ([]Letter, error) {
	rows, err := s.pool.Query(ctx, `
		select l.id, l.post_id, p.title, p.slug, l.from_user, u.username, u.author_name, u.display_name, l.to_user, l.body, l.created_at, l.read_at
		from letters l
		join users u on u.id = l.from_user
		join posts p on p.id = l.post_id
		where l.to_user = $1
		  and (l.read_at is null or l.created_at >= $2)
		order by l.created_at desc, l.id desc
		limit $3
	`, userID, since, limit)
	if err != nil {
		return nil, fmt.Errorf("listing letters: %w", err)
	}
	defer rows.Close()

	var letters []Letter
	for rows.Next() {
		var letter Letter
		if err := rows.Scan(letterWithPostScanFields(&letter)...); err != nil {
			return nil, fmt.Errorf("scanning letter: %w", err)
		}
		letters = append(letters, letter)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating letters: %w", err)
	}
	return letters, nil
}

func (s *Store) LetterForUser(ctx context.Context, letterID, userID int64) (Letter, error) {
	var letter Letter
	err := s.pool.QueryRow(ctx, `
		select l.id, l.post_id, p.title, p.slug, l.from_user, u.username, u.author_name, u.display_name, l.to_user, l.body, l.created_at, l.read_at
		from letters l
		join users u on u.id = l.from_user
		join posts p on p.id = l.post_id
		where l.id = $1 and l.to_user = $2
	`, letterID, userID).Scan(letterWithPostScanFields(&letter)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return Letter{}, ErrNotFound
	}
	if err != nil {
		return Letter{}, fmt.Errorf("finding letter: %w", err)
	}
	return letter, nil
}

func (s *Store) MarkLetterRead(ctx context.Context, letterID, userID int64) error {
	_, err := s.pool.Exec(ctx, `
		update letters
		set read_at = coalesce(read_at, now())
		where id = $1 and to_user = $2
	`, letterID, userID)
	if err != nil {
		return fmt.Errorf("marking letter read: %w", err)
	}
	return nil
}

func letterScanFields(l *Letter) []any {
	return []any{
		&l.ID,
		&l.PostID,
		&l.FromUserID,
		&l.ToUserID,
		&l.Body,
		&l.CreatedAt,
		letterReadAtDest(l),
	}
}

func letterWithPostScanFields(l *Letter) []any {
	return []any{
		&l.ID,
		&l.PostID,
		&l.PostTitle,
		&l.PostSlug,
		&l.FromUserID,
		&l.FromUsername,
		&l.FromAuthorName,
		&l.FromDisplayName,
		&l.ToUserID,
		&l.Body,
		&l.CreatedAt,
		letterReadAtDest(l),
	}
}

type letterReadAtScanner struct {
	letter *Letter
}

func letterReadAtDest(l *Letter) any {
	return letterReadAtScanner{letter: l}
}

func (s letterReadAtScanner) Scan(value any) error {
	var t sql.NullTime
	if err := t.Scan(value); err != nil {
		return err
	}
	if t.Valid {
		readAt := t.Time
		s.letter.ReadAt = &readAt
	}
	return nil
}
