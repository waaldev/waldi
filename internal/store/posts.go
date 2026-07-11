package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type scanner interface {
	Scan(dest ...any) error
}

type Post struct {
	ID           int64
	UserID       int64
	Username     string
	AuthorName   string
	DisplayName  string
	Title        string
	Slug         string
	Doc          json.RawMessage
	HTML         string
	Status       string
	Type         string
	PagePosition *int
	WordCount    int
	BlogLang     string
	PublishedAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

const (
	PostTypePost = "post"
	PostTypePage = "page"
)

type ImportPostParams struct {
	UserID      int64
	Title       string
	Slug        string
	Doc         json.RawMessage
	HTML        string
	WordCount   int
	Status      string
	PublishedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (s *Store) ImportPost(ctx context.Context, p ImportPostParams) (Post, error) {
	createdAt := p.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	updatedAt := p.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}

	var out Post
	err := s.pool.QueryRow(ctx, `
		with ins as (
			insert into posts (user_id, title, slug, doc, html, status, word_count, published_at, created_at, updated_at)
			values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			returning id, user_id, title, slug, doc, html, status, type, page_position,
			          word_count, published_at, created_at, updated_at
		)
		select ins.id, ins.user_id, ins.title, ins.slug, ins.doc, ins.html, ins.status, ins.type, ins.page_position,
		       ins.word_count, ins.published_at, ins.created_at, ins.updated_at, u.blog_lang
		from ins
		join users u on u.id = ins.user_id
	`, p.UserID, p.Title, p.Slug, p.Doc, p.HTML, p.Status, p.WordCount, p.PublishedAt, createdAt, updatedAt).Scan(postScanFields(&out)...)
	if err != nil {
		return Post{}, fmt.Errorf("importing post: %w", err)
	}
	return out, nil
}

func (s *Store) CreateDraft(ctx context.Context, userID int64, title, slug string, doc json.RawMessage, html string, wordCount int) (Post, error) {
	var p Post
	err := s.pool.QueryRow(ctx, `
		with ins as (
			insert into posts (user_id, title, slug, doc, html, status, word_count)
			values ($1, $2, $3, $4, $5, 'draft', $6)
			returning id, user_id, title, slug, doc, html, status, type, page_position,
			          word_count, published_at, created_at, updated_at
		)
		select ins.id, ins.user_id, ins.title, ins.slug, ins.doc, ins.html, ins.status, ins.type, ins.page_position,
		       ins.word_count, ins.published_at, ins.created_at, ins.updated_at, u.blog_lang
		from ins
		join users u on u.id = ins.user_id
	`, userID, title, slug, doc, html, wordCount).Scan(postScanFields(&p)...)
	if err != nil {
		return Post{}, fmt.Errorf("creating draft: %w", err)
	}
	return p, nil
}

// MaxPagesPerUser is the maximum number of static pages a blog may have.
const MaxPagesPerUser = 4

func (s *Store) CreatePageDraft(ctx context.Context, userID int64, title, slug string, doc json.RawMessage, html string, wordCount int) (Post, error) {
	var p Post
	err := s.pool.QueryRow(ctx, `
		with ins as (
			insert into posts (user_id, title, slug, doc, html, status, word_count, type, page_position)
			values (
				$1, $2, $3, $4, $5, 'draft', $6, 'page',
				(select coalesce(max(page_position) + 1, 0) from posts where user_id = $1 and type = 'page')
			)
			returning id, user_id, title, slug, doc, html, status, type, page_position,
			          word_count, published_at, created_at, updated_at
		)
		select ins.id, ins.user_id, ins.title, ins.slug, ins.doc, ins.html, ins.status, ins.type, ins.page_position,
		       ins.word_count, ins.published_at, ins.created_at, ins.updated_at, u.blog_lang
		from ins
		join users u on u.id = ins.user_id
	`, userID, title, slug, doc, html, wordCount).Scan(postScanFields(&p)...)
	if err != nil {
		return Post{}, fmt.Errorf("creating page draft: %w", err)
	}
	return p, nil
}

func (s *Store) PagesByUser(ctx context.Context, userID int64) ([]Post, error) {
	rows, err := s.pool.Query(ctx, `
		select p.id, p.user_id, p.title, p.slug, p.doc, p.html, p.status, p.type, p.page_position, p.word_count, p.published_at, p.created_at, p.updated_at, u.blog_lang
		from posts p
		join users u on u.id = p.user_id
		where p.user_id = $1 and p.type = 'page'
		order by p.page_position asc, p.id asc
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("listing pages: %w", err)
	}
	defer rows.Close()

	var pages []Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(postScanFields(&p)...); err != nil {
			return nil, fmt.Errorf("scanning page: %w", err)
		}
		pages = append(pages, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating pages: %w", err)
	}
	return pages, nil
}

func (s *Store) PublishedPagesByUsername(ctx context.Context, username string) ([]Post, error) {
	rows, err := s.pool.Query(ctx, `
		select p.id, p.user_id, u.username, u.author_name, u.display_name, p.title, p.slug, p.doc, p.html, p.status, p.type, p.page_position,
		       p.word_count, p.published_at, p.created_at, p.updated_at, u.blog_lang
		from posts p
		join users u on u.id = p.user_id
		where u.username = $1 and p.type = 'page' and p.status = 'published'
		order by p.page_position asc, p.id asc
	`, username)
	if err != nil {
		return nil, fmt.Errorf("listing published pages: %w", err)
	}
	defer rows.Close()

	var pages []Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(postWithUserScanFields(&p)...); err != nil {
			return nil, fmt.Errorf("scanning published page: %w", err)
		}
		pages = append(pages, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating published pages: %w", err)
	}
	return pages, nil
}

// SwapPagePositions swaps page_position between two of a user's pages, used
// to back move-up/move-down reordering in Settings.
func (s *Store) SwapPagePositions(ctx context.Context, userID, postID1, postID2 int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning page reorder: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var pos1, pos2 *int
	if err := tx.QueryRow(ctx, `select page_position from posts where id = $1 and user_id = $2 and type = 'page'`, postID1, userID).Scan(&pos1); err != nil {
		return fmt.Errorf("loading page position: %w", err)
	}
	if err := tx.QueryRow(ctx, `select page_position from posts where id = $1 and user_id = $2 and type = 'page'`, postID2, userID).Scan(&pos2); err != nil {
		return fmt.Errorf("loading page position: %w", err)
	}
	if _, err := tx.Exec(ctx, `update posts set page_position = $3 where id = $1 and user_id = $2`, postID1, userID, pos2); err != nil {
		return fmt.Errorf("swapping page position: %w", err)
	}
	if _, err := tx.Exec(ctx, `update posts set page_position = $3 where id = $1 and user_id = $2`, postID2, userID, pos1); err != nil {
		return fmt.Errorf("swapping page position: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing page reorder: %w", err)
	}
	return nil
}

func (s *Store) DeletePage(ctx context.Context, userID, postID int64) error {
	tag, err := s.pool.Exec(ctx, `delete from posts where id = $1 and user_id = $2 and type = 'page'`, postID, userID)
	if err != nil {
		return fmt.Errorf("deleting page: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// AllPostsByUser returns every post (not pages) a user has ever written,
// oldest first, for the full data-export download.
func (s *Store) AllPostsByUser(ctx context.Context, userID int64) ([]Post, error) {
	rows, err := s.pool.Query(ctx, `
		select p.id, p.user_id, p.title, p.slug, p.doc, p.html, p.status, p.type, p.page_position, p.word_count, p.published_at, p.created_at, p.updated_at, u.blog_lang
		from posts p
		join users u on u.id = p.user_id
		where p.user_id = $1 and p.type = 'post'
		order by p.created_at asc
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("listing all user posts: %w", err)
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(postScanFields(&p)...); err != nil {
			return nil, fmt.Errorf("scanning user post: %w", err)
		}
		posts = append(posts, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating all user posts: %w", err)
	}
	return posts, nil
}

func (s *Store) UpdatePost(ctx context.Context, postID, userID int64, title, slug string, doc json.RawMessage, html string, wordCount int) (Post, error) {
	var p Post
	err := s.pool.QueryRow(ctx, `
		with upd as (
			update posts
			set title = $3, slug = $4, doc = $5, html = $6, word_count = $7, updated_at = now()
			where id = $1 and user_id = $2
			returning id, user_id, title, slug, doc, html, status, type, page_position,
			          word_count, published_at, created_at, updated_at
		)
		select upd.id, upd.user_id, upd.title, upd.slug, upd.doc, upd.html, upd.status, upd.type, upd.page_position,
		       upd.word_count, upd.published_at, upd.created_at, upd.updated_at, u.blog_lang
		from upd
		join users u on u.id = upd.user_id
	`, postID, userID, title, slug, doc, html, wordCount).Scan(postScanFields(&p)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return Post{}, ErrNotFound
	}
	if err != nil {
		return Post{}, fmt.Errorf("updating post: %w", err)
	}
	return p, nil
}

// UpdateDraft is an alias for UpdatePost kept for clarity at call sites that only touch drafts.
func (s *Store) UpdateDraft(ctx context.Context, postID, userID int64, title, slug string, doc json.RawMessage, html string, wordCount int) (Post, error) {
	return s.UpdatePost(ctx, postID, userID, title, slug, doc, html, wordCount)
}

func (s *Store) PublishPost(ctx context.Context, postID, userID int64) (Post, error) {
	var p Post
	err := s.pool.QueryRow(ctx, `
		with upd as (
			update posts
			set status = 'published',
			    published_at = coalesce(published_at, now()),
			    updated_at = now()
			where id = $1 and user_id = $2
			returning id, user_id, title, slug, doc, html, status, type, page_position,
			          word_count, published_at, created_at, updated_at
		)
		select upd.id, upd.user_id, upd.title, upd.slug, upd.doc, upd.html, upd.status, upd.type, upd.page_position,
		       upd.word_count, upd.published_at, upd.created_at, upd.updated_at, u.blog_lang
		from upd
		join users u on u.id = upd.user_id
	`, postID, userID).Scan(postScanFields(&p)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return Post{}, ErrNotFound
	}
	if err != nil {
		return Post{}, fmt.Errorf("publishing post: %w", err)
	}
	return p, nil
}

func (s *Store) UnpublishPost(ctx context.Context, postID, userID int64) (Post, error) {
	var p Post
	err := s.pool.QueryRow(ctx, `
		with upd as (
			update posts
			set status = 'draft',
			    updated_at = now()
			where id = $1 and user_id = $2 and status = 'published'
			returning id, user_id, title, slug, doc, html, status, type, page_position,
			          word_count, published_at, created_at, updated_at
		)
		select upd.id, upd.user_id, upd.title, upd.slug, upd.doc, upd.html, upd.status, upd.type, upd.page_position,
		       upd.word_count, upd.published_at, upd.created_at, upd.updated_at, u.blog_lang
		from upd
		join users u on u.id = upd.user_id
	`, postID, userID).Scan(postScanFields(&p)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return Post{}, ErrNotFound
	}
	if err != nil {
		return Post{}, fmt.Errorf("unpublishing post: %w", err)
	}
	return p, nil
}

// UpdatePublishedAt changes the displayed publish date of one of userID's
// own published posts. It never touches drafts (they get a real
// published_at the first time via PublishPost).
func (s *Store) UpdatePublishedAt(ctx context.Context, postID, userID int64, publishedAt time.Time) (Post, error) {
	var p Post
	err := s.pool.QueryRow(ctx, `
		with upd as (
			update posts
			set published_at = $3,
			    updated_at = now()
			where id = $1 and user_id = $2 and status = 'published'
			returning id, user_id, title, slug, doc, html, status, type, page_position,
			          word_count, published_at, created_at, updated_at
		)
		select upd.id, upd.user_id, upd.title, upd.slug, upd.doc, upd.html, upd.status, upd.type, upd.page_position,
		       upd.word_count, upd.published_at, upd.created_at, upd.updated_at, u.blog_lang
		from upd
		join users u on u.id = upd.user_id
	`, postID, userID, publishedAt).Scan(postScanFields(&p)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return Post{}, ErrNotFound
	}
	if err != nil {
		return Post{}, fmt.Errorf("updating published_at: %w", err)
	}
	return p, nil
}

// DeletePost removes one of userID's own posts (draft or published). The
// slug is recorded in deleted_posts first so its public URL can keep
// answering 410 Gone instead of 404 after the row is gone.
func (s *Store) DeletePost(ctx context.Context, postID, userID int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning post delete: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var slug string
	var status string
	err = tx.QueryRow(ctx, `select slug, status from posts where id = $1 and user_id = $2 and type = 'post'`, postID, userID).Scan(&slug, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("loading post for delete: %w", err)
	}

	if status == "published" {
		if _, err := tx.Exec(ctx, `
			insert into deleted_posts (user_id, slug)
			values ($1, $2)
			on conflict (user_id, slug) do update set deleted_at = now()
		`, userID, slug); err != nil {
			return fmt.Errorf("recording deleted post: %w", err)
		}
	}

	if _, err := tx.Exec(ctx, `delete from posts where id = $1 and user_id = $2`, postID, userID); err != nil {
		return fmt.Errorf("deleting post: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing post delete: %w", err)
	}
	return nil
}

// WasPostDeleted reports whether username once had a published post at slug
// that was subsequently deleted, so callers can respond 410 Gone instead of
// a plain 404 for its old URL.
func (s *Store) WasPostDeleted(ctx context.Context, username, slug string) (bool, error) {
	var exists bool
	if err := s.pool.QueryRow(ctx, `
		select exists(
			select 1
			from deleted_posts d
			join users u on u.id = d.user_id
			where u.username = $1 and d.slug = $2
		)
	`, username, slug).Scan(&exists); err != nil {
		return false, fmt.Errorf("checking deleted post: %w", err)
	}
	return exists, nil
}

func (s *Store) PostByIDForUser(ctx context.Context, postID, userID int64) (Post, error) {
	var p Post
	err := s.pool.QueryRow(ctx, `
		select p.id, p.user_id, p.title, p.slug, p.doc, p.html, p.status, p.type, p.page_position, p.word_count, p.published_at, p.created_at, p.updated_at, u.blog_lang
		from posts p
		join users u on u.id = p.user_id
		where p.id = $1 and p.user_id = $2
	`, postID, userID).Scan(postScanFields(&p)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return Post{}, ErrNotFound
	}
	if err != nil {
		return Post{}, fmt.Errorf("finding post: %w", err)
	}
	return p, nil
}

func (s *Store) PublishedPostByID(ctx context.Context, postID int64) (Post, error) {
	var p Post
	err := s.pool.QueryRow(ctx, `
		select p.id, p.user_id, u.username, u.author_name, u.display_name, p.title, p.slug, p.doc, p.html, p.status, p.type, p.page_position,
		       p.word_count, p.published_at, p.created_at, p.updated_at, u.blog_lang
		from posts p
		join users u on u.id = p.user_id
		where p.id = $1 and p.status = 'published'
	`, postID).Scan(postWithUserScanFields(&p)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return Post{}, ErrNotFound
	}
	if err != nil {
		return Post{}, fmt.Errorf("finding published post by id: %w", err)
	}
	return p, nil
}

func (s *Store) PublishedPostByUsernameAndSlug(ctx context.Context, username, slug string) (Post, error) {
	var p Post
	err := s.pool.QueryRow(ctx, `
		select p.id, p.user_id, u.username, u.author_name, u.display_name, p.title, p.slug, p.doc, p.html, p.status, p.type, p.page_position,
		       p.word_count, p.published_at, p.created_at, p.updated_at, u.blog_lang
		from posts p
		join users u on u.id = p.user_id
		where u.username = $1 and p.slug = $2 and p.status = 'published'
	`, username, slug).Scan(postWithUserScanFields(&p)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return Post{}, ErrNotFound
	}
	if err != nil {
		return Post{}, fmt.Errorf("finding published post: %w", err)
	}
	return p, nil
}

func (s *Store) PublishedPostsByUsername(ctx context.Context, username string, limit int, cursor PageCursor) ([]Post, error) {
	before, lastID := cursorArgs(cursor)
	rows, err := s.pool.Query(ctx, `
		select p.id, p.user_id, u.username, u.author_name, u.display_name, p.title, p.slug, p.doc, p.html, p.status, p.type, p.page_position,
		       p.word_count, p.published_at, p.created_at, p.updated_at, u.blog_lang
		from posts p
		join users u on u.id = p.user_id
		where u.username = $1 and p.status = 'published' and p.type = 'post'
		  and (
		    $3::timestamptz is null
		    or p.published_at < $3::timestamptz
		    or ($4::bigint is not null and p.published_at = $3::timestamptz and p.id < $4::bigint)
		  )
		order by p.published_at desc, p.id desc
		limit $2
	`, username, limit, before, lastID)
	if err != nil {
		return nil, fmt.Errorf("listing published posts: %w", err)
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(postWithUserScanFields(&p)...); err != nil {
			return nil, fmt.Errorf("scanning published post: %w", err)
		}
		posts = append(posts, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating published posts: %w", err)
	}
	return posts, nil
}

func (s *Store) RecentPublishedPosts(ctx context.Context, limit int) ([]Post, error) {
	rows, err := s.pool.Query(ctx, `
		select p.id, p.user_id, u.username, u.author_name, u.display_name, p.title, p.slug, p.doc, p.html, p.status, p.type, p.page_position,
		       p.word_count, p.published_at, p.created_at, p.updated_at, u.blog_lang
		from posts p
		join users u on u.id = p.user_id
		where p.status = 'published' and p.type = 'post'
		order by p.published_at desc
		limit $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("listing recent published posts: %w", err)
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(postWithUserScanFields(&p)...); err != nil {
			return nil, fmt.Errorf("scanning recent published post: %w", err)
		}
		posts = append(posts, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating recent published posts: %w", err)
	}
	return posts, nil
}

func (s *Store) DraftsByUser(ctx context.Context, userID int64, limit int) ([]Post, error) {
	rows, err := s.pool.Query(ctx, `
		select p.id, p.user_id, p.title, p.slug, p.doc, p.html, p.status, p.type, p.page_position, p.word_count, p.published_at, p.created_at, p.updated_at, u.blog_lang
		from posts p
		join users u on u.id = p.user_id
		where p.user_id = $1 and p.type = 'post' and p.status = 'draft'
		order by p.updated_at desc, p.id desc
		limit $2
	`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("listing user drafts: %w", err)
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(postScanFields(&p)...); err != nil {
			return nil, fmt.Errorf("scanning user draft: %w", err)
		}
		posts = append(posts, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating user drafts: %w", err)
	}
	return posts, nil
}

func (s *Store) PublishedPostsByUser(ctx context.Context, userID int64, limit int, cursor PageCursor) ([]Post, error) {
	before, lastID := cursorArgs(cursor)
	rows, err := s.pool.Query(ctx, `
		select p.id, p.user_id, p.title, p.slug, p.doc, p.html, p.status, p.type, p.page_position, p.word_count, p.published_at, p.created_at, p.updated_at, u.blog_lang
		from posts p
		join users u on u.id = p.user_id
		where p.user_id = $1 and p.type = 'post' and p.status = 'published'
		  and (
		    $3::timestamptz is null
		    or p.published_at < $3::timestamptz
		    or ($4::bigint is not null and p.published_at = $3::timestamptz and p.id < $4::bigint)
		  )
		order by p.published_at desc nulls last, p.id desc
		limit $2
	`, userID, limit, before, lastID)
	if err != nil {
		return nil, fmt.Errorf("listing user published posts: %w", err)
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(postScanFields(&p)...); err != nil {
			return nil, fmt.Errorf("scanning user published post: %w", err)
		}
		posts = append(posts, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating user published posts: %w", err)
	}
	return posts, nil
}

func (s *Store) PostsByUser(ctx context.Context, userID int64, limit int, cursor PageCursor) ([]Post, error) {
	before, lastID := cursorArgs(cursor)
	rows, err := s.pool.Query(ctx, `
		select p.id, p.user_id, p.title, p.slug, p.doc, p.html, p.status, p.type, p.page_position, p.word_count, p.published_at, p.created_at, p.updated_at, u.blog_lang
		from posts p
		join users u on u.id = p.user_id
		where p.user_id = $1 and p.type = 'post'
		  and (
		    $3::timestamptz is null
		    or (`+postDashboardSortSQL+`) < $3::timestamptz
		    or (
		      $4::bigint is not null
		      and (`+postDashboardSortSQL+`) = $3::timestamptz
		      and id < $4::bigint
		    )
		  )
		order by `+postDashboardSortSQL+` desc, id desc
		limit $2
	`, userID, limit, before, lastID)
	if err != nil {
		return nil, fmt.Errorf("listing user posts: %w", err)
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(postScanFields(&p)...); err != nil {
			return nil, fmt.Errorf("scanning user post: %w", err)
		}
		posts = append(posts, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating user posts: %w", err)
	}
	return posts, nil
}

func (s *Store) FeedPosts(ctx context.Context, userID int64, since time.Time, limit int) ([]Post, error) {
	rows, err := s.pool.Query(ctx, `
		select p.id, p.user_id, u.username, u.author_name, u.display_name, p.title, p.slug, p.doc, p.html, p.status, p.type, p.page_position,
		       p.word_count, p.published_at, p.created_at, p.updated_at, u.blog_lang
		from posts p
		join users u on u.id = p.user_id
		join follows f on f.followee_id = p.user_id
		where f.follower_id = $1 and p.status = 'published' and p.type = 'post' and p.published_at >= $2
		order by p.published_at desc, p.id desc
		limit $3
	`, userID, since, limit)
	if err != nil {
		return nil, fmt.Errorf("listing feed posts: %w", err)
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(postWithUserScanFields(&p)...); err != nil {
			return nil, fmt.Errorf("scanning feed post: %w", err)
		}
		posts = append(posts, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating feed posts: %w", err)
	}
	return posts, nil
}

func (s *Store) WildcardCandidate(ctx context.Context, userID int64, readerLang string, day time.Time, impressionFloor int) (Post, error) {
	var p Post
	err := s.pool.QueryRow(ctx, `
		with candidates as (
			select p.id, p.user_id, u.username, u.author_name, u.display_name, p.title, p.slug, p.doc, p.html, p.status, p.type, p.page_position,
			       p.word_count, p.published_at, p.created_at, p.updated_at, u.blog_lang,
			       count(distinct i.reader_key) filter (where i.source = 'wildcard')::int as wildcard_impressions
			from posts p
			join users u on u.id = p.user_id
			left join impressions i on i.post_id = p.id
			where p.status = 'published'
			  and p.type = 'post'
			  and p.word_count >= 50
			  and lower(p.title) !~ '(^|[^a-z])test([^a-z]|$)'
			  and lower(p.slug) !~ '(^|[^a-z])test([^a-z]|$)'
			  and u.blog_lang = $4
			  and p.user_id <> $1
			  and not exists (
			    select 1 from follows f
			    where f.follower_id = $1 and f.followee_id = p.user_id
			  )
			  and not exists (
			    select 1 from wildcards w
			    where w.user_id = $1 and w.post_id = p.id and w.date < $2
			  )
			group by p.id, u.username, u.author_name, u.display_name, u.blog_lang
		)
		select id, user_id, username, author_name, display_name, title, slug, doc, html, status, type, page_position,
		       word_count, published_at, created_at, updated_at, blog_lang
		from candidates
		order by
		  case when wildcard_impressions < $3 then 0 else 1 end,
		  case when wildcard_impressions < $3 then wildcard_impressions end asc,
		  case when wildcard_impressions < $3 then published_at end asc,
		  published_at desc
		limit 1
	`, userID, day, impressionFloor, readerLang).Scan(postWithUserScanFields(&p)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return Post{}, ErrNotFound
	}
	if err != nil {
		return Post{}, fmt.Errorf("finding wildcard candidate: %w", err)
	}
	return p, nil
}

func (s *Store) AssignedWildcard(ctx context.Context, userID int64, day time.Time) (Post, error) {
	var p Post
	err := s.pool.QueryRow(ctx, `
		select p.id, p.user_id, u.username, u.author_name, u.display_name, p.title, p.slug, p.doc, p.html, p.status, p.type, p.page_position,
		       p.word_count, p.published_at, p.created_at, p.updated_at, u.blog_lang
		from wildcards w
		join posts p on p.id = w.post_id
		join users u on u.id = p.user_id
		where w.user_id = $1 and w.date = $2 and not w.skipped and p.status = 'published'
	`, userID, day).Scan(postWithUserScanFields(&p)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return Post{}, ErrNotFound
	}
	if err != nil {
		return Post{}, fmt.Errorf("finding assigned wildcard: %w", err)
	}
	return p, nil
}

func (s *Store) AssignWildcard(ctx context.Context, userID, postID int64, day time.Time) error {
	_, err := s.pool.Exec(ctx, `
		insert into wildcards (user_id, post_id, date)
		values ($1, $2, $3)
		on conflict (user_id, date) do nothing
	`, userID, postID, day)
	if err != nil {
		return fmt.Errorf("assigning wildcard: %w", err)
	}
	return nil
}

func (s *Store) SetWildcard(ctx context.Context, userID, postID int64, day time.Time) error {
	_, err := s.pool.Exec(ctx, `
		insert into wildcards (user_id, post_id, date, skipped)
		values ($1, $2, $3, false)
		on conflict (user_id, date) do update
		set post_id = excluded.post_id, skipped = false
	`, userID, postID, day)
	if err != nil {
		return fmt.Errorf("setting wildcard: %w", err)
	}
	return nil
}

func (s *Store) WildcardSlotTaken(ctx context.Context, userID int64, day time.Time) (bool, error) {
	var taken bool
	if err := s.pool.QueryRow(ctx, `
		select exists(
			select 1 from wildcards where user_id = $1 and date = $2
		)
	`, userID, day).Scan(&taken); err != nil {
		return false, fmt.Errorf("checking wildcard slot: %w", err)
	}
	return taken, nil
}

func (s *Store) SkipWildcard(ctx context.Context, userID int64, day time.Time) error {
	_, err := s.pool.Exec(ctx, `
		update wildcards
		set skipped = true
		where user_id = $1 and date = $2
	`, userID, day)
	if err != nil {
		return fmt.Errorf("skipping wildcard: %w", err)
	}
	return nil
}

func (s *Store) UsersNeedingWildcard(ctx context.Context, day time.Time, limit int) ([]User, error) {
	rows, err := s.pool.Query(ctx, `
		select u.id, u.username, u.email, u.password_hash, u.locale,
		       u.display_name, u.author_name, u.bio, u.blog_lang,
		       u.email_verified_at, u.email_verify_token, u.email_verify_sent_at,
		       u.password_reset_token, u.password_reset_expires_at,
		       u.created_at, u.custom_domain, u.custom_domain_token, u.custom_domain_verified_at,
		       u.digest_unsubscribe_token, u.digest_unsubscribed_at, u.can_write
		from users u
		where not exists (
			select 1 from wildcards w
			where w.user_id = u.id and w.date = $1
		)
		order by u.id
		limit $2
	`, day, limit)
	if err != nil {
		return nil, fmt.Errorf("loading users needing wildcard: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(scanUser(&user)...); err != nil {
			return nil, fmt.Errorf("scanning wildcard user: %w", err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating wildcard users: %w", err)
	}
	return users, nil
}

// AddToWildcardPool queues a published post as an admin-curated wildcard
// candidate; it stays in the pool (and can be handed out to many readers)
// until explicitly removed.
func (s *Store) AddToWildcardPool(ctx context.Context, postID int64) error {
	_, err := s.pool.Exec(ctx, `
		insert into wildcard_pool (post_id)
		values ($1)
		on conflict (post_id) do nothing
	`, postID)
	if err != nil {
		return fmt.Errorf("adding to wildcard pool: %w", err)
	}
	return nil
}

func (s *Store) RemoveFromWildcardPool(ctx context.Context, postID int64) error {
	_, err := s.pool.Exec(ctx, `delete from wildcard_pool where post_id = $1`, postID)
	if err != nil {
		return fmt.Errorf("removing from wildcard pool: %w", err)
	}
	return nil
}

// ListWildcardPool returns pooled posts oldest-added first.
func (s *Store) ListWildcardPool(ctx context.Context, limit int) ([]Post, error) {
	rows, err := s.pool.Query(ctx, `
		select p.id, p.user_id, u.username, u.author_name, u.display_name, p.title, p.slug, p.doc, p.html, p.status, p.type, p.page_position,
		       p.word_count, p.published_at, p.created_at, p.updated_at, u.blog_lang
		from wildcard_pool wp
		join posts p on p.id = wp.post_id
		join users u on u.id = p.user_id
		order by wp.added_at asc
		limit $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("listing wildcard pool: %w", err)
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(postWithUserScanFields(&p)...); err != nil {
			return nil, fmt.Errorf("scanning wildcard pool post: %w", err)
		}
		posts = append(posts, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating wildcard pool: %w", err)
	}
	return posts, nil
}

// WildcardPoolCandidate picks the oldest-added pooled post this reader
// hasn't already been shown, matching their reading language and excluding
// their own posts and people they already follow. Returns ErrNotFound if
// the pool has nothing suitable, so callers can fall back to the
// impression-based algorithm.
func (s *Store) WildcardPoolCandidate(ctx context.Context, userID int64, readerLang string, day time.Time) (Post, error) {
	var p Post
	err := s.pool.QueryRow(ctx, `
		select p.id, p.user_id, u.username, u.author_name, u.display_name, p.title, p.slug, p.doc, p.html, p.status, p.type, p.page_position,
		       p.word_count, p.published_at, p.created_at, p.updated_at, u.blog_lang
		from wildcard_pool wp
		join posts p on p.id = wp.post_id
		join users u on u.id = p.user_id
		where p.status = 'published'
		  and u.blog_lang = $2
		  and p.user_id <> $1
		  and not exists (
		    select 1 from follows f
		    where f.follower_id = $1 and f.followee_id = p.user_id
		  )
		  and not exists (
		    select 1 from wildcards w
		    where w.user_id = $1 and w.post_id = p.id and w.date < $3
		  )
		order by wp.added_at asc
		limit 1
	`, userID, readerLang, day).Scan(postWithUserScanFields(&p)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return Post{}, ErrNotFound
	}
	if err != nil {
		return Post{}, fmt.Errorf("finding wildcard pool candidate: %w", err)
	}
	return p, nil
}

func (s *Store) UniqueSlug(ctx context.Context, userID int64, base string, excludePostID int64) (string, error) {
	slug := base
	for i := 2; i < 1000; i++ {
		available, err := s.slugAvailable(ctx, userID, slug, excludePostID)
		if err != nil {
			return "", err
		}
		if available {
			return slug, nil
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
	return "", fmt.Errorf("finding unique slug for %q: too many collisions", base)
}

// SlugAvailable reports whether slug is free for userID, ignoring
// excludePostID (pass 0 when checking a brand-new post/page).
func (s *Store) SlugAvailable(ctx context.Context, userID int64, slug string, excludePostID int64) (bool, error) {
	return s.slugAvailable(ctx, userID, slug, excludePostID)
}

// RenamePageSlug updates only a page's slug (leaving its content/title
// untouched), used by the Settings pages list to let owners pick their own
// URL rather than one derived from the title.
func (s *Store) RenamePageSlug(ctx context.Context, userID, postID int64, slug string) error {
	tag, err := s.pool.Exec(ctx, `
		update posts set slug = $3, updated_at = now()
		where id = $1 and user_id = $2 and type = 'page'
	`, postID, userID, slug)
	if err != nil {
		return fmt.Errorf("renaming page slug: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) slugAvailable(ctx context.Context, userID int64, slug string, excludePostID int64) (bool, error) {
	var exists bool
	if err := s.pool.QueryRow(ctx, `
		select exists(
			select 1
			from posts
			where user_id = $1 and slug = $2 and id <> $3
		)
	`, userID, slug, excludePostID).Scan(&exists); err != nil {
		return false, fmt.Errorf("checking slug availability: %w", err)
	}
	return !exists, nil
}

func postScanFields(p *Post) []any {
	return []any{
		&p.ID,
		&p.UserID,
		&p.Title,
		&p.Slug,
		&p.Doc,
		&p.HTML,
		&p.Status,
		&p.Type,
		&p.PagePosition,
		&p.WordCount,
		publishedAtDest(p),
		&p.CreatedAt,
		&p.UpdatedAt,
		&p.BlogLang,
	}
}

func postWithUserScanFields(p *Post) []any {
	return []any{
		&p.ID,
		&p.UserID,
		&p.Username,
		&p.AuthorName,
		&p.DisplayName,
		&p.Title,
		&p.Slug,
		&p.Doc,
		&p.HTML,
		&p.Status,
		&p.Type,
		&p.PagePosition,
		&p.WordCount,
		publishedAtDest(p),
		&p.CreatedAt,
		&p.UpdatedAt,
		&p.BlogLang,
	}
}

type publishedAtScanner struct {
	post *Post
}

func publishedAtDest(p *Post) any {
	return publishedAtScanner{post: p}
}

func (s publishedAtScanner) Scan(value any) error {
	var t sql.NullTime
	if err := t.Scan(value); err != nil {
		return err
	}
	if t.Valid {
		published := t.Time
		s.post.PublishedAt = &published
	}
	return nil
}
