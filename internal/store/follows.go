package store

import (
	"context"
	"fmt"
)

func (s *Store) Follow(ctx context.Context, followerID, followeeID int64, sourcePostID *int64) error {
	_, err := s.pool.Exec(ctx, `
		insert into follows (follower_id, followee_id, source_post_id)
		values ($1, $2, $3)
		on conflict (follower_id, followee_id) do nothing
	`, followerID, followeeID, sourcePostID)
	if err != nil {
		return fmt.Errorf("following user: %w", err)
	}
	return nil
}

func (s *Store) Unfollow(ctx context.Context, followerID, followeeID int64) error {
	_, err := s.pool.Exec(ctx, `
		delete from follows
		where follower_id = $1 and followee_id = $2
	`, followerID, followeeID)
	if err != nil {
		return fmt.Errorf("unfollowing user: %w", err)
	}
	return nil
}

func (s *Store) IsFollowing(ctx context.Context, followerID, followeeID int64) (bool, error) {
	var following bool
	if err := s.pool.QueryRow(ctx, `
		select exists(
			select 1
			from follows
			where follower_id = $1 and followee_id = $2
		)
	`, followerID, followeeID).Scan(&following); err != nil {
		return false, fmt.Errorf("checking follow: %w", err)
	}
	return following, nil
}
