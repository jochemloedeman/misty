package notifications

import (
	"context"
	"log/slog"
)

type outbox interface {
	ListUnsent(context.Context) ([]Notification, error)
}

type deliver func(context.Context, Notification) error

type Notifier struct {
	outbox  outbox
	deliver func(context.Context, Notification) error
}

func NewNotifier(outbox outbox, deliver deliver) *Notifier {
	return &Notifier{
		outbox:  outbox,
		deliver: deliver,
	}
}

func (n *Notifier) Notify(ctx context.Context) error {
	notifications, err := n.outbox.ListUnsent(ctx)
	if err != nil {
		return err
	}

	slog.Debug("delivering notifications", "count", len(notifications))

	for _, notification := range notifications {
		if err := n.deliver(ctx, notification); err != nil {
			return err
		}
		slog.Info(
			"notification delivered",
			"notification_id",
			notification.ID,
			"recipient_id",
			notification.RecipientID,
		)
	}

	return nil
}
