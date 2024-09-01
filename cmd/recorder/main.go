package main

import (
	"log/slog"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/zanzhit/studio_recorder/internal/config"
	authhandler "github.com/zanzhit/studio_recorder/internal/http-server/handlers/auth"
	"github.com/zanzhit/studio_recorder/internal/http-server/middleware/logger"
	authservice "github.com/zanzhit/studio_recorder/internal/services/auth"
	"github.com/zanzhit/studio_recorder/internal/storage/postgres"
	authstorage "github.com/zanzhit/studio_recorder/internal/storage/postgres/auth"
)

const (
	envLocal = "local"
	envDev   = "dev"
	envProd  = "prod"
)

func main() {
	cfg := config.MustLoad()

	log := setupLogger(cfg.Env)

	log.Info("starting application", slog.Any("config", cfg))

	cfg.DB.Password = os.Getenv("POSTGRES_PASSWORD")
	if cfg.DB.Password == "" {
		panic("POSTGRES_PASSWORD is required")
	}

	storage, err := postgres.New(cfg.DB)
	if err != nil {
		panic(err)
	}

	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(logger.New(log))
	router.Use(middleware.Recoverer)
	router.Use(middleware.URLFormat)

	authStorage := authstorage.New(storage)

	authService := authservice.New(log, authStorage, authStorage, cfg.TokenTTL, cfg.Secret)

	authHandler := authhandler.New(log, authService)
}

func setupLogger(env string) *slog.Logger {
	var log *slog.Logger

	switch env {
	case envLocal:
		log = slog.New(
			slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		)
	case envDev:
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		)
	case envProd:
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
		)
	}

	return log
}
