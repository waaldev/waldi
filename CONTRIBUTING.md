# Contributing to Waldi

Waldi is a production app first, an open-source project second — one person still shapes the roadmap and product direction (see `ROADMAP.md`). Contributions are welcome. For anything bigger than a small fix, open an issue first so we can talk it through before you spend time on a PR.

## Local setup

```bash
cp .env.example .env
make db        # start Postgres (+ MinIO) via docker compose for local dev
make migrate   # run migrations
make dev       # air (Go live reload) + esbuild --watch for the editor, in parallel
```

See the README's Quickstart section for local subdomain testing (`username.localhost:8080`, `waldi.test` for cross-subdomain cookie behavior).

## Before opening a PR

```bash
make fmt    # gofmt -w cmd internal
make lint   # golangci-lint run
make test   # go test ./...
```

CI runs the same three steps (lint → test → build) on every push and pull request.

- Keep PRs small and focused — one change, one PR.
- `internal/post` (the Tiptap document model: schema validation, sanitized HTML rendering, embeds, footnotes) is the most heavily unit-tested package in the repo. Changes there need tests; it's pure domain logic with no DB or HTTP dependencies, so tests are cheap to write and fast to run.
- If you're adding a new template page, add a view struct and wire it into `web.PageData` (see `internal/web/render.go`) rather than passing ad hoc data into templates.
- If you're touching user-facing strings, route them through `internal/i18n` (`T(lang, key, args...)`) rather than hardcoding English — see the README's Multi-language section for how the catalog works.

## Database migrations

Migrations live in `migrations/00N_*.sql`, run in order by `internal/migrate` (golang-migrate style). They're **append-only**: never edit a migration that's already been applied anywhere, including in this repo's history — add a new numbered file instead.

## Adding an importer

Every blog importer shares one job: turn some platform's export format into a waldi post. `internal/importblogir` (blog.ir) is the reference implementation — follow its shape for a new platform:

1. Create `internal/import<platform>` with an `Export`/`LoadExport(path) (Export, error)` for parsing that platform's export file, and a `Post` type for one entry.
2. Convert each post's HTML with `internal/importcommon`: `importcommon.Converter{}.ConvertPost(rawHTML)` returns a validated waldi document, sanitized HTML, and a word count. This is the same converter every importer uses — platform-specific quirks (odd markup, embeds, image URLs) belong in your package's pre-processing of the raw HTML before it reaches `ConvertPost`, not in the shared converter.
3. Add an `Importer{Store, User, Opts}` with a `Run(ctx, posts) (Result, error)` method that loops the posts, converts them, and calls `store.ImportPost` — see `internal/importblogir/import.go`.
4. Wire it into `cmd/waldi/import.go`'s `runImport` dispatch and, if it should be user-facing (not just a CLI tool for you), add a settings page following `internal/web/handlers_import_blogir.go` and `web/templates/import_blogir.html`.

There's no runtime plugin system — a new importer is a new Go package, compiled in.

## Branches

- `main` tracks production.
- `develop` is where day-to-day work lands; branch your PRs from `develop` unless you have a reason to target `main` directly (e.g. a hotfix).


## Good first contributions

- **A new interface language** — the whole recipe is four small steps, documented in the README's [Multi-language / i18n](README.md#multi-language--i18n) section. No template changes needed; the catalog is the only thing that grows.
- **A new blog importer** — `internal/importblogir` is the reference implementation; the pattern is documented step-by-step in [Adding an importer](#adding-an-importer) above. Blogfa, Persianblog, and WordPress are wanted (see ROADMAP.md).
- **Design/template work** — read [docs/DESIGN.md](docs/DESIGN.md) first; it defines the tokens, the typography, and the (long) list of things deliberately banned. PRs that add shadows, badges, or a third typeface will be declined with love.

## Reporting bugs / requesting features

Use the issue templates. For security issues, see `SECURITY.md` instead of filing a public issue.
