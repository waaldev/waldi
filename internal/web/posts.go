package web

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode"

	postdoc "waldi/internal/post"
)

var slugDash = regexp.MustCompile(`-+`)

// reservedPageSlugs are the app's own top-level route segments (see
// routes() in server.go). The same mux serves every host, so a page whose
// slug matches one of these would be permanently shadowed by the literal
// route and never reachable through the /{slug} catch-all.
var reservedPageSlugs = map[string]bool{
	"static":          true,
	"media":           true,
	"internal":        true,
	"robots.txt":      true,
	"feed.xml":        true,
	"sitemap.xml":     true,
	"lang":            true,
	"signup":          true,
	"login":           true,
	"logout":          true,
	"verify-email":    true,
	"forgot-password": true,
	"reset-password":  true,
	"api":             true,
	"auth":            true,
	"you":             true,
	"settings":        true,
	"write":           true,
	"follow":          true,
	"unfollow":        true,
	"wildcard":        true,
	"inbox":           true,
	"letters":         true,
}

func slugFromTitle(title string) string {
	title = strings.TrimSpace(strings.ToLower(title))
	var b strings.Builder
	lastDash := false
	for _, r := range title {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastDash = false
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	slug := strings.Trim(slugDash.ReplaceAllString(b.String(), "-"), "-")
	if slug == "" {
		return "post"
	}
	if len([]rune(slug)) > 80 {
		slug = string([]rune(slug)[:80])
		slug = strings.Trim(slug, "-")
	}
	return slug
}

func defaultDocJSON() json.RawMessage {
	return json.RawMessage(`{"type":"doc","content":[{"type":"paragraph"}]}`)
}

func renderPostDoc(raw json.RawMessage) (string, int, error) {
	doc, err := postdoc.Parse(raw)
	if err != nil {
		return "", 0, err
	}
	html, err := postdoc.RenderHTML(doc)
	if err != nil {
		return "", 0, err
	}
	return html, wordCount(doc), nil
}

func wordCount(doc postdoc.Document) int {
	count := 0
	var walk func(nodes []postdoc.Node)
	walk = func(nodes []postdoc.Node) {
		for _, node := range nodes {
			if node.Text != "" {
				count += len(strings.Fields(node.Text))
			}
			walk(node.Content)
		}
	}
	walk(doc.Content)
	return count
}
