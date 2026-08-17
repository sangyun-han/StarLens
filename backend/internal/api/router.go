package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sangyun-han/StarLens/backend/config"
)

// Router builds the HTTP surface of the StarLens API.
//
// Every product endpoint lives under /api/v1 so the SPA can be served from the
// same origin in production without path collisions.
func Router(
	cfg config.ServerConfig,
	health *HealthHandler,
	cluster *ClusterHandler,
	loads *RoutineLoadHandler,
	alerts *AlertHandler,
) *gin.Engine {
	gin.SetMode(ginMode(cfg.GinMode))

	engine := gin.New()
	engine.Use(gin.Logger(), gin.Recovery(), cors(cfg.AllowedOrigins))

	engine.GET("/healthz", health.Live)

	v1 := engine.Group("/api/v1")
	v1.GET("/health", health.Ready)
	cluster.register(v1)
	loads.register(v1)
	alerts.register(v1)

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
