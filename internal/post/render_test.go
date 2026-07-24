package post

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderHTMLImageSrc(t *testing.T) {
	doc := Document{
		Type: "doc",
		Content: []Node{{
			Type:  "image",
			Attrs: Attrs{Src: "https://res.cloudinary.com/aminzamani/image/upload/q_auto/blog/photo.jpg", Alt: "caption"},
		}},
	}
	got, err := RenderHTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `src="https://res.cloudinary.com/aminzamani/image/upload/q_auto/blog/photo.jpg"`) {
		t.Fatalf("got %q", got)
	}
}

func TestRenderHTMLMediaImageSrc(t *testing.T) {
	doc := Document{
		Type: "doc",
		Content: []Node{{
			Type:  "image",
			Attrs: Attrs{Src: "/media/mahi/photo.jpg", Alt: "photo"},
		}},
	}
	got, err := RenderHTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `src="/media/mahi/photo.jpg"`) {
		t.Fatalf("got %q", got)
	}
}

func TestRenderHTML(t *testing.T) {
	doc := Document{
		Type: "doc",
		Content: []Node{
			{
				Type: "paragraph",
				Content: []Node{
					{Type: "text", Text: "سلام "},
					{Type: "text", Text: "والدی", Marks: []Mark{{Type: "bold"}}},
				},
			},
			{
				Type: "heading",
				Content: []Node{
					{Type: "text", Text: "تیتر دوم"},
				},
			},
		},
	}

	got, err := RenderHTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	want := "<p>سلام <strong>والدی</strong></p><h2>تیتر دوم</h2>"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRenderHTMLEscapesTextAndAttributes(t *testing.T) {
	doc := Document{
		Type: "doc",
		Content: []Node{
			{
				Type: "paragraph",
				Content: []Node{
					{
						Type: "text",
						Text: `<script>alert("x")</script>`,
						Marks: []Mark{
							{Type: "link", Attrs: Attrs{Href: `https://example.com?a="b"`}},
						},
					},
				},
			},
		},
	}

	got, err := RenderHTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "<script>") {
		t.Fatalf("rendered unsafe script tag: %s", got)
	}
	if !strings.Contains(got, "&#34;") {
		t.Fatalf("expected escaped quote in href: %s", got)
	}
}

func TestParseRejectsUnsupportedNodes(t *testing.T) {
	raw := json.RawMessage(`{"type":"doc","content":[{"type":"codeBlock","content":[{"type":"text","text":"no"}]}]}`)
	_, err := Parse(raw)
	if err == nil {
		t.Fatal("expected unsupported node error")
	}
}

func TestParseTipTapBlockquote(t *testing.T) {
	raw := json.RawMessage(`{"type":"doc","content":[{"type":"blockquote","content":[{"type":"paragraph","content":[{"type":"text","text":"quoted"}]}]}]}`)
	_, err := Parse(raw)
	if err != nil {
		t.Fatalf("expected blockquote with paragraph to parse, got: %v", err)
	}
}

func TestRenderHTMLBlockquoteWithParagraphs(t *testing.T) {
	raw := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"intro"}]},{"type":"heading","content":[{"type":"text","text":"Section"}]},{"type":"blockquote","content":[{"type":"paragraph","content":[{"type":"text","text":"quoted"}]}]},{"type":"horizontalRule"},{"type":"paragraph","content":[{"type":"text","text":"after"}]}]}`)
	doc, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	got, err := RenderHTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	want := "<p>intro</p><h2>Section</h2><blockquote><p>quoted</p></blockquote><hr class=\"divider\"><p>after</p>"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestParseTipTapAside(t *testing.T) {
	raw := json.RawMessage(`{"type":"doc","content":[{"type":"aside","content":[{"type":"paragraph","content":[{"type":"text","text":"note"}]}]}]}`)
	_, err := Parse(raw)
	if err != nil {
		t.Fatalf("expected aside with paragraph to parse, got: %v", err)
	}
}

func TestRenderHTMLAsideWithParagraphs(t *testing.T) {
	raw := json.RawMessage(`{"type":"doc","content":[{"type":"aside","content":[{"type":"paragraph","content":[{"type":"text","text":"note"},{"type":"text","text":"link","marks":[{"type":"link","attrs":{"href":"https://example.com"}}]}]}]}]}`)
	doc, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	got, err := RenderHTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	want := `<div class="aside"><p>note<a href="https://example.com" rel="nofollow">link</a></p></div>`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestParseRejectsEmptyAside(t *testing.T) {
	raw := json.RawMessage(`{"type":"doc","content":[{"type":"aside","content":[]}]}`)
	_, err := Parse(raw)
	if err == nil {
		t.Fatal("expected empty aside error")
	}
}

func TestRenderHTMLLists(t *testing.T) {
	raw := json.RawMessage(`{"type":"doc","content":[{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"one"}]}]},{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"two"}]},{"type":"orderedList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"nested"}]}]}]}]}]}]}`)
	doc, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	got, err := RenderHTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	want := "<ul><li><p>one</p></li><li><p>two</p><ol><li><p>nested</p></li></ol></li></ul>"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestParseRejectsInvalidListChildren(t *testing.T) {
	cases := []string{
		`{"type":"doc","content":[{"type":"bulletList","content":[]}]}`,
		`{"type":"doc","content":[{"type":"bulletList","content":[{"type":"paragraph","content":[{"type":"text","text":"no"}]}]}]}`,
		`{"type":"doc","content":[{"type":"orderedList","content":[{"type":"listItem","content":[{"type":"blockquote","content":[{"type":"paragraph"}]}]}]}]}`,
		`{"type":"doc","content":[{"type":"listItem","content":[{"type":"paragraph"}]}]}`,
	}
	for _, raw := range cases {
		if _, err := Parse(json.RawMessage(raw)); err == nil {
			t.Errorf("expected parse error for %s", raw)
		}
	}
}

func TestRenderHTMLFootnotes(t *testing.T) {
	raw := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello","marks":[{"type":"footnote","attrs":{"id":"fn1","text":"A quiet note."}}]}]}]}`)
	doc, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	got, err := RenderHTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `id="fnref-1"`) {
		t.Fatalf("missing reference link: %s", got)
	}
	if !strings.Contains(got, `class="footnotes"`) {
		t.Fatalf("missing footnotes list: %s", got)
	}
	if !strings.Contains(got, "A quiet note.") {
		t.Fatalf("missing footnote body: %s", got)
	}
	if strings.Contains(got, "<script>") {
		t.Fatalf("unsafe html: %s", got)
	}
}

func TestRenderHTMLDirectionMarkLegacy(t *testing.T) {
	raw := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello","marks":[{"type":"dir","attrs":{"dir":"ltr"}}]}]}]}`)
	doc, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	got, err := RenderHTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	want := `<p><span dir="ltr">Hello</span></p>`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestParseRejectsInvalidDirectionMark(t *testing.T) {
	raw := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"x","marks":[{"type":"dir","attrs":{"dir":"up"}}]}]}]}`)
	_, err := Parse(raw)
	if err == nil {
		t.Fatal("expected invalid dir error")
	}
}

func TestRenderHTMLBlockDirection(t *testing.T) {
	raw := json.RawMessage(`{"type":"doc","content":[{"type":"blockquote","attrs":{"dir":"ltr"},"content":[{"type":"paragraph","content":[{"type":"text","text":"quoted"}]}]},{"type":"paragraph","attrs":{"dir":"ltr"},"content":[{"type":"text","text":"para"}]},{"type":"heading","attrs":{"dir":"ltr"},"content":[{"type":"text","text":"head"}]}]}`)
	doc, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	got, err := RenderHTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	want := `<blockquote dir="ltr"><p>quoted</p></blockquote><p dir="ltr">para</p><h2 dir="ltr">head</h2>`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestParseRejectsInvalidBlockDirection(t *testing.T) {
	raw := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","attrs":{"dir":"up"},"content":[{"type":"text","text":"x"}]}]}`)
	_, err := Parse(raw)
	if err == nil {
		t.Fatal("expected invalid dir error")
	}
}

func TestParseRejectsInvalidFootnote(t *testing.T) {
	raw := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"x","marks":[{"type":"footnote","attrs":{"id":"bad","text":"nope"}}]}]}]}`)
	_, err := Parse(raw)
	if err == nil {
		t.Fatal("expected invalid footnote id error")
	}
}

func TestRenderHTMLEmbedYouTube(t *testing.T) {
	doc := Document{
		Type: "doc",
		Content: []Node{{
			Type:  "embed",
			Attrs: Attrs{Provider: "youtube", ID: "dQw4w9WgXcQ"},
		}},
	}
	got, err := RenderHTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `class="embed embed--youtube"`) {
		t.Fatalf("missing embed wrapper class: %s", got)
	}
	if !strings.Contains(got, `src="https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ"`) {
		t.Fatalf("missing youtube iframe src: %s", got)
	}
	if !strings.Contains(got, `allowfullscreen="true"`) {
		t.Fatalf("missing allowfullscreen: %s", got)
	}
}

func TestRenderHTMLEmbedSpotify(t *testing.T) {
	doc := Document{
		Type: "doc",
		Content: []Node{{
			Type:  "embed",
			Attrs: Attrs{Provider: "spotify", ID: "track:4uLU6hMCjMI75M1A2tKUQC"},
		}},
	}
	got, err := RenderHTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `src="https://open.spotify.com/embed/track/4uLU6hMCjMI75M1A2tKUQC"`) {
		t.Fatalf("missing spotify iframe src: %s", got)
	}
}

func TestRenderHTMLEmbedSoundCloud(t *testing.T) {
	doc := Document{
		Type: "doc",
		Content: []Node{{
			Type: "embed",
			Attrs: Attrs{
				Provider: "soundcloud",
				Src:      "https://soundcloud.com/artist/track",
			},
		}},
	}
	got, err := RenderHTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `src="https://w.soundcloud.com/player/?url=https%3A%2F%2Fsoundcloud.com%2Fartist%2Ftrack`) {
		t.Fatalf("missing soundcloud iframe src: %s", got)
	}
}

func TestParseRejectsInvalidEmbed(t *testing.T) {
	cases := []string{
		`{"type":"doc","content":[{"type":"embed","attrs":{"provider":"youtube","id":"not-a-valid-id!!"}}]}`,
		`{"type":"doc","content":[{"type":"embed","attrs":{"provider":"spotify","id":"track:tooshort"}}]}`,
		`{"type":"doc","content":[{"type":"embed","attrs":{"provider":"soundcloud","src":"https://evil.example/track"}}]}`,
		`{"type":"doc","content":[{"type":"embed","attrs":{"provider":"vimeo","id":"12345"}}]}`,
	}
	for _, raw := range cases {
		if _, err := Parse(json.RawMessage(raw)); err == nil {
			t.Fatalf("expected validation error for %s", raw)
		}
	}
}
