package web

import (
	"archive/zip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
	"waldi/internal/i18n"
	"waldi/internal/storage"
	"waldi/internal/store"
)

type exportPost struct {
	Title       string  `json:"title"`
	Slug        string  `json:"slug"`
	Status      string  `json:"status"`
	HTML        string  `json:"html"`
	Doc         any     `json:"doc"`
	PublishedAt *string `json:"published_at,omitempty"`
	CreatedAt   string  `json:"created_at"`
}

type blogExport struct {
	renderer   *Renderer
	appBaseURL string
	owner      store.User
	posts      []store.Post
	pages      []store.Post
	images     storage.UploadReader
	logger     *slog.Logger

	zw    *zip.Writer
	saved map[string]string
	now   time.Time
}

var (
	exportImgSrcRe  = regexp.MustCompile(`(<img\b[^>]*?\bsrc=")([^"]+)(")`)
	exportFontURLRe = regexp.MustCompile(`url\("/static/fonts/([^"/]+)"\)`)
)

func (e *blogExport) write(ctx context.Context, w io.Writer) error {
	e.zw = zip.NewWriter(w)
	e.saved = map[string]string{}
	e.now = time.Now()

	if err := e.writePostsJSON(); err != nil {
		return err
	}
	if err := e.writeAssets(); err != nil {
		return err
	}

	lang := blogPageLang(e.owner)
	var published, drafts []store.Post
	for _, p := range e.posts {
		if p.Status == "published" && p.PublishedAt != nil {
			published = append(published, p)
		} else {
			drafts = append(drafts, p)
		}
	}
	sort.SliceStable(published, func(i, j int) bool {
		return published[i].PublishedAt.After(*published[j].PublishedAt)
	})
	sort.SliceStable(drafts, func(i, j int) bool {
		return drafts[i].UpdatedAt.After(drafts[j].UpdatedAt)
	})

	for _, p := range published {
		if err := e.writePost(ctx, "posts/"+p.Slug+".html", p); err != nil {
			return err
		}
	}
	for _, p := range drafts {
		if err := e.writePost(ctx, "drafts/"+p.Slug+".html", p); err != nil {
			return err
		}
	}
	for _, p := range e.pages {
		if err := e.writePost(ctx, "pages/"+p.Slug+".html", p); err != nil {
			return err
		}
	}

	blog := blogViewFromUser(e.owner, "")
	blog.Posts = postViewsForBlog(published, lang)
	for i := range blog.Posts {
		blog.Posts[i].URL = "posts/" + blog.Posts[i].Slug + ".html"
	}
	blog.Archives = buildArchives(blog.Posts)
	blog.Empty = len(published) == 0
	for _, p := range e.pages {
		blog.Pages = append(blog.Pages, PageNavView{Title: p.Title, URL: "pages/" + p.Slug + ".html"})
	}
	draftViews := postViewsForBlog(drafts, lang)
	for i := range draftViews {
		draftViews[i].URL = "drafts/" + draftViews[i].Slug + ".html"
	}

	pd := e.pageData(i18n.T(lang, "profile.title", blog.DisplayName), "")
	pd.Blog = &blog
	pd.Export.Drafts = draftViews
	if err := e.render("index.html", "export-index.html", pd); err != nil {
		return err
	}

	if err := e.zw.Close(); err != nil {
		return fmt.Errorf("closing zip: %w", err)
	}
	return nil
}

func (e *blogExport) pageData(title, root string) PageData {
	lang := blogPageLang(e.owner)
	blog := blogViewFromUser(e.owner, "")
	return PageData{
		Title:      title,
		Lang:       lang,
		Dir:        i18n.Dir(lang),
		AppBaseURL: e.appBaseURL,
		Blog:       &blog,
		Export:     &ExportView{Root: root},
	}
}

func (e *blogExport) writePostsJSON() error {
	out := make([]exportPost, 0, len(e.posts))
	for _, p := range e.posts {
		var doc any
		if err := json.Unmarshal(p.Doc, &doc); err != nil {
			doc = nil
		}
		var publishedAt *string
		if p.PublishedAt != nil {
			formatted := p.PublishedAt.UTC().Format("2006-01-02T15:04:05Z")
			publishedAt = &formatted
		}
		out = append(out, exportPost{
			Title:       p.Title,
			Slug:        p.Slug,
			Status:      p.Status,
			HTML:        p.HTML,
			Doc:         doc,
			PublishedAt: publishedAt,
			CreatedAt:   p.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}
	f, err := e.create("posts.json")
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return fmt.Errorf("writing posts.json: %w", err)
	}
	return nil
}

func (e *blogExport) writeAssets() error {
	css, err := exportStylesheet(e.renderer.files)
	if err != nil {
		return err
	}
	if err := e.writeFile("assets/style.css", []byte(css)); err != nil {
		return err
	}
	for name, src := range map[string]string{
		"assets/theme.js":    "web/static/js/theme.js",
		"assets/favicon.png": "web/static/favicon.png",
	} {
		b, err := fs.ReadFile(e.renderer.files, src)
		if err != nil {
			return fmt.Errorf("reading %s: %w", src, err)
		}
		if err := e.writeFile(name, b); err != nil {
			return err
		}
	}
	return nil
}

func exportStylesheet(files fs.FS) (string, error) {
	fonts, err := fs.ReadFile(files, "web/static/css/fonts.css")
	if err != nil {
		return "", fmt.Errorf("reading fonts.css: %w", err)
	}
	main, err := fs.ReadFile(files, "web/static/css/main.css")
	if err != nil {
		return "", fmt.Errorf("reading main.css: %w", err)
	}
	var readErr error
	inlined := exportFontURLRe.ReplaceAllStringFunc(string(fonts), func(m string) string {
		name := exportFontURLRe.FindStringSubmatch(m)[1]
		b, err := fs.ReadFile(files, "web/static/fonts/"+name)
		if err != nil {
			readErr = fmt.Errorf("reading font %s: %w", name, err)
			return m
		}
		mime := "font/woff2"
		if strings.HasSuffix(name, ".woff") {
			mime = "font/woff"
		}
		return `url("data:` + mime + `;base64,` + base64.StdEncoding.EncodeToString(b) + `")`
	})
	if readErr != nil {
		return "", readErr
	}
	return inlined + "\n" + string(main), nil
}

func (e *blogExport) writePost(ctx context.Context, name string, p store.Post) error {
	body, err := e.localizeImages(ctx, p.HTML, "../")
	if err != nil {
		return err
	}
	view := postView(p)
	view.HTML = template.HTML(body)
	view.WriterLabel = writerLabelFromUser(e.owner)
	pd := e.pageData(p.Title, "../")
	pd.Post = &view
	return e.render(name, "export-post.html", pd)
}

func (e *blogExport) render(name, tmpl string, pd PageData) error {
	f, err := e.createSiteFile(name)
	if err != nil {
		return err
	}
	if err := e.renderer.Execute(f, tmpl, pd); err != nil {
		return fmt.Errorf("writing %s: %w", name, err)
	}
	return nil
}

func (e *blogExport) createSiteFile(name string) (io.Writer, error) {
	return e.create("blog/" + name)
}

func (e *blogExport) create(name string) (io.Writer, error) {
	f, err := e.zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: e.now})
	if err != nil {
		return nil, fmt.Errorf("creating %s: %w", name, err)
	}
	return f, nil
}

func (e *blogExport) writeFile(name string, b []byte) error {
	f, err := e.createSiteFile(name)
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		return fmt.Errorf("writing %s: %w", name, err)
	}
	return nil
}

func (e *blogExport) localizeImages(ctx context.Context, body, root string) (string, error) {
	var writeErr error
	out := exportImgSrcRe.ReplaceAllStringFunc(body, func(m string) string {
		sub := exportImgSrcRe.FindStringSubmatch(m)
		src := html.UnescapeString(sub[2])
		local, ok := e.saved[src]
		if !ok {
			var err error
			local, err = e.saveImage(ctx, src)
			if err != nil {
				writeErr = err
				return m
			}
			e.saved[src] = local
		}
		if local == "" {
			return m
		}
		return sub[1] + root + local + sub[3]
	})
	return out, writeErr
}

func (e *blogExport) saveImage(ctx context.Context, src string) (string, error) {
	if e.images == nil {
		return "", nil
	}
	rc, err := e.images.OpenUpload(ctx, e.owner.Username, src)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			e.logger.Error("reading image for export", "src", src, "err", err)
		}
		return "", nil
	}
	defer func() { _ = rc.Close() }()
	name := "images/" + path.Base(src)
	f, err := e.createSiteFile(name)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, rc); err != nil {
		return "", fmt.Errorf("writing %s: %w", name, err)
	}
	return name, nil
}
