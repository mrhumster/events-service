//go:generate mockgen -source=events_service.go -destination=./mock/events_service_mock.go -package=servicemock

package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/mrhumster/events-service/internal/domain/models"
)

var ErrInvalidEvent = errors.New("invalid event")

// EventsService backs both the ingest path (Record, used by the asynq worker)
// and the read path (Feed/Unread/Read/ReadAll, used by the REST reader).
type EventsService interface {
	Record(ctx context.Context, event *models.ActivityEvent) error
	Feed(ctx context.Context, userID uuid.UUID, limit int, before *time.Time) ([]*models.ActivityEvent, error)
	Unread(ctx context.Context, userID uuid.UUID) (int64, error)
	Read(ctx context.Context, userID, eventID uuid.UUID) error
	ReadAll(ctx context.Context, userID uuid.UUID) error
}