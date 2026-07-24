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
		view.Excerpt = postExcerpt(p.HTML, 200)
		view.WriteMeta = writeDraftMeta(lang, p.UpdatedAt, p.WordCount, now)
		views = append(views, view)
	}
	return views
}

func writePublishedViews(posts []store.Post, engagement map[int64]store.PostEngagement, lang string) []PostView {
	views := make([]PostView, 0, len(posts))
	for _, p := range posts {
		view := postView(p)
		e := engagement[p.ID]
		view.WriteMeta = writePublishedMeta(lang, p.PublishedAt, e.Readers, e.Letters)
		views = append(views, view)
	}
	return views
}

func writeDraftMeta(lang string, updated time.Time, words int, now time.Time) string {
	rel := formatRelativePast(updated, now, lang)
	return i18n.T(lang, "write.draft.meta", rel, i18n.TN(lang, "write.words", words))
}

func writePublishedMeta(lang string, publishedAt *time.Time, readers, letters int) string {
	if publishedAt == nil {
		return ""
	}
	date := formatArticleDate(*publishedAt, lang)
	readersClause := i18n.TN(lang, "write.published.meta.readers", readers)
	if letters > 0 {
		lettersClause := i18n.TN(lang, "write.published.meta.letters", letters)
		return i18n.T(lang, "write.published.meta.full", date, readersClause, lettersClause)
	}
	return i18n.T(lang, "write.published.meta", date, readersClause)
}
