package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/sangyun-han/StarLens/backend/config"
	"github.com/sangyun-han/StarLens/backend/internal/alert"
	"github.com/sangyun-han/StarLens/backend/internal/model"
	"github.com/sangyun-han/StarLens/backend/internal/service"
)

type stubClusterService struct {
	topology model.Topology
	err      error
}

func (s stubClusterService) Topology(context.Context) (model.Topology, error) {
	return s.topology, s.err
}

type stubPinger struct{ err error }

func (s stubPinger) Ping(context.Context) error { return s.err }
func (s stubPinger) Addr() string               { return "127.0.0.1:9030" }

type stubRoutineLoadService struct {
	snapshot model.RoutineLoadSnapshot
	err      error
}

func (s stubRoutineLoadService) Snapshot(context.Context) (model.RoutineLoadSnapshot, error) {
	return s.snapshot, s.err
}

type stubAlertStore struct {
	recent  []alert.Alert
	results map[string]string
}

func (s stubAlertStore) Recent() []alert.Alert { return s.recent }
func (s stubAlertStore) Test(context.Context) (alert.Alert, map[string]string) {
	return alert.Alert{RuleID: "test", Key: "test"}, s.results
}

type stubQueryService struct {
	result    model.QueryResult
	databases []string
	err       error
}

func (s stubQueryService) Execute(context.Context, model.QueryRequest) (model.QueryResult, error) {
	return s.result, s.err
}

func (s stubQueryService) Databases(context.Context) ([]string, error) {
	return s.databases, s.err
}

func newTestRouter(cluster topologyReader, db pinger) *gin.Engine {
	return newTestRouterFull(cluster, db, stubRoutineLoadService{}, stubAlertStore{}, stubQueryService{})
}

func newTestRouterFull(cluster topologyReader, db pinger, loads routineLoadSnapshotter, alerts alertStore, queries queryExecutor) *gin.Engine {
	gin.SetMode(gin.TestMode)
	return Router(
		config.ServerConfig{GinMode: gin.TestMode, AllowedOrigins: []string{"http://localhost:5173"}},
		NewHealthHandler(db),
		NewClusterHandler(cluster),
		NewRoutineLoadHandler(loads),
		NewAlertHandler(alerts),
		NewQueryHandler(queries),
	)
}

func sampleTopology() model.Topology {
	tabletNum := int64(1024)
	return model.Topology{
		CollectedAt: time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC),
		Summary: model.TopologySummary{
			FrontendTotal: 1, FrontendAlive: 1,
			BackendTotal: 1, BackendAlive: 1,
			LeaderHost: "10.0.0.1", TabletTotal: tabletNum, Healthy: true,
		},
		Frontends: []model.Node{{
			ID: "fe:fe1", Name: "fe1", Type: model.NodeTypeFrontend,
			Role: model.RoleLeader, Status: model.StatusHealthy, Alive: true, Host: "10.0.0.1",
		}},
		Backends: []model.Node{{
			ID: "be:10001", Name: "10.0.0.2", Type: model.NodeTypeBackend,
			Role: model.RoleBackend, Status: model.StatusHealthy, Alive: true, Host: "10.0.0.2",
			TabletNum: &tabletNum,
		}},
	}
}

// Both the spec path and the shorthand alias must serve the same payload.
func TestTopologyRoutes(t *testing.T) {
	for _, path := range []string{"/api/v1/cluster/topology", "/api/v1/topology"} {
		t.Run(path, func(t *testing.T) {
			router := newTestRouter(stubClusterService{topology: sampleTopology()}, stubPinger{})

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
			}

			var got model.Topology
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode body: %v (body: %s)", err, rec.Body.String())
			}
			if len(got.Frontends) != 1 || got.Frontends[0].Role != model.RoleLeader {
				t.Errorf("frontends = %+v", got.Frontends)
			}
			if !got.Summary.Healthy || got.Summary.TabletTotal != 1024 {
				t.Errorf("summary = %+v", got.Summary)
			}
		})
	}
}

// An unreachable cluster is a 503 with a machine-readable code, not a 500.
func TestTopologyUnavailableReturns503(t *testing.T) {
	cause := fmt.Errorf("%w: SHOW FRONTENDS failed: connection refused", service.ErrUnavailable)
	router := newTestRouter(stubClusterService{err: cause}, stubPinger{})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/cluster/topology", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (body: %s)", rec.Code, rec.Body.String())
	}

	var body errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error.Code != "starrocks_unavailable" {
		t.Errorf("error.code = %q, want starrocks_unavailable", body.Error.Code)
	}
	if body.Error.Detail == "" {
		t.Error("error.detail should carry the driver cause for operators")
	}
}

func TestTopologyUnexpectedErrorReturns500(t *testing.T) {
	router := newTestRouter(stubClusterService{err: errors.New("boom")}, stubPinger{})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/cluster/topology", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestHealthEndpoints(t *testing.T) {
	t.Run("liveness ignores StarRocks", func(t *testing.T) {
		router := newTestRouter(stubClusterService{}, stubPinger{err: errors.New("down")})

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200: the API is up even when the cluster is not", rec.Code)
		}
	})

	t.Run("readiness reflects StarRocks", func(t *testing.T) {
		router := newTestRouter(stubClusterService{}, stubPinger{err: errors.New("down")})

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want 503", rec.Code)
		}
	})
}

func TestCORSAllowsConfiguredOrigin(t *testing.T) {
	router := newTestRouter(stubClusterService{topology: sampleTopology()}, stubPinger{})

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/cluster/topology", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("preflight status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("Access-Control-Allow-Origin = %q", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/cluster/topology", nil)
	req.Header.Set("Origin", "http://evil.example.com")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("unlisted origin was allowed: %q", got)
	}
}

func TestUnknownRouteReturnsErrorEnvelope(t *testing.T) {
	router := newTestRouter(stubClusterService{}, stubPinger{})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/nope", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}

	var body errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error.Code != "not_found" {
		t.Errorf("error.code = %q, want not_found", body.Error.Code)
	}
}
