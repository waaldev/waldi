package store

import (
	"context"
	"fmt"
)

// WildcardImpressionFloor returns the minimum wildcard-impression count a
// published post must reach before it's no longer prioritized as a stranger
// recommendation; posts under the floor are preferred so the pool of
// candidate posts gets even exposure.
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
