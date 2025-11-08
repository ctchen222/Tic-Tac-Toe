package main

import (
	"context"
	"ctchen222/Tic-Tac-Toe/internal/db"
	"ctchen222/Tic-Tac-Toe/internal/hub"
	"ctchen222/Tic-Tac-Toe/internal/repository"
	"ctchen222/Tic-Tac-Toe/internal/server"
	"ctchen222/Tic-Tac-Toe/internal/telemetry"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx := context.Background()

	// Initialize telemetry
	shutdown, err := telemetry.InitOtel()
	if err != nil {
		log.Fatalf("failed to initialize telemetry: %v", err)
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			log.Printf("Error shutting down telemetry: %v", err)
		}
	}()

	// Initialize Redis
	rdb, err := db.NewRedisClient(ctx)
	if err != nil {
		log.Fatalf("failed to initialize redis: %v", err)
	}

	// Create repositories
	gameRepo := repository.NewGameRepository(rdb)
	playerRepo := repository.NewPlayerRepository(rdb)
	matchmakingRepo := repository.NewMatchmakingRepository(rdb)

	// Create hub
	hub := hub.NewHub(gameRepo, playerRepo, matchmakingRepo, rdb)
	go hub.Run()

	// Create and start the server
	srv := server.NewServer(hub)
	srv.RegisterHandlers()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	httpServer := &http.Server{Addr: ":8080"}

	go func() {
		log.Println("http server started on :8080")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe: %v", err)
		}
	}()

	<-stop

	log.Println("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}
