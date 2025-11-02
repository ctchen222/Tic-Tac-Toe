package main

import (
	"ctchen222/Tic-Tac-Toe/internal/hub"
	"ctchen222/Tic-Tac-Toe/internal/hub/types"
	"ctchen222/Tic-Tac-Toe/internal/room"
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

	player := &room.Player{
		ID:   uuid.New().String(),
		Conn: conn,
	}

	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "human"
	}
	difficulty := r.URL.Query().Get("difficulty")
	if mode == "bot" && difficulty == "" {
		difficulty = "easy"
	}

	req := &types.RegistrationRequest{
		Player:     player,
		Mode:       mode,
		Difficulty: difficulty,
	}

	hub.Register() <- req
}

func main() {
	hub := hub.NewHub()
	go hub.Run()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})

	log.Println("http server started on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
