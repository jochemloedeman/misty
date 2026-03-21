#!/usr/bin/env bash
set -euo pipefail

REGISTRY="ghcr.io/jochemloedeman"
APP_IMAGE="${REGISTRY}/misty:latest"
CADDY_IMAGE="${REGISTRY}/misty/caddy:latest"
PLATFORM="linux/amd64"

echo "Building ${APP_IMAGE} (${PLATFORM})..."
docker buildx build --platform "${PLATFORM}" -t "${APP_IMAGE}" --push .

echo "Building ${CADDY_IMAGE} (${PLATFORM})..."
docker buildx build --platform "${PLATFORM}" -t "${CADDY_IMAGE}" --push ./caddy

echo "Done."
