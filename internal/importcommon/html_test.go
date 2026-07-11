package importcommon

import (
	"testing"
	"waldi/internal/post"
)

func TestHTMLToDocumentParagraphsAndLinks(t *testing.T) {
	c := Converter{}
	doc, err := c.HTMLToDocument(`<p>سلام <a href="https://example.com">دنیا</a></p>`)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Content) != 1 || doc.Content[0].Type != "paragraph" {
		t.Fatalf("content = %#v", doc.Content)
	}
}

func TestHTMLToDocumentImageFigure(t *testing.T) {
	c := Converter{}
	doc, err := c.HTMLToDocument(`<figure class="kg-image-card"><img src="https://cdn.example/a.jpg" alt=""><figcaption>caption</figcaption></figure>`)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Content) != 1 || doc.Content[0].Type != "image" {
		t.Fatalf("content = %#v", doc.Content)
	}
	if doc.Content[0].Attrs.Alt != "caption" {
		t.Fatalf("alt = %q", doc.Content[0].Attrs.Alt)
	}
}

func TestHTMLToDocumentList(t *testing.T) {
	c := Converter{}
	doc, err := c.HTMLToDocument(`<ul><li>one</li><li>two</li></ul>`)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Content) != 2 {
		t.Fatalf("blocks = %d", len(doc.Content))
	}
	if doc.Content[0].Content[0].Text != "• one" {
		t.Fatalf("first item = %q", doc.Content[0].Content[0].Text)
	}
}

func TestHTMLToDocumentGallery(t *testing.T) {
	c := Converter{}
	doc, err := c.HTMLToDocument(`<figure class="kg-card kg-gallery-card"><div class="kg-gallery-container"><div class="kg-gallery-row"><div class="kg-gallery-image"><img src="https://cdn.example/a.jpg" alt=""></div><div class="kg-gallery-image"><img src="https://cdn.example/b.jpg" alt=""></div></div></div></figure>`)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Content) != 2 {
		t.Fatalf("blocks = %d, want 2 images", len(doc.Content))
	}
	if doc.Content[0].Type != "image" || doc.Content[1].Type != "image" {
		t.Fatalf("content = %#v", doc.Content)
	}
	if doc.Content[0].Attrs.Src != "https://cdn.example/a.jpg" || doc.Content[1].Attrs.Src != "https://cdn.example/b.jpg" {
		t.Fatalf("srcs = %q, %q", doc.Content[0].Attrs.Src, doc.Content[1].Attrs.Src)
	}
}

func TestHTMLToDocumentBookmarkCard(t *testing.T) {
	c := Converter{}
	doc, err := c.HTMLToDocument(`<figure class="kg-card kg-bookmark-card"><a class="kg-bookmark-container" href="https://example.com/post"><div class="kg-bookmark-content"><div class="kg-bookmark-title">Hello</div><div class="kg-bookmark-metadata"><img class="kg-bookmark-icon" src="https://cdn.example/icon.png" alt=""></div></div><div class="kg-bookmark-thumbnail"><img src="https://cdn.example/thumb.jpg" alt=""></div></a></figure>`)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Content) != 1 || doc.Content[0].Type != "paragraph" {
		t.Fatalf("content = %#v", doc.Content)
	}
	link := findLink(doc)
	if link == nil || link.Attrs.Href != "https://example.com/post" {
		t.Fatalf("link = %#v", link)
	}
}

func TestConvertPost(t *testing.T) {
	c := Converter{}
	converted, err := c.ConvertPost(`<p>hello <strong>world</strong></p>`)
	if err != nil {
		t.Fatal(err)
	}
	if converted.Words != 2 {
		t.Fatalf("words = %d", converted.Words)
	}
}

func findLink(doc post.Document) *post.Mark {
	for _, block := range doc.Content {
		for _, inline := range block.Content {
			for i := range inline.Marks {
				if inline.Marks[i].Type == "link" {
					return &inline.Marks[i]
				}
			}
		}
	}
	return nil
}
