package alert

import (
	"context"
	"log/slog"
)

// LogNotifier writes alerts to the server log. Always registered, so alerting
// works out of the box before any external channel is configured.
type LogNotifier struct {
	logger *slog.Logger
}

// NewLogNotifier builds a log-backed notifier.
func NewLogNotifier(logger *slog.Logger) *LogNotifier {
	if logger == nil {
		logger = slog.Default()
	}
	return &LogNotifier{logger: logger}
}

// Name implements Notifier.
func (n *LogNotifier) Name() string { return "log" }

// Send implements Notifier.
func (n *LogNotifier) Send(_ context.Context, alert Alert) error {
	level := slog.LevelInfo
	switch alert.Severity {
	case SeverityWarning:
		level = slog.LevelWarn
	case SeverityCritical:
		level = slog.LevelError
	}

	args := []any{"rule", alert.RuleID, "key", alert.Key, "message", alert.Message}
	for k, v := range alert.Labels {
		args = append(args, "label."+k, v)
	}
	n.logger.Log(context.Background(), level, "ALERT: "+alert.Title, args...)
	return nil
}
