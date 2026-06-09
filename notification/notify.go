package notification

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("github.com/jochemloedeman/misty/notification")

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

func NewNotifier(
	outbox outbox,
	deliver deliver,
	now func() time.Time,
) *Notifier {
	return &Notifier{
		outbox:  outbox,
		deliver: deliver,
		now:     now,
	}
}

func (n *Notifier) Notify(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "notify")
	defer span.End()

	notifications, err := n.outbox.ListUnsent(ctx)
	if err != nil {
		return err
	}
	span.SetAttributes(attribute.Int("notification.count", len(notifications)))

	slog.DebugContext(
		ctx,
		"delivering notifications",
		"count",
		len(notifications),
	)

	var errs []error
	for _, notif := range notifications {
		if err := n.deliverOne(ctx, notif); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (n *Notifier) deliverOne(ctx context.Context, notif Fog) (err error) {
	ctx, span := tracer.Start(ctx, "notify.deliver", trace.WithAttributes(
		attribute.String("notification.id", notif.ID.String()),
		attribute.String("recipient.id", notif.RecipientID.String()),
	))
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	if err := n.deliver(ctx, notif); err != nil {
		return fmt.Errorf("deliver notification %s: %w", notif.ID, err)
	}
	if err := n.outbox.MarkSent(ctx, notif.ID, n.now()); err != nil {
		return fmt.Errorf("mark notification %s as sent: %w", notif.ID, err)
	}

	slog.InfoContext(
		ctx,
		"notification delivered",
		"notification_id",
		notif.ID,
		"recipient_id",
		notif.RecipientID,
	)
	return nil
}
