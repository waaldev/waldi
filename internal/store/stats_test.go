package store

import (
	"testing"
	"time"
)

func TestBeginningOfDay(t *testing.T) {
	loc := time.FixedZone("test", 3*60*60)
	got := beginningOfDay(time.Date(2026, 7, 3, 12, 30, 15, 0, loc))
	want := time.Date(2026, 7, 3, 0, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("got %s, want %s", got, want)
	}
}
