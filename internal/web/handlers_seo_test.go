package web

import "testing"

func TestCDATAEscape(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "plain html",
			in:   "<p>hello</p>",
			want: "<![CDATA[<p>hello</p>]]>",
		},
		{
			name: "embedded cdata terminator",
			in:   "<p>a]]>b</p>",
			want: "<![CDATA[<p>a]]]]><![CDATA[>b</p>]]>",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cdataEscape(tt.in); got != tt.want {
				t.Fatalf("cdataEscape(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
