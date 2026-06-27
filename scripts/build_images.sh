#!/usr/bin/env bash
set -euo pipefail

: "${GITHUB_REPOSITORY:?GITHUB_REPOSITORY must be set}"
REGISTRY="ghcr.io/${GITHUB_REPOSITORY}"
APP_IMAGE="${REGISTRY}:latest"
CADDY_IMAGE="${REGISTRY}/caddy:latest"
BACKUP_IMAGE="${REGISTRY}/backup:latest"
PLATFORM="linux/amd64"

echo "Building ${APP_IMAGE} (${PLATFORM})..."
docker buildx build --platform "${PLATFORM}" -t "${APP_IMAGE}" --push .

echo "Building ${CADDY_IMAGE} (${PLATFORM})..."
docker buildx build --platform "${PLATFORM}" -t "${CADDY_IMAGE}" --push ./caddy

echo "Building ${BACKUP_IMAGE} (${PLATFORM})..."
docker buildx build --platform "${PLATFORM}" -t "${BACKUP_IMAGE}" --push ./backup

echo "Done."
