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
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

var (
	tracer = otel.Tracer("github.com/jochemloedeman/misty/notification")
	meter  = otel.Meter("github.com/jochemloedeman/misty/notification")
)

// ErrUndeliverable signals that a notification reached no device — the recipient
// has no push token, or Apple reported the token as unregistered. It is terminal,
// not a transient failure: retrying will not help until a new token is registered.
var ErrUndeliverable = errors.New("no deliverable push token")

type outbox interface {
	ListUnsent(ctx context.Context) ([]Fog, error)
	Find(ctx context.Context, id uuid.UUID) (Fog, bool, error)
	MarkSent(ctx context.Context, id uuid.UUID, sentAt time.Time) error
	MarkExpired(ctx context.Context, id uuid.UUID) error
	MarkUndeliverable(ctx context.Context, id uuid.UUID) error
}

type deliver func(context.Context, Fog) error

type deliveryOutcome string

const (
	outcomeDelivered     deliveryOutcome = "delivered"
	outcomeExpired       deliveryOutcome = "expired"
	outcomeFailed        deliveryOutcome = "failed"
	outcomeUndeliverable deliveryOutcome = "undeliverable"
)

type metrics struct {
	delivered metric.Int64Counter
}

func newMetrics() (*metrics, error) {
	delivered, err := meter.Int64Counter(
		"notifications.delivered",
		metric.WithDescription("Number of notification delivery attempts"),
		metric.WithUnit("{notification}"),
	)
	return &metrics{delivered: delivered}, err
}

type Notifier struct {
	outbox  outbox
	deliver func(context.Context, Fog) error
	now     func() time.Time
	metrics *metrics
}

func NewNotifier(
	outbox outbox,
	deliver deliver,
	now func() time.Time,
) (*Notifier, error) {
	m, err := newMetrics()
	if err != nil {
		return nil, fmt.Errorf("create notification metrics: %w", err)
	}
	return &Notifier{
		outbox:  outbox,
		deliver: deliver,
		now:     now,
		metrics: m,
	}, nil
}

func (n *Notifier) Notify(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "notify.all")
	defer span.End()

	notifications, err := n.outbox.ListUnsent(ctx)
	if err != nil {
		return err
	}
	span.SetAttributes(attribute.Int("notification.count", len(notifications)))

	slog.DebugContext(ctx, "delivering notifications", "count", len(notifications))

	var errs []error
	for _, notif := range notifications {
		if err := n.deliverOne(ctx, notif); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (n *Notifier) NotifyOne(ctx context.Context, id uuid.UUID) error {
	notif, ok, err := n.outbox.Find(ctx, id)
	if err != nil {
		return err
	}
	if !ok {
		slog.DebugContext(ctx, "notification already delivered, skipping", "notification_id", id)
		return nil
	}
	return n.deliverOne(ctx, notif)
}

func (n *Notifier) deliverOne(ctx context.Context, notif Fog) (err error) {
	ctx, span := tracer.Start(ctx, "notify.deliver", trace.WithAttributes(
		attribute.String("notification.id", notif.ID.String()),
		attribute.String("recipient.id", notif.RecipientID.String()),
	))
	outcome := outcomeDelivered
	defer func() {
		if err != nil {
			outcome = outcomeFailed
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		n.metrics.delivered.Add(ctx, 1, metric.WithAttributes(
			attribute.String("notification.outcome", string(outcome)),
		))
		span.End()
	}()

	if !notif.FogEnd.IsZero() && notif.FogEnd.Before(n.now()) {
		outcome = outcomeExpired
		slog.InfoContext(ctx, "expiring notification past its fog window",
			"notification_id", notif.ID, "recipient_id", notif.RecipientID, "fog_end", notif.FogEnd)
		if err := n.outbox.MarkExpired(ctx, notif.ID); err != nil {
			return fmt.Errorf("mark notification %s expired: %w", notif.ID, err)
		}
		return nil
	}

	if err := n.deliver(ctx, notif); err != nil {
		if errors.Is(err, ErrUndeliverable) {
			outcome = outcomeUndeliverable
			slog.InfoContext(ctx, "notification undeliverable, no push token",
				"notification_id", notif.ID, "recipient_id", notif.RecipientID)
			if err := n.outbox.MarkUndeliverable(ctx, notif.ID); err != nil {
				return fmt.Errorf("mark notification %s undeliverable: %w", notif.ID, err)
			}
			return nil
		}
		return fmt.Errorf("deliver notification %s: %w", notif.ID, err)
	}
	if err := n.outbox.MarkSent(ctx, notif.ID, n.now()); err != nil {
		return fmt.Errorf("mark notification %s as sent: %w", notif.ID, err)
	}

	slog.InfoContext(ctx, "notification delivered", "notification_id", notif.ID, "recipient_id", notif.RecipientID)
	return nil
}
