package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/common-nighthawk/go-figure"
	"github.com/mrhumster/events-service/config"
	"github.com/mrhumster/events-service/internal/database"
	"github.com/mrhumster/events-service/internal/delivery/http/routes"
	"github.com/mrhumster/events-service/internal/repository"
	"github.com/mrhumster/events-service/internal/service"
)

var (
	version   = "dev"
	buildDate = "unknown"
)

func main() {
	figure.NewFigure("events-reader "+version, "graffiti", true).Print()

	opts := &slog.HandlerOptions{Level: slog.LevelDebug, AddSource: true}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, opts)))
	slog.Info("Start events reader", "version", version, "build_date", buildDate)

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("❌ error load config: %v", err)
	}

	db, err := database.SetupDatabase(cfg)
	if err != nil {
		log.Fatalf("❌ error open database: %v", err)
	}

	tokens, err := service.NewTokenService(&cfg.JWT)
	if err != nil {
		log.Fatalf("❌ error init token service: %v", err)
	}

	repo := repository.NewGormEventRepository(db)
	svc := service.NewEventsServiceImpl(repo)

	r := routes.SetupRoutes(db, cfg, svc, tokens)

	srv := &http.Server{
		Addr:         cfg.Server.ServerAddr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	defer func() {
		sqlDB, err := db.DB()
		if err != nil {
			log.Printf("failed to get sql.DB: %s", err.Error())
			return
		}
		if err := sqlDB.Close(); err != nil {
			log.Printf("🟢 Database pool closed")
		}
	}()

	go func() {
		log.Printf("🚀 events reader listening on %s", cfg.Server.ServerAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("🔴 server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("🟡 shutting down events reader...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}