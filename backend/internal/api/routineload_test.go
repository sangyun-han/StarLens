package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sangyun-han/StarLens/backend/internal/alert"
	"github.com/sangyun-han/StarLens/backend/internal/model"
	"github.com/sangyun-han/StarLens/backend/internal/service"
)

func sampleRoutineLoadSnapshot() model.RoutineLoadSnapshot {
	lag := int64(1100)
	return model.RoutineLoadSnapshot{
		CollectedAt: time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC),
		Source:      "information_schema",
		Summary: model.RoutineLoadSummary{
			Total: 2, Running: 1, Paused: 1, Unhealthy: 1, TotalErrorRows: 150,
		},
		Jobs: []model.RoutineLoadJob{
			{
				Name: "clicks_load", Database: "web", Table: "clicks",
				State:                model.RoutineLoadStatePaused,
				ReasonOfStateChanged: "too many filtered rows",
			},
			{
				Name: "orders_load", Database: "shop", Table: "orders",
				State: model.RoutineLoadStateRunning, OffsetLag: &lag,
				Statistics: &model.RoutineLoadStatistics{TotalRows: 10000, ErrorRows: 150},
			},
		},
	}
}

func TestRoutineLoadSnapshotRoute(t *testing.T) {
	router := newTestRouterFull(
		stubClusterService{}, stubPinger{},
		stubRoutineLoadService{snapshot: sampleRoutineLoadSnapshot()}, stubAlertStore{}, stubQueryService{},
	)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/loads/routine", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	var got model.RoutineLoadSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Summary.Unhealthy != 1 || len(got.Jobs) != 2 {
		t.Errorf("snapshot = %+v", got)
	}
	if got.Jobs[1].OffsetLag == nil || *got.Jobs[1].OffsetLag != 1100 {
		t.Errorf("offsetLag round-trip failed: %+v", got.Jobs[1])
	}
}

func TestRoutineLoadUnavailableReturns503(t *testing.T) {
	cause := fmt.Errorf("%w: reading routine load jobs failed", service.ErrUnavailable)
	router := newTestRouterFull(
		stubClusterService{}, stubPinger{},
		stubRoutineLoadService{err: cause}, stubAlertStore{}, stubQueryService{},
	)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/loads/routine", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestAlertRoutes(t *testing.T) {
	store := stubAlertStore{
		recent: []alert.Alert{{
			Key: "routine_load_paused|web.clicks_load", RuleID: "routine_load_paused",
			Severity: alert.SeverityWarning, Title: "paused",
		}},
		results: map[string]string{"log": "ok"},
	}
	router := newTestRouterFull(stubClusterService{}, stubPinger{}, stubRoutineLoadService{}, store, stubQueryService{})

	t.Run("recent", func(t *testing.T) {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/alerts", nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var body struct {
			Alerts []alert.Alert `json:"alerts"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if len(body.Alerts) != 1 || body.Alerts[0].RuleID != "routine_load_paused" {
			t.Errorf("alerts = %+v", body.Alerts)
		}
	})

	t.Run("test fire", func(t *testing.T) {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/alerts/test", nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var body struct {
			Results map[string]string `json:"results"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Results["log"] != "ok" {
			t.Errorf("results = %v", body.Results)
		}
	})

	t.Run("empty history is [] not null", func(t *testing.T) {
		emptyRouter := newTestRouterFull(stubClusterService{}, stubPinger{}, stubRoutineLoadService{}, stubAlertStore{}, stubQueryService{})
		rec := httptest.NewRecorder()
		emptyRouter.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/alerts", nil))

		var raw map[string]json.RawMessage
		if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if string(raw["alerts"]) == "null" {
			t.Error("alerts must serialize as [], not null")
		}
	})
}
