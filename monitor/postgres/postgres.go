package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jochemloedeman/misty/monitor"
	"github.com/jochemloedeman/misty/monitor/postgres/sqlc"
)

func NewRunAtomically(pool *pgxpool.Pool) monitor.RunAtomically {
	return func(ctx context.Context, fn func(s monitor.AtomicStores) error) error {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback(ctx) }()

		queries := sqlc.New(tx)

		s := monitor.AtomicStores{
			MonitorStore:  NewMonitorStore(queries),
			ForecastStore: NewForecastStore(queries),
			Outbox:        NewNotificationOutbox(queries),
		}
		if err := fn(s); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
}

func dbTime(ts time.Time) pgtype.Timestamptz {
	if ts.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: ts, Valid: true}
}

func dbUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}
