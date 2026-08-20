package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sangyun-han/StarLens/backend/internal/model"
	"github.com/sangyun-han/StarLens/backend/internal/service"
)

func storageRouter(storage storageReader) http.Handler {
	return newTestRouterWithStorage(storage)
}

func TestStorageStatisticRoute(t *testing.T) {
	router := storageRouter(stubStorageService{statistic: model.StorageStatistic{
		Databases: []model.DatabaseStatistic{
			{Database: "demo", TableNum: 2, TabletNum: 3, UnhealthyTabletNum: 1},
		},
		Totals: model.DatabaseStatistic{TableNum: 2, TabletNum: 3, UnhealthyTabletNum: 1},
	}})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/storage/statistic", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var got model.StorageStatistic
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Totals.UnhealthyTabletNum != 1 || len(got.Databases) != 1 {
		t.Errorf("statistic = %+v", got)
	}
}

func TestStorageTablesRequiresDatabase(t *testing.T) {
	router := storageRouter(stubStorageService{})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/storage/tables", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 without ?database=", rec.Code)
	}
	var body errorBody
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Error.Code != "invalid_request" {
		t.Errorf("code = %q", body.Error.Code)
	}
}

func TestStorageTablesRoute(t *testing.T) {
	rows := int64(10)
	router := storageRouter(stubStorageService{list: model.TableList{
		Database: "demo",
		Tables:   []model.TableSummary{{Database: "demo", Name: "orders", ID: "11004", Rows: &rows}},
	}})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/storage/tables?database=demo", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got model.TableList
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(got.Tables) != 1 || got.Tables[0].Name != "orders" {
		t.Errorf("tables = %+v", got.Tables)
	}
}

func TestStorageTableDetailRoute(t *testing.T) {
	ratio := 400.0
	router := storageRouter(stubStorageService{detail: model.TableDetail{
		Table:       model.TableSummary{Database: "demo", Name: "skewed"},
		TabletTotal: 4, RowsetTotal: 16, SegmentTotal: 3, MaxRowsetsPerTablet: 9,
		Skew: model.TabletSkew{
			AcrossTablets: model.SkewMeasure{RowsRatio: &ratio, Skewed: true},
			Skewed:        true,
		},
	}})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/storage/tables/demo/skewed", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got model.TableDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if !got.Skew.Skewed || got.Skew.AcrossTablets.RowsRatio == nil || *got.Skew.AcrossTablets.RowsRatio != 400 {
		t.Errorf("skew did not round-trip: %+v", got.Skew)
	}
	if got.MaxRowsetsPerTablet != 9 {
		t.Errorf("MaxRowsetsPerTablet = %d", got.MaxRowsetsPerTablet)
	}
}

func TestStorageErrorMapping(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"missing table", fmt.Errorf("%w: no such table", service.ErrNotFound), http.StatusNotFound},
		{"cluster down", fmt.Errorf("%w: refused", service.ErrUnavailable), http.StatusServiceUnavailable},
		{"unexpected", errors.New("boom"), http.StatusInternalServerError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router := storageRouter(stubStorageService{err: tc.err})
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/storage/statistic", nil))

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
		})
	}
}
