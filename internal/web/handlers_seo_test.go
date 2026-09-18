package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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

func TestAppSitemapListsPublicPagesWithoutFakeDates(t *testing.T) {
	s := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "https://waldi.blog/sitemap.xml", nil)
	rec := httptest.NewRecorder()
	s.handleAppSitemap(rec, req)

	body := rec.Body.String()
	for _, want := range []string{"<loc>https://waldi.blog/</loc>", "<loc>https://waldi.blog/how-it-works</loc>", "<loc>https://waldi.blog/explore</loc>", "<loc>https://waldi.blog/write/invite</loc>"} {
		if !strings.Contains(body, want) {
			t.Errorf("sitemap missing %q", want)
		}
	}
	if strings.Contains(body, "<lastmod>") {
		t.Errorf("sitemap has lastmod without any posts: %s", body)
	}
}
