package server

import (
	"ctchen222/Tic-Tac-Toe/internal/hub"
	"ctchen222/Tic-Tac-Toe/internal/hub/types"
	"ctchen222/Tic-Tac-Toe/internal/player"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Server holds the dependencies for the HTTP server.
type Server struct {
	hub      *hub.Hub
	upgrader websocket.Upgrader
}

// New creates a new Server.
func New(h *hub.Hub) *Server {
	return &Server{
		hub: h,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins
			},
		},
	}
}

// RegisterHandlers registers the HTTP handlers for the server.
func (s *Server) RegisterHandlers() {
	// Serve static files from the "web" directory
	fs := http.FileServer(http.Dir("./web"))
	http.Handle("/", fs)

	// Handle WebSocket connections
	http.HandleFunc("/ws", s.handleWebSocket)
}

// handleWebSocket handles the WebSocket connection requests.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
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
		Ctx:        ctx,
	}

	s.hub.Register() <- req
}
