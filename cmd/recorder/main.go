package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/lib/pq"

	"github.com/zanzhit/studio_recorder/internal/config"
	authhandler "github.com/zanzhit/studio_recorder/internal/http-server/handlers/auth"
	camerahandler "github.com/zanzhit/studio_recorder/internal/http-server/handlers/cameras"
	recordinghandler "github.com/zanzhit/studio_recorder/internal/http-server/handlers/recordings"
	authmid "github.com/zanzhit/studio_recorder/internal/http-server/middleware/auth"
	"github.com/zanzhit/studio_recorder/internal/http-server/middleware/logger"
	"github.com/zanzhit/studio_recorder/internal/lib/sl"
	authservice "github.com/zanzhit/studio_recorder/internal/services/auth"
	cameraservice "github.com/zanzhit/studio_recorder/internal/services/cameras"
	recordingservice "github.com/zanzhit/studio_recorder/internal/services/recordings"
	"github.com/zanzhit/studio_recorder/internal/services/recordings/opencast"
	"github.com/zanzhit/studio_recorder/internal/storage/postgres"
	authstorage "github.com/zanzhit/studio_recorder/internal/storage/postgres/auth"
	camerastorage "github.com/zanzhit/studio_recorder/internal/storage/postgres/cameras"
	recordingstorage "github.com/zanzhit/studio_recorder/internal/storage/postgres/recordings"
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

	if err := authService.CreateInitialAdmin(); err != nil {
		panic(err)
	}

	cameraStorage := camerastorage.New(storage)
	cameraService := cameraservice.New(log, cfg.VideosPath, cameraStorage)
	cameraHandler := camerahandler.New(log, cameraService, cameraStorage)

	opencast := opencast.MustLoad(cfg.VideoService)

	recordingStorage := recordingstorage.New(storage)
	recordingService := recordingservice.New(log, recordingStorage, recordingStorage, cameraStorage, opencast, cfg.VideosPath)
	recordingHandler := recordinghandler.New(log, recordingService, recordingService)

	router.Post("/login", authHandler.Login)

	router.With(authmid.JWTAuth(cfg.Secret)).Group(func(r chi.Router) {
		r.With(authmid.AdminRequired).Route("/users", func(r chi.Router) {
			r.Post("/", authHandler.RegisterNewUser)
			r.Patch("/", authHandler.UpdatePassword)
			r.Delete("/", authHandler.DeleteUser)
		})

		r.Route("/cameras", func(r chi.Router) {
			r.Get("/", cameraHandler.Cameras)
			r.With(authmid.AdminRequired).Group(func(r chi.Router) {
				r.Post("/", cameraHandler.SaveCamera)
				r.Patch("/{cameraID}", cameraHandler.UpdateCamera)
				r.Delete("/{cameraID}", cameraHandler.DeleteCamera)
			})
		})

		r.Route("/recordings", func(r chi.Router) {
			r.Get("/{cameraID}", recordingHandler.Recordings)
			r.Get("/{recordID}/download", recordingHandler.Download)
			r.Post("/start", recordingHandler.Start)
			r.Post("/schedule", recordingHandler.Schedule)
			r.Post("/{recordID}/stop", recordingHandler.Stop)
			r.Delete("/{recordID}", recordingHandler.Delete)
			if cfg.VideoService != "" {
				r.Post("/{recordID}/move", recordingHandler.Move)
			}
		})
	})

	log.Info("starting http server", slog.String("address", cfg.Address))

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	srv := http.Server{
		Addr:         cfg.Address,
		Handler:      router,
		ReadTimeout:  cfg.Timeout,
		WriteTimeout: cfg.Timeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Error("failed to start server")
		}
	}()

	<-done
	log.Error("stopping server")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("failed to stop server", sl.Err(err))

		return
	}

	if err := storage.Close(); err != nil {
		log.Error("failed to close storage", sl.Err(err))

		return
	}

	log.Info("server stopped")
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
