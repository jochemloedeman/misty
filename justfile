compose_local := "docker compose -f compose.yaml -f compose.local.yaml"

# Start the local docker compose stack
[group('local')]
local:
    {{ compose_local }} up --build -d

# Stop the local docker compose stack
[group('local')]
local-down:
    {{ compose_local }} down

# Stop the local stack and delete all volumes/data
[group('local')]
local-refresh:
    {{ compose_local }} down -v

# Expose the local app via Tailscale Serve (HTTPS with valid cert)
[group('local')]
local-serve:
    tailscale serve --bg --set-path / http://localhost:8080

# Stop Tailscale Serve
[group('local')]
local-serve-off:
    tailscale serve off

# Show Tailscale Serve status
[group('local')]
local-serve-status:
    tailscale serve status

# Build and push production images
[group('deploy')]
build-images:
    ./scripts/build_images.sh
