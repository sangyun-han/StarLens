package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sangyun-han/StarLens/backend/internal/model"
	"github.com/sangyun-han/StarLens/backend/internal/service"
)

func queryRouter(queries queryExecutor) http.Handler {
	return newTestRouterFull(stubClusterService{}, stubPinger{}, stubRoutineLoadService{}, stubAlertStore{}, queries)
}

func postQuery(t *testing.T, router http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/query", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestQueryExecuteRoute(t *testing.T) {
	router := queryRouter(stubQueryService{result: model.QueryResult{
		Columns:   []model.QueryColumn{{Name: "id", Type: "BIGINT"}},
		Rows:      [][]any{{"1"}, {nil}},
		RowCount:  2,
		MaxRows:   1000,
		ElapsedMs: 12,
		Statement: "SELECT id FROM t",
	}})

	rec := postQuery(t, router, `{"sql": "SELECT id FROM t"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	var got model.QueryResult
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.RowCount != 2 || got.Columns[0].Type != "BIGINT" {
		t.Errorf("result = %+v", got)
	}
	// NULL must round-trip as JSON null.
	if got.Rows[1][0] != nil {
		t.Errorf("rows[1][0] = %v, want nil", got.Rows[1][0])
	}
}

func TestQueryStatusMapping(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"blocked", fmt.Errorf("%w: insert", service.ErrQueryBlocked), http.StatusBadRequest, "query_blocked"},
		{"failed", fmt.Errorf("%w: syntax error", service.ErrQueryFailed), http.StatusBadRequest, "query_failed"},
		{"empty", service.ErrQueryEmpty, http.StatusBadRequest, "query_empty"},
		{"timeout", fmt.Errorf("%w: budget", service.ErrQueryTimeout), http.StatusGatewayTimeout, "query_timeout"},
		{"unavailable", fmt.Errorf("%w: refused", service.ErrUnavailable), http.StatusServiceUnavailable, "starrocks_unavailable"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := postQuery(t, queryRouter(stubQueryService{err: tc.err}), `{"sql": "SELECT 1"}`)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			var body errorBody
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body.Error.Code != tc.wantCode {
				t.Errorf("code = %q, want %q", body.Error.Code, tc.wantCode)
			}
		})
	}
}

func TestQueryRejectsMissingSQL(t *testing.T) {
	rec := postQuery(t, queryRouter(stubQueryService{}), `{"database": "shop"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestDatabasesRoute(t *testing.T) {
	router := queryRouter(stubQueryService{databases: []string{"shop", "web"}})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/databases", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Databases []string `json:"databases"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(body.Databases) != 2 || body.Databases[0] != "shop" {
		t.Errorf("databases = %v", body.Databases)
	}
}
