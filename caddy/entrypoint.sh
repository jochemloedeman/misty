#!/bin/sh
if [ -f /run/secrets/cloudflare_api_token ]; then
    export CLOUDFLARE_API_TOKEN=$(cat /run/secrets/cloudflare_api_token)
fi
exec caddy run --config /etc/caddy/Caddyfile --adapter caddyfile
