package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// RegisterImageWorkbenchRoutes registers the user-facing image generation workbench.
func RegisterImageWorkbenchRoutes(v1 *gin.RouterGroup, apiKeyService *service.APIKeyService, jwtAuth middleware.JWTAuthMiddleware, cfg *config.Config) {
	h := handler.NewImageWorkbenchHandler(apiKeyService, cfg)
	group := v1.Group("/image-workbench")
	group.GET("/images/:log_id/:index", h.ImageContent)
	group.Use(gin.HandlerFunc(jwtAuth))
	{
		group.POST("/generations", h.Submit)
		group.GET("/generations/:task_id", h.PollTask)
		group.GET("/logs", h.Logs)
	}
}
