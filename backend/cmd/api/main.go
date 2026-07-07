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

	"github.com/YankongLi/talent-profile/backend/internal/config"
	"github.com/YankongLi/talent-profile/backend/internal/database"
	"github.com/YankongLi/talent-profile/backend/internal/httpapi"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config failed", "error", err)
		os.Exit(1)
	}

	if cfg.Database.DSN == "" {
		logger.Warn("database dsn is empty; database connection disabled")
	} else {
		dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		db, err := database.Open(dbCtx, cfg.Database)
		cancel()
		if err != nil {
			logger.Error("connect database failed", "error", err)
			os.Exit(1)
		}
		defer db.Close()
		logger.Info("database connected")
	}

	server := httpapi.NewServer(cfg)
	errCh := make(chan error, 1)

	go func() {
		logger.Info("talentpage api started", "addr", cfg.HTTP.Addr, "env", cfg.Env)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errCh:
		logger.Error("talentpage api stopped unexpectedly", "error", err)
		os.Exit(1)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
		defer cancel()

		startedAt := time.Now()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("talentpage api shutdown failed", "error", err)
			os.Exit(1)
		}
		logger.Info("talentpage api stopped", "duration", time.Since(startedAt).String())
	}
}
