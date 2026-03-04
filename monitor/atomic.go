package monitor

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jochemloedeman/misty/monitor/sqlc"
)

func NewRunAtomically(pool *pgxpool.Pool) RunAtomically {
	return func(ctx context.Context, fn func(s AtomicStores) error) error {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)

		queries := sqlc.New(tx)

		s := AtomicStores{
			MonitorStore:  NewPostgresMonitorStore(queries),
			ForecastStore: NewPostgresForecastStore(queries),
			Outbox:        NewPostgresNotificationOutbox(queries),
		}
		if err := fn(s); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
}
