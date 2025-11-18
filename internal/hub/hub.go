package hub

import (
	"context"
	"ctchen222/Tic-Tac-Toe/internal/hub/types"
	"ctchen222/Tic-Tac-Toe/internal/player"
	"ctchen222/Tic-Tac-Toe/internal/repository"
	"ctchen222/Tic-Tac-Toe/internal/room"
	"ctchen222/Tic-Tac-Toe/pkg/proto"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const moveTimeout = 15 * time.Second

var (
	activeRoomsCounter metric.Int64UpDownCounter
	gamesPlayedCounter metric.Int64Counter

	tracer = otel.Tracer("hub")
	meter  = otel.Meter("hub")
)

func init() {
	var err error
	activeRoomsCounter, err = meter.Int64UpDownCounter("active_rooms", metric.WithDescription("The number of active rooms."))
	if err != nil {
		panic(err)
	}

	gamesPlayedCounter, err = meter.Int64Counter("games_played_total", metric.WithDescription("The total number of games played."))
	if err != nil {
		panic(err)
	}
}

type Hub struct {
	rdb             *redis.Client
	gameRepo        repository.GameRepository
	playerRepo      repository.PlayerRepository
	matchmakingRepo repository.MatchmakingRepository
	serverID        string
	localPlayers    map[string]*player.Player
	localRooms      map[string]*room.Room

	register      chan *types.RegistrationRequest
	unregister    chan *player.Player
	returnToLobby chan *player.Player
	reconnect     chan *types.ReconnectRequest // New channel for reconnection requests
}

// NewHub creates a new hub.
func NewHub(gameRepo repository.GameRepository, playerRepo repository.PlayerRepository, matchmakingRepo repository.MatchmakingRepository, rdb *redis.Client) *Hub {
	return &Hub{
		rdb:             rdb,
		gameRepo:        gameRepo,
		playerRepo:      playerRepo,
		matchmakingRepo: matchmakingRepo,
		serverID:        uuid.New().String(),
		localPlayers:    make(map[string]*player.Player),
		localRooms:      make(map[string]*room.Room),
		register:        make(chan *types.RegistrationRequest),
		unregister:      make(chan *player.Player),
		returnToLobby:   make(chan *player.Player),
		reconnect:       make(chan *types.ReconnectRequest), // Initialize new channel
	}
}

// Run starts the hub.
func (h *Hub) Run() {
	slog.Info("Hub starting", "server.id", h.serverID)

	go h.runMatcher(context.Background())
	go h.runEventSubscriber(context.Background())

	for {
		select {
		case req := <-h.register:
			hubCtx := context.Background()
			ctx, span := tracer.Start(hubCtx, "hub.register", trace.WithAttributes(
				attribute.String("player.id", req.Player.ID),
				attribute.String("server.id", h.serverID),
			))

			slog.InfoContext(ctx, "Received registration request, placing player in lobby", "player.id", req.Player.ID)

			// Add player to the hub's local player list
			h.localPlayers[req.Player.ID] = req.Player
			req.Player.State = player.StateLobby

			// Start a dedicated message pump for the player in the lobby
			if err := h.playerRepo.SetInitialState(ctx, req.Player.ID, h.serverID); err != nil {
				slog.ErrorContext(ctx, "Failed to set initial player state in Redis", "player.id", req.Player.ID, "error", err)
				continue
			}
			go h.LobbyReadPump(req.Player)

			msg := &proto.ServerToClientMessage{Type: "lobby_joined"}
			data, _ := json.Marshal(msg)
			if err := req.Player.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
				slog.ErrorContext(ctx, "Failed to send lobby_joined message", "player.id", req.Player.ID, "error", err)
				h.unregister <- req.Player
			}

			span.End()

		case p := <-h.unregister:
			hubCtx := context.Background()
			slog.InfoContext(hubCtx, "Player unregistered", "player.id", p.ID)

			delete(h.localPlayers, p.ID)

			// Clean up from matchmaking queue and Redis state
			if err := h.matchmakingRepo.RemoveFromQueue(hubCtx, p.ID); err != nil {
				slog.WarnContext(hubCtx, "Failed to remove player from matchmaking queue on unregister", "player.id", p.ID, "error", err)
			}

			if err := h.playerRepo.SetOffline(hubCtx, p.ID); err != nil {
				slog.ErrorContext(hubCtx, "Failed to set player status to offline", "player.id", p.ID, "error", err)
			}

		case p := <-h.returnToLobby:
			hubCtx := context.Background()
			slog.InfoContext(hubCtx, "Player returning to lobby", "player.id", p.ID)

			p.State = player.StateLobby
			if err := h.playerRepo.SetInitialState(hubCtx, p.ID, h.serverID); err != nil {
				slog.ErrorContext(hubCtx, "Failed to set player state to lobby in Redis", "player.id", p.ID, "error", err)
			}

			go h.LobbyReadPump(p)

			msg := &proto.ServerToClientMessage{Type: "lobby_joined"}
			data, _ := json.Marshal(msg)
			if err := p.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
				slog.ErrorContext(hubCtx, "Failed to send lobby_joined message on return", "player.id", p.ID, "error", err)
				h.unregister <- p
			}

		case req := <-h.reconnect:
			hubCtx := context.Background()
			ctx, span := tracer.Start(hubCtx, "hub.reconnect", trace.WithAttributes(
				attribute.String("player.id", req.Player.ID),
				attribute.String("room.id", req.RoomID),
				attribute.String("server.id", h.serverID),
			))

			slog.InfoContext(ctx, "Received reconnection request", "player.id", req.Player.ID, "room.id", req.RoomID)

			if room, ok := h.localRooms[req.RoomID]; ok {
				room.HandleReconnect(req.Player)
				slog.InfoContext(ctx, "Player reconnected to local room", "player.id", req.Player.ID, "room.id", req.RoomID)
			} else {
				slog.WarnContext(ctx, "Reconnect failed: room not found locally. Sending player to lobby.", "player.id", req.Player.ID, "room.id", req.RoomID)
				h.returnToLobby <- req.Player
			}

			span.End()
		}
	}
}

// Register returns the register channel.
func (h *Hub) Register() chan<- *types.RegistrationRequest {
	return h.register
}

// Unregister returns the unregister channel.
func (h *Hub) Unregister() chan<- *player.Player {
	return h.unregister
}

// ReturnToLobby returns the returnToLobby channel.
func (h *Hub) ReturnToLobby() chan<- *player.Player {
	return h.returnToLobby
}

// Reconnect returns the reconnect channel.
func (h *Hub) Reconnect() chan<- *types.ReconnectRequest {
	return h.reconnect
}
