#!/usr/bin/env bash
set -euo pipefail

log() { printf 'level=%s msg="%s" %s\n' "$1" "$2" "${3:-}"; }
trap 'rc=$?; log error "backup failed" "rc=$rc"; exit $rc' ERR

PGPASSWORD="$(cat /run/secrets/postgres_password)"
export PGPASSWORD

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
object="${RCLONE_REMOTE}/misty-${timestamp}.pgcustom"

log info "backup started" "remote=${RCLONE_REMOTE}"

pg_dump -Fc | rclone rcat "$object"

bytes="$(rclone lsf --format s "$object")"

# pg_restore --list reads only the archive TOC and may close the stream early,
# sending SIGPIPE to rclone cat; only pg_restore's own exit status tells us the
# archive is corrupt, so drop pipefail for this one pipeline.
set +o pipefail
if ! rclone cat "$object" | pg_restore --list >/dev/null; then
  log error "backup not restorable" "object=misty-${timestamp}.pgcustom"
  exit 1
fi
set -o pipefail

log info "backup complete" "object=misty-${timestamp}.pgcustom bytes=${bytes}"

rclone delete "$RCLONE_REMOTE" --min-age "${RETENTION_DAYS}d"
log info "retention pruned" "older_than=${RETENTION_DAYS}d"
