package entity

import (
	"time"

	"github.com/google/uuid"
)

type Outbox struct {
	ObjectID    uuid.UUID
	ObjectType  string
	EventType   string
	Payload     string
	CreatedAt   time.Time
	PublishedAt time.Time
}
