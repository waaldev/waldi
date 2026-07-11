package web

import (
	"html"
	"regexp"
	"strings"
	"waldi/internal/store"
)

var htmlTagRe = regexp.MustCompile(`<[^>]*>`)

func postExcerpt(rawHTML string, maxRunes int) string {
	text := htmlTagRe.ReplaceAllString(rawHTML, " ")
	text = html.UnescapeString(text)
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if text == "" {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes]) + "…"
}

func buildArchives(posts []PostView) []ArchiveYear {
	var archives []ArchiveYear
	var current *ArchiveYear
	for _, p := range posts {
		year := p.PublishedYear
		if year == "" {
			year = "?"
		}
		if current == nil || current.Year != year {
			archives = append(archives, ArchiveYear{Year: year})
			current = &archives[len(archives)-1]
		}
		current.Posts = append(current.Posts, p)
	}
	return archives
}

func postViewsForBlog(posts []store.Post, blogLang string) []PostView {
	views := make([]PostView, 0, len(posts))
	for _, p := range posts {
		view := postView(p)
		if p.PublishedAt != nil {
			view.FeedDate = formatFeedDate(*p.PublishedAt, blogLang)
			view.PublishedYear = formatYear(*p.PublishedAt, blogLang)
		}
		view.Excerpt = postExcerpt(p.HTML, 200)
		views = append(views, view)
	}
	return views
}
