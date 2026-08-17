package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Webhook payload formats.
const (
	// WebhookFormatGeneric posts the Alert struct as JSON — for Alertmanager-style
	// receivers, n8n, or custom glue.
	WebhookFormatGeneric = "generic"
	// WebhookFormatSlack posts {"text": ...}, the shape Slack (and compatible
	// receivers such as Mattermost and Discord's /slack endpoint) expect.
	WebhookFormatSlack = "slack"
)

// WebhookNotifier POSTs alerts to an HTTP endpoint.
type WebhookNotifier struct {
	url    string
	format string
	client *http.Client
}

// NewWebhookNotifier builds a webhook channel. format must be one of the
// WebhookFormat* constants.
func NewWebhookNotifier(url, format string) (*WebhookNotifier, error) {
	if url == "" {
		return nil, fmt.Errorf("alert: webhook URL is empty")
	}
	if format != WebhookFormatGeneric && format != WebhookFormatSlack {
		return nil, fmt.Errorf("alert: unknown webhook format %q (want %q or %q)",
			format, WebhookFormatGeneric, WebhookFormatSlack)
	}
	return &WebhookNotifier{
		url:    url,
		format: format,
		client: &http.Client{Timeout: 10 * time.Second},
	}, nil
}

// Name implements Notifier.
func (n *WebhookNotifier) Name() string { return "webhook" }

// Send implements Notifier.
func (n *WebhookNotifier) Send(ctx context.Context, alert Alert) error {
	payload, err := n.payload(alert)
	if err != nil {
		return fmt.Errorf("alert: encode webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("alert: build webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "starlens-alerts")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("alert: post webhook: %w", err)
	}
	defer resp.Body.Close()
	// Drain so the connection can be reused.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("alert: webhook returned %s", resp.Status)
	}
	return nil
}

func (n *WebhookNotifier) payload(alert Alert) ([]byte, error) {
	if n.format == WebhookFormatSlack {
		return json.Marshal(map[string]string{"text": slackText(alert)})
	}

	// The generic envelope wraps the alert with a stable source marker so one
	// receiver can ingest events from several tools.
	return json.Marshal(map[string]any{
		"source": "starlens",
		"alert":  alert,
	})
}

func slackText(alert Alert) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[%s] %s", strings.ToUpper(string(alert.Severity)), alert.Title)
	if alert.Message != "" {
		b.WriteString("\n")
		b.WriteString(alert.Message)
	}
	if len(alert.Labels) > 0 {
		b.WriteString("\n")
		first := true
		// Deterministic enough for humans; Slack renders it as one line.
		for _, k := range sortedKeys(alert.Labels) {
			if !first {
				b.WriteString(" · ")
			}
			fmt.Fprintf(&b, "%s=%s", k, alert.Labels[k])
			first = false
		}
	}
	return b.String()
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Insertion sort: label sets are tiny.
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}
