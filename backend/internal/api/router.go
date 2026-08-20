package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sangyun-han/StarLens/backend/config"
)

// Handlers groups every controller the router mounts.
type Handlers struct {
	Health      *HealthHandler
	Cluster     *ClusterHandler
	Loads       *RoutineLoadHandler
	Alerts      *AlertHandler
	AlertConfig *AlertConfigHandler
	Storage     *StorageHandler
	Queries     *QueryHandler
}

// Router builds the HTTP surface of the StarLens API.
//
// Every product endpoint lives under /api/v1 so the SPA can be served from the
// same origin in production without path collisions.
func Router(cfg config.ServerConfig, handlers Handlers) *gin.Engine {
	gin.SetMode(ginMode(cfg.GinMode))

	engine := gin.New()
	engine.Use(gin.Logger(), gin.Recovery(), cors(cfg.AllowedOrigins))

	engine.GET("/healthz", handlers.Health.Live)

	v1 := engine.Group("/api/v1")
	v1.GET("/health", handlers.Health.Ready)
	handlers.Cluster.register(v1)
	handlers.Loads.register(v1)
	handlers.Alerts.register(v1)
	handlers.AlertConfig.register(v1)
	handlers.Queries.register(v1)
	handlers.Storage.register(v1)

	engine.NoRoute(func(c *gin.Context) {
		respondError(c, http.StatusNotFound, "not_found",
			"No route matches "+c.Request.Method+" "+c.Request.URL.Path+".", nil)
	})

	return engine
}

func ginMode(mode string) string {
	switch mode {
	case gin.ReleaseMode, gin.TestMode:
		return mode
	default:
		return gin.DebugMode
	}
}
