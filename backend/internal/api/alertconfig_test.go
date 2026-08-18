package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/sangyun-han/StarLens/backend/internal/alert"
)

func newConfigRouter(t *testing.T, defaults alert.Config, editable bool) (*gin.Engine, *int) {
	t.Helper()
	settings, err := alert.LoadSettings(filepath.Join(t.TempDir(), "alerts.json"), defaults)
	if err != nil {
		t.Fatal(err)
	}

	applied := 0
	handler := NewAlertConfigHandler(settings, func(alert.Config) error {
		applied++
		return nil
	}, editable)

	return newTestRouterWithConfig(
		stubClusterService{}, stubPinger{}, stubRoutineLoadService{}, stubAlertStore{}, stubQueryService{},
		handler,
	), &applied
}

func configDefaults() alert.Config {
	return alert.Config{
		Enabled:           true,
		PollInterval:      30 * time.Second,
		Cooldown:          10 * time.Minute,
		WebhookURL:        "https://hooks.slack.com/services/T0/B0/topsecret",
		WebhookFormat:     alert.WebhookFormatSlack,
		ErrorRowsRatio:    0.01,
		ErrorRowsMinTotal: 10_000,
	}
}

func TestAlertConfigGetMasksWebhook(t *testing.T) {
	router, _ := newConfigRouter(t, configDefaults(), true)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/alerts/config", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "topsecret") {
		t.Error("response leaked the webhook URL")
	}

	var view struct {
		Editable bool `json:"editable"`
		Config   struct {
			WebhookConfigured bool   `json:"webhookConfigured"`
			WebhookHint       string `json:"webhookHint"`
			PollInterval      string `json:"pollInterval"`
		} `json:"config"`
		Overridden []string `json:"overridden"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if !view.Editable || !view.Config.WebhookConfigured {
		t.Errorf("view = %+v", view)
	}
	if view.Config.WebhookHint != "https://hooks.slack.com/•••" {
		t.Errorf("webhookHint = %q", view.Config.WebhookHint)
	}
	if view.Config.PollInterval != "30s" {
		t.Errorf("pollInterval = %q", view.Config.PollInterval)
	}
	if view.Overridden == nil {
		t.Error("overridden must serialize as [], not null")
	}
}

func TestAlertConfigPutAppliesAndReturnsView(t *testing.T) {
	router, applied := newConfigRouter(t, configDefaults(), true)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/alerts/config",
		strings.NewReader(`{"cooldown": "5m", "errorRowsRatio": 0.05}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if *applied != 1 {
		t.Errorf("apply calls = %d, want 1", *applied)
	}

	var view struct {
		Config struct {
			Cooldown       string  `json:"cooldown"`
			ErrorRowsRatio float64 `json:"errorRowsRatio"`
		} `json:"config"`
		Overridden []string `json:"overridden"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if view.Config.Cooldown != "5m0s" || view.Config.ErrorRowsRatio != 0.05 {
		t.Errorf("config = %+v", view.Config)
	}
	if len(view.Overridden) != 2 {
		t.Errorf("overridden = %v, want the two patched fields", view.Overridden)
	}
}

func TestAlertConfigPutValidationErrorIs400(t *testing.T) {
	router, applied := newConfigRouter(t, configDefaults(), true)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/alerts/config",
		strings.NewReader(`{"webhookFormat": "teams"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var body errorBody
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Error.Code != "invalid_config" {
		t.Errorf("code = %q", body.Error.Code)
	}
	if *applied != 0 {
		t.Error("invalid patches must not be applied")
	}
}

func TestAlertConfigPutForbiddenWhenUIDisabled(t *testing.T) {
	router, applied := newConfigRouter(t, configDefaults(), false)

	// Reads stay available…
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/alerts/config", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200", rec.Code)
	}

	// …but writes are refused.
	req := httptest.NewRequest(http.MethodPut, "/api/v1/alerts/config",
		strings.NewReader(`{"cooldown": "1m"}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("PUT status = %d, want 403", rec.Code)
	}
	var body errorBody
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Error.Code != "config_ui_disabled" {
		t.Errorf("code = %q", body.Error.Code)
	}
	if *applied != 0 {
		t.Error("disabled UI must never apply changes")
	}
}

func TestAlertConfigPutReset(t *testing.T) {
	router, _ := newConfigRouter(t, configDefaults(), true)

	put := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/alerts/config", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	put(`{"cooldown": "5m"}`)
	rec := put(`{"reset": true}`)

	var view struct {
		Config     struct{ Cooldown string }
		Overridden []string `json:"overridden"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(view.Overridden) != 0 {
		t.Errorf("overridden after reset = %v, want empty", view.Overridden)
	}
}
