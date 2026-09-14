package queue_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/mrhumster/events-service/internal/domain/models"
	"github.com/mrhumster/events-service/internal/queue"
	"github.com/mrhumster/events-service/internal/service"
	servicemock "github.com/mrhumster/events-service/internal/service/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func testTask(t *testing.T, p queue.ActivityEventPayload) *asynq.Task {
	t.Helper()
	task, err := queue.NewActivityEventTask(p)
	require.NoError(t, err)
	return task
}

func TestHandleActivityEventTask_Persists(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := servicemock.NewMockEventsService(ctrl)
	h := queue.NewHandleActivityEvent(svc)

	eventID := uuid.New()
	userID := uuid.New()
	streamID := uuid.New()
	payload := queue.ActivityEventPayload{
		EventID:    eventID,
		UserID:     userID,
		EventType:  models.EventTranscodeCompleted,
		StreamID:   &streamID,
		Payload:    json.RawMessage(`{"progress":100}`),
		OccurredAt: time.Now().UTC(),
	}

	svc.EXPECT().Record(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, e *models.ActivityEvent) error {
			assert.Equal(t, eventID, e.EventID)
			assert.Equal(t, userID, e.UserID)
			assert.Equal(t, models.EventTranscodeCompleted, e.EventType)
			assert.NotNil(t, e.StreamID)
			assert.JSONEq(t, `{"progress":100}`, string(e.Payload))
			return nil
		})

	require.NoError(t, h.HandleActivityEventTask(ctx, testTask(t, payload)))
}

func TestHandleActivityEventTask_GeneratesEventIDWhenMissing(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := servicemock.NewMockEventsService(ctrl)
	h := queue.NewHandleActivityEvent(svc)

	payload := queue.ActivityEventPayload{
		UserID:    uuid.New(),
		EventType: models.EventUserLogin,
	}

	svc.EXPECT().Record(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, e *models.ActivityEvent) error {
			assert.NotEqual(t, uuid.Nil, e.EventID)
			return nil
		})

	require.NoError(t, h.HandleActivityEventTask(ctx, testTask(t, payload)))
}

func TestHandleActivityEventTask_BadPayloadSkips(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := servicemock.NewMockEventsService(ctrl)
	h := queue.NewHandleActivityEvent(svc)

	task := asynq.NewTask(queue.TaskActivityEvent, []byte("{not-json"))
	err := h.HandleActivityEventTask(ctx, task)
	require.Error(t, err)
	assert.ErrorIs(t, err, asynq.SkipRetry)
}

func TestHandleActivityEventTask_UnknownTypeSkips(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := servicemock.NewMockEventsService(ctrl)
	h := queue.NewHandleActivityEvent(svc)

	svc.EXPECT().Record(gomock.Any(), gomock.Any()).Return(service.ErrInvalidEvent)

	payload := queue.ActivityEventPayload{
		EventID:   uuid.New(),
		UserID:    uuid.New(),
		EventType: "bogus.type",
	}
	err := h.HandleActivityEventTask(ctx, testTask(t, payload))
	require.Error(t, err)
	assert.ErrorIs(t, err, asynq.SkipRetry)
}