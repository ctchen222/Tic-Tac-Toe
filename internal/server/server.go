package server

import (
	"ctchen222/Tic-Tac-Toe/internal/hub"
	"ctchen222/Tic-Tac-Toe/internal/hub/types"
	"ctchen222/Tic-Tac-Toe/internal/player"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("server")

type Server struct {
	hub      *hub.Hub
	upgrader websocket.Upgrader
}

func NewServer(h *hub.Hub) *Server {
	return &Server{
		hub: h,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

func (s *Server) RegisterHandlers() {
	fs := http.FileServer(http.Dir("./web"))
	http.Handle("/", fs)
	http.HandleFunc("/ws", s.handleWebSocket)
}

// handleWebSocket's only responsibility is to upgrade the connection and
// pass a registration request to the hub. It does not distinguish between
// new and reconnecting players.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "server.handleWebSocket", trace.WithAttributes(
		attribute.String("http.url", r.URL.String()),
		attribute.String("http.method", r.Method),
	))
	defer span.End()

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to upgrade connection")
		return
	}

	// Get playerID from URL, or generate a new one.
	playerID := r.URL.Query().Get("playerId")
	if playerID == "" {
		playerID = uuid.New().String()
	}
	span.SetAttributes(attribute.String("player.id", playerID))

	p := player.NewPlayer(playerID, conn)

	// Get game mode preferences
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "human"
	}
	difficulty := r.URL.Query().Get("difficulty")
	if mode == "bot" && difficulty == "" {
		difficulty = "easy"
	}
	span.SetAttributes(attribute.String("game.mode", mode), attribute.String("game.difficulty", difficulty))

	// Send the registration request to the hub for processing.
	req := &types.RegistrationRequest{
		Player:     p,
		PlayerID:   p.ID,
		Mode:       mode,
		Difficulty: difficulty,
		Ctx:        ctx, // Pass the context with the span
	}
	s.hub.Register() <- req
}

