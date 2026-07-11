package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
	"waldi/internal/post"
	"waldi/internal/storage"
	"waldi/internal/store"
)

func runPostsMigrateImages(args []string) error {
	cfg := loadConfig()
	var email string

	fs := flag.NewFlagSet("posts migrate-images", flag.ContinueOnError)
	fs.StringVar(&cfg.DatabaseURL, "database-url", cfg.DatabaseURL, "Postgres connection URL")
	fs.StringVar(&email, "email", "", "user email address")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if cfg.DatabaseURL == "" {
		return errors.New("WALDI_DATABASE_URL is required")
	}
	if email == "" {
		return errors.New("--email is required")
	}

	imageStore, _, err := newImageStore(cfg)
	if err != nil {
		return fmt.Errorf("opening image store: %w", err)
	}

	ctx := context.Background()
	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("opening store: %w", err)
	}
	defer st.Close()

	user, err := st.UserByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		return fmt.Errorf("finding user: %w", err)
	}

	posts, err := st.PostsByUser(ctx, user.ID, 10000, store.PageCursor{})
	if err != nil {
		return fmt.Errorf("listing posts: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	var updated, fetched, missing int
	for _, p := range posts {
		doc, err := post.Parse(p.Doc)
		if err != nil {
			return fmt.Errorf("post %d: %w", p.ID, err)
		}
		next := post.FixDocumentImageURLs(doc)
		next, got, miss := migrateDocumentImages(ctx, client, imageStore, user.Username, next)
		fetched += got
		missing += miss
		if !documentChanged(doc, next) {
			continue
		}
		rawDoc, err := json.Marshal(next)
		if err != nil {
			return fmt.Errorf("post %d: %w", p.ID, err)
		}
		html, err := post.RenderHTML(next)
		if err != nil {
			return fmt.Errorf("post %d: %w", p.ID, err)
		}
		if _, err := st.UpdatePost(ctx, p.ID, user.ID, p.Title, p.Slug, rawDoc, html, wordCount(next)); err != nil {
			return fmt.Errorf("post %d: %w", p.ID, err)
		}
		updated++
	}

	fmt.Printf("migrate-images: %d posts updated, %d images fetched, %d unavailable for %s\n", updated, fetched, missing, user.Email)
	return nil
}

func documentChanged(before, after post.Document) bool {
	a, err := json.Marshal(before)
	if err != nil {
		return true
	}
	b, err := json.Marshal(after)
	if err != nil {
		return true
	}
	return string(a) != string(b)
}

func migrateDocumentImages(ctx context.Context, client *http.Client, imageStore storage.ImageStore, username string, doc post.Document) (post.Document, int, int) {
	var fetched, missing int
	out := make([]post.Node, len(doc.Content))
	for i, block := range doc.Content {
		out[i], fetched, missing = migrateBlockImages(ctx, client, imageStore, username, block, fetched, missing)
	}
	return post.Document{Type: "doc", Content: out}, fetched, missing
}

func migrateBlockImages(ctx context.Context, client *http.Client, imageStore storage.ImageStore, username string, block post.Node, fetched, missing int) (post.Node, int, int) {
	switch block.Type {
	case "image":
		src := strings.TrimSpace(block.Attrs.Src)
		if src == "" || isWaldiHostedImage(src) {
			return block, fetched, missing
		}
		data, contentType, ok := fetchImage(client, src)
		if !ok {
			missing++
			return block, fetched, missing
		}
		name := uploadNameFromURL(src)
		publicURL, err := imageStore.Save(ctx, username, name, data, contentType)
		if err != nil {
			missing++
			return block, fetched, missing
		}
		block.Attrs.Src = publicURL
		return block, fetched + 1, missing
	case "blockquote":
		for i, child := range block.Content {
			block.Content[i], fetched, missing = migrateBlockImages(ctx, client, imageStore, username, child, fetched, missing)
		}
	}
	return block, fetched, missing
}

func isWaldiHostedImage(src string) bool {
	src = strings.TrimSpace(src)
	if strings.HasPrefix(src, "/static/uploads/") || strings.HasPrefix(src, "/media/") {
		return true
	}
	u, err := url.Parse(src)
	if err != nil {
		return false
	}
	return strings.HasPrefix(u.Path, "/static/uploads/") || strings.HasPrefix(u.Path, "/media/")
}

func fetchImage(client *http.Client, rawURL string) ([]byte, string, bool) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", false
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, "", false
	}
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if !strings.HasPrefix(contentType, "image/") {
		return nil, "", false
	}
	const maxImage = 20 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxImage))
	if err != nil || len(data) == 0 {
		return nil, "", false
	}
	return data, contentType, true
}

func uploadNameFromURL(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return fmt.Sprintf("import-%d.jpg", time.Now().UnixNano())
	}
	base := path.Base(u.Path)
	if base == "" || base == "." || base == "/" {
		return fmt.Sprintf("import-%d.jpg", time.Now().UnixNano())
	}
	return base
}
