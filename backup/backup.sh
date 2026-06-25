#!/usr/bin/env bash
set -euo pipefail

PGPASSWORD="$(cat /run/secrets/postgres_password)"
export PGPASSWORD

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"

pg_dump -Fc | rclone rcat "${RCLONE_REMOTE}/misty-${timestamp}.pgcustom"

rclone delete "$RCLONE_REMOTE" --min-age "${RETENTION_DAYS}d"
