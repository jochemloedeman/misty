package apple

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/jochemloedeman/misty/notification"
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/payload"
)

type TokenResolver interface {
	PushToken(ctx context.Context, userID uuid.UUID) (string, error)
	ClearPushToken(ctx context.Context, userID uuid.UUID, token string) error
}

type Pusher interface {
	PushWithContext(ctx apns2.Context, n *apns2.Notification) (*apns2.Response, error)
}

func NewDeliverer(
	client Pusher,
	tokens TokenResolver,
	topic string,
) func(context.Context, notification.Fog) error {
	return func(ctx context.Context, notif notification.Fog) error {
		deviceToken, err := tokens.PushToken(ctx, notif.RecipientID)
		if err != nil {
			return fmt.Errorf("resolve push token for %s: %w", notif.RecipientID, err)
		}
		if deviceToken == "" {
			slog.WarnContext(ctx, "no push token for user, skipping", "recipient_id", notif.RecipientID)
			return nil
		}

		p := payload.NewPayload().
			Alert(notif.Message).
			Sound("default").
			MutableContent().
			Custom("location_name", notif.LocationName).
			Custom("fog_start", notif.FogStart.Unix()).
			Custom("fog_end", notif.FogEnd.Unix())

		resp, err := client.PushWithContext(ctx, &apns2.Notification{
			DeviceToken: deviceToken,
			Topic:       topic,
			Payload:     p,
		})
		if err != nil {
			return fmt.Errorf("apns push: %w", err)
		}

		// push token invalidated by Apple. clear it.
		if resp.StatusCode == http.StatusGone {
			slog.WarnContext(ctx, "apns token unregistered, clearing",
				"recipient_id", notif.RecipientID, "notification_id", notif.ID)
			if err := tokens.ClearPushToken(ctx, notif.RecipientID, deviceToken); err != nil {
				return fmt.Errorf("clear unregistered token for %s: %w", notif.RecipientID, err)
			}
			return nil
		}
		if !resp.Sent() {
			return fmt.Errorf("apns rejected notification %s: %d %s", notif.ID, resp.StatusCode, resp.Reason)
		}

		return nil
	}
}
