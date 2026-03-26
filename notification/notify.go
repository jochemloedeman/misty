package notification

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

type outbox interface {
	ListUnsent(ctx context.Context) ([]Fog, error)
	MarkSent(ctx context.Context, id uuid.UUID, sentAt time.Time) error
}

type deliver func(context.Context, Fog) error

type Notifier struct {
	outbox  outbox
	deliver func(context.Context, Fog) error
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

	var errs []error
	for _, notif := range notifications {
		if err := n.deliver(ctx, notif); err != nil {
			errs = append(errs, fmt.Errorf("deliver notification %s: %w", notif.ID, err))
			continue
		}
		if err := n.outbox.MarkSent(ctx, notif.ID, n.now()); err != nil {
			errs = append(errs, fmt.Errorf("mark notification %s as sent: %w", notif.ID, err))
			continue
		}
		slog.Info(
			"notification delivered",
			"notification_id",
			notif.ID,
			"recipient_id",
			notif.RecipientID,
		)
	}

	return errors.Join(errs...)
}
