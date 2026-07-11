package importblogir

import (
	"context"
	"fmt"
	"strings"
	"waldi/internal/importcommon"
	"waldi/internal/post"
	"waldi/internal/store"
)

type Options struct {
	Limit  int
	DryRun bool
}

type Result struct {
	Imported int
	Skipped  int
	Failed   []Failure
}

type Failure struct {
	Title string
	Slug  string
	Err   error
}

type Importer struct {
	Store *store.Store
	User  store.User
	Opts  Options
}

func (imp *Importer) Run(ctx context.Context, posts []Post) (Result, error) {
	var res Result
	converter := importcommon.Converter{}
	for _, p := range posts {
		if imp.Opts.Limit > 0 && res.Imported >= imp.Opts.Limit {
			break
		}
		p.Title = strings.TrimSpace(p.Title)
		p.URL = strings.TrimSpace(p.URL)
		if strings.TrimSpace(p.Content) == "" {
			res.Failed = append(res.Failed, Failure{Title: p.Title, Slug: p.URL, Err: fmt.Errorf("empty content")})
			continue
		}

		html := rewriteProtocolRelativeURLs(p.Content)
		converted, err := converter.ConvertPost(html)
		if err != nil {
			res.Failed = append(res.Failed, Failure{Title: p.Title, Slug: p.URL, Err: err})
			continue
		}
		doc, err := post.Parse(converted.Doc)
		if err != nil {
			res.Failed = append(res.Failed, Failure{Title: p.Title, Slug: p.URL, Err: err})
			continue
		}

		slugBase := slugify(p.URL)
		if slugBase == "" {
			slugBase = slugify(p.Title)
		}
		if slugBase == "" {
			slugBase = "post"
		}

		createdAt, err := ParseTime(p.CreatedDate)
		if err != nil {
			res.Failed = append(res.Failed, Failure{Title: p.Title, Slug: p.URL, Err: err})
			continue
		}
		updatedAt, err := ParseTime(p.LastModifiedDate)
		if err != nil {
			updatedAt = createdAt
		}

		if imp.Opts.DryRun {
			fmt.Printf("would import: %q slug=%q words=%d\n", p.Title, slugBase, importcommon.WordCount(doc))
			res.Imported++
			continue
		}

		slug, err := imp.Store.UniqueSlug(ctx, imp.User.ID, slugBase, 0)
		if err != nil {
			res.Failed = append(res.Failed, Failure{Title: p.Title, Slug: p.URL, Err: err})
			continue
		}

		_, err = imp.Store.ImportPost(ctx, store.ImportPostParams{
			UserID:      imp.User.ID,
			Title:       p.Title,
			Slug:        slug,
			Doc:         converted.Doc,
			HTML:        converted.HTML,
			WordCount:   importcommon.WordCount(doc),
			Status:      "published",
			PublishedAt: &createdAt,
			CreatedAt:   createdAt,
			UpdatedAt:   updatedAt,
		})
		if err != nil {
			res.Failed = append(res.Failed, Failure{Title: p.Title, Slug: p.URL, Err: err})
			continue
		}
		res.Imported++
	}
	return res, nil
}
