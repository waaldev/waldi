package web

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPostOpening(t *testing.T) {
	para := func(text string) string {
		return `{"type":"paragraph","content":[{"type":"text","text":"` + text + `"}]}`
	}
	doc := `{"type":"doc","content":[` + para("one") + `,` + para("two") + `,` + para("three") + `,` + para("four") + `]}`
	html, more := postOpening(json.RawMessage(doc), 3)
	if !more {
		t.Fatalf("expected more after three of four paragraphs")
	}
	if !strings.Contains(string(html), "three") || strings.Contains(string(html), "four") {
		t.Fatalf("opening = %q, want the first three paragraphs only", html)
	}

	short := `{"type":"doc","content":[` + para("only") + `]}`
	if _, more := postOpening(json.RawMessage(short), 3); more {
		t.Fatalf("expected no more for a one-paragraph post")
	}

	image := `{"type":"doc","content":[{"type":"image","attrs":{"src":"/a.jpg"}},` + para("caption") + `]}`
	if html, more := postOpening(json.RawMessage(image), 3); html != "" || !more {
		t.Fatalf("image-first post: got %q, %v; want empty opening and more", html, more)
	}
}
