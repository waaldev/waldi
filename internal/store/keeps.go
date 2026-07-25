package store

import (
	"context"
	"fmt"
	"time"
)

// KeptPost is a published post on a reader's private shelf, along with when
// they kept it - the keep time (not the post's publish time) drives the
// shelf's ordering and pagination.
type KeptPost struct {
	Post
	KeptAt time.Time
	// SourceLetterID is set when the post was kept from a received letter
	// rather than the home feed, so the shelf can link back to the letter
	// itself instead of the bare public post. The remaining SourceLetter
	// fields carry that letter's sender and body, so the shelf can render
	// it the way the inbox does rather than as the underlying post.
	SourceLetterID          *int64
	SourceLetterBody        string
	SourceLetterFromName    string
	SourceLetterFromDisplay string
	SourceLetterFromUser    string
}

// Keep adds a post to a reader's private shelf. Kept posts are never shown
// to anyone but the reader who kept them - there is no public keep count.
// letterID records that the keep came from a received letter about the post,
// so the shelf can later link back to that letter; pass nil when keeping
// from the home feed. Re-keeping a post already on the shelf fills in a
// missing source letter rather than overwriting one that's already set.
func (s *Store) Keep(ctx context.Context, userID, postID int64, letterID *int64) error {
	_, err := s.pool.Exec(ctx, `
		insert into keeps (user_id, post_id, source_letter_id)
		values ($1, $2, $3)
		on conflict (user_id, post_id) do update
			set source_letter_id = coalesce(keeps.source_letter_id, excluded.source_letter_id)
	`, userID, postID, letterID)
	if err != nil {
		return fmt.Errorf("keeping post: %w", err)
	}
	return nil
}

func (s *Store) Unkeep(ctx context.Context, userID, postID int64) error {
	_, err := s.pool.Exec(ctx, `
		delete from keeps
		where user_id = $1 and post_id = $2
	`, userID, postID)
	if err != nil {
		return fmt.Errorf("unkeeping post: %w", err)
	}
	return nil
}

func (s *Store) IsKept(ctx context.Context, userID, postID int64) (bool, error) {
	var kept bool
	if err := s.pool.QueryRow(ctx, `
		select exists(
			select 1
			from keeps
			where user_id = $1 and post_id = $2
		)
	`, userID, postID).Scan(&kept); err != nil {
		return false, fmt.Errorf("checking keep: %w", err)
	}
	return kept, nil
}

// KeptPostIDs reports which of the given posts a reader has kept, so a page
// showing many posts at once (a feed, an inbox) can check keep state with
// one query instead of one per post.
func (s *Store) KeptPostIDs(ctx context.Context, userID int64, postIDs []int64) (map[int64]bool, error) {
	kept := make(map[int64]bool, len(postIDs))
	if len(postIDs) == 0 {
		return kept, nil
	}
	rows, err := s.pool.Query(ctx, `
		select post_id
		from keeps
		where user_id = $1 and post_id = any($2)
	`, userID, postIDs)
	if err != nil {
		return nil, fmt.Errorf("checking kept posts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var postID int64
		if err := rows.Scan(&postID); err != nil {
			return nil, fmt.Errorf("scanning kept post id: %w", err)
		}
		kept[postID] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating kept post ids: %w", err)
	}
	return kept, nil
}

// KeptPosts lists a reader's shelf, most recently kept first. Posts that were
// later unpublished or deleted fall off the shelf via the status filter and
// the foreign key cascade.
func (s *Store) KeptPosts(ctx context.Context, userID int64, limit int, cursor PageCursor) ([]KeptPost, error) {
	before, lastID := cursorArgs(cursor)
	rows, err := s.pool.Query(ctx, `
		select p.id, p.user_id, u.username, u.author_name, u.display_name, p.title, p.slug, p.doc, p.html, p.status, p.type, p.page_position,
		       p.word_count, p.published_at, p.created_at, p.updated_at, u.blog_lang, k.created_at, k.source_letter_id,
		       coalesce(l.body, ''), coalesce(lu.author_name, ''), coalesce(lu.display_name, ''), coalesce(lu.username, '')
		from keeps k
		join posts p on p.id = k.post_id
		join users u on u.id = p.user_id
		left join letters l on l.id = k.source_letter_id and l.to_user = k.user_id
		left join users lu on lu.id = l.from_user
		where k.user_id = $1
		  and p.status = 'published'
		  and p.type = 'post'
		  and (
		    $3::timestamptz is null
		    or k.created_at < $3::timestamptz
		    or ($4::bigint is not null and k.created_at = $3::timestamptz and p.id < $4::bigint)
		  )
		order by k.created_at desc, p.id desc
		limit $2
	`, userID, limit, before, lastID)
	if err != nil {
		return nil, fmt.Errorf("listing kept posts: %w", err)
	}
	defer rows.Close()

	var kept []KeptPost
	for rows.Next() {
		var kp KeptPost
		fields := append(postWithUserScanFields(&kp.Post),
			&kp.KeptAt, &kp.SourceLetterID,
			&kp.SourceLetterBody, &kp.SourceLetterFromName, &kp.SourceLetterFromDisplay, &kp.SourceLetterFromUser)
		if err := rows.Scan(fields...); err != nil {
			return nil, fmt.Errorf("scanning kept post: %w", err)
		}
		kept = append(kept, kp)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating kept posts: %w", err)
	}
	return kept, nil
}
