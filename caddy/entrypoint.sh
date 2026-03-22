#!/bin/sh
export CLOUDFLARE_API_TOKEN=$(cat /run/secrets/cloudflare_api_token)
exec caddy run --config /etc/caddy/Caddyfile --adapter caddyfile
