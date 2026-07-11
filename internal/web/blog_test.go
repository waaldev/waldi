package web

import (
	"testing"
	"waldi/internal/store"
)

func TestBlogPageLang(t *testing.T) {
	tests := []struct {
		name  string
		owner store.User
		want  string
	}{
		{
			name:  "blog lang wins",
			owner: store.User{BlogLang: "en", Locale: "fa"},
			want:  "en",
		},
		{
			name:  "falls back to locale",
			owner: store.User{BlogLang: "", Locale: "en"},
			want:  "en",
		},
		{
			name:  "defaults to fa",
			owner: store.User{BlogLang: "", Locale: ""},
			want:  "fa",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := blogPageLang(tt.owner); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWriterLabel(t *testing.T) {
	tests := []struct {
		author, display, username, want string
	}{
		{"", "", "alice", "alice"},
		{"Alice Writer", "", "alice", "Alice Writer"},
		{"", "Alice's Blog", "alice", "Alice's Blog"},
		{"Alice Writer", "Alice's Blog", "alice", "Alice Writer"},
		{"alice", "Alice's Blog", "alice", "alice"},
		{"john", "", "john", "john"},
	}
	for _, tt := range tests {
		if got := writerLabel(tt.author, tt.display, tt.username); got != tt.want {
			t.Fatalf("writerLabel(%q, %q, %q) = %q, want %q", tt.author, tt.display, tt.username, got, tt.want)
		}
	}
}
