package web

import (
	"net/http/httptest"
	"testing"
	"time"
	"waldi/internal/store"
)

func TestParsePageCursor(t *testing.T) {
	r := httptest.NewRequest("GET", "/?before=2026-07-03T10:00:00Z&last=42", nil)
	got, err := parsePageCursor(r)
	if err != nil {
		t.Fatalf("parsePageCursor() err = %v", err)
	}
	if !got.Before.Equal(time.Date(2026, 7, 3, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("Before = %v", got.Before)
	}
	if got.LastID != 42 {
		t.Fatalf("LastID = %d, want 42", got.LastID)
	}
}

func TestParsePageCursorDateOnly(t *testing.T) {
	r := httptest.NewRequest("GET", "/?before=2026-01-15", nil)
	got, err := parsePageCursor(r)
	if err != nil {
		t.Fatalf("parsePageCursor() err = %v", err)
	}
	want := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	if !got.Before.Equal(want) {
		t.Fatalf("Before = %v, want %v", got.Before, want)
	}
}

func TestParsePageCursorEmpty(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	got, err := parsePageCursor(r)
	if err != nil {
		t.Fatalf("parsePageCursor() err = %v", err)
	}
	if got.Active() {
		t.Fatal("expected inactive cursor")
	}
}

func TestParsePageCursorBad(t *testing.T) {
	r := httptest.NewRequest("GET", "/?before=not-a-date", nil)
	if _, err := parsePageCursor(r); err == nil {
		t.Fatal("expected error for bad before")
	}
}

func TestOlderPageURL(t *testing.T) {
	got := olderPageURL("/inbox", time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC), 9)
	if got != "/inbox?before=2026-07-03T12%3A00%3A00Z&last=9" {
		t.Fatalf("got %q", got)
	}
}

func TestPublishedOlderURL(t *testing.T) {
	published := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	posts := []store.Post{{
		ID:          42,
		Status:      "published",
		PublishedAt: &published,
	}}

	got := publishedOlderURL("/write", posts, true)
	want := "/write?before=2026-03-01T12%3A00%3A00Z&last=42"
	if got != want {
		t.Fatalf("publishedOlderURL = %q, want %q", got, want)
	}
}
