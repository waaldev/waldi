package post

import (
	"strings"
)

// NormalizeImageURL upgrades http image links to https so they load on TLS pages.
func NormalizeImageURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if rest, ok := strings.CutPrefix(raw, "http://"); ok {
		return "https://" + rest
	}
	return raw
}

// NeedsImageFix reports whether a document still has http image URLs.
func NeedsImageFix(doc Document) bool {
	var walk func([]Node) bool
	walk = func(nodes []Node) bool {
		for _, node := range nodes {
			if node.Type == "image" && strings.HasPrefix(strings.TrimSpace(node.Attrs.Src), "http://") {
				return true
			}
			if walk(node.Content) {
				return true
			}
		}
		return false
	}
	return walk(doc.Content)
}

// FixDocumentImageURLs normalizes image src values in a document tree.
func FixDocumentImageURLs(doc Document) Document {
	if doc.Type != "doc" {
		return doc
	}
	out := make([]Node, len(doc.Content))
	for i, block := range doc.Content {
		out[i] = fixBlockImageURLs(block)
	}
	return Document{Type: "doc", Content: out}
}

func fixBlockImageURLs(block Node) Node {
	switch block.Type {
	case "image":
		block.Attrs.Src = NormalizeImageURL(block.Attrs.Src)
	case "blockquote":
		for i, child := range block.Content {
			block.Content[i] = fixBlockImageURLs(child)
		}
	}
	return block
}
