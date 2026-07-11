package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"waldi/internal/store"
)

// Notifier is the subset of telegrambot.Bot (or a bare Client looped over
// admin ids) that WeeklyStatsJob needs to push its summary out.
type Notifier interface {
	Notify(ctx context.Context, text string)
}

type WeeklyStatsJob struct {
	Store    *store.Store
	Logger   *slog.Logger
	Notifier Notifier
	Now      func() time.Time
}

func (j WeeklyStatsJob) Run(ctx context.Context) error {
	if j.Store == nil {
		return fmt.Errorf("store is required")
	}
	if j.Notifier == nil {
		return fmt.Errorf("notifier is required")
	}
	logger := j.Logger
	if logger == nil {
		logger = slog.Default()
	}
	now := time.Now
	if j.Now != nil {
		now = j.Now
	}

	until := now()
	since := until.AddDate(0, 0, -7)
	cohortEnd := since.AddDate(0, 0, -21)
	cohortStart := since.AddDate(0, 0, -28)

	stats, err := j.Store.WeeklyStats(ctx, since, until, cohortStart, cohortEnd)
	if err != nil {
		return fmt.Errorf("loading weekly stats: %w", err)
	}

	j.Notifier.Notify(ctx, WeeklyStatsMessage(since, until, stats))
	logger.Info("weekly stats sent", "since", since, "until", until)
	return nil
}

// WeeklyStatsMessage renders stats as a plain-text Telegram message.
func WeeklyStatsMessage(since, until time.Time, s store.WeeklyStats) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("Weekly stats: %s – %s", since.Format("Jan 2"), until.Format("Jan 2")))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Posts published: %d", s.PostsPublished))
	lines = append(lines, fmt.Sprintf("  reaching 100 impressions in 24h: %s", pctString(s.PostsReaching100In24h, s.PostsPublished)))
	lines = append(lines, fmt.Sprintf("  earning ≥1 follow/letter: %s", pctString(s.PostsWithFollowOrLetter, s.PostsPublished)))
	lines = append(lines, fmt.Sprintf("  with a response (letter): %s", pctString(s.PostsWithLetter, s.PostsPublished)))
	lines = append(lines, fmt.Sprintf("  time-to-first-follower: %s", durationString(s.AvgTimeToFirstFollower)))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("W4 writer retention: %s (%d/%d)", pctString(s.CohortRetained, s.CohortSize), s.CohortRetained, s.CohortSize))
	lines = append(lines, fmt.Sprintf("Reader ritual rate (≥3 days/week): %s (%d/%d)", pctString(s.RitualReaders, s.ActiveReaders), s.RitualReaders, s.ActiveReaders))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Wildcard completion: %s (%d/%d)", pctString(s.WildcardsCompleted, s.WildcardsCompletable), s.WildcardsCompleted, s.WildcardsCompletable))
	lines = append(lines, fmt.Sprintf("Wildcard skip rate: %s (%d/%d)", pctString(s.WildcardsSkipped, s.WildcardsShown), s.WildcardsSkipped, s.WildcardsShown))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Weekly readers: %d", s.WeeklyReaders))
	lines = append(lines, fmt.Sprintf("Active writers: %d", s.ActiveWriters))
	return strings.Join(lines, "\n")
}

func pctString(num, den int) string {
	if den == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.0f%%", float64(num)/float64(den)*100)
}

func durationString(d *time.Duration) string {
	if d == nil {
		return "n/a"
	}
	if *d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if *d < 24*time.Hour {
		return fmt.Sprintf("%.1fh", d.Hours())
	}
	return fmt.Sprintf("%.1fd", d.Hours()/24)
}
