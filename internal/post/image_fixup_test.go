package post

import "testing"

func TestNormalizeImageURL(t *testing.T) {
	got := NormalizeImageURL("http://res.cloudinary.com/x.jpg")
	if got != "https://res.cloudinary.com/x.jpg" {
		t.Fatalf("got %q", got)
	}
}

func TestFixDocumentImageURLs(t *testing.T) {
	doc := Document{
		Type: "doc",
		Content: []Node{{
			Type: "image",
			Attrs: Attrs{
				Src: "http://example.com/a.jpg",
				Alt: "a",
			},
		}},
	}
	fixed := FixDocumentImageURLs(doc)
	if fixed.Content[0].Attrs.Src != "https://example.com/a.jpg" {
		t.Fatalf("src = %q", fixed.Content[0].Attrs.Src)
	}
}

func TestNeedsImageFix(t *testing.T) {
	doc := Document{Type: "doc", Content: []Node{{
		Type:  "image",
		Attrs: Attrs{Src: "http://example.com/a.jpg"},
	}}}
	if !NeedsImageFix(doc) {
		t.Fatal("expected true")
	}
}
