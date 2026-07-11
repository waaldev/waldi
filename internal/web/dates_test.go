package web

import (
	"testing"
	"time"
)

func TestFormatFeedDateJalali(t *testing.T) {
	tm := time.Date(2026, time.June, 29, 12, 0, 0, 0, time.UTC)
	got := formatFeedDate(tm, "fa")
	if got == "" || got == "June 29" {
		t.Fatalf("expected jalali feed date, got %q", got)
	}
}

func TestFormatYearJalali(t *testing.T) {
	tm := time.Date(2026, time.June, 29, 12, 0, 0, 0, time.UTC)
	got := formatYear(tm, "fa")
	if got == "2026" {
		t.Fatalf("expected jalali year, got %q", got)
	}
}

func TestFormatDateEnglish(t *testing.T) {
	tm := time.Date(2026, time.June, 29, 12, 0, 0, 0, time.UTC)
	got := formatDate(tm, "en")
	if got != "2026/06/29" {
		t.Fatalf("got %q", got)
	}
}

func TestDaysBetween(t *testing.T) {
	today := time.Date(2026, time.July, 4, 9, 0, 0, 0, time.UTC)
	cases := []struct {
		t    time.Time
		want int
	}{
		{time.Date(2026, time.July, 4, 23, 0, 0, 0, time.UTC), 0},
		{time.Date(2026, time.July, 3, 1, 0, 0, 0, time.UTC), 1},
		{time.Date(2026, time.June, 27, 0, 0, 0, 0, time.UTC), 7},
	}
	for _, c := range cases {
		if got := daysBetween(c.t, today); got != c.want {
			t.Fatalf("daysBetween(%v, %v) = %d, want %d", c.t, today, got, c.want)
		}
	}
}

func TestFeedDayLabel(t *testing.T) {
	today := time.Date(2026, time.July, 4, 9, 0, 0, 0, time.UTC)

	if got := feedDayLabel(time.Date(2026, time.July, 4, 23, 0, 0, 0, time.UTC), today, "en"); got != "Today" {
		t.Fatalf("today label = %q", got)
	}
	if got := feedDayLabel(time.Date(2026, time.July, 3, 1, 0, 0, 0, time.UTC), today, "en"); got != "Yesterday" {
		t.Fatalf("yesterday label = %q", got)
	}
	if got := feedDayLabel(time.Date(2026, time.June, 27, 0, 0, 0, 0, time.UTC), today, "en"); got != "June 27" {
		t.Fatalf("older label = %q", got)
	}
}

func TestFormatRelativePast(t *testing.T) {
	now := time.Date(2026, time.July, 4, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		t    time.Time
		lang string
		want string
	}{
		{time.Date(2026, time.July, 4, 8, 0, 0, 0, time.UTC), "en", "Today"},
		{time.Date(2026, time.July, 3, 8, 0, 0, 0, time.UTC), "en", "yesterday"},
		{time.Date(2026, time.June, 30, 8, 0, 0, 0, time.UTC), "en", "4 days ago"},
	}
	for _, c := range cases {
		if got := formatRelativePast(c.t, now, c.lang); got != c.want {
			t.Fatalf("formatRelativePast(%v, %v, %q) = %q, want %q", c.t, now, c.lang, got, c.want)
		}
	}
}

func TestParsePublishedAtInputRoundTrip(t *testing.T) {
	tm := time.Date(2026, time.July, 4, 0, 0, 0, 0, time.UTC)

	for _, lang := range []string{"en", "fa"} {
		formatted := formatDate(tm, lang)
		got, err := parsePublishedAtInput(formatted, lang)
		if err != nil {
			t.Fatalf("lang %q: parsePublishedAtInput(%q) error: %v", lang, formatted, err)
		}
		gy, gm, gd := got.Date()
		wantLoc := tm
		if lang == "fa" {
			wantLoc = tm.In(mustLoadIran(t))
		}
		wy, wm, wd := wantLoc.Date()
		if gy != wy || gm != wm || gd != wd {
			t.Fatalf("lang %q: round trip mismatch: got %d-%d-%d, want %d-%d-%d", lang, gy, gm, gd, wy, wm, wd)
		}
	}
}

func mustLoadIran(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Tehran")
	if err != nil {
		t.Skipf("Asia/Tehran tzdata unavailable: %v", err)
	}
	return loc
}

func TestParsePublishedAtInputRejectsBadInput(t *testing.T) {
	cases := []string{"", "2026-07-04", "2026/13/01", "2026/07/40", "abcd/ef/gh"}
	for _, c := range cases {
		if _, err := parsePublishedAtInput(c, "en"); err == nil {
			t.Fatalf("expected error for input %q", c)
		}
	}
}

func TestBuildFeedDaysGroupsByCalendarDay(t *testing.T) {
	iso := func(y int, m time.Month, d, h int) string {
		return time.Date(y, m, d, h, 0, 0, 0, time.UTC).Format(time.RFC3339)
	}
	posts := []PostView{
		{Title: "a", PublishedAtISO: iso(2026, time.June, 20, 10)},
		{Title: "b", PublishedAtISO: iso(2026, time.June, 20, 8)},
		{Title: "c", PublishedAtISO: iso(2026, time.June, 19, 20)},
	}

	days := buildFeedDays(posts, "en")
	if len(days) != 2 {
		t.Fatalf("expected 2 day groups, got %d", len(days))
	}
	if len(days[0].Posts) != 2 || len(days[1].Posts) != 1 {
		t.Fatalf("unexpected group sizes: %d, %d", len(days[0].Posts), len(days[1].Posts))
	}
	if days[0].Label == "" || days[1].Label == "" {
		t.Fatalf("expected non-empty day labels")
	}
}
