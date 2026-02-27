db_url := "postgres://postgres:example@localhost:5432/postgres"
migration_dir := "sql/schema"

# Start the PostgreSQL container in the background
db-start:
    @docker compose up -d db

# Stop the PostgreSQL container
db-stop:
    @docker compose down

# Delete all data by removing the database volume
[confirm("This will delete all data in the database. Continue?")]
db-reset:
    @docker compose down -v

# Apply all pending migrations
migrate:
    goose -dir {{migration_dir}} postgres "{{db_url}}" up

# Nuke the database and rebuild from scratch
[confirm("This will destroy and recreate the database. Continue?")]
db-fresh: db-reset db-start
    #!/usr/bin/env sh
    echo "Waiting for PostgreSQL to be ready..."
    until goose -dir {{migration_dir}} postgres "{{db_url}}" status > /dev/null 2>&1; do
      sleep 0.5
    done
    goose -dir {{migration_dir}} postgres "{{db_url}}" up
