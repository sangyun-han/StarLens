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

// Poller periodically runs a CollectFunc and dispatches its alerts.
type Poller struct {
	interval time.Duration
	collect  CollectFunc
	manager  *Manager
	logger   *slog.Logger
}

// NewPoller builds a Poller. Run it in a goroutine; it stops when ctx is done.
func NewPoller(interval time.Duration, collect CollectFunc, manager *Manager, logger *slog.Logger) *Poller {
	if logger == nil {
		logger = slog.Default()
	}
	return &Poller{interval: interval, collect: collect, manager: manager, logger: logger}
}

// Run evaluates immediately, then on every tick until ctx is cancelled. A
// collection failure (cluster down, query timeout) is logged and retried on the
// next tick — the poller itself never exits early.
func (p *Poller) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		p.evaluate(ctx)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
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
