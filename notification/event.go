package notification

import "github.com/google/uuid"

// Queued reports that a notification was persisted to the outbox and is ready
// for delivery.
type Queued struct {
	ID uuid.UUID
}
