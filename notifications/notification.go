package notifications

import (
	"time"

	"github.com/google/uuid"
)

type Notification struct {
	ID          uuid.UUID
	RecipientID uuid.UUID
	Message     string
	SentAt      time.Time
}

func NewNotification(recipientID uuid.UUID, message string) Notification {
	return Notification{
		ID:          uuid.New(),
		RecipientID: recipientID,
		Message:     message,
	}
}
