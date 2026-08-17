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

	clusterService := service.NewClusterService(repository.NewClusterRepository(db))
	router := api.Router(cfg.Server, api.NewHealthHandler(db), api.NewClusterHandler(clusterService))

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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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
