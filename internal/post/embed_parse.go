package post

import (
	"net/url"
	"regexp"
	"strings"
)

var (
	youtubeURLPattern = regexp.MustCompile(`(?i)^(?:https?:)?//(?:www\.)?(?:youtube(?:-nocookie)?\.com/(?:watch\?v=|shorts/|embed/)|youtu\.be/)([A-Za-z0-9_-]{11})`)
	spotifyURLPattern = regexp.MustCompile(`(?i)^(?:https?:)?//open\.spotify\.com/(?:intl-[a-z]{2}/)?(?:embed/)?(track|album|playlist|episode|show)/([A-Za-z0-9]{22})`)
)

// ParseEmbedAttrs maps a pasted or imported media URL to embed node attrs.
func ParseEmbedAttrs(raw string) (Attrs, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Attrs{}, false
	}

	if strings.Contains(raw, "w.soundcloud.com/player") {
		if u, err := url.Parse(raw); err == nil {
			if track := strings.TrimSpace(u.Query().Get("url")); track != "" {
				return ParseEmbedAttrs(track)
			}
		}
	}

	if !strings.Contains(raw, "://") {
		raw = "https://" + strings.TrimPrefix(raw, "//")
	}

	if youtubeURLPattern.MatchString(raw) {
		m := youtubeURLPattern.FindStringSubmatch(raw)
		if len(m) == 2 {
			return Attrs{Provider: "youtube", ID: m[1]}, true
		}
	}

	if spotifyURLPattern.MatchString(raw) {
		m := spotifyURLPattern.FindStringSubmatch(raw)
		if len(m) == 3 {
			return Attrs{Provider: "spotify", ID: m[1] + ":" + m[2]}, true
		}
	}

	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" {
		return Attrs{}, false
	}
	if soundcloudHosts[u.Host] {
		u.Fragment = ""
		u.RawQuery = ""
		return Attrs{Provider: "soundcloud", Src: u.String()}, true
	}

	return Attrs{}, false
}

// NeedsEmbedFix reports whether a document still has import-style embed links.
func NeedsEmbedFix(doc Document) bool {
	for _, block := range doc.Content {
		if _, ok := paragraphAsEmbed(block); ok {
			return true
		}
		if block.Type == "blockquote" {
			for _, inner := range block.Content {
				if _, ok := paragraphAsEmbed(inner); ok {
					return true
				}
			}
		}
	}
	return false
}

// FixDocumentEmbeds converts legacy import paragraphs that only linked embed
// URLs into proper embed blocks.
func FixDocumentEmbeds(doc Document) Document {
	if doc.Type != "doc" {
		return doc
	}
	out := make([]Node, 0, len(doc.Content))
	for _, block := range doc.Content {
		if block.Type == "blockquote" {
			block.Content = fixBlockEmbeds(block.Content)
			out = append(out, block)
			continue
		}
		if embed, ok := paragraphAsEmbed(block); ok {
			out = append(out, embed)
			continue
		}
		out = append(out, block)
	}
	return Document{Type: "doc", Content: out}
}

func fixBlockEmbeds(blocks []Node) []Node {
	out := make([]Node, 0, len(blocks))
	for _, block := range blocks {
		if embed, ok := paragraphAsEmbed(block); ok {
			out = append(out, embed)
			continue
		}
		out = append(out, block)
	}
	return out
}

func paragraphAsEmbed(block Node) (Node, bool) {
	if block.Type != "paragraph" || len(block.Content) != 1 || block.Content[0].Type != "text" {
		return Node{}, false
	}
	textNode := block.Content[0]
	text := strings.TrimSpace(textNode.Text)
	if text == "" {
		return Node{}, false
	}
	if len(textNode.Marks) > 1 {
		return Node{}, false
	}
	for _, mark := range textNode.Marks {
		if mark.Type != "link" {
			return Node{}, false
		}
	}
	candidates := []string{text}
	if len(textNode.Marks) == 1 && strings.TrimSpace(textNode.Marks[0].Attrs.Href) != "" {
		candidates = append([]string{textNode.Marks[0].Attrs.Href}, candidates...)
	}
	for _, candidate := range candidates {
		if attrs, ok := ParseEmbedAttrs(candidate); ok {
			return Node{Type: "embed", Attrs: attrs}, true
		}
	}
	return Node{}, false
}
