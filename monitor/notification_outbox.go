package monitor

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jochemloedeman/misty/monitor/sqlc"
)

func toDomainNotification(row sqlc.Notification) Notification {
	return Notification{
		ID:          uuid.UUID(row.ID.Bytes),
		RecipientID: uuid.UUID(row.RecipientID.Bytes),
		Message:     row.Message,
		SentAt:      row.SentAt.Time,
	}
}

type PostgresNotificationOutbox struct {
	queries *sqlc.Queries
}

func NewPostgresNotificationOutbox(queries *sqlc.Queries) *PostgresNotificationOutbox {
	return &PostgresNotificationOutbox{queries: queries}
}

func (o *PostgresNotificationOutbox) Create(ctx context.Context, notif Notification) (Notification, error) {
	params := sqlc.CreateNotificationParams{
		ID:          dbUUID(notif.ID),
		RecipientID: dbUUID(notif.RecipientID),
		Message:     notif.Message,
		SentAt:      dbTime(notif.SentAt),
	}
	row, err := o.queries.CreateNotification(ctx, params)
	if err != nil {
		return Notification{}, fmt.Errorf("failed to create notification: %w", err)
	}
	return toDomainNotification(row), nil
}

func (o *PostgresNotificationOutbox) ListUnsent(ctx context.Context) ([]Notification, error) {
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
