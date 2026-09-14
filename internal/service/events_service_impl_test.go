package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mrhumster/events-service/internal/domain/models"
	repomock "github.com/mrhumster/events-service/internal/repository/mock"
	"github.com/mrhumster/events-service/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/datatypes"
)

func newSvc(t *testing.T, repo *repomock.MockEventRepository) service.EventsService {
	t.Helper()
	return service.NewEventsServiceImpl(repo)
}

func validEvent() *models.ActivityEvent {
	return &models.ActivityEvent{
		EventID:   uuid.New(),
		UserID:    uuid.New(),
		EventType: models.EventUploadCompleted,
		StreamID:  func() *uuid.UUID { id := uuid.New(); return &id }(),
		Payload:   datatypes.JSON(`{"size": 123}`),
	}
}

func TestRecord_ValidEvent(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := repomock.NewMockEventRepository(ctrl)
	svc := newSvc(t, repo)

	ev := validEvent()
	repo.EXPECT().Insert(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, e *models.ActivityEvent) error {
			assert.NotZero(t, e.CreatedAt)
			return nil
		})

	require.NoError(t, svc.Record(ctx, ev))
}

func TestRecord_DefaultsCreatedAtAndPayload(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := repomock.NewMockEventRepository(ctrl)
	svc := newSvc(t, repo)

	ev := validEvent()
	ev.Payload = nil
	repo.EXPECT().Insert(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, e *models.ActivityEvent) error {
			assert.False(t, e.CreatedAt.IsZero())
			assert.JSONEq(t, `{}`, string(e.Payload))
			return nil
		})

	require.NoError(t, svc.Record(ctx, ev))
}

func TestRecord_RejectsInvalid(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := repomock.NewMockEventRepository(ctrl)
	svc := newSvc(t, repo)

	cases := []struct {
		name string
		ev   *models.ActivityEvent
	}{
		{"nil event", nil},
		{"empty event_id", &models.ActivityEvent{UserID: uuid.New(), EventType: models.EventUserLogin}},
		{"empty user_id", &models.ActivityEvent{EventID: uuid.New(), EventType: models.EventUserLogin}},
		{"unknown type", &models.ActivityEvent{EventID: uuid.New(), UserID: uuid.New(), EventType: "nope"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.Record(ctx, tc.ev)
			require.Error(t, err)
			assert.True(t, errors.Is(err, service.ErrInvalidEvent))
		})
	}
}

func TestFeed_FiltersByUser(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := repomock.NewMockEventRepository(ctrl)
	svc := newSvc(t, repo)

	userID := uuid.New()
	expected := []*models.ActivityEvent{validEvent()}

	repo.EXPECT().
		ListByUser(gomock.Any(), userID, 10, nil).
		Return(expected, nil)

	got, err := svc.Feed(ctx, userID, 10, nil)
	require.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestFeed_RejectsNilUser(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := repomock.NewMockEventRepository(ctrl)
	svc := newSvc(t, repo)

	_, err := svc.Feed(ctx, uuid.Nil, 10, nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, service.ErrInvalidEvent))
}

func TestFeed_WithCursor(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := repomock.NewMockEventRepository(ctrl)
	svc := newSvc(t, repo)

	userID := uuid.New()
	before := time.Now().Add(-time.Hour)
	repo.EXPECT().ListByUser(gomock.Any(), userID, 20, &before).Return(nil, nil)

	_, err := svc.Feed(ctx, userID, 20, &before)
	require.NoError(t, err)
}

func TestUnread_Read_ReadAll(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := repomock.NewMockEventRepository(ctrl)
	svc := newSvc(t, repo)

	userID := uuid.New()
	eventID := uuid.New()

	repo.EXPECT().UnreadCount(gomock.Any(), userID).Return(int64(3), nil)
	repo.EXPECT().MarkRead(gomock.Any(), userID, eventID).Return(nil)
	repo.EXPECT().MarkAllRead(gomock.Any(), userID).Return(nil)

	c, err := svc.Unread(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int64(3), c)

	require.NoError(t, svc.Read(ctx, userID, eventID))
	require.NoError(t, svc.ReadAll(ctx, userID))
}