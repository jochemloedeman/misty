package db

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

const migrationTimeout = 30 * time.Second

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	migrations, err := fs.Sub(embedMigrations, "migrations")
	if err != nil {
		return fmt.Errorf("accessing embedded migrations: %w", err)
	}

	db := stdlib.OpenDBFromPool(pool)
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations)
	if err != nil {
		return fmt.Errorf("creating goose provider: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, migrationTimeout)
	defer cancel()

	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	return nil
}
