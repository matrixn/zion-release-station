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

	"github.com/matrixn/zion-release-station/internal/config"
	"github.com/matrixn/zion-release-station/internal/database"
	"github.com/matrixn/zion-release-station/internal/httpapi"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := config.Load()
	if err := cfg.EnsureDataDirectories(); err != nil {
		logger.Error("runtime directories unavailable", "error", err)
		os.Exit(1)
	}

	db, err := database.Open(context.Background(), cfg.DatabasePath())
	if err != nil {
		logger.Error("database unavailable", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	server := httpapi.NewServer(cfg, db, logger)
	go func() {
		logger.Info("ReleaseStation started", "address", cfg.BindAddress, "version", cfg.Version)
		if serveErr := server.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			logger.Error("HTTP server stopped unexpectedly", "error", serveErr)
			os.Exit(1)
		}
	}()

	stopContext, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-stopContext.Done()

	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("ReleaseStation stopped")
}
