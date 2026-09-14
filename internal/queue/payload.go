package queue

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// Task/queue names mirror the producer contract in identity-service and
// stream-service. Tasks are consumed from the asynq DB (REDIS_QUEUE_DB,
// default 3).
const (
	TaskActivityEvent = "event:activity"
	TaskActivityQueue = "events"
)

// ActivityEventPayload is the wire format producers enqueue for a single
// activity record. EventID is producer-generated (uuid) so replays are
// deduplicated by the consumer (unique index on activity_events.event_id).
type ActivityEventPayload struct {
	EventID    uuid.UUID       `json:"event_id"`
	UserID     uuid.UUID       `json:"user_id"`
	EventType  string          `json:"event_type"`
	StreamID   *uuid.UUID      `json:"stream_id,omitempty"`
	Payload    json.RawMessage `json:"payload"`
	OccurredAt time.Time       `json:"occurred_at"`
}

func NewActivityEventTask(p ActivityEventPayload) (*asynq.Task, error) {
	body, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TaskActivityEvent, body), nil
}