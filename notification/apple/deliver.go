package apple

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/payload"

	"github.com/jochemloedeman/misty/notification"
)

// TokenResolver looks up an APNs device token for a given user ID.
type TokenResolver interface {
	PushToken(ctx context.Context, userID uuid.UUID) (string, error)
}

// NewDeliverer returns a deliver function that sends notifications via APNs.
func NewDeliverer(
	client *apns2.Client,
	tokens TokenResolver,
	topic string,
) func(context.Context, notification.Fog) error {
	return func(ctx context.Context, notif notification.Fog) error {
		deviceToken, err := tokens.PushToken(ctx, notif.RecipientID)
		if err != nil {
			return fmt.Errorf("resolve push token for %s: %w", notif.RecipientID, err)
		}
		if deviceToken == "" {
			slog.Warn("no push token for user, skipping",
				"recipient_id", notif.RecipientID,
			)
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
		if !resp.Sent() {
			return fmt.Errorf("apns rejected notification %s: %d %s",
				notif.ID, resp.StatusCode, resp.Reason,
			)
		}

		return nil
	}
}
