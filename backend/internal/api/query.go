package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sangyun-han/StarLens/backend/internal/model"
	"github.com/sangyun-han/StarLens/backend/internal/service"
)

// queryExecutor is the service capability this controller needs.
type queryExecutor interface {
	Execute(ctx context.Context, req model.QueryRequest) (model.QueryResult, error)
	Databases(ctx context.Context) ([]string, error)
}

// QueryHandler serves the SQL worksheet endpoints.
type QueryHandler struct {
	queries queryExecutor
}

// NewQueryHandler wires the controller to its service.
func NewQueryHandler(queries queryExecutor) *QueryHandler {
	return &QueryHandler{queries: queries}
}

// register mounts worksheet routes on the /api/v1 group.
func (h *QueryHandler) register(v1 *gin.RouterGroup) {
	v1.POST("/query", h.Execute)
	v1.GET("/databases", h.Databases)
}

// Databases handles GET /api/v1/databases — the worksheet's scope picker.
func (h *QueryHandler) Databases(c *gin.Context) {
	names, err := h.queries.Databases(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusServiceUnavailable, "starrocks_unavailable",
			"Could not list databases from StarRocks.", err)
		return
	}
	if names == nil {
		names = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"databases": names})
}

// Execute handles POST /api/v1/query.
//
// Status codes tell the frontend who is at fault: 400 for a statement StarRocks
// rejected or policy blocked (the message is the answer), 504 for a timeout,
// 503 when the cluster itself is unreachable.
func (h *QueryHandler) Execute(c *gin.Context) {
	var req model.QueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request",
			"Request body must be JSON with a non-empty \"sql\" field.", err)
		return
	}

	result, err := h.queries.Execute(c.Request.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrQueryEmpty):
			respondError(c, http.StatusBadRequest, "query_empty",
				"The statement is empty.", err)
		case errors.Is(err, service.ErrQueryBlocked):
			respondError(c, http.StatusBadRequest, "query_blocked",
				"Only read statements are allowed while QUERY_READ_ONLY=true.", err)
		case errors.Is(err, service.ErrQueryFailed):
			respondError(c, http.StatusBadRequest, "query_failed",
				"StarRocks rejected the statement.", err)
		case errors.Is(err, service.ErrQueryTimeout):
			respondError(c, http.StatusGatewayTimeout, "query_timeout",
				"The query did not finish within the server's time budget.", err)
		case errors.Is(err, service.ErrUnavailable):
			respondError(c, http.StatusServiceUnavailable, "starrocks_unavailable",
				"Could not execute the statement against StarRocks.", err)
		default:
			respondError(c, http.StatusInternalServerError, "internal_error",
				"Failed to execute the statement.", err)
		}
		return
	}

	c.JSON(http.StatusOK, result)
}
