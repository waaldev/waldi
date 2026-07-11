package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"strings"
	"waldi/internal/post"
	"waldi/internal/store"
)

func runPosts(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: waldi posts rehtml|fix-embeds|fix-images|migrate-images --email ADDRESS")
	}
	switch args[0] {
	case "rehtml":
		return runPostsRehtml(args[1:])
	case "fix-embeds":
		return runPostsFixEmbeds(args[1:])
	case "fix-images":
		return runPostsFixImages(args[1:])
	case "migrate-images":
		return runPostsMigrateImages(args[1:])
	default:
		return fmt.Errorf("unknown posts command %q", args[0])
	}
}

func runPostsRehtml(args []string) error {
	return runPostsTransform(args, "rehtml", func(doc post.Document) post.Document {
		return doc
	})
}

func runPostsFixEmbeds(args []string) error {
	return runPostsTransform(args, "fix-embeds", post.FixDocumentEmbeds)
}

func runPostsFixImages(args []string) error {
	return runPostsTransform(args, "fix-images", post.FixDocumentImageURLs, postNeedsImageFix)
}

func runPostsTransform(args []string, name string, transform func(post.Document) post.Document, needs ...func(post.Document) bool) error {
	cfg := loadConfig()
	email := ""

	fs := flag.NewFlagSet("posts "+name, flag.ContinueOnError)
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

	var updated int
	for _, p := range posts {
		doc, err := post.Parse(p.Doc)
		if err != nil {
			return fmt.Errorf("post %d: %w", p.ID, err)
		}
		if name == "fix-embeds" && !post.NeedsEmbedFix(doc) {
			continue
		}
		if len(needs) > 0 && needs[0] != nil && !needs[0](doc) {
			continue
		}
		next := transform(doc)
		rawDoc, err := json.Marshal(next)
		if err != nil {
			return fmt.Errorf("post %d: %w", p.ID, err)
		}
		html, err := post.RenderHTML(next)
		if err != nil {
			return fmt.Errorf("post %d: %w", p.ID, err)
		}
		words := wordCount(next)
		if _, err := st.UpdatePost(ctx, p.ID, user.ID, p.Title, p.Slug, rawDoc, html, words); err != nil {
			return fmt.Errorf("post %d: %w", p.ID, err)
		}
		updated++
	}

	fmt.Printf("%s: %d posts updated for %s\n", name, updated, user.Email)
	return nil
}

func postNeedsImageFix(doc post.Document) bool {
	return post.NeedsImageFix(doc)
}

func wordCount(doc post.Document) int {
	count := 0
	var walk func([]post.Node)
	walk = func(nodes []post.Node) {
		for _, node := range nodes {
			if node.Text != "" {
				count += len(strings.Fields(node.Text))
			}
			walk(node.Content)
		}
	}
	walk(doc.Content)
	return count
}
