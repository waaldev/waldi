package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrUsernameTaken = errors.New("username already in use")
	ErrEmailTaken    = errors.New("email already in use")
)

func (s *Store) ListUsersRecent(ctx context.Context, limit int) ([]User, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		select `+strings.TrimSpace(userColumns)+`
		from users
		order by created_at desc
		limit $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("listing recent users: %w", err)
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

func (s *Store) UpdateUsername(ctx context.Context, userID int64, username string) (User, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	var user User
	err := s.pool.QueryRow(ctx, `
		update users set username = $2 where id = $1
		returning `+userColumns+`
	`, userID, username).Scan(scanUser(&user)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return User{}, ErrUsernameTaken
		}
		if errors.As(err, &pgErr) && pgErr.Code == "23514" {
			return User{}, fmt.Errorf("invalid username format")
		}
		return User{}, fmt.Errorf("updating username: %w", err)
	}
	return user, nil
}

func (s *Store) UpdateEmail(ctx context.Context, userID int64, email string) (User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var user User
	err := s.pool.QueryRow(ctx, `
		update users set email = $2 where id = $1
		returning `+userColumns+`
	`, userID, email).Scan(scanUser(&user)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return User{}, ErrEmailTaken
		}
		return User{}, fmt.Errorf("updating email: %w", err)
	}
	return user, nil
}

func (s *Store) DeleteUser(ctx context.Context, userID int64) error {
	tag, err := s.pool.Exec(ctx, `delete from users where id = $1`, userID)
	if err != nil {
		return fmt.Errorf("deleting user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
