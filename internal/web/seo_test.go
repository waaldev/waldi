package web

import (
	"net/http"
	"testing"
	"time"
	"waldi/internal/store"
)

func TestMetaDescription(t *testing.T) {
	short := metaDescription("Hello world")
	if short != "Hello world" {
		t.Fatalf("got %q", short)
	}

	long := metaDescription(string(make([]rune, 200)))
	if len([]rune(long)) != metaDescriptionMax {
		t.Fatalf("got rune count %d, want %d", len([]rune(long)), metaDescriptionMax)
	}
	if long[len(long)-3:] != "…" {
		t.Fatalf("expected ellipsis ending, got %q", long[len(long)-3:])
	}
}

func TestFirstImageSrc(t *testing.T) {
	html := `<p>text</p><figure><img src="/static/uploads/alice/abc.jpg" alt="hi"></figure>`
	if got := firstImageSrc(html); got != "/static/uploads/alice/abc.jpg" {
		t.Fatalf("got %q", got)
	}
	if got := firstImageSrc("<p>no image</p>"); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestPostDocumentTitle(t *testing.T) {
	if got := postDocumentTitle("Draft", "Ada's blog"); got != "Ada's blog - Draft" {
		t.Fatalf("got %q", got)
	}
}

func TestPostSEO(t *testing.T) {
	published := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	owner := storeUserFixture()
	post := storePostFixture(published)

	r := blogRequest("alice.waldi.test", "/hello-world?src=feed")
	seo := postSEO(r, "waldi.test", owner, post)

	if seo.CanonicalURL != "http://alice.waldi.test/hello-world" {
		t.Fatalf("canonical %q", seo.CanonicalURL)
	}
	if seo.OGType != "article" {
		t.Fatalf("og type %q", seo.OGType)
	}
	if seo.ArticlePublished != published.UTC().Format(time.RFC3339) {
		t.Fatalf("published %q", seo.ArticlePublished)
	}
	if seo.JSONLD == "" {
		t.Fatal("expected json-ld")
	}
	if seo.OGImage != "http://waldi.test/static/favicon.png" {
		t.Fatalf("og image %q", seo.OGImage)
	}
	if seo.TwitterCard != "summary_large_image" {
		t.Fatalf("twitter card %q", seo.TwitterCard)
	}
}

func TestBlogSEO(t *testing.T) {
	owner := storeUserFixture()
	owner.Bio = "Notes on writing."

	r := blogRequest("alice.waldi.test", "/")
	seo := blogSEO(r, "waldi.test", owner)

	if seo.Description != "Notes on writing." {
		t.Fatalf("description %q", seo.Description)
	}
	if seo.RSSURL != "http://alice.waldi.test/feed.xml" {
		t.Fatalf("rss %q", seo.RSSURL)
	}
	if seo.OGImage != "http://waldi.test/static/favicon.png" {
		t.Fatalf("og image %q", seo.OGImage)
	}
}

func blogRequest(host, path string) *http.Request {
	r, _ := http.NewRequest(http.MethodGet, "http://"+host+path, nil)
	r.Host = host
	return r
}

func storeUserFixture() store.User {
	return store.User{
		Username:    "alice",
		DisplayName: "Alice",
		AuthorName:  "Alice Writer",
		Bio:         "",
		BlogLang:    "en",
	}
}

func storePostFixture(published time.Time) store.Post {
	return store.Post{
		Username:    "alice",
		Title:       "Hello world",
		Slug:        "hello-world",
		HTML:        `<p>First post.</p>`,
		PublishedAt: &published,
		UpdatedAt:   published,
		BlogLang:    "en",
	}
}
