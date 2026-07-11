package jobs

import (
	"testing"
	"time"
)

func TestWildcardBeginningOfDay(t *testing.T) {
	loc := time.FixedZone("test", 3*60*60)
	got := BeginningOfDay(time.Date(2026, 7, 3, 12, 30, 15, 0, loc))
	want := time.Date(2026, 7, 3, 0, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestDefaultWildcardImpressionFloor(t *testing.T) {
	if DefaultWildcardImpressionFloor != 100 {
		t.Fatalf("DefaultWildcardImpressionFloor = %d, want 100", DefaultWildcardImpressionFloor)
	}
}
