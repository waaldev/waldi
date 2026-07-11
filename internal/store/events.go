package store

import (
	"context"
	"fmt"
)

type ImpressionSource string

const (
	ImpressionDirect   ImpressionSource = "direct"
	ImpressionFeed     ImpressionSource = "feed"
	ImpressionWildcard ImpressionSource = "wildcard"
)

func (s ImpressionSource) Valid() bool {
	switch s {
	case ImpressionDirect, ImpressionFeed, ImpressionWildcard:
		return true
	default:
		return false
	}
}

func (s *Store) EnsureImpression(ctx context.Context, postID int64, readerKey string, userID *int64, source ImpressionSource) (int64, error) {
	if readerKey == "" {
		return 0, fmt.Errorf("reader key is required")
	}
	if !source.Valid() {
		source = ImpressionDirect
	}

	var id int64
	err := s.pool.QueryRow(ctx, `
		insert into impressions (post_id, user_id, source, reader_key)
		values ($1, $2, $3, $4)
		on conflict (post_id, reader_key) do update
		set user_id = coalesce(impressions.user_id, excluded.user_id)
		returning id
	`, postID, userID, string(source), readerKey).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("ensuring impression: %w", err)
	}
	return id, nil
}

func (s *Store) CreateImpression(ctx context.Context, postID int64, userID *int64, source ImpressionSource) (int64, error) {
	readerKey := "legacy:anonymous"
	if userID != nil {
		readerKey = fmt.Sprintf("user:%d", *userID)
	}
	return s.EnsureImpression(ctx, postID, readerKey, userID, source)
}

func (s *Store) UpsertReading(ctx context.Context, impressionID int64, maxScrollPct, dwellSeconds int, completed bool) error {
	_, err := s.pool.Exec(ctx, `
		insert into readings (impression_id, max_scroll_pct, dwell_seconds, completed, updated_at)
		values ($1, $2, $3, $4, now())
		on conflict (impression_id) do update
		set max_scroll_pct = greatest(readings.max_scroll_pct, excluded.max_scroll_pct),
		    dwell_seconds = greatest(readings.dwell_seconds, excluded.dwell_seconds),
		    completed = readings.completed or excluded.completed,
		    updated_at = now()
	`, impressionID, maxScrollPct, dwellSeconds, completed)
	if err != nil {
		return fmt.Errorf("upserting reading: %w", err)
	}
	return nil
}
