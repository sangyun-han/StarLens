// Package api contains the Gin HTTP layer: routing, request validation and
// translation of service errors into status codes. It holds no business logic.
package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sangyun-han/StarLens/backend/internal/model"
	"github.com/sangyun-han/StarLens/backend/internal/service"
)

// topologyReader is the service capability this controller needs.
type topologyReader interface {
	Topology(ctx context.Context) (model.Topology, error)
}

// ClusterHandler serves cluster-level observability endpoints.
type ClusterHandler struct {
	cluster topologyReader
}

// NewClusterHandler wires the controller to its service.
func NewClusterHandler(cluster topologyReader) *ClusterHandler {
	return &ClusterHandler{cluster: cluster}
}

// register mounts cluster routes on the /api/v1 group.
func (h *ClusterHandler) register(v1 *gin.RouterGroup) {
	v1.GET("/cluster/topology", h.Topology)
	// Shorthand alias kept because the dashboard spec refers to both paths.
	v1.GET("/topology", h.Topology)
}

// Topology handles GET /api/v1/cluster/topology.
//
// It returns FE and BE membership with liveness, roles and per-backend capacity.
// A cluster that cannot be reached is a 503: the dashboard is up, the cluster
// is not, and the frontend should retry rather than surface a bug.
func (h *ClusterHandler) Topology(c *gin.Context) {
	topology, err := h.cluster.Topology(c.Request.Context())
	if err != nil {
		if errors.Is(err, service.ErrUnavailable) {
			respondError(c, http.StatusServiceUnavailable, "starrocks_unavailable",
				"Could not read cluster topology from StarRocks.", err)
			return
		}
		respondError(c, http.StatusInternalServerError, "internal_error",
			"Failed to build cluster topology.", err)
		return
	}

	c.JSON(http.StatusOK, topology)
}
