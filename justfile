db_url := "postgres://postgres:password@localhost:5432/postgres"
migration_dir := "db/migrations"

# Start the PostgreSQL container in the background
db-start:
    @docker compose up -d db

# Stop the PostgreSQL container
db-stop:
    @docker compose down

# Delete all data by removing the database volume
db-reset:
    @docker compose down -v

# Apply all pending migrations
migrate:
    goose -dir {{migration_dir}} postgres "{{db_url}}" up

