package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// Event types recorded into the activity feed.
const (
	EventStreamCreated        = "stream.created"
	EventUploadStarted        = "stream.upload.started"
	EventUploadCompleted      = "stream.upload.completed"
	EventTranscodeStarted     = "stream.transcode.started"
	EventTranscodeCompleted   = "stream.transcode.finish"
	EventTranscodeFailed      = "stream.transcode.failed"
	EventStreamReady          = "stream.ready"
	EventStreamPublished      = "stream.published"
	EventStreamUnpublished    = "stream.unpublished"
	EventStreamDeleted        = "stream.deleted"
	EventStreamReprocessed    = "stream.reprocessed"
	EventUserRegistered       = "user.registered"
	EventUserLogin            = "user.login"
	EventEmailVerified        = "user.email.verified"
	EventCommentCreated       = "comment.created"
	EventCommentReplied       = "comment.replied"
	EventReactionLiked        = "reaction.liked"
	EventReactionDisliked     = "reaction.disliked"
)

// ValidEventType reports whether the given string is a known event type.
func ValidEventType(t string) bool {
	switch t {
	case EventStreamCreated, EventUploadStarted, EventUploadCompleted,
		EventTranscodeStarted, EventTranscodeCompleted, EventTranscodeFailed,
		EventStreamReady, EventStreamPublished, EventStreamUnpublished,
		EventStreamDeleted, EventStreamReprocessed,
		EventUserRegistered, EventUserLogin, EventEmailVerified,
		EventCommentCreated, EventCommentReplied,
		EventReactionLiked, EventReactionDisliked:
		return true
	}
	return false
}

// ActivityEvent is a single user-visible activity record. EventID is a
// producer-generated unique identifier used for idempotency on re-delivery.
type ActivityEvent struct {
	ID        uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	EventID   uuid.UUID      `json:"event_id" gorm:"type:uuid;not null;uniqueIndex"`
	UserID    uuid.UUID      `json:"user_id" gorm:"type:uuid;not null;index"`
	EventType string         `json:"event_type" gorm:"type:text;not null"`
	StreamID  *uuid.UUID     `json:"stream_id,omitempty" gorm:"type:uuid"`
	Payload   datatypes.JSON `json:"payload" gorm:"type:jsonb;not null;default:'{}'"`
	ReadAt    *time.Time     `json:"read_at,omitempty"`
	CreatedAt time.Time      `json:"created_at" gorm:"not null"`
}

func (ActivityEvent) TableName() string { return "activity_events" }

// PayloadMap returns the payload as a raw map, or an empty map when absent.
func (e *ActivityEvent) PayloadMap() map[string]any {
	out := map[string]any{}
	if len(e.Payload) == 0 {
		return out
	}
	_ = json.Unmarshal(e.Payload, &out)
	return out
}