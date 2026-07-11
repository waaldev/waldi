package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrNotFound    = errors.New("not found")
	ErrDomainTaken = errors.New("domain already in use")
)

type User struct {
	ID                     int64
	Username               string
	Email                  string
	PasswordHash           string
	Locale                 string
	DisplayName            string
	AuthorName             string
	Bio                    string
	BlogLang               string
	EmailVerifiedAt        *time.Time
	EmailVerifyToken       *string
	EmailVerifySentAt      *time.Time
	PasswordResetToken     *string
	PasswordResetExpiresAt *time.Time
	CreatedAt              time.Time
	CustomDomain           *string
	CustomDomainToken      *string
	CustomDomainVerifiedAt *time.Time
	DigestUnsubscribeToken *string
	DigestUnsubscribedAt   *time.Time
	CanWrite               bool
}

func (u User) EmailVerified() bool {
	return u.EmailVerifiedAt != nil
}

// DigestUnsubscribed reports whether the user has opted out of digest emails.
func (u User) DigestUnsubscribed() bool {
	return u.DigestUnsubscribedAt != nil
}

// ActiveCustomDomain returns the user's custom domain and true if it has
// been verified and should be treated as the blog's canonical host.
func (u User) ActiveCustomDomain() (string, bool) {
	if u.CustomDomain == nil || u.CustomDomainVerifiedAt == nil {
		return "", false
	}
	return *u.CustomDomain, true
}

func scanUser(dest *User) []any {
	return []any{
		&dest.ID,
		&dest.Username,
		&dest.Email,
		&dest.PasswordHash,
		&dest.Locale,
		&dest.DisplayName,
		&dest.AuthorName,
		&dest.Bio,
		&dest.BlogLang,
		&dest.EmailVerifiedAt,
		&dest.EmailVerifyToken,
		&dest.EmailVerifySentAt,
		&dest.PasswordResetToken,
		&dest.PasswordResetExpiresAt,
		&dest.CreatedAt,
		&dest.CustomDomain,
		&dest.CustomDomainToken,
		&dest.CustomDomainVerifiedAt,
		&dest.DigestUnsubscribeToken,
		&dest.DigestUnsubscribedAt,
		&dest.CanWrite,
	}
}

const userColumns = `
	id, username, email, password_hash, locale,
	display_name, author_name, bio, blog_lang,
	email_verified_at, email_verify_token, email_verify_sent_at,
	password_reset_token, password_reset_expires_at,
	created_at, custom_domain, custom_domain_token, custom_domain_verified_at,
	digest_unsubscribe_token, digest_unsubscribed_at, can_write`

func (s *Store) CreateUser(ctx context.Context, username, email, passwordHash, locale, verifyToken string) (User, error) {
	var user User
	err := s.pool.QueryRow(ctx, `
		insert into users (username, email, password_hash, locale, blog_lang, email_verify_token, email_verify_sent_at)
		values ($1, $2, $3, $4, $4, $5, now())
		returning `+userColumns+`
	`, username, email, passwordHash, locale, verifyToken).Scan(scanUser(&user)...)
	if err != nil {
		return User{}, fmt.Errorf("creating user: %w", err)
	}
	return user, nil
}

func (s *Store) UpdateUserLocale(ctx context.Context, userID int64, locale string) error {
	_, err := s.pool.Exec(ctx, `update users set locale = $2 where id = $1`, userID, locale)
	if err != nil {
		return fmt.Errorf("updating user locale: %w", err)
	}
	return nil
}

func (s *Store) UpdateBlogProfile(ctx context.Context, userID int64, displayName, authorName, bio, blogLang string) error {
	_, err := s.pool.Exec(ctx, `
		update users
		set display_name = $2, author_name = $3, bio = $4, blog_lang = $5
		where id = $1
	`, userID, displayName, authorName, bio, blogLang)
	if err != nil {
		return fmt.Errorf("updating blog profile: %w", err)
	}
	return nil
}

func (s *Store) UpdatePasswordHash(ctx context.Context, userID int64, passwordHash string) error {
	_, err := s.pool.Exec(ctx, `update users set password_hash = $2 where id = $1`, userID, passwordHash)
	if err != nil {
		return fmt.Errorf("updating password hash: %w", err)
	}
	return nil
}

func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	return s.userBy(ctx, `email = $1`, email)
}

func (s *Store) UserByUsername(ctx context.Context, username string) (User, error) {
	return s.userBy(ctx, `username = $1`, username)
}

func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	return s.userByID(ctx, id)
}

func (s *Store) ListUsers(ctx context.Context, limit int) ([]User, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := s.pool.Query(ctx, `
		select `+strings.TrimSpace(userColumns)+`
		from users
		order by id
		limit $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
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

func (s *Store) UserBySessionToken(ctx context.Context, token string) (User, error) {
	var user User
	err := s.pool.QueryRow(ctx, `
		select u.id, u.username, u.email, u.password_hash, u.locale,
		       u.display_name, u.author_name, u.bio, u.blog_lang,
		       u.email_verified_at, u.email_verify_token, u.email_verify_sent_at,
		       u.password_reset_token, u.password_reset_expires_at,
		       u.created_at, u.custom_domain, u.custom_domain_token, u.custom_domain_verified_at,
		       u.digest_unsubscribe_token, u.digest_unsubscribed_at, u.can_write
		from sessions s
		join users u on u.id = s.user_id
		where s.token = $1 and s.expires_at > now()
	`, token).Scan(scanUser(&user)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("finding user by session: %w", err)
	}
	return user, nil
}

func (s *Store) UserAndSessionByBridgeToken(ctx context.Context, bridgeToken string) (User, string, error) {
	var user User
	var sessionToken string
	err := s.pool.QueryRow(ctx, `
		select u.id, u.username, u.email, u.password_hash, u.locale,
		       u.display_name, u.author_name, u.bio, u.blog_lang,
		       u.email_verified_at, u.email_verify_token, u.email_verify_sent_at,
		       u.password_reset_token, u.password_reset_expires_at,
		       u.created_at, u.custom_domain, u.custom_domain_token, u.custom_domain_verified_at,
		       u.digest_unsubscribe_token, u.digest_unsubscribed_at, u.can_write,
		       s.token
		from sessions s
		join users u on u.id = s.user_id
		where s.bridge_token = $1 and s.expires_at > now()
	`, bridgeToken).Scan(append(scanUser(&user), &sessionToken)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, "", ErrNotFound
	}
	if err != nil {
		return User{}, "", fmt.Errorf("finding user by bridge token: %w", err)
	}
	return user, sessionToken, nil
}

func (s *Store) userBy(ctx context.Context, where string, arg string) (User, error) {
	var user User
	err := s.pool.QueryRow(ctx, `
		select `+strings.TrimSpace(userColumns)+`
		from users
		where `+where+`
	`, arg).Scan(scanUser(&user)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("finding user: %w", err)
	}
	return user, nil
}

func (s *Store) userByID(ctx context.Context, id int64) (User, error) {
	var user User
	err := s.pool.QueryRow(ctx, `
		select `+strings.TrimSpace(userColumns)+`
		from users
		where id = $1
	`, id).Scan(scanUser(&user)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("finding user by id: %w", err)
	}
	return user, nil
}

func (s *Store) VerifyEmailByToken(ctx context.Context, token string) (User, error) {
	var user User
	err := s.pool.QueryRow(ctx, `
		update users
		set email_verified_at = now(),
		    email_verify_token = null,
		    email_verify_sent_at = null
		where email_verify_token = $1 and email_verified_at is null
		returning `+userColumns+`
	`, token).Scan(scanUser(&user)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("verifying email: %w", err)
	}
	return user, nil
}

func (s *Store) SetEmailVerifyToken(ctx context.Context, userID int64, token string) error {
	_, err := s.pool.Exec(ctx, `
		update users
		set email_verify_token = $2, email_verify_sent_at = now()
		where id = $1
	`, userID, token)
	if err != nil {
		return fmt.Errorf("setting verify token: %w", err)
	}
	return nil
}

func (s *Store) VerifyEmailByAddress(ctx context.Context, email string) (User, error) {
	var user User
	err := s.pool.QueryRow(ctx, `
		update users
		set email_verified_at = coalesce(email_verified_at, now()),
		    email_verify_token = null,
		    email_verify_sent_at = null
		where email = $1
		returning `+userColumns+`
	`, email).Scan(scanUser(&user)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("verifying email by address: %w", err)
	}
	return user, nil
}

func (s *Store) SetPasswordResetToken(ctx context.Context, userID int64, token string, expires time.Time) error {
	_, err := s.pool.Exec(ctx, `
		update users
		set password_reset_token = $2, password_reset_expires_at = $3
		where id = $1
	`, userID, token, expires)
	if err != nil {
		return fmt.Errorf("setting password reset token: %w", err)
	}
	return nil
}

func (s *Store) ResetPasswordByToken(ctx context.Context, token, passwordHash string) (User, error) {
	var user User
	err := s.pool.QueryRow(ctx, `
		update users
		set password_hash = $2,
		    password_reset_token = null,
		    password_reset_expires_at = null
		where password_reset_token = $1
		  and password_reset_expires_at > now()
		returning `+userColumns+`
	`, token, passwordHash).Scan(scanUser(&user)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("resetting password: %w", err)
	}
	return user, nil
}

// SetCustomDomain assigns (or replaces) the user's custom domain and resets
// its verification state; the domain must be re-verified before it becomes
// active.
func (s *Store) SetCustomDomain(ctx context.Context, userID int64, domain, token string) (User, error) {
	var user User
	err := s.pool.QueryRow(ctx, `
		update users
		set custom_domain = $2, custom_domain_token = $3, custom_domain_verified_at = null
		where id = $1
		returning `+userColumns+`
	`, userID, domain, token).Scan(scanUser(&user)...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return User{}, ErrDomainTaken
		}
		return User{}, fmt.Errorf("setting custom domain: %w", err)
	}
	return user, nil
}

// VerifyCustomDomain marks the user's currently-set custom domain as verified.
func (s *Store) VerifyCustomDomain(ctx context.Context, userID int64) (User, error) {
	var user User
	err := s.pool.QueryRow(ctx, `
		update users
		set custom_domain_verified_at = now()
		where id = $1 and custom_domain is not null
		returning `+userColumns+`
	`, userID).Scan(scanUser(&user)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("verifying custom domain: %w", err)
	}
	return user, nil
}

// ClearCustomDomain removes the user's custom domain and its verification state.
func (s *Store) ClearCustomDomain(ctx context.Context, userID int64) error {
	_, err := s.pool.Exec(ctx, `
		update users
		set custom_domain = null, custom_domain_token = null, custom_domain_verified_at = null
		where id = $1
	`, userID)
	if err != nil {
		return fmt.Errorf("clearing custom domain: %w", err)
	}
	return nil
}

// SetDigestUnsubscribeToken assigns a persistent token used to build one-click
// unsubscribe links in digest emails, if the user doesn't already have one,
// and returns the token now on record (existing or newly set).
func (s *Store) SetDigestUnsubscribeToken(ctx context.Context, userID int64, token string) (string, error) {
	var resolved string
	err := s.pool.QueryRow(ctx, `
		update users
		set digest_unsubscribe_token = coalesce(digest_unsubscribe_token, $2)
		where id = $1
		returning digest_unsubscribe_token
	`, userID, token).Scan(&resolved)
	if err != nil {
		return "", fmt.Errorf("setting digest unsubscribe token: %w", err)
	}
	return resolved, nil
}

// UserByDigestUnsubscribeToken looks up the user owning a digest unsubscribe
// token, without changing their subscription state.
func (s *Store) UserByDigestUnsubscribeToken(ctx context.Context, token string) (User, error) {
	return s.userBy(ctx, `digest_unsubscribe_token = $1`, token)
}

// UnsubscribeFromDigestByToken marks the user matching the given token as
// opted out of digest emails.
func (s *Store) UnsubscribeFromDigestByToken(ctx context.Context, token string) (User, error) {
	var user User
	err := s.pool.QueryRow(ctx, `
		update users
		set digest_unsubscribed_at = coalesce(digest_unsubscribed_at, now())
		where digest_unsubscribe_token = $1
		returning `+userColumns+`
	`, token).Scan(scanUser(&user)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("unsubscribing from digest: %w", err)
	}
	return user, nil
}

// UserByCustomDomain looks up the user whose verified custom domain matches
// the given host. Only verified (active) domains are matched.
func (s *Store) UserByCustomDomain(ctx context.Context, domain string) (User, error) {
	return s.userBy(ctx, `custom_domain = $1 and custom_domain_verified_at is not null`, domain)
}
