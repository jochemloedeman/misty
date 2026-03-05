package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jochemloedeman/misty/monitor"
	"github.com/jochemloedeman/misty/db/sqlc"
)

func toDomainNotification(row sqlc.Notification) monitor.Notification {
	return monitor.Notification{
		ID:          uuid.UUID(row.ID.Bytes),
		RecipientID: uuid.UUID(row.RecipientID.Bytes),
		Message:     row.Message,
		SentAt:      row.SentAt.Time,
	}
}

type NotificationOutbox struct {
	queries *sqlc.Queries
}

func NewNotificationOutbox(queries *sqlc.Queries) *NotificationOutbox {
	return &NotificationOutbox{queries: queries}
}

func (o *NotificationOutbox) Create(ctx context.Context, notif monitor.Notification) (monitor.Notification, error) {
	params := sqlc.CreateNotificationParams{
		ID:          dbUUID(notif.ID),
		RecipientID: dbUUID(notif.RecipientID),
		Message:     notif.Message,
		SentAt:      dbTime(notif.SentAt),
	}
	row, err := o.queries.CreateNotification(ctx, params)
	if err != nil {
		return monitor.Notification{}, fmt.Errorf("failed to create notification: %w", err)
	}
	return toDomainNotification(row), nil
}

func (o *NotificationOutbox) ListUnsent(ctx context.Context) ([]monitor.Notification, error) {
	rows, err := o.queries.ListUnsentNotifications(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list unsent notifications: %w", err)
	}
	notifs := make([]monitor.Notification, len(rows))
	for i, row := range rows {
		notifs[i] = toDomainNotification(row)
	}
	return notifs, nil
}
