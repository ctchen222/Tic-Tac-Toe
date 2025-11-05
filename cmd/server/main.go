package main

import (
	"context"
	"ctchen222/Tic-Tac-Toe/internal/hub"
	"ctchen222/Tic-Tac-Toe/internal/server"
	"ctchen222/Tic-Tac-Toe/internal/telemetry"
	"log"
	"net/http"
)

func main() {
	ctx := context.Background()
	shutdown, err := telemetry.InitOtel()
	if err != nil {
		log.Fatalf("failed to initialize OTel: %v", err)
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			log.Fatalf("failed to shutdown OTel: %v", err)
		}
	}()

	// Create and run the hub
	gameHub := hub.NewHub()
	go gameHub.Run()

	// Create the server and register handlers
	srv := server.New(gameHub)
	srv.RegisterHandlers()

	log.Println("http server started on :8080")
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
