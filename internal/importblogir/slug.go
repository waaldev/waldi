package importblogir

import (
	"regexp"
	"strings"
	"unicode"
)

var slugDash = regexp.MustCompile(`-+`)

// slugify mirrors waldi's own slugFromTitle (internal/web/posts.go), keeping
// unicode letters so Persian titles/URLs produce readable slugs instead of
// being flattened to "post".
func slugify(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	var b strings.Builder
	lastDash := false
	for _, r := range raw {
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
	if len([]rune(slug)) > 80 {
		slug = string([]rune(slug)[:80])
		slug = strings.Trim(slug, "-")
	}
	return slug
}
