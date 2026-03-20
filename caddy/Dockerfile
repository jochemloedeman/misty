FROM caddy:2.11.2-builder-alpine AS builder

RUN xcaddy build --with github.com/mholt/caddy-ratelimit

FROM caddy:2.11.2-alpine

COPY --from=builder /usr/bin/caddy /usr/bin/caddy
COPY conf/Caddyfile /etc/caddy/Caddyfile
