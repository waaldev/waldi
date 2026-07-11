package web

import (
	"encoding/json"
	"testing"
)

func TestSlugFromTitle(t *testing.T) {
	tests := map[string]string{
		"Hello, Waldi!":       "hello-waldi",
		"  چند خط درباره من ": "چند-خط-درباره-من",
		"---":                 "post",
	}
	for input, want := range tests {
		if got := slugFromTitle(input); got != want {
			t.Fatalf("slugFromTitle(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestRenderPostDoc(t *testing.T) {
	raw := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"سلام والدی"}]}]}`)
	html, words, err := renderPostDoc(raw)
	if err != nil {
		t.Fatal(err)
	}
	if html != "<p>سلام والدی</p>" {
		t.Fatalf("html = %q", html)
	}
	if words != 2 {
		t.Fatalf("words = %d, want 2", words)
	}
}

func TestRenderPostDocMultiSection(t *testing.T) {
	raw := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"one"}]},{"type":"heading","content":[{"type":"text","text":"two"}]},{"type":"blockquote","content":[{"type":"paragraph","content":[{"type":"text","text":"three"}]}]}]}`)
	_, words, err := renderPostDoc(raw)
	if err != nil {
		t.Fatalf("multi-section doc should save: %v", err)
	}
	if words != 3 {
		t.Fatalf("words = %d, want 3", words)
	}
}
