package alert

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// recentCapacity bounds the in-memory alert history served by GET /api/v1/alerts.
const recentCapacity = 100

// Manager fans alerts out to every registered Notifier, suppressing repeats of
// the same Key within the cooldown window, and keeps a bounded in-memory
// history for the API. History is intentionally not persisted: StarLens treats
// long-term alert storage as the job of the receiving system (Slack,
// Alertmanager, ...).
type Manager struct {
	cooldown time.Duration
	logger   *slog.Logger
	now      func() time.Time

	mu        sync.Mutex
	notifiers []Notifier
	lastFired map[string]time.Time
	recent    []Alert // newest first
}

// NewManager builds a Manager with the given repeat-suppression window.
func NewManager(cooldown time.Duration, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{
		cooldown:  cooldown,
		logger:    logger,
		now:       time.Now,
		lastFired: make(map[string]time.Time),
	}
}

// Register adds a delivery channel. Not safe to call concurrently with Dispatch;
// register everything during startup.
func (m *Manager) Register(notifier Notifier) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifiers = append(m.notifiers, notifier)
}

// Dispatch records the alert and delivers it to every notifier. It returns
// false when the alert was suppressed because the same Key fired within the
// cooldown window. Notifier failures are logged, never propagated: one broken
// webhook must not silence the remaining channels.
func (m *Manager) Dispatch(ctx context.Context, alert Alert) bool {
	m.mu.Lock()
	if last, ok := m.lastFired[alert.Key]; ok && m.now().Sub(last) < m.cooldown {
		m.mu.Unlock()
		return false
	}
	alert.FiredAt = m.now().UTC()
	m.lastFired[alert.Key] = alert.FiredAt
	m.remember(alert)
	notifiers := make([]Notifier, len(m.notifiers))
	copy(notifiers, m.notifiers)
	m.mu.Unlock()

	m.deliver(ctx, notifiers, alert)
	return true
}

// Test sends a synthetic alert through every notifier, bypassing cooldown, and
// reports the per-channel outcome. Used by POST /api/v1/alerts/test so an
// operator can verify a webhook before trusting it with real incidents.
func (m *Manager) Test(ctx context.Context) (Alert, map[string]string) {
	alert := Alert{
		Key:      "test",
		RuleID:   "test",
		Severity: SeverityInfo,
		Title:    "StarLens test alert",
		Message:  "This is a test alert fired manually from StarLens. If you can read this, the channel works.",
		FiredAt:  m.now().UTC(),
	}

	m.mu.Lock()
	m.remember(alert)
	notifiers := make([]Notifier, len(m.notifiers))
	copy(notifiers, m.notifiers)
	m.mu.Unlock()

	results := make(map[string]string, len(notifiers))
	for _, notifier := range notifiers {
		if err := notifier.Send(ctx, alert); err != nil {
			results[notifier.Name()] = err.Error()
		} else {
			results[notifier.Name()] = "ok"
		}
	}
	return alert, results
}

// Recent returns the alert history, newest first.
func (m *Manager) Recent() []Alert {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Alert, len(m.recent))
	copy(out, m.recent)
	return out
}

// remember prepends to the bounded history. Caller must hold m.mu.
func (m *Manager) remember(alert Alert) {
	m.recent = append([]Alert{alert}, m.recent...)
	if len(m.recent) > recentCapacity {
		m.recent = m.recent[:recentCapacity]
	}
}

func (m *Manager) deliver(ctx context.Context, notifiers []Notifier, alert Alert) {
	for _, notifier := range notifiers {
		if err := notifier.Send(ctx, alert); err != nil {
			m.logger.Warn("alert delivery failed",
				"notifier", notifier.Name(), "rule", alert.RuleID, "key", alert.Key, "error", err)
		}
	}
}
