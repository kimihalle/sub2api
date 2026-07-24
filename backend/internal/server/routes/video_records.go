package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterVideoRecordRoutes(v1 *gin.RouterGroup, h *handler.Handlers, jwtAuth middleware.JWTAuthMiddleware) {
	group := v1.Group("/video-records")
	group.Use(gin.HandlerFunc(jwtAuth))
	{
		group.GET("", h.OpenAIGateway.SanbaoVideoLogs)
	}
}
