package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"waldi/internal/i18n"
	"waldi/internal/store"
)

const DefaultWildcardImpressionFloor = 100

type WildcardJob struct {
	Store           *store.Store
	Logger          *slog.Logger
	Now             func() time.Time
	Limit           int
	ImpressionFloor int
}

func (j WildcardJob) Run(ctx context.Context) error {
	if j.Store == nil {
		return fmt.Errorf("store is required")
	}
	logger := j.Logger
	if logger == nil {
		logger = slog.Default()
	}
	now := time.Now
	if j.Now != nil {
		now = j.Now
	}
	limit := j.Limit
	if limit == 0 {
		limit = 1000
	}
	floor := j.ImpressionFloor
	if floor == 0 {
		floor = DefaultWildcardImpressionFloor
	}

	day := BeginningOfDay(now())
	users, err := j.Store.UsersNeedingWildcard(ctx, day, limit)
	if err != nil {
		return err
	}

	for _, user := range users {
		readerLang := i18n.ReaderLang(user.Locale)
		post, fromPool, err := j.pickCandidate(ctx, user.ID, readerLang, day, floor)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				continue
			}
			return err
		}
		if err := j.Store.AssignWildcard(ctx, user.ID, post.ID, day); err != nil {
			return err
		}
		logger.Info("wildcard assigned", "username", user.Username, "reader_lang", readerLang, "post_id", post.ID, "post_author", post.Username, "post_lang", post.BlogLang, "floor", floor, "from_pool", fromPool)
	}
	return nil
}

// pickCandidate prefers an admin-curated wildcard_pool post, falling back to
// the impression-based algorithm when the pool has nothing suitable.
func (j WildcardJob) pickCandidate(ctx context.Context, userID int64, readerLang string, day time.Time, floor int) (store.Post, bool, error) {
	post, err := j.Store.WildcardPoolCandidate(ctx, userID, readerLang, day)
	if err == nil {
		return post, true, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return store.Post{}, false, err
	}
	post, err = j.Store.WildcardCandidate(ctx, userID, readerLang, day, floor)
	return post, false, err
}

// WildcardSetJob hand-picks today's stranger post for one or all readers.
type WildcardSetJob struct {
	Store  *store.Store
	Logger *slog.Logger
	PostID int64
	Author string
	Slug   string
	User   string
	Limit  int
	Day    time.Time
}

func (j WildcardSetJob) Run(ctx context.Context) error {
	if j.Store == nil {
		return fmt.Errorf("store is required")
	}
	logger := j.Logger
	if logger == nil {
		logger = slog.Default()
	}
	limit := j.Limit
	if limit == 0 {
		limit = 1000
	}
	day := j.Day
	if day.IsZero() {
		day = BeginningOfDay(time.Now())
	}

	postID, err := j.resolvePostID(ctx)
	if err != nil {
		return err
	}
	post, err := j.Store.PublishedPostByID(ctx, postID)
	if err != nil {
		return fmt.Errorf("loading post %d: %w", postID, err)
	}

	users, err := j.targetUsers(ctx, limit)
	if err != nil {
		return err
	}
	if len(users) == 0 {
		return fmt.Errorf("no users matched")
	}

	assigned := 0
	for _, user := range users {
		if j.User == "" && i18n.ReaderLang(user.Locale) != i18n.ReaderLang(post.BlogLang) {
			continue
		}
		if err := j.Store.SetWildcard(ctx, user.ID, post.ID, day); err != nil {
			return err
		}
		assigned++
		logger.Info("wildcard set", "username", user.Username, "reader_lang", i18n.ReaderLang(user.Locale), "post_id", post.ID, "post_author", post.Username, "post_lang", post.BlogLang, "date", day.Format("2006-01-02"))
	}
	if assigned == 0 {
		return fmt.Errorf("no users matched post language %q", i18n.ReaderLang(post.BlogLang))
	}
	return nil
}

func (j WildcardSetJob) resolvePostID(ctx context.Context) (int64, error) {
	if j.PostID > 0 {
		return j.PostID, nil
	}
	if j.Author == "" || j.Slug == "" {
		return 0, fmt.Errorf("set --post-id or both --author and --slug")
	}
	post, err := j.Store.PublishedPostByUsernameAndSlug(ctx, j.Author, j.Slug)
	if err != nil {
		return 0, fmt.Errorf("finding post @%s/%s: %w", j.Author, j.Slug, err)
	}
	return post.ID, nil
}

func (j WildcardSetJob) targetUsers(ctx context.Context, limit int) ([]store.User, error) {
	if j.User != "" {
		user, err := j.Store.UserByUsername(ctx, j.User)
		if err != nil {
			return nil, fmt.Errorf("finding user %q: %w", j.User, err)
		}
		return []store.User{user}, nil
	}
	return j.Store.ListUsers(ctx, limit)
}

// EnsureUserWildcard assigns a stranger post on first visit if the daily slot is empty.
func EnsureUserWildcard(ctx context.Context, st *store.Store, userID int64, readerLang string, day time.Time, floor int) error {
	taken, err := st.WildcardSlotTaken(ctx, userID, day)
	if err != nil {
		return err
	}
	if taken {
		return nil
	}
	lang := i18n.ReaderLang(readerLang)
	post, err := st.WildcardPoolCandidate(ctx, userID, lang, day)
	if errors.Is(err, store.ErrNotFound) {
		post, err = st.WildcardCandidate(ctx, userID, lang, day, floor)
	}
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return st.AssignWildcard(ctx, userID, post.ID, day)
}

func BeginningOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
