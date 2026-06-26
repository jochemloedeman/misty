#!/usr/bin/env bash
set -euo pipefail

TEMPLATE="backup/rclone.conf.tmpl"
REMOTE_PATH="/etc/misty/secrets/rclone.conf"

R2_ACCESS_KEY_ID="$(tofu -chdir=infra output -raw r2_access_key_id)"
R2_SECRET_ACCESS_KEY="$(tofu -chdir=infra output -raw r2_secret_access_key)"
R2_ENDPOINT="$(tofu -chdir=infra output -raw r2_endpoint)"

sed \
  -e "s|\${R2_ACCESS_KEY_ID}|${R2_ACCESS_KEY_ID}|g" \
  -e "s|\${R2_SECRET_ACCESS_KEY}|${R2_SECRET_ACCESS_KEY}|g" \
  -e "s|\${R2_ENDPOINT}|${R2_ENDPOINT}|g" \
  "$TEMPLATE" \
  | pass-cli inject \
  | ssh misty "sudo tee ${REMOTE_PATH} > /dev/null && sudo chmod 600 ${REMOTE_PATH}"

echo "Placed ${REMOTE_PATH} on the VPS"
