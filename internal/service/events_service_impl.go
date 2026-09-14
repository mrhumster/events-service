package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/mrhumster/events-service/internal/domain/models"
	"github.com/mrhumster/events-service/internal/repository"
)

type EventsServiceImpl struct {
	repo repository.EventRepository
}

func NewEventsServiceImpl(repo repository.EventRepository) *EventsServiceImpl {
	return &EventsServiceImpl{repo: repo}
}

// Record validates and persists a single activity event. Events produced by
// clients must carry a unique EventID and UserID; anything else is rejected
// so a malformed enqueued task cannot poison the feed.
func (s *EventsServiceImpl) Record(ctx context.Context, event *models.ActivityEvent) error {
	if event == nil {
		return fmt.Errorf("%w: nil event", ErrInvalidEvent)
	}
	if event.EventID == uuid.Nil {
		return fmt.Errorf("%w: event_id is required", ErrInvalidEvent)
	}
	if event.UserID == uuid.Nil {
		return fmt.Errorf("%w: user_id is required", ErrInvalidEvent)
	}
	if !models.ValidEventType(event.EventType) {
		return fmt.Errorf("%w: unknown event_type %q", ErrInvalidEvent, event.EventType)
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	if len(event.Payload) == 0 {
		event.Payload = []byte("{}")
	}
	return s.repo.Insert(ctx, event)
}

func (s *EventsServiceImpl) Feed(ctx context.Context, userID uuid.UUID, limit int, before *time.Time) ([]*models.ActivityEvent, error) {
	if userID == uuid.Nil {
		return nil, fmt.Errorf("%w: user_id is required", ErrInvalidEvent)
	}
	return s.repo.ListByUser(ctx, userID, limit, before)
}

func (s *EventsServiceImpl) Unread(ctx context.Context, userID uuid.UUID) (int64, error) {
	return s.repo.UnreadCount(ctx, userID)
}

func (s *EventsServiceImpl) Read(ctx context.Context, userID, eventID uuid.UUID) error {
	return s.repo.MarkRead(ctx, userID, eventID)
}

func (s *EventsServiceImpl) ReadAll(ctx context.Context, userID uuid.UUID) error {
	return s.repo.MarkAllRead(ctx, userID)
}