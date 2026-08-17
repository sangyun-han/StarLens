package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sangyun-han/StarLens/backend/internal/alert"
)

// alertStore is the slice of the alert.Manager this controller needs.
type alertStore interface {
	Recent() []alert.Alert
	Test(ctx context.Context) (alert.Alert, map[string]string)
}

// AlertHandler exposes the in-memory alert history and a test-fire endpoint.
type AlertHandler struct {
	alerts alertStore
}

// NewAlertHandler wires the controller to the alert manager.
func NewAlertHandler(alerts alertStore) *AlertHandler {
	return &AlertHandler{alerts: alerts}
}

// register mounts alert routes on the /api/v1 group.
func (h *AlertHandler) register(v1 *gin.RouterGroup) {
	v1.GET("/alerts", h.Recent)
	v1.POST("/alerts/test", h.Fire)
}

// Recent handles GET /api/v1/alerts — the fired-alert history, newest first.
// History is in-memory and bounded; long-term storage belongs to the receiving
// channel (Slack, webhook consumer, log pipeline).
func (h *AlertHandler) Recent(c *gin.Context) {
	recent := h.alerts.Recent()
	if recent == nil {
		recent = []alert.Alert{}
	}
	c.JSON(http.StatusOK, gin.H{"alerts": recent})
}

// Fire handles POST /api/v1/alerts/test — pushes a synthetic alert through
// every configured notifier so operators can verify a channel end to end.
func (h *AlertHandler) Fire(c *gin.Context) {
	fired, results := h.alerts.Test(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{
		"alert": fired,
		// Per-notifier outcome: "ok" or the delivery error string.
		"results": results,
	})
}
