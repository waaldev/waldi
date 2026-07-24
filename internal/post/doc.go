package post

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

type Document struct {
	Type    string `json:"type"`
	Content []Node `json:"content"`
}

type Node struct {
	Type    string `json:"type"`
	Text    string `json:"text,omitempty"`
	Attrs   Attrs  `json:"attrs,omitempty"`
	Content []Node `json:"content,omitempty"`
	Marks   []Mark `json:"marks,omitempty"`
}

type Attrs struct {
	Href     string `json:"href,omitempty"`
	Src      string `json:"src,omitempty"`
	Alt      string `json:"alt,omitempty"`
	ID       string `json:"id,omitempty"`
	Text     string `json:"text,omitempty"`
	Provider string `json:"provider,omitempty"`
	Dir      string `json:"dir,omitempty"`
}

type Mark struct {
	Type  string `json:"type"`
	Attrs Attrs  `json:"attrs,omitempty"`
}

func Parse(raw json.RawMessage) (Document, error) {
	var doc Document
	if len(raw) == 0 {
		return doc, errors.New("empty document")
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return doc, fmt.Errorf("parsing document json: %w", err)
	}
	if err := Validate(doc); err != nil {
		return doc, err
	}
	return doc, nil
}

func Validate(doc Document) error {
	if doc.Type != "doc" {
		return fmt.Errorf("invalid document type %q", doc.Type)
	}
	if len(doc.Content) == 0 {
		return errors.New("document has no content")
	}
	for i, node := range doc.Content {
		if err := validateBlock(node); err != nil {
			return fmt.Errorf("content[%d]: %w", i, err)
		}
	}
	return nil
}

func validateBlock(node Node) error {
	switch node.Type {
	case "paragraph", "heading":
		if !validBlockDir(node.Attrs.Dir) {
			return fmt.Errorf("invalid dir %q on %s", node.Attrs.Dir, node.Type)
		}
		return validateInlineContainer(node)
	case "blockquote":
		if !validBlockDir(node.Attrs.Dir) {
			return errors.New("invalid dir on blockquote")
		}
		if len(node.Content) == 0 {
			return errors.New("blockquote cannot be empty")
		}
		for i, child := range node.Content {
			switch child.Type {
			case "paragraph", "heading":
				if !validBlockDir(child.Attrs.Dir) {
					return fmt.Errorf("child[%d]: invalid dir on %s", i, child.Type)
				}
				if err := validateInlineContainer(child); err != nil {
					return fmt.Errorf("child[%d]: %w", i, err)
				}
			default:
				return fmt.Errorf("unsupported blockquote child %q", child.Type)
			}
		}
	case "aside":
		if !validBlockDir(node.Attrs.Dir) {
			return errors.New("invalid dir on aside")
		}
		if len(node.Content) == 0 {
			return errors.New("aside cannot be empty")
		}
		for i, child := range node.Content {
			switch child.Type {
			case "paragraph", "heading":
				if !validBlockDir(child.Attrs.Dir) {
					return fmt.Errorf("child[%d]: invalid dir on %s", i, child.Type)
				}
				if err := validateInlineContainer(child); err != nil {
					return fmt.Errorf("child[%d]: %w", i, err)
				}
			default:
				return fmt.Errorf("unsupported aside child %q", child.Type)
			}
		}
	case "bulletList", "orderedList":
		if len(node.Content) == 0 {
			return fmt.Errorf("%s cannot be empty", node.Type)
		}
		for i, child := range node.Content {
			if child.Type != "listItem" {
				return fmt.Errorf("unsupported %s child %q", node.Type, child.Type)
			}
			if err := validateListItem(child); err != nil {
				return fmt.Errorf("item[%d]: %w", i, err)
			}
		}
	case "horizontalRule":
		if len(node.Content) > 0 || node.Text != "" {
			return errors.New("divider cannot contain content")
		}
	case "image":
		if strings.TrimSpace(node.Attrs.Src) == "" {
			return errors.New("image src is required")
		}
		if !safeURL(node.Attrs.Src) {
			return errors.New("image src uses unsupported scheme")
		}
	case "embed":
		return validateEmbed(node.Attrs)
	default:
		return fmt.Errorf("unsupported block node %q", node.Type)
	}
	return nil
}

func validateListItem(node Node) error {
	if len(node.Content) == 0 {
		return errors.New("list item cannot be empty")
	}
	for i, child := range node.Content {
		switch child.Type {
		case "paragraph":
			if !validBlockDir(child.Attrs.Dir) {
				return fmt.Errorf("child[%d]: invalid dir on paragraph", i)
			}
			if err := validateInlineContainer(child); err != nil {
				return fmt.Errorf("child[%d]: %w", i, err)
			}
		case "bulletList", "orderedList":
			if err := validateBlock(child); err != nil {
				return fmt.Errorf("child[%d]: %w", i, err)
			}
		default:
			return fmt.Errorf("unsupported list item child %q", child.Type)
		}
	}
	return nil
}

func validateInlineContainer(node Node) error {
	for i, child := range node.Content {
		if err := validateInline(child); err != nil {
			return fmt.Errorf("child[%d]: %w", i, err)
		}
	}
	return nil
}

func validateInline(node Node) error {
	if node.Type != "text" {
		return fmt.Errorf("unsupported inline node %q", node.Type)
	}
	for _, mark := range node.Marks {
		switch mark.Type {
		case "bold", "italic":
		case "dir":
			if mark.Attrs.Dir != "ltr" && mark.Attrs.Dir != "rtl" {
				return errors.New("dir mark requires ltr or rtl")
			}
		case "link":
			if strings.TrimSpace(mark.Attrs.Href) == "" {
				return errors.New("link href is required")
			}
			if !safeURL(mark.Attrs.Href) {
				return errors.New("link href uses unsupported scheme")
			}
		case "footnote":
			if !validFootnoteID(mark.Attrs.ID) {
				return errors.New("footnote id is invalid")
			}
			text := strings.TrimSpace(mark.Attrs.Text)
			if text == "" {
				return errors.New("footnote text is required")
			}
			if utf8.RuneCountInString(text) > 2000 {
				return errors.New("footnote text is too long")
			}
		default:
			return fmt.Errorf("unsupported mark %q", mark.Type)
		}
	}
	return nil
}

var (
	youtubeIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
	spotifyIDPattern = regexp.MustCompile(`^(track|album|playlist|episode|show):[A-Za-z0-9]{22}$`)
)

var soundcloudHosts = map[string]bool{
	"soundcloud.com":     true,
	"www.soundcloud.com": true,
	"m.soundcloud.com":   true,
	"on.soundcloud.com":  true,
	"api.soundcloud.com": true,
}

func validateEmbed(attrs Attrs) error {
	switch attrs.Provider {
	case "youtube":
		if !youtubeIDPattern.MatchString(strings.TrimSpace(attrs.ID)) {
			return errors.New("youtube embed id is invalid")
		}
	case "spotify":
		if !spotifyIDPattern.MatchString(strings.TrimSpace(attrs.ID)) {
			return errors.New("spotify embed id is invalid")
		}
	case "soundcloud":
		u, err := url.Parse(strings.TrimSpace(attrs.Src))
		if err != nil || u.Scheme != "https" || !soundcloudHosts[u.Host] {
			return errors.New("soundcloud embed src is invalid")
		}
	default:
		return fmt.Errorf("unsupported embed provider %q", attrs.Provider)
	}
	return nil
}

func safeURL(raw string) bool {
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

func validBlockDir(dir string) bool {
	return dir == "" || dir == "ltr" || dir == "rtl"
}

var footnoteIDPattern = regexp.MustCompile(`^fn[1-9][0-9]*$`)

func validFootnoteID(id string) bool {
	return footnoteIDPattern.MatchString(strings.TrimSpace(id))
}
