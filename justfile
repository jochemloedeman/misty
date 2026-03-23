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

# Build and push production images
[group('deploy')]
build-images:
    ./scripts/build_images.sh
