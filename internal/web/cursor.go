package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"waldi/internal/store"
)

const (
	writePageSize     = 50
	publishedPageSize = 10

	feedWindow    = 7 * 24 * time.Hour
	feedHardLimit = 200

	inboxWindow    = 7 * 24 * time.Hour
	inboxHardLimit = 200

	letterArchivePageSize = 10
)

var errBadPageCursor = fmt.Errorf("bad page cursor")

func parsePageCursor(r *http.Request) (store.PageCursor, error) {
	beforeRaw := strings.TrimSpace(r.URL.Query().Get("before"))
	if beforeRaw == "" {
		return store.PageCursor{}, nil
	}
	before, err := parseBeforeTime(beforeRaw)
	if err != nil {
		return store.PageCursor{}, errBadPageCursor
	}
	var lastID int64
	if lastRaw := strings.TrimSpace(r.URL.Query().Get("last")); lastRaw != "" {
		lastID, err = strconv.ParseInt(lastRaw, 10, 64)
		if err != nil || lastID < 1 {
			return store.PageCursor{}, errBadPageCursor
		}
	}
	return store.PageCursor{Before: before, LastID: lastID}, nil
}

func parseBeforeTime(raw string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("unrecognized before time")
}

func olderPageURL(path string, before time.Time, lastID int64) string {
	v := url.Values{}
	v.Set("before", before.UTC().Format(time.RFC3339))
	v.Set("last", strconv.FormatInt(lastID, 10))
	return path + "?" + v.Encode()
}

func trimPage[T any](items []T, pageSize int) ([]T, bool) {
	if len(items) <= pageSize {
		return items, false
	}
	return items[:pageSize], true
}

func publishedOlderURL(path string, posts []store.Post, hasMore bool) string {
	if !hasMore || len(posts) == 0 {
		return ""
	}
	last := posts[len(posts)-1]
	if last.PublishedAt == nil {
		return ""
	}
	return olderPageURL(path, *last.PublishedAt, last.ID)
}

func lettersOlderURL(path string, letters []store.Letter, hasMore bool) string {
	if !hasMore || len(letters) == 0 {
		return ""
	}
	last := letters[len(letters)-1]
	return olderPageURL(path, last.CreatedAt, last.ID)
}
