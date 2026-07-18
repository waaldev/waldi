# Contributing to Waldi

Waldi is a production app first and an open-source project second. One person still dictates the roadmap and the product direction (see `ROADMAP.md`). But contributions matter. For anything larger than a trivial typo fix, open an issue first. We need to talk it through before you burn a weekend on a PR I won't merge.

## Local setup

```bash
cp .env.example .env
make db        # starts Postgres (+ MinIO) via docker compose
make migrate   # runs migrations
make dev       # air (Go live reload) + esbuild --watch for the editor
```

Check the README's Quickstart section for local subdomain testing (`username.localhost:8080`, `waldi.test` to test cross-subdomain cookie behavior).

## Before opening a PR

```bash
make fmt    # formats cmd and internal
make lint   # golangci-lint
make test   # go test
```

CI aggressively runs these exact three steps on every push.

- **Keep PRs tight.** One change, one PR. Do not bundle refactors with bug fixes.
- **Test the domain logic.** `internal/post` (the Tiptap document model, schema validation, HTML rendering) is the most heavily tested package in the codebase. Changes there demand tests. It's pure domain logic without database or HTTP noise, so tests are cheap. Write them.
- **Routing templates.** If you add a new template page, define a view struct. Wire it into `web.PageData` (`internal/web/render.go`). Do not pass ad hoc `map[string]any` data into templates.
- **Language catalog.** If you touch user-facing text, route it through `internal/i18n` (`T(lang, key, args...)`). Never hardcode English. Read the README's Multi-language section to see how the catalog works.

## Database migrations

Migrations live in `migrations/00N_*.sql`. They are strictly **append-only**. Never edit a migration that has already run anywhere, including the history of this repository. Add a new numbered file instead. We run them sequentially via `internal/migrate`.

## Adding an importer

Every blog importer has one job: translate a dead platform's export into a living Waldi post. The blog.ir importer (`internal/importblogir`) is your reference implementation. Follow this exact blueprint:

1. Create `internal/import<platform>` with an `Export`/`LoadExport(path) (Export, error)` to parse the raw file. Define a `Post` type for the entries.
2. Pipe each post's HTML into `internal/importcommon`. The `importcommon.Converter{}.ConvertPost(rawHTML)` method spits out a validated Waldi document, sanitized HTML, and a word count. Clean up platform-specific garbage *before* calling `ConvertPost`. Don't pollute the shared converter.
3. Build an `Importer{Store, User, Opts}` featuring a `Run(ctx, posts) (Result, error)` method. Loop the posts, convert them, and hit `store.ImportPost`.
4. Wire your new package into `cmd/waldi/import.go`. If it's ready for humans, build a settings page matching `internal/web/handlers_import_blogir.go`.

There are no runtime plugins. A new importer is a compiled Go package.

## Branches

- `main` is production. Period.
- `develop` catches the day-to-day chaos. Branch your PRs from `develop` unless you're deploying a critical hotfix directly to `main`.

## Good first contributions

- **A new interface language.** The entire recipe is four steps, detailed in the README. You don't touch templates. The catalog just grows.
- **A new blog importer.** We need Blogfa, Persianblog, and WordPress. Study `internal/importblogir` and build the next one.
- **Design work.** Read `docs/DESIGN.md` before you touch a pixel. It defines our typography and outlaws a massive list of UI cliches. If your PR adds a drop shadow, a badge, or a third typeface, I will decline it with love.

## Reporting bugs / requesting features

Use the issue templates. If it's a security flaw, read `SECURITY.md` and email me directly. Don't file a public issue.
