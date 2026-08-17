package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

// pinger reports whether the StarRocks connection pool can reach a frontend.
type pinger interface {
	Ping(ctx context.Context) error
	Addr() string
}

// HealthHandler exposes liveness and StarRocks-reachability probes.
type HealthHandler struct {
	db pinger
}

// NewHealthHandler wires the probe to the connection pool.
func NewHealthHandler(db pinger) *HealthHandler {
	return &HealthHandler{db: db}
}

// Live handles GET /healthz — is the API process itself serving?
func (h *HealthHandler) Live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Ready handles GET /api/v1/health — can the API reach StarRocks right now?
func (h *HealthHandler) Ready(c *gin.Context) {
	if err := h.db.Ping(c.Request.Context()); err != nil {
		respondError(c, http.StatusServiceUnavailable, "starrocks_unreachable",
			"StarRocks is not reachable at "+h.db.Addr()+".", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"starrocks": gin.H{"addr": h.db.Addr(), "reachable": true},
	})
}
