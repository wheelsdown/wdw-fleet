package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wheelsdown/wdw-fleet/internal/blob"
	"github.com/wheelsdown/wdw-fleet/internal/config"
	"github.com/wheelsdown/wdw-fleet/internal/database"
	"github.com/wheelsdown/wdw-fleet/internal/server/api"
	"github.com/wheelsdown/wdw-fleet/internal/store"
	"github.com/wheelsdown/wdw-fleet/internal/version"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	setupLogging(cfg.LogLevel)

	slog.Info("starting wdw-fleet",
		"version", version.Version,
		"commit", version.Commit,
		"build_date", version.BuildDate,
		"listen_addr", cfg.ListenAddr)

	db, err := database.Connect(context.Background(), cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer db.Close()

	if err := database.Migrate(context.Background(), db); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	apiServer := &api.Server{
		DB:       db,
		Logger:   slog.Default(),
		Blobs:    blob.New(cfg.BlobRoot),
		Vehicles: store.NewVehicles(db),
	}

	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      apiServer.Handler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		slog.Info("shutting down", "signal", sig)
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown error: %w", err)
	}

	slog.Info("server stopped")
	return nil
}

func setupLogging(level string) {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})))
}
