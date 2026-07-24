package post

import (
	"bytes"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

// sanitizePolicy is a defense-in-depth pass behind the strict Tiptap
// node-type validation in doc.go: it only allows the exact HTML shape
// RenderHTML itself produces.
var sanitizePolicy = newSanitizePolicy()

// embedSrcPattern locks the iframe src we ever emit down to the exact
// origins/paths renderEmbed builds — nothing user-supplied reaches this
// attribute unescaped except the soundcloud track URL, which is itself
// validated (scheme + host) in doc.go before it's percent-encoded here.
var embedSrcPattern = regexp.MustCompile(
	`^https://(?:www\.youtube-nocookie\.com/embed/[A-Za-z0-9_-]{11}` +
		`|open\.spotify\.com/embed/(?:track|album|playlist|episode|show)/[A-Za-z0-9]{22}` +
		`|w\.soundcloud\.com/player/\?url=)`,
)

func newSanitizePolicy() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowURLSchemes("http", "https", "mailto")
	p.AllowRelativeURLs(true)
	p.AllowElements("p", "h2", "blockquote", "hr", "strong", "em", "figure", "sup", "ul", "ol", "li", "a", "img", "div", "iframe", "span")
	p.AllowAttrs("class").Matching(regexp.MustCompile(`^(divider|fn-ref|fn-back|footnotes)$`)).OnElements("hr", "sup", "ol", "a")
	p.AllowAttrs("dir").Matching(regexp.MustCompile(`^(ltr|rtl)$`)).OnElements("span", "p", "h2", "blockquote", "div")
	p.AllowAttrs("class").Matching(regexp.MustCompile(`^(aside|embed embed--(?:youtube|spotify|soundcloud))$`)).OnElements("div")
	p.AllowAttrs("id").Matching(regexp.MustCompile(`^fn(ref)?-[0-9]+$`)).OnElements("a", "li")
	p.AllowAttrs("href").Matching(regexp.MustCompile(`(?i)^(?:https?://\S+|mailto:\S+|#fn(?:ref)?-[0-9]+)$`)).OnElements("a")
	p.AllowAttrs("src").Matching(regexp.MustCompile(`(?i)^(?:https?://|/media/|/static/uploads/)`)).OnElements("img")
	p.AllowAttrs("alt").OnElements("img")
	p.AllowAttrs("src").Matching(embedSrcPattern).OnElements("iframe")
	p.AllowAttrs("loading").Matching(regexp.MustCompile(`^lazy$`)).OnElements("iframe")
	p.AllowAttrs("allowfullscreen").Matching(regexp.MustCompile(`^true$`)).OnElements("iframe")
	p.AllowAttrs("allow").Matching(regexp.MustCompile(`^[a-z-]+(?:; [a-z-]+)*$`)).OnElements("iframe")
	p.AllowAttrs("title").OnElements("iframe")
	p.RequireNoFollowOnLinks(true)
	return p
}

type footnoteIndex struct {
	order []string
	byID  map[string]footnoteEntry
}

type footnoteEntry struct {
	num  int
	text string
}

func collectFootnotes(doc Document) footnoteIndex {
	idx := footnoteIndex{byID: make(map[string]footnoteEntry)}
	var walk func(nodes []Node)
	walk = func(nodes []Node) {
		for _, node := range nodes {
			if node.Type == "text" {
				for _, mark := range node.Marks {
					if mark.Type == "footnote" {
						idx.add(mark.Attrs.ID, mark.Attrs.Text)
					}
				}
			}
			walk(node.Content)
		}
	}
	walk(doc.Content)
	return idx
}

func (idx *footnoteIndex) add(id, text string) {
	id = strings.TrimSpace(id)
	text = strings.TrimSpace(text)
	if id == "" || text == "" {
		return
	}
	if _, ok := idx.byID[id]; ok {
		return
	}
	idx.order = append(idx.order, id)
	idx.byID[id] = footnoteEntry{
		num:  len(idx.order),
		text: text,
	}
}

func RenderHTML(doc Document) (string, error) {
	if err := Validate(doc); err != nil {
		return "", err
	}

	footnotes := collectFootnotes(doc)
	var buf bytes.Buffer
	for _, node := range doc.Content {
		renderBlock(&buf, node, &footnotes)
	}
	if len(footnotes.order) > 0 {
		renderFootnotesList(&buf, &footnotes)
	}
	return sanitizePolicy.Sanitize(buf.String()), nil
}

func renderBlock(buf *bytes.Buffer, node Node, footnotes *footnoteIndex) {
	switch node.Type {
	case "paragraph":
		buf.WriteString("<p" + dirAttr(node.Attrs.Dir) + ">")
		renderInline(buf, node.Content, footnotes)
		buf.WriteString("</p>")
	case "heading":
		buf.WriteString("<h2" + dirAttr(node.Attrs.Dir) + ">")
		renderInline(buf, node.Content, footnotes)
		buf.WriteString("</h2>")
	case "blockquote":
		buf.WriteString("<blockquote" + dirAttr(node.Attrs.Dir) + ">")
		for _, child := range node.Content {
			renderBlock(buf, child, footnotes)
		}
		buf.WriteString("</blockquote>")
	case "bulletList":
		buf.WriteString("<ul>")
		for _, child := range node.Content {
			renderBlock(buf, child, footnotes)
		}
		buf.WriteString("</ul>")
	case "orderedList":
		buf.WriteString("<ol>")
		for _, child := range node.Content {
			renderBlock(buf, child, footnotes)
		}
		buf.WriteString("</ol>")
	case "listItem":
		buf.WriteString("<li>")
		for _, child := range node.Content {
			renderBlock(buf, child, footnotes)
		}
		buf.WriteString("</li>")
	case "horizontalRule":
		buf.WriteString(`<hr class="divider">`)
	case "image":
		buf.WriteString(`<figure><img src="`)
		buf.WriteString(escapeAttr(node.Attrs.Src))
		buf.WriteString(`" alt="`)
		buf.WriteString(escapeAttr(node.Attrs.Alt))
		buf.WriteString(`"></figure>`)
	case "embed":
		renderEmbed(buf, node.Attrs)
	case "aside":
		buf.WriteString(`<div class="aside"` + dirAttr(node.Attrs.Dir) + `>`)
		for _, child := range node.Content {
			renderBlock(buf, child, footnotes)
		}
		buf.WriteString("</div>")
	}
}

// renderEmbed builds the iframe src itself from vetted, provider-specific
// values (doc.go's validateEmbed already checked the shape); user input is
// never interpolated directly except the soundcloud track URL, which is
// percent-encoded and restricted to soundcloud.com hosts before it ever
// reaches here.
func renderEmbed(buf *bytes.Buffer, attrs Attrs) {
	var src, allow string
	fullscreen := false

	switch attrs.Provider {
	case "youtube":
		src = "https://www.youtube-nocookie.com/embed/" + attrs.ID
		allow = "accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share"
		fullscreen = true
	case "spotify":
		kind, id, ok := strings.Cut(attrs.ID, ":")
		if !ok {
			return
		}
		src = "https://open.spotify.com/embed/" + kind + "/" + id
		allow = "autoplay; clipboard-write; encrypted-media; fullscreen; picture-in-picture"
	case "soundcloud":
		src = "https://w.soundcloud.com/player/?url=" + url.QueryEscape(attrs.Src) +
			"&color=%233d5a80&auto_play=false&show_user=true"
		allow = "autoplay"
	default:
		return
	}

	buf.WriteString(`<div class="embed embed--`)
	buf.WriteString(escapeAttr(attrs.Provider))
	buf.WriteString(`"><iframe src="`)
	buf.WriteString(escapeAttr(src))
	buf.WriteString(`" loading="lazy" allow="`)
	buf.WriteString(escapeAttr(allow))
	buf.WriteString(`" title="Embedded content"`)
	if fullscreen {
		buf.WriteString(` allowfullscreen="true"`)
	}
	buf.WriteString(`></iframe></div>`)
}

func renderInline(buf *bytes.Buffer, nodes []Node, footnotes *footnoteIndex) {
	for _, node := range nodes {
		text := html.EscapeString(node.Text)
		for _, mark := range node.Marks {
			switch mark.Type {
			case "bold":
				text = "<strong>" + text + "</strong>"
			case "italic":
				text = "<em>" + text + "</em>"
			case "dir":
				if mark.Attrs.Dir == "ltr" || mark.Attrs.Dir == "rtl" {
					text = `<span dir="` + mark.Attrs.Dir + `">` + text + "</span>"
				}
			case "link":
				text = `<a href="` + escapeAttr(mark.Attrs.Href) + `">` + text + "</a>"
			case "footnote":
				if entry, ok := footnotes.byID[mark.Attrs.ID]; ok {
					num := strconv.Itoa(entry.num)
					text += fmt.Sprintf(
						`<sup class="fn-ref"><a href="#fn-%s" id="fnref-%s">%s</a></sup>`,
						num, num, num,
					)
				}
			}
		}
		buf.WriteString(text)
	}
}

func renderFootnotesList(buf *bytes.Buffer, footnotes *footnoteIndex) {
	buf.WriteString(`<ol class="footnotes">`)
	for _, id := range footnotes.order {
		entry := footnotes.byID[id]
		num := strconv.Itoa(entry.num)
		buf.WriteString(`<li id="fn-`)
		buf.WriteString(num)
		buf.WriteString(`">`)
		buf.WriteString(html.EscapeString(entry.text))
		buf.WriteString(` <a href="#fnref-`)
		buf.WriteString(num)
		buf.WriteString(`" class="fn-back">↩</a></li>`)
	}
	buf.WriteString(`</ol>`)
}

func escapeAttr(value string) string {
	return html.EscapeString(strings.TrimSpace(value))
}

func dirAttr(dir string) string {
	if dir != "ltr" && dir != "rtl" {
		return ""
	}
	return ` dir="` + dir + `"`
}
