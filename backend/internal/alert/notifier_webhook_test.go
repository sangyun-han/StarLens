package alert

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func sampleAlert() Alert {
	return Alert{
		Key:      "routine_load_paused|shop.orders_load",
		RuleID:   "routine_load_paused",
		Severity: SeverityWarning,
		Title:    `Routine load job "shop.orders_load" is PAUSED`,
		Message:  "too many filtered rows",
		Labels:   map[string]string{"database": "shop", "job": "orders_load"},
		FiredAt:  time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC),
	}
}

func TestWebhookGenericPayload(t *testing.T) {
	var (
		gotBody        []byte
		gotContentType string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		gotContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	n, err := NewWebhookNotifier(server.URL, WebhookFormatGeneric)
	if err != nil {
		t.Fatalf("NewWebhookNotifier() error = %v", err)
	}
	if err := n.Send(context.Background(), sampleAlert()); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q", gotContentType)
	}

	var payload struct {
		Source string `json:"source"`
		Alert  Alert  `json:"alert"`
	}
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("decode payload: %v (body: %s)", err, gotBody)
	}
	if payload.Source != "starlens" {
		t.Errorf("source = %q", payload.Source)
	}
	if payload.Alert.Key != sampleAlert().Key || payload.Alert.Severity != SeverityWarning {
		t.Errorf("alert = %+v", payload.Alert)
	}
	if payload.Alert.Labels["job"] != "orders_load" {
		t.Errorf("labels = %v", payload.Alert.Labels)
	}
}

func TestWebhookSlackPayload(t *testing.T) {
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	n, err := NewWebhookNotifier(server.URL, WebhookFormatSlack)
	if err != nil {
		t.Fatalf("NewWebhookNotifier() error = %v", err)
	}
	if err := n.Send(context.Background(), sampleAlert()); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	var payload map[string]string
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	text := payload["text"]
	if text == "" {
		t.Fatal("slack payload must carry a text field")
	}
	for _, want := range []string{"[WARNING]", "shop.orders_load", "too many filtered rows", "job=orders_load"} {
		if !strings.Contains(text, want) {
			t.Errorf("text %q missing %q", text, want)
		}
	}
}

func TestWebhookNon2xxIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "no", http.StatusForbidden)
	}))
	defer server.Close()

	n, _ := NewWebhookNotifier(server.URL, WebhookFormatGeneric)
	if err := n.Send(context.Background(), sampleAlert()); err == nil {
		t.Error("Send() = nil, want error on 403")
	}
}

func TestWebhookRejectsBadConfig(t *testing.T) {
	if _, err := NewWebhookNotifier("", WebhookFormatGeneric); err == nil {
		t.Error("empty URL must be rejected")
	}
	if _, err := NewWebhookNotifier("http://x", "teams"); err == nil {
		t.Error("unknown format must be rejected")
	}
}
