# Deploying waldi on a single VM

The reference deployment is a single **DigitalOcean Droplet** running Docker Compose. Nothing here is DigitalOcean-specific beyond the DNS step, so any VPS provider works the same way. (A managed-platform deploy, e.g. Heroku, is on the roadmap — see `ROADMAP.md` — but isn't supported yet since the app expects to own its own TLS/Caddy layer and a mounted volume for Caddy's certificate cache.)

Production stack: **Caddy** (HTTPS) → **waldi** (HTTP on :8080) → **Postgres** + **Cloudflare R2** (S3-compatible object storage) + SMTP (any provider).

## Prerequisites

- A VM with Docker and Docker Compose (2 GB RAM minimum, 4 GB recommended) — e.g. a DigitalOcean Droplet
- Domain you control (e.g. `waldi.blog`)
- [Cloudflare](https://cloudflare.com) account with the domain on Cloudflare DNS (required for `*.waldi.blog` wildcard TLS, and used for edge caching)
- [Cloudflare R2](https://developers.cloudflare.com/r2/) bucket for image uploads and database backups (any S3-compatible store also works)
- An SMTP provider (e.g. [AhaSend](https://ahasend.com), [Brevo](https://brevo.com), Amazon SES, Resend) for transactional and digest email

## DNS

Point your domain at the VPS:

```
waldi.blog            A     <vps-ip>   (proxied — orange cloud)
*.waldi.blog          A     <vps-ip>   (proxied)
cname.waldi.blog      A     <vps-ip>   (not proxied — custom domain routing target)
```

Enable the orange cloud (proxy) so traffic flows through Cloudflare and edge caching works.

`cname.waldi.blog` is the hostname users CNAME their custom domains to. It is reserved and does not host a blog.

For user custom domains, they must add:

1. **TXT** `_waldi-challenge.example.com` — ownership verification (shown in blog settings)
2. Routing, one of:
   - **CNAME** `www.example.com` → `cname.waldi.blog` (subdomains)
   - **A/AAAA** `example.com` → same address(es) `cname.waldi.blog` resolves to, or an **ALIAS/ANAME** to `cname.waldi.blog` if your provider supports it (root/apex domains, which can't hold a CNAME)

Verification checks the TXT record plus whichever routing record is present before the domain goes live.

## Configure environment

Copy the example and fill in production values:

```bash
cp .env.example .env
```

Required for production:

| Variable | Description |
|----------|-------------|
| `WALDI_ENV` | `prod` |
| `WALDI_BASE_DOMAIN` | Your domain, e.g. `waldi.blog` |
| `WALDI_DATABASE_URL` | Postgres URL (default uses bundled `postgres` service) |
| `POSTGRES_PASSWORD` | Password for bundled Postgres |
| `WALDI_SMTP_*` | SMTP credentials, plus `WALDI_SMTP_FROM` (transactional: verification, password reset) and `WALDI_SMTP_DIGEST_FROM` (bulk: writer/reader digests) |
| `WALDI_S3_*` | Object storage for image uploads (Cloudflare R2, or any S3-compatible store) |
| `CLOUDFLARE_API_TOKEN` | API token with `Zone:DNS:Edit` (and `Zone:Cache Purge` if using CDN purge) |
| `WALDI_CF_ZONE_ID` | Optional — enables Cloudflare cache purge on publish |
| `CADDY_EMAIL` | Email for Let's Encrypt notifications |

Leave `WALDI_TLS_ENABLE` unset (or `false`). Caddy terminates TLS.

### External Postgres

To use a managed database instead of the bundled Postgres service, set `WALDI_DATABASE_URL` to your external connection string and remove the `postgres` service and its `depends_on` from `docker-compose.prod.yml`.

### Already running a reverse proxy on that host?

If port 80/443 are already owned by another proxy on the VM (serving other apps too), use `docker-compose.shared-proxy.yml` instead of `docker-compose.prod.yml` — it starts only `waldi` + Postgres on an external Docker network your existing proxy also joins, without its own Caddy container. Point your existing proxy at the `waldi` service on port 8080, and route `POST /internal/caddy-ask` to it for on-demand custom-domain TLS if your proxy supports it. See the comments at the top of that file.

## Deploy

```bash
docker compose -f docker-compose.prod.yml build
docker compose -f docker-compose.prod.yml up -d
docker compose -f docker-compose.prod.yml run --rm waldi /app/waldi migrate
```

Verify:

```bash
curl -I https://waldi.blog
docker compose -f docker-compose.prod.yml logs -f waldi
```

## Cron jobs

Run daily jobs on the VPS (only once). Replace `/opt/waldi` with your install path.

```cron
0 3 * * * /opt/waldi/deploy/backup-db.sh >> /var/log/waldi-backup.log 2>&1
0 6 * * * cd /opt/waldi && docker compose -f docker-compose.prod.yml exec -T waldi /app/waldi wildcard
10 6 * * * cd /opt/waldi && docker compose -f docker-compose.prod.yml exec -T waldi /app/waldi reader-digest
0 7 * * * cd /opt/waldi && docker compose -f docker-compose.prod.yml exec -T waldi /app/waldi digest
15 7 * * * cd /opt/waldi && docker compose -f docker-compose.prod.yml exec -T waldi /app/waldi reactivation
0 21 * * 5 cd /opt/waldi && docker compose -f docker-compose.prod.yml exec -T waldi /app/waldi weekly-stats
```

`digest` and `reader-digest` require SMTP to be configured. `reader-digest` must run after `wildcard` (it reads that day's wildcard assignment), which is why it's scheduled ten minutes behind it. `reactivation` pauses digests for accounts inactive ~30 days and sends a one-time re-permission email; it's scheduled after `digest` so a user's last digest before going quiet still sends normally. `weekly-stats` requires `WALDI_TELEGRAM_BOT_TOKEN` and `WALDI_TELEGRAM_ADMIN_IDS`; it pushes the trailing-7-day growth summary to every configured Telegram admin (Friday 21:00, `5` = Friday).

Make the backup script executable after deploy:

```bash
chmod +x deploy/backup-db.sh
```

## Database backups (R2)

The backup script dumps Postgres, keeps a short-lived local copy under `deploy/backups/`, and uploads the `.sql.gz` to your R2 bucket (same credentials as image storage).

Objects are stored at `s3://<WALDI_S3_BUCKET>/backups/waldi-YYYYMMDDTHHMMSSZ.sql.gz`. Keep the bucket **private** — do not expose the `backups/` prefix via a public URL.

Ensure these are already set in `.env` (same as image uploads):

```bash
WALDI_S3_ENDPOINT=<account-id>.r2.cloudflarestorage.com
WALDI_S3_ACCESS_KEY=...
WALDI_S3_SECRET_KEY=...
WALDI_S3_BUCKET=waldi
WALDI_S3_USE_SSL=true
WALDI_BACKUP_PREFIX=backups
BACKUP_RETENTION_DAYS=7
```

The R2 API token needs **Object Read & Write** on the bucket. The script uses the `amazon/aws-cli` Docker image for upload.

### Test manually

```bash
./deploy/backup-db.sh
```

Check the object in the Cloudflare R2 dashboard under `backups/`.

### Restore from R2

Download:

```bash
docker run --rm -v "$PWD:/work" -w /work \
  -e AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY -e AWS_DEFAULT_REGION=auto \
  amazon/aws-cli:2 s3 cp "s3://waldi/backups/waldi-YYYYMMDDTHHMMSSZ.sql.gz" ./restore.sql.gz \
  --endpoint-url "https://<account-id>.r2.cloudflarestorage.com"
```

Restore (stop waldi first for a clean import):

```bash
gunzip -c restore.sql.gz | \
  docker compose -f docker-compose.prod.yml exec -T postgres \
  psql -U waldi -d waldi
```

## Updates

```bash
git pull
docker compose -f docker-compose.prod.yml build waldi
docker compose -f docker-compose.prod.yml up -d waldi
docker compose -f docker-compose.prod.yml run --rm waldi /app/waldi migrate
```

Caddy reloads automatically when the stack restarts. Waldi data lives in the `waldi-postgres-data` volume and Caddy certs in `caddy-data`.

## How TLS works

| Host | Certificate |
|------|-------------|
| `waldi.blog`, `*.waldi.blog` | Wildcard via Cloudflare DNS challenge |
| Verified user custom domains | On-demand Let's Encrypt, approved by `GET /internal/caddy-ask` |

The ask endpoint only accepts requests from private networks (Docker internal). Do not publish waldi's port 8080 to the internet.

## Cloudflare CDN cache

Waldi already sends `Cache-Control: public, max-age=86400` on anonymous HTML pages. With Cloudflare proxying enabled, those pages are cached at the edge.

### Setup in Cloudflare dashboard

1. **Cache Rules** → cache responses that have a `Cache-Control` header containing `public`
2. **Cache Rules** → bypass cache when the `Cookie` header contains `waldi_session` (so logged-in users always hit origin)
3. Find your **Zone ID** under Overview → API → set `WALDI_CF_ZONE_ID` in `.env`
4. Add `Zone:Cache Purge` to your API token (or reuse `CLOUDFLARE_API_TOKEN`)

On publish, settings save, or custom-domain change, waldi purges:

- `https://waldi.blog/` (home feed)
- `https://username.waldi.blog/` (writer blog)
- `https://custom-domain/` if the writer has a verified custom domain on your zone

Purges run asynchronously via the [Cloudflare Purge Cache API](https://developers.cloudflare.com/api/resources/cache/methods/purge/). Waldi does not cache pages itself — Cloudflare's edge is the only HTTP cache layer, so purging it is what actually clears stale content.

**Note:** custom domains CNAME to `cname.waldi.blog` and are proxied through Cloudflare when users enable the orange cloud on their DNS. Purge by prefix still runs for the custom domain hostname.

## Local development

Use the dev compose file and Makefile instead:

```bash
make db          # starts postgres (+ minio) via docker-compose.yml
make migrate
make dev
```

See `.env.example` for local settings.
