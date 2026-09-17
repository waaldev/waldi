package web

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
	"waldi/internal/storage"
	"waldi/internal/store"
)

type fakeUploads map[string]string

func (f fakeUploads) OpenUpload(_ context.Context, username, publicURL string) (io.ReadCloser, error) {
	body, ok := f[publicURL]
	if !ok || !strings.Contains(publicURL, "/"+username+"/") {
		return nil, storage.ErrNotFound
	}
	return io.NopCloser(strings.NewReader(body)), nil
}

func TestBlogExport(t *testing.T) {
	srv := testServer(t)
	published := time.Date(2026, time.June, 20, 22, 0, 0, 0, time.UTC)
	older := published.AddDate(-1, 0, 0)
	owner := store.User{Username: "amin", DisplayName: "Amin's blog", BlogLang: "fa", Bio: "Words"}
	posts := []store.Post{
		{
			Title:       "Old one",
			Slug:        "old",
			HTML:        `<p>old</p>`,
			Status:      "published",
			BlogLang:    "fa",
			PublishedAt: &older,
			CreatedAt:   older,
		},
		{
			Title:       "Tips & <script>alert(1)</script>",
			Slug:        "tips",
			HTML:        `<p>body</p><figure><img src="/static/uploads/amin/a.webp" alt=""></figure><img src="https://example.com/x.png" alt="">`,
			Status:      "published",
			BlogLang:    "fa",
			Doc:         json.RawMessage(`{"type":"doc"}`),
			PublishedAt: &published,
			CreatedAt:   published,
		},
		{
			Title:     "Unfinished",
			Slug:      "unfinished",
			HTML:      `<p>wip</p><img src="/static/uploads/amin/a.webp" alt="">`,
			Status:    "draft",
			BlogLang:  "fa",
			CreatedAt: published,
		},
	}
	pages := []store.Post{{Title: "About", Slug: "about", HTML: "<p>me</p>", Status: "published", BlogLang: "fa"}}

	exp := blogExport{
		renderer:   srv.renderer,
		appBaseURL: "https://waldi.blog",
		owner:      owner,
		posts:      posts,
		pages:      pages,
		images:     fakeUploads{"/static/uploads/amin/a.webp": "IMG"},
		logger:     srv.logger,
	}
	var buf bytes.Buffer
	if err := exp.write(context.Background(), &buf); err != nil {
		t.Fatalf("write: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	files := map[string]string{}
	for _, f := range zr.File {
		if _, dup := files[f.Name]; dup {
			t.Fatalf("duplicate entry %s", f.Name)
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		b, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		files[f.Name] = string(b)
	}
	get := func(name string) string {
		t.Helper()
		body, ok := files[name]
		if !ok {
			t.Fatalf("missing %s", name)
		}
		return body
	}

	var dump []exportPost
	if err := json.Unmarshal([]byte(get("posts.json")), &dump); err != nil {
		t.Fatalf("posts.json: %v", err)
	}
	if len(dump) != 3 {
		t.Fatalf("unexpected posts.json: %+v", dump)
	}

	css := get("blog/assets/style.css")
	if strings.Contains(css, `url("/`) || !strings.Contains(css, "data:font/woff2;base64,") {
		t.Fatalf("stylesheet not self-contained")
	}
	get("blog/assets/theme.js")
	get("blog/assets/favicon.png")
	if get("blog/images/a.webp") != "IMG" {
		t.Fatalf("image not exported")
	}

	index := get("blog/index.html")
	for _, want := range []string{
		`<html lang="fa" dir="rtl">`,
		`href="assets/style.css"`,
		`href="posts/tips.html"`,
		`href="drafts/unfinished.html"`,
		`href="pages/about.html"`,
		"Amin&#39;s blog",
	} {
		if !strings.Contains(index, want) {
			t.Fatalf("index missing %q:\n%s", want, index)
		}
	}
	if strings.Index(index, "posts/tips.html") > strings.Index(index, "posts/old.html") {
		t.Fatalf("expected newest post first")
	}

	post := get("blog/posts/tips.html")
	if strings.Contains(post, "<script>alert") || !strings.Contains(post, "Tips &amp; &lt;script&gt;") {
		t.Fatalf("title not escaped: %s", post)
	}
	for _, want := range []string{
		`href="../assets/style.css"`,
		`href="../index.html"`,
		`src="../images/a.webp"`,
		`src="https://example.com/x.png"`,
	} {
		if !strings.Contains(post, want) {
			t.Fatalf("post missing %q:\n%s", want, post)
		}
	}
	if strings.Contains(post, "/static/") {
		t.Fatalf("post still links to the live site: %s", post)
	}

	if !strings.Contains(get("blog/drafts/unfinished.html"), `src="../images/a.webp"`) {
		t.Fatalf("draft image not localized")
	}
	get("blog/pages/about.html")
	get("blog/posts/old.html")
}
