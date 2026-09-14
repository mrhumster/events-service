//go:generate mockgen -source=event_repository.go -destination=./mock/event_repository_mock.go -package=repomock

package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/mrhumster/events-service/internal/domain/models"
)

type EventRepository interface {
	Insert(ctx context.Context, event *models.ActivityEvent) error
	ListByUser(ctx context.Context, userID uuid.UUID, limit int, before *time.Time) ([]*models.ActivityEvent, error)
	UnreadCount(ctx context.Context, userID uuid.UUID) (int64, error)
	MarkRead(ctx context.Context, userID, eventID uuid.UUID) error
	MarkAllRead(ctx context.Context, userID uuid.UUID) error
}