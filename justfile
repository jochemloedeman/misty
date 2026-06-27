compose_local := "docker compose -f compose.yaml -f compose.local.yaml -f compose.observability.yaml"
compose_obs := "docker compose -f compose.yaml -f compose.observability.yaml"
pass := "pass-cli run --env-file .env --"

[group('local')]
local:
    {{ compose_local }} up --build -d

[group('local')]
logs:
    {{ compose_local }} logs

[group('local')]
local-down:
    {{ compose_local }} down

[group('local')]
local-refresh:
    {{ compose_local }} down -v

[group('local')]
local-serve:
    tailscale serve --bg --set-path / http://localhost:8080

[group('local')]
local-serve-off:
    tailscale serve off

[group('local')]
local-serve-status:
    tailscale serve status

[group('deploy')]
build-images:
    ./scripts/build_images.sh

[group('deploy')]
generate-secrets:
    ./scripts/generate-secrets.sh

[group('infra')]
plan:
    {{ pass }} tofu -chdir=infra plan

[group('infra')]
apply:
    {{ pass }} tofu -chdir=infra apply

[group('backup')]
deploy-rclone-config:
    {{ pass }} scripts/render-rclone-config.sh | ssh misty "sudo tee /etc/misty/secrets/rclone.conf > /dev/null && sudo chmod 600 /etc/misty/secrets/rclone.conf"

[group('backup')]
deploy-rclone-config-local:
    {{ pass }} scripts/render-rclone-config.sh > secrets/rclone.conf && chmod 600 secrets/rclone.conf

[group('backup')]
backup-now:
    {{ compose_obs }} run --rm db-backup /backup.sh

[group('backup')]
backup-list:
    {{ compose_obs }} run --rm db-backup sh -c 'rclone lsl "$RCLONE_REMOTE"'

[group('backup')]
restore key:
    {{ compose_obs }} run --rm db-backup sh -c 'export PGPASSWORD=$(cat /run/secrets/postgres_password); rclone cat "$RCLONE_REMOTE/{{ key }}" | pg_restore --clean --if-exists --no-owner -d "$PGDATABASE"'
