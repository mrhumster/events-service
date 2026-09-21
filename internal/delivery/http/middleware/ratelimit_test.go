package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mrhumster/events-service/internal/delivery/http/middleware"
	"github.com/stretchr/testify/assert"
)

// authedStub places a user id in the context the same way AuthMiddleware
// does, so RateLimitPerMin keys on the user rather than the client IP.
func authedStub(userID uuid.UUID) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(middleware.KeyUserID, userID)
		c.Set("claims", struct{}{})
		c.Next()
	}
}

func TestRateLimitPerMin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("allows up to limit then rejects per user", func(t *testing.T) {
		router := gin.New()
		userID := uuid.New()
		router.GET("/events", authedStub(userID), middleware.RateLimitPerMin(3), func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		for i := 0; i < 3; i++ {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/events", nil)
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code, "request %d should pass", i+1)
		}

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/events", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusTooManyRequests, w.Code)
	})

	t.Run("different users have independent budgets", func(t *testing.T) {
		router := gin.New()
		router.GET("/events", func(c *gin.Context) {
			uid := c.GetHeader("X-User")
			parsed, err := uuid.Parse(uid)
			if err != nil {
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			c.Set(middleware.KeyUserID, parsed)
			c.Set("claims", struct{}{})
			c.Next()
		}, middleware.RateLimitPerMin(1), func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		first := uuid.NewString()
		second := uuid.NewString()

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/events", nil)
		req.Header.Set("X-User", first)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/events", nil)
		req.Header.Set("X-User", first)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusTooManyRequests, w.Code)

		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/events", nil)
		req.Header.Set("X-User", second)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}