package notification

import (
	"time"

	"github.com/google/uuid"
)

type Fog struct {
	ID           uuid.UUID
	RecipientID  uuid.UUID
	Message      string
	LocationName string
	FogStart     time.Time
	FogEnd       time.Time
	SentAt       time.Time
}

func New(
	recipientID uuid.UUID,
	message, locationName string,
	fogStart, fogEnd time.Time,
) Fog {
	return Fog{
		ID:           uuid.New(),
		RecipientID:  recipientID,
		Message:      message,
		LocationName: locationName,
		FogStart:     fogStart,
		FogEnd:       fogEnd,
	}
}
