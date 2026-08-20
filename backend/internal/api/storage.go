package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sangyun-han/StarLens/backend/internal/model"
	"github.com/sangyun-han/StarLens/backend/internal/service"
)

// storageReader is the service capability this controller needs.
type storageReader interface {
	Statistic(ctx context.Context) (model.StorageStatistic, error)
	Tables(ctx context.Context, database string) (model.TableList, error)
	TableDetail(ctx context.Context, database, table string) (model.TableDetail, error)
}

// StorageHandler serves catalog, partition and tablet observability.
type StorageHandler struct {
	storage storageReader
}

// NewStorageHandler wires the controller to its service.
func NewStorageHandler(storage storageReader) *StorageHandler {
	return &StorageHandler{storage: storage}
}

// register mounts storage routes on the /api/v1 group.
func (h *StorageHandler) register(v1 *gin.RouterGroup) {
	v1.GET("/storage/statistic", h.Statistic)
	v1.GET("/storage/tables", h.Tables)
	v1.GET("/storage/tables/:database/:table", h.TableDetail)
}

// Statistic handles GET /api/v1/storage/statistic — per-database catalog counts
// and tablet health (unhealthy, inconsistent, cloning, error-state).
func (h *StorageHandler) Statistic(c *gin.Context) {
	statistic, err := h.storage.Statistic(c.Request.Context())
	if err != nil {
		respondStorageError(c, err, "Could not read cluster statistics from StarRocks.")
		return
	}
	c.JSON(http.StatusOK, statistic)
}

// Tables handles GET /api/v1/storage/tables?database=<db>.
func (h *StorageHandler) Tables(c *gin.Context) {
	list, err := h.storage.Tables(c.Request.Context(), c.Query("database"))
	if err != nil {
		respondStorageError(c, err, "Could not list tables from StarRocks.")
		return
	}
	c.JSON(http.StatusOK, list)
}

// TableDetail handles GET /api/v1/storage/tables/:database/:table — partitions,
// per-backend tablet distribution, rowset/segment counts and the skew ratio.
func (h *StorageHandler) TableDetail(c *gin.Context) {
	detail, err := h.storage.TableDetail(c.Request.Context(), c.Param("database"), c.Param("table"))
	if err != nil {
		respondStorageError(c, err, "Could not read table detail from StarRocks.")
		return
	}
	c.JSON(http.StatusOK, detail)
}

// respondStorageError maps service errors to the status the caller can act on:
// a caller mistake is 400, a missing table is 404, and a cluster that cannot
// answer is 503.
func respondStorageError(c *gin.Context, err error, message string) {
	switch {
	case errors.Is(err, service.ErrInvalidArgument):
		respondError(c, http.StatusBadRequest, "invalid_request",
			"The request is missing a required parameter.", err)
	case errors.Is(err, service.ErrNotFound):
		respondError(c, http.StatusNotFound, "not_found",
			"The requested catalog object does not exist.", err)
	case errors.Is(err, service.ErrUnavailable):
		respondError(c, http.StatusServiceUnavailable, "starrocks_unavailable", message, err)
	default:
		respondError(c, http.StatusInternalServerError, "internal_error", message, err)
	}
}
