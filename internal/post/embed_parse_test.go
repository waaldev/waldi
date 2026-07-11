package post

import "testing"

func TestParseEmbedAttrsSoundCloudPlayer(t *testing.T) {
	raw := "https://w.soundcloud.com/player/?url=https%3A//api.soundcloud.com/playlists/1024730941&color=%23ff5500"
	attrs, ok := ParseEmbedAttrs(raw)
	if !ok {
		t.Fatal("expected soundcloud embed")
	}
	if attrs.Provider != "soundcloud" {
		t.Fatalf("provider = %q", attrs.Provider)
	}
	if attrs.Src != "https://api.soundcloud.com/playlists/1024730941" {
		t.Fatalf("src = %q", attrs.Src)
	}
}

func TestParseEmbedAttrsSpotifyEmbedURL(t *testing.T) {
	raw := "https://open.spotify.com/embed/track/1BncfTJAWxrsxyT9culBrj?si=b9b45afe329a43fd"
	attrs, ok := ParseEmbedAttrs(raw)
	if !ok {
		t.Fatal("expected spotify embed")
	}
	if attrs.Provider != "spotify" || attrs.ID != "track:1BncfTJAWxrsxyT9culBrj" {
		t.Fatalf("attrs = %#v", attrs)
	}
}

func TestFixDocumentEmbedsFromParagraphLink(t *testing.T) {
	doc := Document{
		Type: "doc",
		Content: []Node{{
			Type: "paragraph",
			Content: []Node{{
				Type: "text",
				Text: "https://w.soundcloud.com/player/?url=https%3A//api.soundcloud.com/playlists/1024730941",
				Marks: []Mark{{
					Type:  "link",
					Attrs: Attrs{Href: "https://w.soundcloud.com/player/?url=https%3A//api.soundcloud.com/playlists/1024730941"},
				}},
			}},
		}},
	}
	fixed := FixDocumentEmbeds(doc)
	if len(fixed.Content) != 1 || fixed.Content[0].Type != "embed" {
		t.Fatalf("content = %#v", fixed.Content)
	}
	if fixed.Content[0].Attrs.Provider != "soundcloud" {
		t.Fatalf("provider = %q", fixed.Content[0].Attrs.Provider)
	}
}

func TestFixDocumentEmbedsLeavesNormalParagraph(t *testing.T) {
	doc := Document{
		Type: "doc",
		Content: []Node{{
			Type:    "paragraph",
			Content: []Node{{Type: "text", Text: "hello world"}},
		}},
	}
	fixed := FixDocumentEmbeds(doc)
	if fixed.Content[0].Type != "paragraph" {
		t.Fatalf("paragraph changed: %#v", fixed.Content[0])
	}
}
