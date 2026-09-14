package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/common-nighthawk/go-figure"
	"github.com/hibiken/asynq"
	sharedmetrics "github.com/mrhumster/go-shared/metrics"
	sharedworker "github.com/mrhumster/go-shared/worker"
	"github.com/mrhumster/events-service/config"
	"github.com/mrhumster/events-service/internal/database"
	"github.com/mrhumster/events-service/internal/queue"
	"github.com/mrhumster/events-service/internal/repository"
	"github.com/mrhumster/events-service/internal/service"
)

var (
	version   = "dev"
	buildDate = "unknown"
)

func main() {
	figure.NewFigure("events "+version, "graffiti", true).Print()

	opts := &slog.HandlerOptions{Level: slog.LevelDebug, AddSource: true}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, opts)))

	cfg, err := config.LoadConfig()
	if err != nil {
		slog.Error("error load config", "error", err)
		os.Exit(1)
	}

	db, err := database.SetupDatabase(cfg)
	if err != nil {
		slog.Error("database setup failed", "error", err)
		os.Exit(1)
	}

	repo := repository.NewGormEventRepository(db)
	svc := service.NewEventsServiceImpl(repo)

	srv, err := sharedworker.NewAsynqServer(sharedworker.Options{
		Addr:            cfg.Redis.Addr,
		Password:        cfg.Redis.Password,
		DB:              cfg.Redis.QueueDB,
		Concurrency:     cfg.Worker.Concurrency,
		ShutdownTimeout: parseDuration(cfg.Worker.ShutdownTimeout, "50m"),
		Queues:          map[string]int{queue.TaskActivityQueue: 6},
		MetricsAddr:     cfg.Server.MetricsAddr,
	})
	if err != nil {
		slog.Error("error init asynq worker", "error", err)
		os.Exit(1)
	}

	h := queue.NewHandleActivityEvent(svc)
	mux := asynq.NewServeMux()
	mux.HandleFunc(
		queue.TaskActivityEvent,
		sharedmetrics.Instrument(queue.TaskActivityEvent, h.HandleActivityEventTask),
	)

	slog.Info("events worker started", "queue", queue.TaskActivityQueue, "redis_db", cfg.Redis.QueueDB)
	if err := srv.Run(mux); err != nil {
		slog.Error("could not run asynq server", "error", err)
		os.Exit(1)
	}
}

func parseDuration(raw string, fallback string) time.Duration {
	d, err := time.ParseDuration(raw)
	if err != nil {
		d, _ = time.ParseDuration(fallback)
	}
	return d
}