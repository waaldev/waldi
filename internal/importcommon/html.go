// Package importcommon holds HTML-to-document conversion shared by every
// blog importer (see internal/importblogir for an example). To add a new
// importer for another platform, create internal/import<platform>, parse
// that platform's export format into HTML, and call Converter.ConvertPost
// to turn it into a waldi post. See CONTRIBUTING.md for the full pattern.
package importcommon

import (
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"waldi/internal/post"

	"golang.org/x/net/html"
)

type Converter struct{}

type ConvertedPost struct {
	Doc   json.RawMessage
	HTML  string
	Words int
}

// ConvertPost turns raw HTML into a validated waldi document, sanitized HTML,
// and a word count.
func (c Converter) ConvertPost(rawHTML string) (ConvertedPost, error) {
	doc, err := c.HTMLToDocument(rawHTML)
	if err != nil {
		return ConvertedPost{}, err
	}
	doc = post.FixDocumentEmbeds(doc)
	doc = post.FixDocumentImageURLs(doc)
	rawDoc, err := json.Marshal(doc)
	if err != nil {
		return ConvertedPost{}, fmt.Errorf("encoding doc: %w", err)
	}
	htmlOut, err := post.RenderHTML(doc)
	if err != nil {
		return ConvertedPost{}, fmt.Errorf("rendering html: %w", err)
	}
	return ConvertedPost{
		Doc:   rawDoc,
		HTML:  htmlOut,
		Words: WordCount(doc),
	}, nil
}

func WordCount(doc post.Document) int {
	count := 0
	var walk func([]post.Node)
	walk = func(nodes []post.Node) {
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

func (c Converter) HTMLToDocument(raw string) (post.Document, error) {
	if strings.TrimSpace(raw) == "" {
		return post.Document{Type: "doc", Content: []post.Node{{Type: "paragraph"}}}, nil
	}

	root, err := html.Parse(strings.NewReader(raw))
	if err != nil {
		return post.Document{}, fmt.Errorf("parsing html: %w", err)
	}

	blocks := c.blocksFrom(root)
	if len(blocks) == 0 {
		blocks = []post.Node{{Type: "paragraph"}}
	}

	doc := post.Document{Type: "doc", Content: blocks}
	if err := post.Validate(doc); err != nil {
		return post.Document{}, fmt.Errorf("invalid converted document: %w", err)
	}
	return doc, nil
}

func (c Converter) blocksFrom(root *html.Node) []post.Node {
	var blocks []post.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			switch child.Type {
			case html.TextNode:
				text := strings.TrimSpace(child.Data)
				if text != "" {
					blocks = append(blocks, post.Node{
						Type:    "paragraph",
						Content: []post.Node{{Type: "text", Text: text}},
					})
				}
			case html.ElementNode:
				switch child.Data {
				case "html", "head", "body":
					walk(child)
				case "p":
					if imgs := findAllElements(child, "img"); len(imgs) > 0 {
						for _, img := range imgs {
							blocks = append(blocks, imageNode(img, ""))
						}
						if inline := c.inlineFrom(child, nil); len(inline) > 0 {
							blocks = append(blocks, post.Node{Type: "paragraph", Content: inline})
						}
					} else if inline := c.inlineFrom(child, nil); len(inline) > 0 {
						blocks = append(blocks, post.Node{Type: "paragraph", Content: inline})
					} else {
						blocks = append(blocks, post.Node{Type: "paragraph"})
					}
				case "h1", "h2", "h3", "h4", "h5", "h6":
					if inline := c.inlineFrom(child, nil); len(inline) > 0 {
						blocks = append(blocks, post.Node{Type: "heading", Content: inline})
					}
				case "blockquote":
					inner := c.blocksFrom(child)
					if len(inner) == 0 {
						if inline := c.inlineFrom(child, nil); len(inline) > 0 {
							inner = []post.Node{{Type: "paragraph", Content: inline}}
						}
					}
					if len(inner) > 0 {
						blocks = append(blocks, post.Node{Type: "blockquote", Content: inner})
					}
				case "hr":
					blocks = append(blocks, post.Node{Type: "horizontalRule"})
				case "figure":
					if hasClass(child, "kg-bookmark-card") {
						if link := bookmarkBlock(child); link != nil {
							blocks = append(blocks, *link)
						}
					} else if imgs := findAllElements(child, "img"); len(imgs) > 0 {
						caption := figcaptionText(child)
						for i, img := range imgs {
							if hasClass(img, "kg-bookmark-icon") {
								continue
							}
							cap := caption
							if i > 0 {
								cap = ""
							}
							blocks = append(blocks, imageNode(img, cap))
						}
					} else {
						walk(child)
					}
				case "img":
					blocks = append(blocks, imageNode(child, ""))
				case "ul", "ol":
					blocks = append(blocks, c.listBlocks(child, child.Data == "ol")...)
				case "iframe":
					if embed := c.embedFromIframe(child); embed != nil {
						blocks = append(blocks, *embed)
					}
				case "div", "section", "article", "main":
					walk(child)
				default:
					if inline := c.inlineFrom(child, nil); len(inline) > 0 {
						blocks = append(blocks, post.Node{Type: "paragraph", Content: inline})
					} else {
						walk(child)
					}
				}
			}
		}
	}
	walk(root)
	return blocks
}

func (c Converter) listBlocks(list *html.Node, ordered bool) []post.Node {
	var blocks []post.Node
	i := 1
	for li := list.FirstChild; li != nil; li = li.NextSibling {
		if li.Type != html.ElementNode || li.Data != "li" {
			continue
		}
		prefix := "• "
		if ordered {
			prefix = fmt.Sprintf("%d. ", i)
			i++
		}
		inline := c.inlineFrom(li, nil)
		inline = prependText(inline, prefix)
		if len(inline) > 0 {
			blocks = append(blocks, post.Node{Type: "paragraph", Content: inline})
		}
	}
	return blocks
}

func (c Converter) embedFromIframe(iframe *html.Node) *post.Node {
	src := strings.TrimSpace(attr(iframe, "src"))
	if src == "" {
		return nil
	}
	attrs, ok := post.ParseEmbedAttrs(src)
	if !ok {
		return nil
	}
	return &post.Node{Type: "embed", Attrs: attrs}
}

func (c Converter) inlineFrom(n *html.Node, marks []post.Mark) []post.Node {
	var out []post.Node
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		switch child.Type {
		case html.TextNode:
			text := child.Data
			if text == "" {
				continue
			}
			out = append(out, post.Node{Type: "text", Text: text, Marks: cloneMarks(marks)})
		case html.ElementNode:
			switch child.Data {
			case "br":
				out = append(out, post.Node{Type: "text", Text: " ", Marks: cloneMarks(marks)})
			case "strong", "b":
				out = append(out, c.inlineFrom(child, append(marks, post.Mark{Type: "bold"}))...)
			case "em", "i":
				out = append(out, c.inlineFrom(child, append(marks, post.Mark{Type: "italic"}))...)
			case "a":
				href := strings.TrimSpace(attr(child, "href"))
				nextMarks := marks
				if safeHref(href) {
					nextMarks = append(marks, post.Mark{Type: "link", Attrs: post.Attrs{Href: href}})
				}
				out = append(out, c.inlineFrom(child, nextMarks)...)
			case "sup", "sub", "span", "code", "small":
				out = append(out, c.inlineFrom(child, marks)...)
			default:
				out = append(out, c.inlineFrom(child, marks)...)
			}
		}
	}
	return mergeAdjacentText(out)
}

func imageNode(img *html.Node, caption string) post.Node {
	src := post.NormalizeImageURL(attr(img, "src"))
	alt := strings.TrimSpace(attr(img, "alt"))
	if alt == "" {
		alt = caption
	}
	return post.Node{
		Type: "image",
		Attrs: post.Attrs{
			Src: src,
			Alt: alt,
		},
	}
}

func figcaptionText(figure *html.Node) string {
	cap := findElement(figure, "figcaption")
	if cap == nil {
		return ""
	}
	return strings.TrimSpace(textContent(cap))
}

func textContent(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

func findElement(root *html.Node, tag string) *html.Node {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == tag {
			found = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return found
}

func findAllElements(root *html.Node, tag string) []*html.Node {
	var found []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == tag {
			found = append(found, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return found
}

func hasClass(n *html.Node, class string) bool {
	for _, a := range n.Attr {
		if a.Key != "class" {
			continue
		}
		if slices.Contains(strings.Fields(a.Val), class) {
			return true
		}
	}
	return false
}

func bookmarkBlock(figure *html.Node) *post.Node {
	link := findElement(figure, "a")
	if link == nil {
		return nil
	}
	href := strings.TrimSpace(attr(link, "href"))
	if !safeHref(href) {
		return nil
	}
	title := strings.TrimSpace(textContent(findElementByClass(figure, "kg-bookmark-title")))
	if title == "" {
		title = href
	}
	return &post.Node{
		Type: "paragraph",
		Content: []post.Node{{
			Type: "text",
			Text: title,
			Marks: []post.Mark{{
				Type:  "link",
				Attrs: post.Attrs{Href: href},
			}},
		}},
	}
}

func findElementByClass(root *html.Node, class string) *html.Node {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode && hasClass(n, class) {
			found = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return found
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func cloneMarks(marks []post.Mark) []post.Mark {
	if len(marks) == 0 {
		return nil
	}
	out := make([]post.Mark, len(marks))
	copy(out, marks)
	return out
}

func prependText(inline []post.Node, prefix string) []post.Node {
	if len(inline) == 0 {
		return []post.Node{{Type: "text", Text: strings.TrimSpace(prefix)}}
	}
	inline[0].Text = prefix + inline[0].Text
	return inline
}

func mergeAdjacentText(nodes []post.Node) []post.Node {
	if len(nodes) == 0 {
		return nil
	}
	out := make([]post.Node, 0, len(nodes))
	for _, node := range nodes {
		if node.Type != "text" {
			out = append(out, node)
			continue
		}
		if len(out) > 0 && out[len(out)-1].Type == "text" && marksEqual(out[len(out)-1].Marks, node.Marks) {
			out[len(out)-1].Text += node.Text
			continue
		}
		out = append(out, node)
	}
	return out
}

func marksEqual(a, b []post.Mark) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Type != b[i].Type || a[i].Attrs.Href != b[i].Attrs.Href {
			return false
		}
	}
	return true
}

func safeHref(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	if u.Scheme == "" {
		return strings.HasPrefix(u.Path, "/")
	}
	switch u.Scheme {
	case "http", "https", "mailto":
		return true
	default:
		return false
	}
}
