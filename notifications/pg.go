package notifications

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jochemloedeman/misty/db/sqlc"
)

func dbTime(ts time.Time) pgtype.Timestamptz {
	if ts.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: ts, Valid: true}
}

func dbUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func toDomainNotification(row sqlc.Notification) Notification {
	return Notification{
		ID:          uuid.UUID(row.ID.Bytes),
		RecipientID: uuid.UUID(row.RecipientID.Bytes),
		Message:     row.Message,
		SentAt:      row.SentAt.Time,
	}
}

// pgOutbox implements the outbox pattern for notifications backed by PostgreSQL.
type pgOutbox struct {
	queries *sqlc.Queries
}

func NewOutbox(queries *sqlc.Queries) *pgOutbox {
	return &pgOutbox{queries: queries}
}

func (o *pgOutbox) Create(
	ctx context.Context,
	notif Notification,
) (Notification, error) {
	params := sqlc.CreateNotificationParams{
		ID:          dbUUID(notif.ID),
		RecipientID: dbUUID(notif.RecipientID),
		Message:     notif.Message,
		SentAt:      dbTime(notif.SentAt),
	}
	row, err := o.queries.CreateNotification(ctx, params)
	if err != nil {
		return Notification{}, fmt.Errorf(
			"failed to create notification: %w",
			err,
		)
	}
	return toDomainNotification(row), nil
}

func (o *pgOutbox) ListUnsent(ctx context.Context) ([]Notification, error) {
	rows, err := o.queries.ListUnsentNotifications(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list unsent notifications: %w", err)
	}
	notifs := make([]Notification, len(rows))
	for i, row := range rows {
		notifs[i] = toDomainNotification(row)
	}
	return notifs, nil
}
