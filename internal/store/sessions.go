package store

import (
	"context"
	"fmt"
	"time"
)

func (s *Store) CreateSession(ctx context.Context, token, bridgeToken string, userID int64, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		insert into sessions (token, bridge_token, user_id, expires_at)
		values ($1, $2, $3, $4)
	`, token, nullIfEmpty(bridgeToken), userID, expiresAt)
	if err != nil {
		return fmt.Errorf("creating session: %w", err)
	}
	return nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.pool.Exec(ctx, `delete from sessions where token = $1`, token)
	if err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}
	return nil
}
