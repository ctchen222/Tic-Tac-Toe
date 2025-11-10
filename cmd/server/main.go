package main

import (
	"context"
	"ctchen222/Tic-Tac-Toe/internal/api/controller"
	apirepository "ctchen222/Tic-Tac-Toe/internal/api/repository"
	"ctchen222/Tic-Tac-Toe/internal/api/service"
	"ctchen222/Tic-Tac-Toe/internal/db"
	"ctchen222/Tic-Tac-Toe/internal/hub"
	"ctchen222/Tic-Tac-Toe/internal/logger"
	"ctchen222/Tic-Tac-Toe/internal/repository"
	"ctchen222/Tic-Tac-Toe/internal/server"
	"ctchen222/Tic-Tac-Toe/internal/telemetry"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx := context.Background()
	logger.Init()

	// Initialize telemetry
	shutdown, err := telemetry.InitOtel()
	if err != nil {
		slog.Error("failed to initialize telemetry", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			slog.Error("Error shutting down telemetry", "error", err)
		}
	}()

	// Initialize Redis
	rdb, err := db.NewRedisClient(ctx)
	if err != nil {
		slog.Error("failed to initialize redis", "error", err)
		os.Exit(1)
	}

	// Initialize SQLite DB
	if err := db.InitializeDB(); err != nil {
		slog.Error("failed to initialize sqlite db", "error", err)
		os.Exit(1)
	}
	DB, err := db.DBConnect()
	if err != nil {
		slog.Error("failed to get sqlite db connection", "error", err)
		os.Exit(1)
	}

	// Create repositories
	gameRepo := repository.NewGameRepository(rdb)
	playerRepo := repository.NewPlayerRepository(rdb)
	matchmakingRepo := repository.NewMatchmakingRepository(rdb)
	userRepo := apirepository.NewUserRepository(DB)

	// Create services
	userService := service.NewUserService(userRepo)

	// Create controllers
	userController := controller.NewUserController(userService)

	// Create hub
	hub := hub.NewHub(gameRepo, playerRepo, matchmakingRepo, rdb)
	go hub.Run()

	// Create the Gin-based server
	srv := server.NewServer(hub, userController)

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	httpServer := &http.Server{
		Addr:    ":8080",
		Handler: srv.Engine(),
	}

	go func() {
		slog.Info("http server started on :8080")
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("ListenAndServe error", "error", err)
			os.Exit(1)
		}
	}()

	<-stop

	slog.Info("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("Server exiting")
}
