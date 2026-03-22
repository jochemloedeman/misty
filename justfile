compose_dev := "docker compose -f compose.yaml -f compose.dev.yaml"

# Start the dev docker compose stack
[group('dev')]
dev:
    {{ compose_dev }} up --build -d

# Stop the dev docker compose stack
[group('dev')]
dev-down:
    {{ compose_dev }} down

# Stop the dev stack and delete all volumes/data
[group('dev')]
dev-refresh:
    {{ compose_dev }} down -v

# Build and push production images
[group('deploy')]
build-images:
    ./scripts/build_images.sh
