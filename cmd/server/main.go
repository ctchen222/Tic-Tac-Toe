package main

import (
	"ctchen222/Tic-Tac-Toe/internal/hub"
	"ctchen222/Tic-Tac-Toe/internal/hub/types"
	"ctchen222/Tic-Tac-Toe/internal/player"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func serveWs(hub *hub.Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	// Check for reconnection
	reconnectID := r.URL.Query().Get("playerId")

	var p *player.Player
	var playerID string

	if reconnectID != "" {
		playerID = reconnectID
	} else {
		playerID = uuid.New().String()
	}
	p = player.NewPlayer(playerID, conn)

	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "human"
	}
	difficulty := r.URL.Query().Get("difficulty")
	if mode == "bot" && difficulty == "" {
		difficulty = "easy"
	}

	req := &types.RegistrationRequest{
		Player:     p,
		PlayerID:   reconnectID, // Pass the reconnectID here (will be empty for new players)
		Mode:       mode,
		Difficulty: difficulty,
	}

	hub.Register() <- req
}

func main() {
	hub := hub.NewHub()
	go hub.Run()

	// Serve static files from the "web" directory
	fs := http.FileServer(http.Dir("./web"))
	http.Handle("/", fs)

	// Handle WebSocket connections
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})

	log.Println("http server started on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
