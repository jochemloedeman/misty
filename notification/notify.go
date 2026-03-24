package notification

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

type outbox interface {
	ListUnsent(ctx context.Context) ([]Notification, error)
	MarkSent(ctx context.Context, id uuid.UUID, sentAt time.Time) error
}

type deliver func(context.Context, Notification) error

type Notifier struct {
	outbox  outbox
	deliver func(context.Context, Notification) error
	now     func() time.Time
}

func NewNotifier(outbox outbox, deliver deliver, now func() time.Time) *Notifier {
	return &Notifier{
		outbox:  outbox,
		deliver: deliver,
		now:     now,
	}
}

func (n *Notifier) Notify(ctx context.Context) error {
	notifications, err := n.outbox.ListUnsent(ctx)
	if err != nil {
		return err
	}

	slog.Debug("delivering notifications", "count", len(notifications))

	for _, notif := range notifications {
		if err := n.deliver(ctx, notif); err != nil {
			return fmt.Errorf("deliver notification %s: %w", notif.ID, err)
		}
		if err := n.outbox.MarkSent(ctx, notif.ID, n.now()); err != nil {
			return fmt.Errorf("mark notification %s as sent: %w", notif.ID, err)
		}
		slog.Info(
			"notification delivered",
			"notification_id",
			notif.ID,
			"recipient_id",
			notif.RecipientID,
		)
	}

	return nil
}
