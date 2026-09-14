package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mrhumster/events-service/internal/domain/models"
	"github.com/mrhumster/events-service/internal/delivery/http/middleware"
	"github.com/mrhumster/events-service/internal/service"
)

const defaultFeedLimit = 50

type EventsHandler struct {
	svc service.EventsService
}

func NewEventsHandler(svc service.EventsService) *EventsHandler {
	return &EventsHandler{svc: svc}
}

// Feed returns the current user's activity feed, newest first.
//   - limit  (default 50, max 100)
//   - cursor — RFC3339 created_at; returns events older than it
type FeedResponse struct {
	Events     []*models.ActivityEvent `json:"events"`
	NextCursor *time.Time              `json:"next_cursor,omitempty"`
}

func (h *EventsHandler) Feed(c *gin.Context) {
	userID := middleware.UserID(c)

	limit := defaultFeedLimit
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}
		limit = n
	}

	var before *time.Time
	if raw := c.Query("cursor"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cursor"})
			return
		}
		before = &t
	}

	events, err := h.svc.Feed(c.Request.Context(), userID, limit, before)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	resp := FeedResponse{Events: events}
	if len(events) > 0 && len(events) >= limit {
		t := events[len(events)-1].CreatedAt
		resp.NextCursor = &t
	}
	c.JSON(http.StatusOK, resp)
}

func (h *EventsHandler) UnreadCount(c *gin.Context) {
	userID := middleware.UserID(c)

	count, err := h.svc.Unread(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": count})
}

func (h *EventsHandler) Read(c *gin.Context) {
	userID := middleware.UserID(c)

	eventID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid event id"})
		return
	}

	if err := h.svc.Read(c.Request.Context(), userID, eventID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *EventsHandler) ReadAll(c *gin.Context) {
	userID := middleware.UserID(c)

	if err := h.svc.ReadAll(c.Request.Context(), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}