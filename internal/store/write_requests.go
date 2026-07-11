package store

import (
	"context"
	"fmt"
	"time"
)

type WriteRequest struct {
	ID        int64
	UserID    int64
	BlogLink  string
	Note      string
	CreatedAt time.Time
}

func (s *Store) CreateWriteRequest(ctx context.Context, userID int64, blogLink, note string) (WriteRequest, error) {
	var wr WriteRequest
	err := s.pool.QueryRow(ctx, `
		insert into write_requests (user_id, blog_link, note)
		values ($1, $2, $3)
		returning id, user_id, blog_link, note, created_at
	`, userID, blogLink, note).Scan(&wr.ID, &wr.UserID, &wr.BlogLink, &wr.Note, &wr.CreatedAt)
	if err != nil {
		return WriteRequest{}, fmt.Errorf("creating write request: %w", err)
	}
	return wr, nil
}
