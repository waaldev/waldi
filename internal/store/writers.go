package store

import (
	"context"
	"fmt"
)

func (s *Store) MarkPublishOnVerify(ctx context.Context, postID, userID int64) error {
	_, err := s.pool.Exec(ctx, `
		update posts set publish_on_verify = true
		where id = $1 and user_id = $2 and status = 'draft'
	`, postID, userID)
	if err != nil {
		return fmt.Errorf("marking post to publish on verify: %w", err)
	}
	return nil
}

func (s *Store) TakePublishOnVerify(ctx context.Context, userID int64) ([]int64, error) {
	rows, err := s.pool.Query(ctx, `
		update posts set publish_on_verify = false
		where user_id = $1 and publish_on_verify and status = 'draft'
		returning id
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("taking posts to publish on verify: %w", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning post to publish on verify: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
