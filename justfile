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
