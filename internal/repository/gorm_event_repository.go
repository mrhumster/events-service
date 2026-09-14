package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/mrhumster/events-service/internal/domain/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	maxEventListLimit = 100
)

type GormEventRepository struct {
	db *gorm.DB
}

func NewGormEventRepository(db *gorm.DB) *GormEventRepository {
	return &GormEventRepository{db: db}
}

// Insert stores an event. Re-delivery of the same EventID is a no-op
// (ON CONFLICT DO NOTHING) which makes the ingest path idempotent.
func (r *GormEventRepository) Insert(ctx context.Context, event *models.ActivityEvent) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "event_id"}},
			DoNothing: true,
		}).
		Create(event).
		Error
}

func (r *GormEventRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit int, before *time.Time) ([]*models.ActivityEvent, error) {
	if limit <= 0 || limit > maxEventListLimit {
		limit = maxEventListLimit
	}

	q := r.db.WithContext(ctx).
		Where("user_id = ?", userID)
	if before != nil {
		q = q.Where("created_at < ?", *before)
	}

	var events []*models.ActivityEvent
	if err := q.Order("created_at DESC").
		Limit(limit).
		Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}

func (r *GormEventRepository) UnreadCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.ActivityEvent{}).
		Where("user_id = ? AND read_at IS NULL", userID).
		Count(&count).Error
	return count, err
}

func (r *GormEventRepository) MarkRead(ctx context.Context, userID, eventID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&models.ActivityEvent{}).
		Where("user_id = ? AND id = ?", userID, eventID).
		UpdateColumn("read_at", time.Now().UTC()).
		Error
}

func (r *GormEventRepository) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&models.ActivityEvent{}).
		Where("user_id = ? AND read_at IS NULL", userID).
		UpdateColumn("read_at", time.Now().UTC()).
		Error
}