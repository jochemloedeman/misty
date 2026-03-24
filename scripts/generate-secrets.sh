#!/usr/bin/env bash
set -euo pipefail

SECRETS_DIR=/etc/misty/secrets

ssh misty "
  sudo mkdir -p $SECRETS_DIR
  for secret in postgres_password signing_secret signing_secret_previous; do
    openssl rand -base64 32 | sudo tee $SECRETS_DIR/\${secret}.txt > /dev/null
    echo \"Generated \${secret}.txt\"
  done
"
