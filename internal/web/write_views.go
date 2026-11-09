package web

import (
	"time"
	"waldi/internal/i18n"
	"waldi/internal/store"
)

func writeDraftViews(posts []store.Post, lang string, now time.Time) []PostView {
	views := make([]PostView, 0, len(posts))
	for _, p := range posts {
		view := postView(p)
		view.WriteMeta = deskDate(p.UpdatedAt, now, lang)
		views = append(views, view)
	}
	return views
}

func writePublishedViews(posts []store.Post, lang string, now time.Time) []PostView {
	views := make([]PostView, 0, len(posts))
	for _, p := range posts {
		view := postView(p)
		if p.PublishedAt != nil {
			view.WriteMeta = deskDate(*p.PublishedAt, now, lang)
		}
		views = append(views, view)
	}
	return views
}

func deskDate(t, now time.Time, lang string) string {
	switch daysBetween(t.In(now.Location()), now) {
	case 0:
		return i18n.T(lang, "home.daylabel")
	case 1:
		return i18n.T(lang, "home.daylabel.yesterday")
	default:
		return formatFeedDate(t, lang)
	}
}
