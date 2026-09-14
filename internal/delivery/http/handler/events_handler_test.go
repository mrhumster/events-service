package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mrhumster/events-service/internal/delivery/http/handler"
	"github.com/mrhumster/events-service/internal/delivery/http/middleware"
	"github.com/mrhumster/events-service/internal/domain/models"
	servicemock "github.com/mrhumster/events-service/internal/service/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/datatypes"
)

func userCtx(userID uuid.UUID) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(middleware.KeyUserID, userID)
		c.Next()
	}
}

func TestFeed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := servicemock.NewMockEventsService(ctrl)
	userID := uuid.New()
	now := time.Now().UTC()

	events := []*models.ActivityEvent{
		{
			ID:        uuid.New(),
			EventID:   uuid.New(),
			UserID:    userID,
			EventType: models.EventUploadCompleted,
			Payload:   datatypes.JSON(`{}`),
			CreatedAt: now,
		},
	}
	svc.EXPECT().Feed(gomock.Any(), userID, 50, nil).Return(events, nil)

	r := gin.New()
	r.GET("/events", func(c *gin.Context) {
		c.Set(middleware.KeyUserID, userID)
		handler.NewEventsHandler(svc).Feed(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/events", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Contains(t, body, "events")
	// feed is not full (1 < default limit) => no pagination cursor
	assert.NotContains(t, body, "next_cursor")
}

func TestFeed_InvalidLimit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := servicemock.NewMockEventsService(ctrl)
	r := gin.New()
	r.GET("/events", func(c *gin.Context) {
		c.Set(middleware.KeyUserID, uuid.New())
		handler.NewEventsHandler(svc).Feed(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/events?limit=abc", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFeed_InvalidCursor(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := servicemock.NewMockEventsService(ctrl)
	r := gin.New()
	r.GET("/events", func(c *gin.Context) {
		c.Set(middleware.KeyUserID, uuid.New())
		handler.NewEventsHandler(svc).Feed(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/events?cursor=yesterday", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUnreadCount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := servicemock.NewMockEventsService(ctrl)
	userID := uuid.New()
	svc.EXPECT().Unread(gomock.Any(), userID).Return(int64(2), nil)

	r := gin.New()
	r.GET("/events/unread-count", func(c *gin.Context) {
		c.Set(middleware.KeyUserID, userID)
		handler.NewEventsHandler(svc).UnreadCount(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/events/unread-count", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"count":2}`, w.Body.String())
}

func TestRead_InvalidID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := servicemock.NewMockEventsService(ctrl)
	r := gin.New()
	r.POST("/events/:id/read", func(c *gin.Context) {
		c.Set(middleware.KeyUserID, uuid.New())
		handler.NewEventsHandler(svc).Read(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/events/not-a-uuid/read", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRead_Valid(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := servicemock.NewMockEventsService(ctrl)
	userID := uuid.New()
	eventID := uuid.New()
	svc.EXPECT().Read(gomock.Any(), userID, eventID).Return(nil)

	r := gin.New()
	r.POST("/events/:id/read", func(c *gin.Context) {
		c.Set(middleware.KeyUserID, userID)
		handler.NewEventsHandler(svc).Read(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/events/"+eventID.String()+"/read", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"ok":true}`, w.Body.String())
}

func TestReadAll(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := servicemock.NewMockEventsService(ctrl)
	userID := uuid.New()
	svc.EXPECT().ReadAll(gomock.Any(), userID).Return(nil)

	r := gin.New()
	r.POST("/events/read-all", func(c *gin.Context) {
		c.Set(middleware.KeyUserID, userID)
		handler.NewEventsHandler(svc).ReadAll(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/events/read-all", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}