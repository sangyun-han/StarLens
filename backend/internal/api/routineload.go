package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sangyun-han/StarLens/backend/internal/model"
	"github.com/sangyun-han/StarLens/backend/internal/service"
)

// routineLoadSnapshotter is the service capability this controller needs.
type routineLoadSnapshotter interface {
	Snapshot(ctx context.Context) (model.RoutineLoadSnapshot, error)
}

// RoutineLoadHandler serves streaming ingestion monitoring endpoints.
type RoutineLoadHandler struct {
	loads routineLoadSnapshotter
}

// NewRoutineLoadHandler wires the controller to its service.
func NewRoutineLoadHandler(loads routineLoadSnapshotter) *RoutineLoadHandler {
	return &RoutineLoadHandler{loads: loads}
}

// register mounts routine load routes on the /api/v1 group.
func (h *RoutineLoadHandler) register(v1 *gin.RouterGroup) {
	v1.GET("/loads/routine", h.Snapshot)
}

// Snapshot handles GET /api/v1/loads/routine.
//
// It returns every routine load job across all databases with state, ingestion
// statistics and (when the StarRocks version reports source positions) an
// approximate offset lag.
func (h *RoutineLoadHandler) Snapshot(c *gin.Context) {
	snapshot, err := h.loads.Snapshot(c.Request.Context())
	if err != nil {
		if errors.Is(err, service.ErrUnavailable) {
			respondError(c, http.StatusServiceUnavailable, "starrocks_unavailable",
				"Could not read routine load jobs from StarRocks.", err)
			return
		}
		respondError(c, http.StatusInternalServerError, "internal_error",
			"Failed to build routine load snapshot.", err)
		return
	}

	c.JSON(http.StatusOK, snapshot)
}
