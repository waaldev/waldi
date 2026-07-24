# Deploying Waldi on a Single VM

The reference deployment is a single **DigitalOcean Droplet** running Docker Compose. There is nothing uniquely tied to DigitalOcean here except the DNS configuration step. Any VPS provider works exactly the same way. We plan to support managed platforms (like Heroku), but we don't yet. Why? Because Waldi demands total control over its TLS/Caddy layer and requires a mounted volume for Caddy's certificate cache.

The production stack is brutally simple: **Caddy** (HTTPS) → **Waldi** (HTTP on :8080) → **Postgres** + **Cloudflare R2** (S3-compatible object storage) + **SMTP** (any provider you trust).

## Prerequisites

- A VM running Docker and Docker Compose. (2 GB RAM is the absolute minimum; 4 GB is recommended.)
- A domain you actually control (e.g., `waldi.blog`).
- A [Cloudflare](https://cloudflare.com) account. The domain must be on Cloudflare DNS. We rely on this for the `*.waldi.blog` wildcard TLS and edge caching.
- A [Cloudflare R2](https://developers.cloudflare.com/r2/) bucket for image uploads and database backups. (Any S3-compatible store will suffice.)
- An SMTP provider (like AhaSend, Brevo, Amazon SES, or Resend) to fire off transactional and digest emails.

## DNS

Point your domain at the VPS. 

```
waldi.blog            A     <vps-ip>   (proxied - orange cloud)
*.waldi.blog          A     <vps-ip>   (proxied)
cname.waldi.blog      A     <vps-ip>   (not proxied - custom domain routing target)
```

Enable the orange cloud (proxy) on the first two. Traffic must flow through Cloudflare for edge caching to work. 

The `cname.waldi.blog` hostname is the routing target where users CNAME their custom domains. It is strictly reserved. It does not host a blog.

For users adding custom domains, they must add:

1. **TXT** `_waldi-challenge.example.com` - this verifies ownership (surface this in blog settings).
2. Routing. They need one of the following:
   - **CNAME** `www.example.com` → `cname.waldi.blog` (for subdomains).
   - **A/AAAA** `example.com` → the exact IP addresses `cname.waldi.blog` resolves to. Alternatively, an **ALIAS/ANAME** to `cname.waldi.blog` if their provider allows it (for root/apex domains that cannot hold a CNAME).

Our verification process checks the TXT record and the routing record before allowing the domain to go live.

## Configure the Environment

Copy the example file and inject your production values:

```bash
cp .env.example .env
```

The required production variables:

| Variable | Description |
|----------|-------------|
| `WALDI_ENV` | `prod` |
| `WALDI_BASE_DOMAIN` | Your domain, e.g. `waldi.blog` |
| `WALDI_DATABASE_URL` | Postgres URL (default targets the bundled `postgres` service) |
| `POSTGRES_PASSWORD` | The password for the bundled Postgres instance |
| `WALDI_SMTP_*` | Your SMTP credentials. Also set `WALDI_SMTP_FROM` (transactional: verification, resets) and `WALDI_SMTP_DIGEST_FROM` (bulk: digests). |
| `WALDI_S3_*` | Object storage for image uploads. (Cloudflare R2 or equivalent S3 store.) |
| `CLOUDFLARE_API_TOKEN` | An API token with `Zone:DNS:Edit` (and `Zone:Cache Purge` if using the CDN purge feature). |
| `WALDI_CF_ZONE_ID` | Optional. Setting this enables Cloudflare cache purge on publish. |
| `CADDY_EMAIL` | The email address Let's Encrypt will use for notifications. |

Leave `WALDI_TLS_ENABLE` unset (or set it to `false`). Caddy terminates TLS for you.

### External Postgres

If you refuse to use the bundled Postgres container and prefer a managed database, point `WALDI_DATABASE_URL` to your external connection string. Delete the `postgres` service and its `depends_on` block from `docker-compose.prod.yml`.

### Already running a reverse proxy?

If another proxy already owns ports 80 and 443 on your VM, ignore `docker-compose.prod.yml`. Use `docker-compose.shared-proxy.yml` instead. It boots `waldi` and Postgres on an external Docker network without spinning up Caddy. Point your existing proxy at the `waldi` service on port 8080. Route `POST /internal/caddy-ask` to it if you need on-demand custom-domain TLS. Read the comments at the top of that file for details.

## Deploy

Execute the build and launch the stack:

```bash
docker compose -f docker-compose.prod.yml build
docker compose -f docker-compose.prod.yml up -d
docker compose -f docker-compose.prod.yml run --rm waldi /app/waldi migrate
```

Verify it's alive:

```bash
curl -I https://waldi.blog
docker compose -f docker-compose.prod.yml logs -f waldi
```

## Cron Jobs

Schedule these daily jobs on the VPS crontab. Run them once. Replace `/opt/waldi` with your actual installation path.

```cron
0 3 * * * /opt/waldi/deploy/backup-db.sh >> /var/log/waldi-backup.log 2>&1
0 6 * * * cd /opt/waldi && docker compose -f docker-compose.prod.yml exec -T waldi /app/waldi wildcard
10 6 * * * cd /opt/waldi && docker compose -f docker-compose.prod.yml exec -T waldi /app/waldi reader-digest
0 7 * * * cd /opt/waldi && docker compose -f docker-compose.prod.yml exec -T waldi /app/waldi digest
15 7 * * * cd /opt/waldi && docker compose -f docker-compose.prod.yml exec -T waldi /app/waldi reactivation
0 21 * * 5 cd /opt/waldi && docker compose -f docker-compose.prod.yml exec -T waldi /app/waldi weekly-stats
```

Both `digest` and `reader-digest` fail without SMTP. `reader-digest` explicitly reads the daily wildcard assignment, which is why it runs exactly ten minutes after the `wildcard` job. `reactivation` pauses digests for silent accounts (inactive for ~30 days) and fires a one-time re-permission email. It runs after `digest` so their final email sends normally.

`weekly-stats` requires `WALDI_TELEGRAM_BOT_TOKEN` and `WALDI_TELEGRAM_ADMIN_IDS`. It blasts a trailing-7-day growth summary to Telegram admins every Friday at 21:00.

Do not forget to make the backup script executable:

```bash
chmod +x deploy/backup-db.sh
```

## Database Backups (R2)

The backup script dumps Postgres, saves a transient local copy under `deploy/backups/`, and forcefully uploads a `.sql.gz` archive to your R2 bucket. It uses the exact same credentials as your image storage.

Objects land at `s3://<WALDI_S3_BUCKET>/backups/waldi-YYYYMMDDTHHMMSSZ.sql.gz`. Keep the bucket **private**. Never expose the `backups/` prefix to the internet.

Ensure these variables exist in `.env`:

```bash
WALDI_S3_ENDPOINT=<account-id>.r2.cloudflarestorage.com
WALDI_S3_ACCESS_KEY=...
WALDI_S3_SECRET_KEY=...
WALDI_S3_BUCKET=waldi
WALDI_S3_USE_SSL=true
WALDI_BACKUP_PREFIX=backups
BACKUP_RETENTION_DAYS=7
```

Your R2 API token must possess **Object Read & Write** permissions on the bucket. The script executes the upload via the `amazon/aws-cli` Docker image.

### Test Manually

Run it yourself:

```bash
./deploy/backup-db.sh
```

Verify the `.sql.gz` object appeared in the Cloudflare R2 dashboard under `backups/`.

### Restore from R2

Pull the archive down:

```bash
docker run --rm -v "$PWD:/work" -w /work \
  -e AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY -e AWS_DEFAULT_REGION=auto \
  amazon/aws-cli:2 s3 cp "s3://waldi/backups/waldi-YYYYMMDDTHHMMSSZ.sql.gz" ./restore.sql.gz \
  --endpoint-url "https://<account-id>.r2.cloudflarestorage.com"
```

Stop Waldi to guarantee a clean import, then restore:

```bash
gunzip -c restore.sql.gz | \
  docker compose -f docker-compose.prod.yml exec -T postgres \
  psql -U waldi -d waldi
```

## Updates

Pull the code and rebuild:

```bash
git pull
docker compose -f docker-compose.prod.yml build waldi
docker compose -f docker-compose.prod.yml up -d waldi
docker compose -f docker-compose.prod.yml run --rm waldi /app/waldi migrate
```

Caddy automatically reloads when the stack restarts. Your data is protected: Postgres lives in the `waldi-postgres-data` volume, and Caddy certs reside in `caddy-data`.

## How TLS Works

| Host | Certificate |
|------|-------------|
| `waldi.blog`, `*.waldi.blog` | Wildcard generated via Cloudflare DNS challenge |
| Verified user custom domains | On-demand Let's Encrypt, verified via `GET /internal/caddy-ask` |

The ask endpoint strictly accepts requests from the private Docker network. If you expose Waldi's port 8080 to the public internet, you compromise this security layer. Don't do it.

## Cloudflare CDN Cache

Waldi inherently serves `Cache-Control: public, max-age=86400` headers on all anonymous HTML pages. When you enable the Cloudflare proxy, those pages are cached instantly at the edge.

### Setup in Cloudflare Dashboard

1. **Cache Rules** → Cache responses where the `Cache-Control` header contains `public`.
2. **Cache Rules** → Bypass cache when the `Cookie` header contains `waldi_session`. This ensures logged-in writers hit the origin database.
3. Locate your **Zone ID** under Overview → API. Set `WALDI_CF_ZONE_ID` in `.env`.
4. Attach `Zone:Cache Purge` permissions to your API token.

Every time a writer publishes, updates settings, or links a custom domain, Waldi aggressively purges:

- `https://waldi.blog/` (The home feed)
- `https://username.waldi.blog/` (The writer's blog)
- `https://custom-domain/` (If they have a verified custom domain on your zone)

These purges fire asynchronously against the Cloudflare Purge Cache API. Waldi does not operate its own internal HTML cache. The edge is the only caching layer. When we purge it, the old content dies.

**Note:** Custom domains use a CNAME to `cname.waldi.blog`. If the user enables the orange proxy cloud on their DNS, the purge-by-prefix logic still successfully hits the custom domain hostname.

## Local Development

Ditch the production compose file. Use `make`:

```bash
make db          # Boots Postgres and MinIO
make migrate
make dev
```

Review `.env.example` to set up your local environment.
