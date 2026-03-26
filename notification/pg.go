package notification

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

func toFog(row sqlc.Notification) Fog {
	return Fog{
		ID:           uuid.UUID(row.ID.Bytes),
		RecipientID:  uuid.UUID(row.RecipientID.Bytes),
		Message:      row.Message,
		LocationName: row.LocationName,
		FogStart:     row.FogStart.Time,
		FogEnd:       row.FogEnd.Time,
		SentAt:       row.SentAt.Time,
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
	notif Fog,
) (Fog, error) {
	params := sqlc.CreateNotificationParams{
		ID:           dbUUID(notif.ID),
		RecipientID:  dbUUID(notif.RecipientID),
		Message:      notif.Message,
		LocationName: notif.LocationName,
		FogStart:     dbTime(notif.FogStart),
		FogEnd:       dbTime(notif.FogEnd),
		SentAt:       dbTime(notif.SentAt),
	}
	row, err := o.queries.CreateNotification(ctx, params)
	if err != nil {
		return Fog{}, fmt.Errorf(
			"failed to create notification: %w",
			err,
		)
	}
	return toFog(row), nil
}

func (o *pgOutbox) ListUnsent(ctx context.Context) ([]Fog, error) {
	rows, err := o.queries.ListUnsentNotifications(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list unsent notifications: %w", err)
	}
	notifs := make([]Fog, len(rows))
	for i, row := range rows {
		notifs[i] = toFog(row)
	}
	return notifs, nil
}

func (o *pgOutbox) MarkSent(ctx context.Context, id uuid.UUID, sentAt time.Time) error {
	_, err := o.queries.UpdateNotificationSentAt(ctx, sqlc.UpdateNotificationSentAtParams{
		ID:     dbUUID(id),
		SentAt: dbTime(sentAt),
	})
	if err != nil {
		return fmt.Errorf("mark notification sent: %w", err)
	}
	return nil
}
