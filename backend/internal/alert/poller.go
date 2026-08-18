package alert

import (
	"context"
	"log/slog"
	"time"
)

// CollectFunc evaluates alert rules against current cluster state and returns
// the alerts that should fire right now. Returning the same condition on
// consecutive ticks is expected — the Manager's cooldown handles repeats.
type CollectFunc func(ctx context.Context) ([]Alert, error)

// PollSettings is what the poller re-reads before every tick, so interval and
// enablement changes from the settings UI apply without a restart.
type PollSettings struct {
	Interval time.Duration
	Enabled  bool
}

// PollSettingsFunc supplies the current PollSettings.
type PollSettingsFunc func() PollSettings

// fallbackInterval guards against a zero interval from a misconfigured source.
const fallbackInterval = 30 * time.Second

// Poller periodically runs a CollectFunc and dispatches its alerts.
type Poller struct {
	settings PollSettingsFunc
	collect  CollectFunc
	manager  *Manager
	logger   *slog.Logger
}

// NewPoller builds a Poller. Run it in a goroutine; it stops when ctx is done.
func NewPoller(settings PollSettingsFunc, collect CollectFunc, manager *Manager, logger *slog.Logger) *Poller {
	if logger == nil {
		logger = slog.Default()
	}
	return &Poller{settings: settings, collect: collect, manager: manager, logger: logger}
}

// Run evaluates immediately, then on every tick until ctx is cancelled.
// Settings are re-read each iteration: disabling alerting skips evaluation but
// keeps the loop alive so re-enabling needs no restart. A collection failure
// (cluster down, query timeout) is logged and retried on the next tick.
func (p *Poller) Run(ctx context.Context) {
	for {
		settings := p.settings()
		if settings.Enabled {
			p.evaluate(ctx)
		}

		interval := settings.Interval
		if interval <= 0 {
			interval = fallbackInterval
		}

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (p *Poller) evaluate(ctx context.Context) {
	alerts, err := p.collect(ctx)
	if err != nil {
		if ctx.Err() == nil {
			p.logger.Warn("alert evaluation failed; will retry", "error", err)
		}
		return
	}

	for _, alert := range alerts {
		p.manager.Dispatch(ctx, alert)
	}
}
