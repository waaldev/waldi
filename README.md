# Waldi

A quiet place to write, and to be read.

Live at **[waldi.blog](https://waldi.blog)**. Reading is open to the world. Writing requires an invitation—for now—while the founding writers shape the culture.

![A Waldi post in light and dark mode](docs/screenshot.jpg)

Waldi is a multi-tenant blogging engine built for people who miss blogs. Under the hood, it's brutalist engineering: a single Go binary, one Postgres database, server-rendered `html/template` pages, and a solitary Tiptap editor island. Every writer claims `username.<base-domain>` (or their own custom domain). The binary serves everyone by simply reading the request's `Host` header at runtime. Zero per-tenant deployments. Zero infrastructure bloat.

The entire system exists to support one specific loop: you publish a post, a hundred strangers read it, and a private letter comes back. No public like counts. No comment sections. No toxic leaderboards. We replaced replies with private letters. Every post gets a guaranteed floor of human readers before any black-box algorithm judges it.

New writers never publish into a void.

## Why

Read the origin story here: [Serendipity](https://amin.waldi.blog/serendipity).

The product's rules—letters instead of comments, zero public metrics, a hard list of features we will absolutely never build—are etched into [ROADMAP.md](ROADMAP.md) and [docs/DESIGN.md](docs/DESIGN.md).

## Architecture

### Request flow / multi-tenancy

Every request routes through `internal/web/server.go: ServeHTTP`. It checks if the `Host` header maps to a blog (`internal/web/host.go: isBlogHost`):

1. **Cheap subdomain match:** `username.<base-domain>` (or `.localhost` / `.waldi.test` in dev) triggers `BlogFromHost`.
2. **Database fallback:** A verified custom domain lookup against Postgres, cached in-process (`customDomainCache`, positive/negative TTLs) to kill per-request database hits.

Handlers branch immediately depending on whether the host is a blog or the main app domain (`internal/web/blog.go`, `handlers_read.go`). Session cookies scope to `cookieDomain` so a main domain login carries over to subdomains. Custom domains stay isolated.

### Layering

- `internal/web` — The HTTP spine. Routing (`server.go`), handlers (one file per feature area), template data structs, session/locale resolution.
- `internal/store` — Pure SQL wrapping a `*pgxpool.Pool` (`store.go`). One file per entity. Handlers call these directly. No repository bloat. No service interfaces.
- `internal/post` — Uncompromising domain logic for the Tiptap document model. JSON schema validation, sanitized HTML rendering, embeds. Zero database or HTTP dependencies. This is the most heavily unit-tested package.
- `internal/jobs` — Daily background jobs invoked as CLI subcommands. No heavy queue or worker processes.
- `internal/mail` — SMTP abstraction. `NoopMailer` steps in for local dev.
- `internal/storage` — Image uploads to local disk or S3 (Cloudflare R2 in production). Resizes everything to WebP on upload.
- `internal/cdn` — Cache invalidation for Cloudflare on publish.
- `internal/i18n` — Translation catalog feeding Go and the templates.
- `internal/telegrambot` — The admin brain (more below).
- `internal/importcommon` — Shared HTML-to-document converter.
- `internal/importblogir` — The blog.ir importer. Use this as the blueprint for adding new platforms (see `CONTRIBUTING.md`).
- `internal/migrate` — Runs numbered, append-only SQL patches from `migrations/`.
- `cmd/waldi` — CLI entrypoint. Subcommands dispatch from `main.go`.

### Frontend

- `web/templates/*.html` — Server-rendered pages. Loaded via `template.ParseFS`. Data flows through a single `web.PageData` struct.
- `web/static/css/main.css` — Hand-written. No frameworks. None.
- `web/editor/` — The Tiptap island (TypeScript), bundled violently fast by esbuild directly to `web/static/js/editor.js`.
- Server-side enforcement in `internal/post/doc.go` dictates the editor's document schema. The client does not validate itself. The server stops invalid documents.

### Data model

Postgres rules everything (`migrations/00N_*.sql`). Core tables include `users`, `sessions`, `posts` (`doc` jsonb acts as the Tiptap source of truth, `html` pre-rendered), `follows`, `letters`, `impressions`/`readings`, `wildcards`, and `digests`.

## Infrastructure

| Concern | Tooling |
|---|---|
| Database | PostgreSQL |
| Object storage | Cloudflare R2 (or any S3-compatible store). Resizes and converts to WebP. |
| Edge cache | Cloudflare. Anonymous HTML serves `Cache-Control: public`, purged via API on publish. |
| TLS + custom domains | Caddy. Wildcard certs via Cloudflare DNS challenge. On-demand Let's Encrypt certs for verified custom domains. |
| Email | Any SMTP provider. Separate From addresses for transactional vs. bulk. |
| Deploy | Docker Compose on a single VM (DigitalOcean Droplet). PaaS targets like Heroku are planned but unsupported since the app owns its TLS layer. |
| CI/CD | GitHub Actions. Lint → test → build on push/PR, deploy over SSH on merge to `main`. |
| Backups | Daily cron dumps Postgres to a gzip archive in R2. |
| Admin | A Telegram bot completely replaces a web admin panel. |

Read `DEPLOY.md` for the exact production recipe.

## Self-hosting

Yes, run it yourself. The MIT license allows it, and `DEPLOY.md` holds the keys. But understand two realities: **waldi.blog is the canonical community.** It is the only instance I operate. Self-hosted instances become quiet, isolated islands because the guaranteed-readers loop only works if you have readers. Also, the roadmap serves waldi.blog first. I welcome self-hoster issues, but hosted concerns take priority.

## Multi-language / i18n

We embed translation strings into flat JSON catalogs (`internal/i18n/locales/en.json`, `fa.json`). The codebase calls `T(lang, key, args...)` and templates share these universally. We never duplicate templates per language.

Per-request language priority: user's stored locale → `locale` cookie → `CF-IPCountry` header → default (`fa`).

**To add a new language:**
1. Drop `internal/i18n/locales/<code>.json`, mirroring keys from `en.json`.
2. Add `"<code>": true` to `supported` in `internal/i18n/catalog.go`.
3. If RTL, add a case to `Dir()` in the same file.
4. Update `i18n.LangFromCountry` to auto-route visitors.

No template changes required. Only the catalog grows.

## Background jobs / cron

Three scheduled jobs live in `internal/jobs/`. They execute as CLI commands via the host crontab:

```cron
0 3 * * *  ./deploy/backup-db.sh      # daily Postgres backup -> R2
0 6 * * *  waldi wildcard             # pick today's wildcard post per reader
10 6 * * * waldi reader-digest        # reader + anonymous reader digest (reads the wildcard assignment above)
0 7 * * *  waldi digest               # writer digest
0 21 * * 5 waldi weekly-stats         # growth summary, pushed to Telegram admins
```

### Wildcard selection

Every reader receives one "wildcard" post daily. A stranger's post. A guaranteed impressions floor. Zero algorithmic manipulation. The logic (`internal/jobs/wildcard.go`) works in two tiers:

1. **Admin-curated pool:** Posts flagged by an admin via Telegram take priority.
2. **Fallback heuristic:** A brutally simple SQL query fetching published, non-test posts (≥50 words) in the reader's language. We exclude their own posts, followed authors, and previously read items. We prioritize posts sitting under the 100-impression floor.

In the future (see `ROADMAP.md`), a formal scoring system replaces this. It will weigh follow-rates and completion-rates, escalating exposure as a post proves itself.

## Telegram admin bot

There is no web admin panel. A Telegram bot (`internal/telegrambot`) handles moderation and support. It locks to specific admin Telegram user IDs via environment variables.

Commands include `/users`, `/posts` (with inline wildcard buttons), `/pool`, `/verify`, `/delete`, and `/invite`.

## Quickstart

```bash
cp .env.example .env
make db        # spin up Postgres and MinIO
make migrate   # run migrations
make dev       # parallel run: air (Go live reload) + esbuild --watch
```

Use `waldi.test` (with `/etc/hosts` entries) to test cross-subdomain cookie logic.

## Commands

```
make db        # start Postgres/MinIO via docker compose
make migrate   # run migrations (go run ./cmd/waldi migrate)
make dev       # air + esbuild --watch
make build     # build editor bundle, then go build ./cmd/waldi
make test      # go test ./...
make lint      # golangci-lint run
make fmt       # gofmt -w cmd internal
```

## Contributing

See `CONTRIBUTING.md`. Day-to-day chaos lands in `develop`. `main` is production.

## Roadmap

Check `ROADMAP.md` for our shipped features, current obsession, and the things we will deliberately never build.

## License

MIT — see `LICENSE`.
