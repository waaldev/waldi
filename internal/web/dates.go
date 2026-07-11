package web

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"waldi/internal/i18n"

	ptime "github.com/yaa110/go-persian-calendar"
)

var errBadDate = errors.New("invalid published_at date")

// parsePublishedAtInput parses the numeral YYYY/MM/DD date produced by
// formatDate back into a time.Time, converting from the Jalali calendar for
// fa posts (the reverse of formatArticleDate/formatYear's ptime.New calls).
func parsePublishedAtInput(raw, lang string) (time.Time, error) {
	parts := strings.Split(strings.TrimSpace(raw), "/")
	if len(parts) != 3 {
		return time.Time{}, errBadDate
	}
	y, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	d, err3 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil || err3 != nil || m < 1 || m > 12 || d < 1 || d > 31 {
		return time.Time{}, errBadDate
	}
	if lang == "fa" {
		return ptime.Date(y, ptime.Month(m), d, 0, 0, 0, 0, ptime.Iran()).Time(), nil
	}
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.Local), nil
}

func formatDate(t time.Time, lang string) string {
	if lang == "fa" {
		pc := ptime.New(t.In(ptime.Iran()))
		return fmt.Sprintf("%d/%s/%s",
			pc.Year(),
			twoDigits(int(pc.Month())),
			twoDigits(pc.Day()),
		)
	}
	year, month, day := t.Date()
	return strconv.Itoa(year) + "/" + twoDigits(int(month)) + "/" + twoDigits(day)
}

// formatArticleDate renders a single post's byline date, e.g. "9 Tir 1405"
// for fa (day, Persian month name, year) or "July 9, 2026" for en.
func formatArticleDate(t time.Time, lang string) string {
	if lang == "fa" {
		pc := ptime.New(t.In(ptime.Iran()))
		return fmt.Sprintf("%d %s %d", pc.Day(), pc.Month().String(), pc.Year())
	}
	return t.Format("January 2, 2006")
}

func formatFeedDate(t time.Time, lang string) string {
	if lang == "fa" {
		pc := ptime.New(t.In(ptime.Iran()))
		return fmt.Sprintf("%d %s", pc.Day(), pc.Month().String())
	}
	return t.Format("January 2")
}

func formatRelativePast(t, now time.Time, lang string) string {
	d := daysBetween(t.In(now.Location()), now)
	switch d {
	case 0:
		return i18n.T(lang, "home.daylabel")
	case 1:
		return i18n.T(lang, "write.relative.yesterday")
	default:
		return i18n.T(lang, "write.relative.days", d)
	}
}

func formatYear(t time.Time, lang string) string {
	if lang == "fa" {
		pc := ptime.New(t.In(ptime.Iran()))
		return strconv.Itoa(pc.Year())
	}
	return strconv.Itoa(t.Year())
}

// buildFeedDays buckets already-sorted (descending) feed posts into groups
// by calendar day, labeling each group "Today", "Yesterday", or a formatted
// date for anything older.
func buildFeedDays(posts []PostView, lang string) []FeedDay {
	if len(posts) == 0 {
		return nil
	}
	day := today()
	var days []FeedDay
	var currentKey string
	for _, p := range posts {
		t, err := time.Parse(time.RFC3339, p.PublishedAtISO)
		if err != nil {
			continue
		}
		local := t.In(day.Location())
		key := local.Format("2006-01-02")
		if len(days) == 0 || currentKey != key {
			days = append(days, FeedDay{Label: feedDayLabel(local, day, lang)})
			currentKey = key
		}
		days[len(days)-1].Posts = append(days[len(days)-1].Posts, p)
	}
	return days
}

func feedDayLabel(t, today time.Time, lang string) string {
	switch daysBetween(t, today) {
	case 0:
		return i18n.T(lang, "home.daylabel")
	case 1:
		return i18n.T(lang, "home.daylabel.yesterday")
	default:
		return formatFeedDate(t, lang)
	}
}

// daysBetween counts whole calendar days between a and b, ignoring
// time-of-day and any DST offset within either day.
func daysBetween(a, b time.Time) int {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	au := time.Date(ay, am, ad, 0, 0, 0, 0, time.UTC)
	bu := time.Date(by, bm, bd, 0, 0, 0, 0, time.UTC)
	return int(bu.Sub(au).Hours() / 24)
}

func postLang(pLang string) string {
	if i18n.Supported(pLang) {
		return pLang
	}
	return i18n.Default
}

func twoDigits(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}
