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

# -f /dev/null reads the whole archive (script mode), unlike --list which only
# reads the TOC, so a truncated dump and a failed rclone cat both fail here.
if ! rclone cat "$object" | pg_restore -f /dev/null; then
  log error "backup not restorable" "object=misty-${timestamp}.pgcustom"
  exit 1
fi

log info "backup complete" "object=misty-${timestamp}.pgcustom bytes=${bytes}"

if ! [[ "$RETENTION_DAYS" =~ ^[1-9][0-9]*$ ]]; then
  log error "invalid retention, skipping prune" "RETENTION_DAYS=${RETENTION_DAYS}"
  exit 1
fi

rclone delete "$RCLONE_REMOTE" --min-age "${RETENTION_DAYS}d"
log info "retention pruned" "older_than=${RETENTION_DAYS}d"
