package migrate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Up(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	if _, err := pool.Exec(ctx, `
		create table if not exists schema_migrations (
			filename text primary key,
			applied_at timestamptz not null default now()
		)
	`); err != nil {
		return fmt.Errorf("ensuring schema_migrations: %w", err)
	}

	files, err := migrationFiles(dir)
	if err != nil {
		return err
	}

	for _, file := range files {
		name := filepath.Base(file)
		applied, err := isApplied(ctx, pool, name)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		if err := applyFile(ctx, pool, file, name); err != nil {
			return err
		}
	}
	return nil
}

func migrationFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading migrations dir: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(files)
	return files, nil
}

func isApplied(ctx context.Context, pool *pgxpool.Pool, name string) (bool, error) {
	var exists bool
	if err := pool.QueryRow(ctx, `select exists(select 1 from schema_migrations where filename = $1)`, name).Scan(&exists); err != nil {
		return false, fmt.Errorf("checking migration %s: %w", name, err)
	}
	return exists, nil
}

func applyFile(ctx context.Context, pool *pgxpool.Pool, file, name string) error {
	sql, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("reading migration %s: %w", name, err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning migration %s: %w", name, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, string(sql)); err != nil {
		return fmt.Errorf("applying migration %s: %w", name, err)
	}
	if _, err := tx.Exec(ctx, `insert into schema_migrations (filename) values ($1)`, name); err != nil {
		return fmt.Errorf("recording migration %s: %w", name, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing migration %s: %w", name, err)
	}
	return nil
}
