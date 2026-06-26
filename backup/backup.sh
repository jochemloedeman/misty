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

bytes="$(rclone size "$object" --json | sed 's/.*"bytes":\([0-9]*\).*/\1/')"
log info "backup complete" "object=misty-${timestamp}.pgcustom bytes=${bytes}"

rclone delete "$RCLONE_REMOTE" --min-age "${RETENTION_DAYS}d"
log info "retention pruned" "older_than=${RETENTION_DAYS}d"
