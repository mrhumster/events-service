package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mrhumster/events-service/internal/service"
	"github.com/mrhumster/identity-service/pkg/dto"
)

const KeyUserID = "user_id"

// AuthMiddleware authenticates Bearer access tokens and places the user ID
// into the gin context.
func AuthMiddleware(tokens *service.TokenService) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.GetHeader("Authorization")
		token, ok := strings.CutPrefix(raw, "Bearer ")
		if !ok || token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "auth token required"})
			return
		}

		claims, err := tokens.ValidateAccessToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		userID, err := uuid.Parse(claims.UserID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
			return
		}

		c.Set(KeyUserID, userID)
		c.Set("claims", claims)
		c.Next()
	}
}

func UserID(c *gin.Context) uuid.UUID {
	v, _ := c.Get(KeyUserID)
	id, _ := v.(uuid.UUID)
	return id
}

func Claims(c *gin.Context) *dto.AccessClaims {
	v, _ := c.Get("claims")
	cl, _ := v.(*dto.AccessClaims)
	return cl
}