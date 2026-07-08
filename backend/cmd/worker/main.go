package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/YankongLi/talent-profile/backend/internal/config"
	"github.com/YankongLi/talent-profile/backend/internal/database"
	"github.com/YankongLi/talent-profile/backend/internal/resume"
	"github.com/YankongLi/talent-profile/backend/internal/storage"
	"github.com/hibiken/asynq"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config failed", "error", err)
		os.Exit(1)
	}
	if cfg.Database.DSN == "" {
		logger.Error("database dsn is required")
		os.Exit(1)
	}
	if !storageConfigured(cfg.Storage) {
		logger.Error("storage config is required")
		os.Exit(1)
	}

	dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	db, err := database.Open(dbCtx, cfg.Database)
	cancel()
	if err != nil {
		logger.Error("connect database failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	storageClient, err := storage.NewMinIOClient(cfg.Storage)
	if err != nil {
		logger.Error("create storage client failed", "error", err)
		os.Exit(1)
	}

	server := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     cfg.Redis.Addr,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		},
		asynq.Config{
			Concurrency: 2,
			Queues: map[string]int{
				"default": 1,
			},
		},
	)

	mux := asynq.NewServeMux()
	resume.NewTextExtractionProcessor(
		resume.NewPostgresStore(db),
		storageClient,
		resume.DefaultTextExtractor{},
	).Register(mux)

	errCh := make(chan error, 1)
	go func() {
		logger.Info("talentpage worker started", "redis", cfg.Redis.Addr)
		if err := server.Run(mux); err != nil {
			errCh <- err
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errCh:
		logger.Error("talentpage worker stopped unexpectedly", "error", err)
		os.Exit(1)
	case <-ctx.Done():
		server.Shutdown()
		logger.Info("talentpage worker stopped")
	}
}

func storageConfigured(cfg config.StorageConfig) bool {
	return cfg.Endpoint != "" &&
		cfg.Bucket != "" &&
		cfg.AccessKeyID != "" &&
		cfg.SecretAccessKey != ""
}
