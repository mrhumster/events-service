package routes

import (
	"log"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/mrhumster/events-service/config"
	"github.com/mrhumster/events-service/internal/delivery/http/handler"
	"github.com/mrhumster/events-service/internal/delivery/http/middleware"
	"github.com/mrhumster/events-service/internal/service"
	"gorm.io/gorm"
)

// SetupRoutes builds the REST reader: an authenticated read API for the
// activity feed plus health/metrics endpoints.
func SetupRoutes(db *gorm.DB, cfg *config.Config, svc service.EventsService, tokens *service.TokenService) *gin.Engine {
	if cfg.Server.Mode == "test" {
		gin.SetMode(gin.TestMode)
	} else if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.MetricsMiddleware())
	r.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.Server.AllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	h := handler.NewEventsHandler(svc)

	authed := r.Group("/events", middleware.AuthMiddleware(tokens))
	{
		authed.GET("", h.Feed)
		authed.GET("/unread-count", h.UnreadCount)
		authed.POST("/:id/read", h.Read)
		authed.POST("/read-all", h.ReadAll)
	}

	r.GET("/health", func(c *gin.Context) {
		if sqlDB, err := db.DB(); err == nil {
			if err := sqlDB.Ping(); err != nil {
				log.Println("⚠️ PG ERROR: ", err.Error())
				c.JSON(http.StatusServiceUnavailable, gin.H{"status": "down", "error": err.Error()})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"status": "up"})
	})

	return r
}