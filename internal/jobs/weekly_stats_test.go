package jobs

import (
	"strings"
	"testing"
	"time"
	"waldi/internal/store"
)

func TestWeeklyStatsMessage(t *testing.T) {
	since := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	until := since.AddDate(0, 0, 7)
	ttff := 90 * time.Minute

	msg := WeeklyStatsMessage(since, until, store.WeeklyStats{
		PostsPublished:          10,
		PostsReaching100In24h:   3,
		PostsWithFollowOrLetter: 4,
		PostsWithLetter:         2,
		AvgTimeToFirstFollower:  &ttff,
		CohortSize:              5,
		CohortRetained:          2,
		ActiveReaders:           40,
		RitualReaders:           10,
		WildcardsShown:          20,
		WildcardsSkipped:        5,
		WildcardsCompletable:    15,
		WildcardsCompleted:      9,
		WeeklyReaders:           123,
		ActiveWriters:           8,
	})

	for _, want := range []string{
		"Posts published: 10",
		"reaching 100 impressions in 24h: 30%",
		"earning ≥1 follow/letter: 40%",
		"with a response (letter): 20%",
		"time-to-first-follower: 1.5h",
		"W4 writer retention: 40% (2/5)",
		"Reader ritual rate (≥3 days/week): 25% (10/40)",
		"Wildcard completion: 60% (9/15)",
		"Wildcard skip rate: 25% (5/20)",
		"Weekly readers: 123",
		"Active writers: 8",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing %q\ngot:\n%s", want, msg)
		}
	}
}

func TestWeeklyStatsMessageHandlesZeroDenominators(t *testing.T) {
	since := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	until := since.AddDate(0, 0, 7)

	msg := WeeklyStatsMessage(since, until, store.WeeklyStats{})

	if !strings.Contains(msg, "n/a") {
		t.Errorf("expected n/a placeholders for zero denominators, got:\n%s", msg)
	}
	if strings.Contains(msg, "NaN") || strings.Contains(msg, "+Inf") {
		t.Errorf("message contains invalid float output:\n%s", msg)
	}
}
