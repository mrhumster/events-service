package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
	"github.com/google/uuid"
	"github.com/mrhumster/events-service/internal/domain/models"
	"github.com/mrhumster/events-service/internal/metrics"
	"github.com/mrhumster/events-service/internal/service"
	"gorm.io/datatypes"
)

// HandleActivityEvent consumes event:activity tasks and persists them via
// EventsService. Malformed payloads are skipped permanently; persistence
// errors are retried by asynq.
type HandleActivityEvent struct {
	svc service.EventsService
}

func NewHandleActivityEvent(svc service.EventsService) *HandleActivityEvent {
	return &HandleActivityEvent{svc: svc}
}

func (h *HandleActivityEvent) HandleActivityEventTask(ctx context.Context, t *asynq.Task) error {
	var p ActivityEventPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		metrics.RecordedError()
		slog.Warn("event:bad payload", "error", err)
		return fmt.Errorf("unmarshal activity payload: %w", asynq.SkipRetry)
	}

	// Default the event ID when producers omit it. This weakens replay
	// dedupe, but keeps the record valid.
	eventID := p.EventID
	if eventID == uuid.Nil {
		eventID = uuid.New()
	}

	start := time.Now()
	err := h.svc.Record(ctx, &models.ActivityEvent{
		EventID:   eventID,
		UserID:    p.UserID,
		EventType: p.EventType,
		StreamID:  p.StreamID,
		Payload:   datatypes.JSON(p.Payload),
		CreatedAt: p.OccurredAt,
	})
	if err != nil {
		if errors.Is(err, service.ErrInvalidEvent) {
			metrics.RecordedError()
			slog.Warn("event:rejected", "error", err)
			return fmt.Errorf("reject activity event: %w", asynq.SkipRetry)
		}
		metrics.RecordedError()
		return fmt.Errorf("record activity event: %w", err)
	}

	metrics.RecordedSuccess()
	metrics.Duration.Observe(time.Since(start).Seconds())
	return nil
}