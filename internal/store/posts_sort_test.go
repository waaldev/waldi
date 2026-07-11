package store

import (
	"testing"
	"time"
)

func TestDashboardSortAt(t *testing.T) {
	created := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	published := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	updated := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	draft := Post{
		Status:    "draft",
		CreatedAt: created,
		UpdatedAt: updated,
	}
	if got := draft.DashboardSortAt(); !got.Equal(created) {
		t.Fatalf("draft sort = %v, want created %v", got, created)
	}

	pub := Post{
		Status:      "published",
		CreatedAt:   created,
		UpdatedAt:   updated,
		PublishedAt: &published,
	}
	if got := pub.DashboardSortAt(); !got.Equal(published) {
		t.Fatalf("published sort = %v, want published %v", got, published)
	}

	unpub := Post{
		Status:      "draft",
		CreatedAt:   created,
		UpdatedAt:   updated,
		PublishedAt: &published,
	}
	if got := unpub.DashboardSortAt(); !got.Equal(created) {
		t.Fatalf("unpublished draft sort = %v, want created %v", got, created)
	}
}
