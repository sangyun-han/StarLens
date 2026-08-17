// Package alert is StarLens' alerting subsystem: rule evaluations elsewhere in
// the codebase produce Alert values, and a Manager fans them out to pluggable
// Notifier implementations with deduplication and cooldown.
package alert

import (
	"context"
	"time"
)

// Severity orders alerts by urgency.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// Alert is one condition worth telling an operator about.
type Alert struct {
	// Key identifies the ongoing condition (rule + subject) and drives
	// deduplication: the same key is not re-sent within the cooldown window.
	Key string `json:"key"`
	// RuleID names the rule that fired, e.g. "routine_load_paused".
	RuleID   string   `json:"ruleId"`
	Severity Severity `json:"severity"`
	// Title is a one-line human summary, e.g. `Routine load job "orders" is PAUSED`.
	Title string `json:"title"`
	// Message carries the detail an operator acts on (reason, thresholds, values).
	Message string `json:"message"`
	// Labels are structured dimensions (database, job, state) for machine
	// consumers such as webhook receivers.
	Labels map[string]string `json:"labels,omitempty"`
	// FiredAt is stamped by the Manager at dispatch time.
	FiredAt time.Time `json:"firedAt"`
}

// Notifier delivers alerts to one destination. Implementations must be safe for
// concurrent use; delivery failures are logged by the Manager, never fatal.
//
// This is the extension point for new channels (email, PagerDuty, Opsgenie):
// implement Notifier and register it on the Manager in cmd/server.
type Notifier interface {
	// Name identifies the channel in logs and test-delivery results.
	Name() string
	// Send delivers a single alert, honoring ctx for cancellation.
	Send(ctx context.Context, alert Alert) error
}
