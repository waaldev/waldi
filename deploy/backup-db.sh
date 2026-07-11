#!/usr/bin/env bash
# Dump the bundled Postgres database and upload the archive to R2/S3.
# Usage: ./deploy/backup-db.sh
# Cron:  0 3 * * * /opt/waldi/deploy/backup-db.sh >> /var/log/waldi-backup.log 2>&1

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.prod.yml}"
ENV_FILE="${ENV_FILE:-.env}"
BACKUP_DIR="${BACKUP_DIR:-$ROOT/deploy/backups}"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-7}"
AWS_CLI_IMAGE="${AWS_CLI_IMAGE:-amazon/aws-cli:2}"

log() {
	printf '[%s] %s\n' "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" "$*"
}

die() {
	log "ERROR: $*"
	exit 1
}

load_env() {
	if [[ ! -f "$ENV_FILE" ]]; then
		die "missing $ENV_FILE"
	fi
	set -a
	# shellcheck disable=SC1090
	source "$ENV_FILE"
	set +a
}

s3_endpoint_url() {
	local endpoint="${WALDI_S3_ENDPOINT:?WALDI_S3_ENDPOINT is required}"
	local ssl="${WALDI_S3_USE_SSL:-true}"
	endpoint="${endpoint#https://}"
	endpoint="${endpoint#http://}"
	case "$(printf '%s' "$ssl" | tr '[:upper:]' '[:lower:]')" in
		1 | true | yes | on) printf 'https://%s\n' "$endpoint" ;;
		*) printf 'http://%s\n' "$endpoint" ;;
	esac
}

backup_prefix() {
	local prefix="${WALDI_BACKUP_PREFIX:-backups}"
	prefix="${prefix#/}"
	prefix="${prefix%/}"
	printf '%s/\n' "$prefix"
}

aws_s3() {
	docker run --rm \
		-e AWS_ACCESS_KEY_ID="${WALDI_S3_ACCESS_KEY:?WALDI_S3_ACCESS_KEY is required}" \
		-e AWS_SECRET_ACCESS_KEY="${WALDI_S3_SECRET_KEY:?WALDI_S3_SECRET_KEY is required}" \
		-e AWS_DEFAULT_REGION=auto \
		"$AWS_CLI_IMAGE" \
		s3 "$@" --endpoint-url "$(s3_endpoint_url)"
}

prune_local_backups() {
	[[ "$RETENTION_DAYS" -gt 0 ]] || return 0
	find "$BACKUP_DIR" -maxdepth 1 -type f -name 'waldi-*.sql.gz' -mtime +"$RETENTION_DAYS" -delete
}

prune_remote_backups() {
	[[ "$RETENTION_DAYS" -gt 0 ]] || return 0

	local bucket prefix cutoff object_key ts
	bucket="${WALDI_S3_BUCKET:?WALDI_S3_BUCKET is required}"
	prefix="$(backup_prefix)"
	cutoff="$(date -u -d "$RETENTION_DAYS days ago" +%Y%m%dT%H%M%SZ)"

	while IFS= read -r line; do
		[[ -z "$line" ]] && continue
		object_key="${line##* }"
		[[ "$object_key" == waldi-* ]] || continue
		ts="${object_key#waldi-}"
		ts="${ts%.sql.gz}"
		if [[ "$ts" < "$cutoff" ]]; then
			log "deleting remote backup s3://${bucket}/${prefix}${object_key}"
			aws_s3 rm "s3://${bucket}/${prefix}${object_key}"
		fi
	done < <(aws_s3 ls "s3://${bucket}/${prefix}")
}

upload_backup() {
	local file="$1"
	local object_key="$2"
	docker run --rm \
		-v "${file}:/backup.sql.gz:ro" \
		-e AWS_ACCESS_KEY_ID="${WALDI_S3_ACCESS_KEY}" \
		-e AWS_SECRET_ACCESS_KEY="${WALDI_S3_SECRET_KEY}" \
		-e AWS_DEFAULT_REGION=auto \
		"$AWS_CLI_IMAGE" \
		s3 cp /backup.sql.gz "s3://${WALDI_S3_BUCKET}/${object_key}" \
		--endpoint-url "$(s3_endpoint_url)"
}

load_env

: "${POSTGRES_PASSWORD:?POSTGRES_PASSWORD is required in $ENV_FILE}"

mkdir -p "$BACKUP_DIR"

timestamp="$(date -u +"%Y%m%dT%H%M%SZ")"
backup_file="$BACKUP_DIR/waldi-${timestamp}.sql.gz"
object_key="$(backup_prefix)waldi-${timestamp}.sql.gz"
tmp_file="$(mktemp "${TMPDIR:-/tmp}/waldi-backup.XXXXXX.sql.gz")"
cleanup() { rm -f "$tmp_file"; }
trap cleanup EXIT

log "starting database backup"

if ! docker compose -f "$COMPOSE_FILE" exec -T postgres pg_isready -U waldi -d waldi >/dev/null 2>&1; then
	die "postgres is not ready"
fi

if ! docker compose -f "$COMPOSE_FILE" exec -T postgres \
	pg_dump -U waldi -d waldi --no-owner --no-acl | gzip -9 >"$tmp_file"; then
	die "pg_dump failed"
fi

size="$(wc -c <"$tmp_file" | tr -d ' ')"
if [[ "$size" -lt 32 ]]; then
	die "backup file is unexpectedly small (${size} bytes)"
fi

cp "$tmp_file" "$backup_file"
prune_local_backups

human_size="$(du -h "$backup_file" | awk '{print $1}')"
log "backup created: $backup_file ($human_size)"

if ! upload_backup "$backup_file" "$object_key"; then
	die "r2 upload failed"
fi

prune_remote_backups

log "backup uploaded to s3://${WALDI_S3_BUCKET}/${object_key}"
