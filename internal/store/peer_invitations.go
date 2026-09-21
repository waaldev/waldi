package store

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const InvitationsPerUser = 2

const peerInviteCodeBytes = 9

type UserInvitation struct {
	Code      string
	CreatedAt time.Time
	UsedAt    *time.Time
	Invitee   *User
	Following bool
	FirstPost *Post
}

func (s *Store) EnsureUserInvitations(ctx context.Context, userID int64) error {
	var have int
	if err := s.pool.QueryRow(ctx, `
		select count(*) from invitations where invited_by_user_id = $1
	`, userID).Scan(&have); err != nil {
		return fmt.Errorf("counting user invitations: %w", err)
	}
	for i := have; i < InvitationsPerUser; i++ {
		code, err := newPeerInviteCode()
		if err != nil {
			return fmt.Errorf("generating invite code: %w", err)
		}
		if _, err := s.pool.Exec(ctx, `
			insert into invitations (code, invited_by_user_id)
			values ($1, $2)
		`, code, userID); err != nil {
			return fmt.Errorf("creating user invitation: %w", err)
		}
	}
	return nil
}

func (s *Store) UserInvitations(ctx context.Context, userID int64) ([]UserInvitation, error) {
	rows, err := s.pool.Query(ctx, `
		select i.code, i.created_at, i.used_at, i.used_by_user_id,
		       exists(
		         select 1 from follows f
		         where f.follower_id = $1 and f.followee_id = i.used_by_user_id
		       )
		from invitations i
		where i.invited_by_user_id = $1
		order by i.used_at is not null, i.used_at desc, i.id
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("listing user invitations: %w", err)
	}

	var invitations []UserInvitation
	var inviteeIDs []*int64
	for rows.Next() {
		var inv UserInvitation
		var inviteeID *int64
		if err := rows.Scan(&inv.Code, &inv.CreatedAt, &inv.UsedAt, &inviteeID, &inv.Following); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scanning user invitation: %w", err)
		}
		invitations = append(invitations, inv)
		inviteeIDs = append(inviteeIDs, inviteeID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating user invitations: %w", err)
	}

	for i, inviteeID := range inviteeIDs {
		if inviteeID == nil {
			continue
		}
		invitee, err := s.UserByID(ctx, *inviteeID)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		invitations[i].Invitee = &invitee

		post, err := s.firstPublishedPost(ctx, invitee.ID)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		invitations[i].FirstPost = &post
	}
	return invitations, nil
}

func (s *Store) firstPublishedPost(ctx context.Context, userID int64) (Post, error) {
	var p Post
	err := s.pool.QueryRow(ctx, `
		select p.id, p.user_id, p.title, p.slug, p.doc, p.html, p.status, p.type, p.page_position,
		       p.word_count, p.published_at, p.created_at, p.updated_at, u.blog_lang
		from posts p
		join users u on u.id = p.user_id
		where p.user_id = $1 and p.status = 'published' and p.type = 'post'
		order by p.published_at, p.id
		limit 1
	`, userID).Scan(postScanFields(&p)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return Post{}, ErrNotFound
	}
	if err != nil {
		return Post{}, fmt.Errorf("finding first published post: %w", err)
	}
	return p, nil
}

func (s *Store) InvitationInviter(ctx context.Context, code string) (User, error) {
	var inviterID *int64
	err := s.pool.QueryRow(ctx, `
		select invited_by_user_id from invitations
		where code = $1 and used_at is null
	`, code).Scan(&inviterID)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrInviteInvalid
	}
	if err != nil {
		return User{}, fmt.Errorf("finding invitation: %w", err)
	}
	if inviterID == nil {
		return User{}, ErrNotFound
	}
	return s.UserByID(ctx, *inviterID)
}

func (s *Store) ClaimFirstPostNotification(ctx context.Context, inviteeID int64) (User, error) {
	var inviterID int64
	err := s.pool.QueryRow(ctx, `
		update invitations
		set first_post_notified_at = now()
		where used_by_user_id = $1
		  and invited_by_user_id is not null
		  and first_post_notified_at is null
		returning invited_by_user_id
	`, inviteeID).Scan(&inviterID)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("claiming first post notification: %w", err)
	}
	return s.UserByID(ctx, inviterID)
}

func (s *Store) StrangerHold(ctx context.Context, userID int64) (bool, error) {
	var held bool
	err := s.pool.QueryRow(ctx, `select stranger_hold from users where id = $1`, userID).Scan(&held)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrNotFound
	}
	if err != nil {
		return false, fmt.Errorf("checking stranger hold: %w", err)
	}
	return held, nil
}

func (s *Store) ReleaseStrangerHold(ctx context.Context, userID int64) error {
	tag, err := s.pool.Exec(ctx, `update users set stranger_hold = false where id = $1`, userID)
	if err != nil {
		return fmt.Errorf("releasing stranger hold: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) StrangerHeldUsers(ctx context.Context) ([]User, error) {
	rows, err := s.pool.Query(ctx, `
		select `+strings.TrimSpace(userColumns)+`
		from users
		where stranger_hold
		order by created_at desc
	`)
	if err != nil {
		return nil, fmt.Errorf("listing held writers: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(scanUser(&user)...); err != nil {
			return nil, fmt.Errorf("scanning held writer: %w", err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating held writers: %w", err)
	}
	return users, nil
}

func newPeerInviteCode() (string, error) {
	buf := make([]byte, peerInviteCodeBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
