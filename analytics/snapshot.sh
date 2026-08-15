#!/usr/bin/env bash
set -euo pipefail

log() { printf 'level=%s msg="%s" %s\n' "$1" "$2" "${3:-}"; }
trap 'rc=$?; log error "snapshot failed" "rc=$rc"; exit $rc' ERR

PGPASSWORD="$(cat /run/secrets/postgres_password)"
export PGPASSWORD

staging=/data/staging/misty.duckdb
live=/data/misty.duckdb
tables=(users monitors forecasts notifications)

mkdir -p /data/staging
rm -f "$staging" "$staging".wal

log info "snapshot started" "host=${PGHOST}"

duckdb -bail -batch :memory: < /snapshot.sql

counts=""
for t in "${tables[@]}"; do
  n="$(duckdb -bail -batch -noheader -list -readonly "$staging" "SELECT count(*) FROM ${t};")"
  counts="${counts}${t}=${n} "
done

mv -f "$staging" "$live"

log info "snapshot complete" "bytes=$(stat -c %s "$live") ${counts% }"
