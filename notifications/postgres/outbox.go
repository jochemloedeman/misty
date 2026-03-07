package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jochemloedeman/misty/db/sqlc"
	"github.com/jochemloedeman/misty/notifications"
)

func toDomainNotification(row sqlc.Notification) notifications.Notification {
	return notifications.Notification{
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

func (o *NotificationOutbox) Create(ctx context.Context, notif notifications.Notification) (notifications.Notification, error) {
	params := sqlc.CreateNotificationParams{
		ID:          dbUUID(notif.ID),
		RecipientID: dbUUID(notif.RecipientID),
		Message:     notif.Message,
		SentAt:      dbTime(notif.SentAt),
	}
	row, err := o.queries.CreateNotification(ctx, params)
	if err != nil {
		return notifications.Notification{}, fmt.Errorf("failed to create notification: %w", err)
	}
	return toDomainNotification(row), nil
}

func (o *NotificationOutbox) ListUnsent(ctx context.Context) ([]notifications.Notification, error) {
	rows, err := o.queries.ListUnsentNotifications(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list unsent notifications: %w", err)
	}
	notifs := make([]notifications.Notification, len(rows))
	for i, row := range rows {
		notifs[i] = toDomainNotification(row)
	}
	return notifs, nil
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
