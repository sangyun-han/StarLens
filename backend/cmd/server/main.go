// Command server is the StarLens API entrypoint: it wires configuration, the
// StarRocks connection pool, services and the HTTP router, then serves until
// interrupted.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sangyun-han/StarLens/backend/config"
	"github.com/sangyun-han/StarLens/backend/internal/alert"
	"github.com/sangyun-han/StarLens/backend/internal/api"
	"github.com/sangyun-han/StarLens/backend/internal/repository"
	"github.com/sangyun-han/StarLens/backend/internal/service"
)

const shutdownTimeout = 10 * time.Second

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("starlens exited", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	db, err := repository.NewDB(cfg.StarRocks)
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Warn("closing StarRocks pool failed", "error", err)
		}
	}()

	// Reachability is reported, not required: an observability dashboard has to
	// boot while the cluster it watches is down, and say so through /healthz.
	pingCtx, cancelPing := context.WithTimeout(context.Background(), cfg.StarRocks.DialTimeout+time.Second)
	if err := db.Ping(pingCtx); err != nil {
		logger.Warn("StarRocks is not reachable yet; the API will keep retrying per request",
			"addr", cfg.StarRocks.Addr, "error", err)
	} else {
		logger.Info("connected to StarRocks", "addr", cfg.StarRocks.Addr)
	}
	cancelPing()

	// Signal context created up front so background workers share the server's
	// lifetime.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	clusterService := service.NewClusterService(repository.NewClusterRepository(db))
	clusterService.SetAlertPolicy(service.ClusterAlertPolicy{MaxJournalLag: cfg.Alert.MaxJournalLag})
	routineLoadService := service.NewRoutineLoadService(
		repository.NewRoutineLoadRepository(db),
		service.RoutineLoadAlertPolicy{
			ErrorRowsRatio:    cfg.Alert.ErrorRowsRatio,
			ErrorRowsMinTotal: cfg.Alert.ErrorRowsMinTotal,
			MaxOffsetLag:      cfg.Alert.MaxOffsetLag,
		},
	)

	alertManager := alert.NewManager(cfg.Alert.Cooldown, logger)
	alertManager.Register(alert.NewLogNotifier(logger))

	// Environment values are the defaults; the settings file carries what the
	// UI has overridden. applyAlertConfig pushes the effective result into the
	// live components, at boot and again after every PUT /alerts/config.
	alertSettings, err := alert.LoadSettings(cfg.Alert.OverrideFile, alert.Config{
		Enabled:           cfg.Alert.Enabled,
		PollInterval:      cfg.Alert.PollInterval,
		Cooldown:          cfg.Alert.Cooldown,
		WebhookURL:        cfg.Alert.WebhookURL,
		WebhookFormat:     cfg.Alert.WebhookFormat,
		ErrorRowsRatio:    cfg.Alert.ErrorRowsRatio,
		ErrorRowsMinTotal: cfg.Alert.ErrorRowsMinTotal,
		MaxOffsetLag:      cfg.Alert.MaxOffsetLag,
		MaxJournalLag:     cfg.Alert.MaxJournalLag,
	})
	if err != nil {
		logger.Warn("ignoring unreadable alert override file; environment defaults are in effect", "error", err)
	}

	applyAlertConfig := func(c alert.Config) error {
		alertManager.SetCooldown(c.Cooldown)
		if c.WebhookURL == "" {
			alertManager.SetWebhook(nil)
		} else {
			webhook, err := alert.NewWebhookNotifier(c.WebhookURL, c.WebhookFormat)
			if err != nil {
				return err
			}
			alertManager.SetWebhook(webhook)
		}
		routineLoadService.SetPolicy(service.RoutineLoadAlertPolicy{
			ErrorRowsRatio:    c.ErrorRowsRatio,
			ErrorRowsMinTotal: c.ErrorRowsMinTotal,
			MaxOffsetLag:      c.MaxOffsetLag,
		})
		clusterService.SetAlertPolicy(service.ClusterAlertPolicy{MaxJournalLag: c.MaxJournalLag})
		return nil
	}
	if err := applyAlertConfig(alertSettings.Effective()); err != nil {
		return err
	}

	// The poller always runs; enablement and interval are re-read every tick,
	// so settings changes apply without a restart.
	poller := alert.NewPoller(func() alert.PollSettings {
		effective := alertSettings.Effective()
		return alert.PollSettings{Interval: effective.PollInterval, Enabled: effective.Enabled}
	}, alert.Combine(clusterService.CollectAlerts, routineLoadService.CollectAlerts), alertManager, logger)
	go poller.Run(ctx)

	effective := alertSettings.Effective()
	logger.Info("alerting configured",
		"enabled", effective.Enabled, "interval", effective.PollInterval,
		"cooldown", effective.Cooldown, "webhook", effective.WebhookURL != "",
		"uiEditable", cfg.Alert.UIEditable, "overrideFile", cfg.Alert.OverrideFile)

	queryService := service.NewQueryService(
		repository.NewQueryRepository(db, cfg.StarRocks.Database),
		service.QueryPolicy{
			ReadOnly: cfg.Query.ReadOnly,
			MaxRows:  cfg.Query.MaxRows,
			Timeout:  cfg.Query.Timeout,
		},
	)

	router := api.Router(cfg.Server, api.Handlers{
		Health:      api.NewHealthHandler(db),
		Cluster:     api.NewClusterHandler(clusterService),
		Loads:       api.NewRoutineLoadHandler(routineLoadService),
		Alerts:      api.NewAlertHandler(alertManager),
		AlertConfig: api.NewAlertConfigHandler(alertSettings, applyAlertConfig, cfg.Alert.UIEditable),
		Queries:     api.NewQueryHandler(queryService),
		Storage:     api.NewStorageHandler(service.NewStorageService(repository.NewStorageRepository(db))),
	})

	server := &http.Server{
		Addr:              ":" + cfg.Server.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Serve in the background so the main goroutine can wait on signals.
	serveErr := make(chan error, 1)
	go func() {
		logger.Info("StarLens API listening", "addr", server.Addr, "allowedOrigins", cfg.Server.AllowedOrigins)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
		logger.Info("shutdown signal received; draining connections")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return err
	}
	return <-serveErr
}
