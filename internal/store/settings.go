package store

import (
	"context"
	"fmt"
)

// WildcardImpressionFloor returns how many verified strangers a post must be
// delivered to before it stops being prioritized as a stranger post.
func (s *Store) WildcardImpressionFloor(ctx context.Context) (int, error) {
	var floor int
	err := s.pool.QueryRow(ctx, `select wildcard_impression_floor from app_settings where id = true`).Scan(&floor)
	if err != nil {
		return 0, fmt.Errorf("loading wildcard impression floor: %w", err)
	}
	return floor, nil
}

func (s *Store) SetWildcardImpressionFloor(ctx context.Context, floor int) error {
	_, err := s.pool.Exec(ctx, `update app_settings set wildcard_impression_floor = $1 where id = true`, floor)
	if err != nil {
		return fmt.Errorf("setting wildcard impression floor: %w", err)
	}
	return nil
}
