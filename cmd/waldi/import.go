package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"
	"waldi/internal/importblogir"
	"waldi/internal/store"
)

// runImport dispatches to a platform-specific importer. To add support for
// another platform, add a case here and an internal/import<platform>
// package implementing LoadExport + Importer.Run following the blogir
// pattern - see CONTRIBUTING.md.
func runImport(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: waldi import blogir --file PATH --email ADDRESS")
	}
	switch args[0] {
	case "blogir":
		return runImportBlogir(args[1:])
	default:
		return fmt.Errorf("unknown import command %q", args[0])
	}
}

func runImportBlogir(args []string) error {
	cfg := loadConfig()
	var (
		file   string
		email  string
		dryRun bool
		limit  int
	)

	fs := flag.NewFlagSet("import blogir", flag.ContinueOnError)
	fs.StringVar(&cfg.DatabaseURL, "database-url", cfg.DatabaseURL, "Postgres connection URL")
	fs.StringVar(&file, "file", "", "blog.ir posts archive XML export file")
	fs.StringVar(&email, "email", "", "Waldi account email to import into")
	fs.BoolVar(&dryRun, "dry-run", false, "preview import without writing")
	fs.IntVar(&limit, "limit", 0, "maximum posts to import (0 = all)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if cfg.DatabaseURL == "" && !dryRun {
		return errors.New("WALDI_DATABASE_URL is required")
	}
	if file == "" {
		return errors.New("--file is required")
	}
	if email == "" && !dryRun {
		return errors.New("--email is required")
	}

	exp, err := importblogir.LoadExport(file)
	if err != nil {
		return err
	}

	imp := importblogir.Importer{
		Opts: importblogir.Options{
			Limit:  limit,
			DryRun: dryRun,
		},
	}

	if !dryRun {
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
		imp.Store = st
		imp.User = user
	}

	result, err := imp.Run(context.Background(), exp.Posts)
	if err != nil {
		return err
	}

	fmt.Printf("imported: %d\n", result.Imported)
	fmt.Printf("skipped: %d\n", result.Skipped)
	if len(result.Failed) > 0 {
		fmt.Printf("failed: %d\n", len(result.Failed))
		for _, f := range result.Failed {
			fmt.Printf("  - %q (%s): %v\n", f.Title, f.Slug, f.Err)
		}
		return fmt.Errorf("import finished with %d failures", len(result.Failed))
	}
	return nil
}
