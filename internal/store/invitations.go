package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

var ErrInviteInvalid = errors.New("invite invalid")

type Invitation struct {
	ID        int64
	Code      string
	Note      string
	CreatedAt time.Time
	UsedAt    *time.Time
	UsedBy    *int64
}

func (s *Store) CreateInvitation(ctx context.Context, code, note string) (Invitation, error) {
	var inv Invitation
	err := s.pool.QueryRow(ctx, `
		insert into invitations (code, note)
		values ($1, $2)
		returning id, code, note, created_at, used_at, used_by_user_id
	`, code, note).Scan(&inv.ID, &inv.Code, &inv.Note, &inv.CreatedAt, &inv.UsedAt, &inv.UsedBy)
	if err != nil {
		return Invitation{}, fmt.Errorf("creating invitation: %w", err)
	}
	return inv, nil
}

func (s *Store) ListInvitations(ctx context.Context, limit int) ([]Invitation, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.pool.Query(ctx, `
		select id, code, note, created_at, used_at, used_by_user_id
		from invitations
		order by created_at desc
		limit $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("listing invitations: %w", err)
	}
	defer rows.Close()

	var invitations []Invitation
	for rows.Next() {
		var inv Invitation
		if err := rows.Scan(&inv.ID, &inv.Code, &inv.Note, &inv.CreatedAt, &inv.UsedAt, &inv.UsedBy); err != nil {
			return nil, fmt.Errorf("scanning invitation: %w", err)
		}
		invitations = append(invitations, inv)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating invitations: %w", err)
	}
	return invitations, nil
}

func (s *Store) InvitationAvailable(ctx context.Context, code string) (bool, error) {
	var available bool
	err := s.pool.QueryRow(ctx, `
		select exists(
			select 1 from invitations
			where code = $1 and used_at is null
		)
	`, code).Scan(&available)
	if err != nil {
		return false, fmt.Errorf("checking invitation: %w", err)
	}
	return available, nil
}

// CreateUserAndRedeemInvitation creates a new account and immediately
// redeems an invite code for it in one transaction, granting write access
// on creation - the `?invite=` signup shortcut.
func (s *Store) CreateUserAndRedeemInvitation(ctx context.Context, inviteCode, username, email, passwordHash, locale, verifyToken, displayName, bio string) (User, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return User{}, fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	inviteID, inviterID, err := lockInvitation(ctx, tx, inviteCode)
	if err != nil {
		return User{}, err
	}

	var user User
	err = tx.QueryRow(ctx, `
		insert into users (username, email, password_hash, locale, blog_lang, email_verify_token, email_verify_sent_at, can_write, display_name, bio)
		values ($1, $2, $3, $4, $4, $5, now(), true, $6, $7)
		returning `+userColumns+`
	`, username, email, passwordHash, locale, verifyToken, displayName, bio).Scan(scanUser(&user)...)
	if err != nil {
		return User{}, fmt.Errorf("creating user: %w", err)
	}

	if err := markInvitationUsed(ctx, tx, inviteID, inviterID, user.ID); err != nil {
		return User{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return User{}, fmt.Errorf("committing transaction: %w", err)
	}
	return user, nil
}

// RedeemInvitationForUser grants write access to an already-existing account
// by redeeming an invite code for it - the "I already have a reader account,
// redeeming a code later" path used by the write-invite page.
func (s *Store) RedeemInvitationForUser(ctx context.Context, code string, userID int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	inviteID, inviterID, err := lockInvitation(ctx, tx, code)
	if err != nil {
		return err
	}
	if inviterID != nil && *inviterID == userID {
		return ErrInviteInvalid
	}

	if err := markInvitationUsed(ctx, tx, inviteID, inviterID, userID); err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `update users set can_write = true, blog_lang = locale where id = $1`, userID)
	if err != nil {
		return fmt.Errorf("granting write access: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	return nil
}

func lockInvitation(ctx context.Context, tx pgx.Tx, code string) (int64, *int64, error) {
	var inviteID int64
	var inviterID *int64
	err := tx.QueryRow(ctx, `
		select id, invited_by_user_id from invitations
		where code = $1 and used_at is null
		for update
	`, code).Scan(&inviteID, &inviterID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil, ErrInviteInvalid
	}
	if err != nil {
		return 0, nil, fmt.Errorf("locking invitation: %w", err)
	}
	return inviteID, inviterID, nil
}

func markInvitationUsed(ctx context.Context, tx pgx.Tx, inviteID int64, inviterID *int64, userID int64) error {
	_, err := tx.Exec(ctx, `
		update invitations
		set used_at = now(), used_by_user_id = $2
		where id = $1
	`, inviteID, userID)
	if err != nil {
		return fmt.Errorf("redeeming invitation: %w", err)
	}
	if inviterID == nil {
		return nil
	}

	_, err = tx.Exec(ctx, `update users set stranger_hold = true where id = $1`, userID)
	if err != nil {
		return fmt.Errorf("holding invited writer from strangers: %w", err)
	}
	_, err = tx.Exec(ctx, `
		insert into follows (follower_id, followee_id)
		values ($1, $2)
		on conflict (follower_id, followee_id) do nothing
	`, *inviterID, userID)
	if err != nil {
		return fmt.Errorf("following invited writer: %w", err)
	}
	return nil
}
