package hub

import (
	"context"
	"ctchen222/Tic-Tac-Toe/internal/hub/types"
	"ctchen222/Tic-Tac-Toe/internal/player"
	"ctchen222/Tic-Tac-Toe/internal/repository"
	"ctchen222/Tic-Tac-Toe/internal/room"
	"log/slog"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
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

	register   chan *types.RegistrationRequest
	unregister chan *player.Player
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
			traceCtx, span := tracer.Start(req.Ctx, "hub.register", trace.WithAttributes(
				attribute.String("player.id", req.Player.ID),
				attribute.String("server.id", h.serverID),
			))

			hubCtx := context.Background()
			slog.InfoContext(traceCtx, "Received registration request", "player.id", req.Player.ID)

			h.localPlayers[req.Player.ID] = req.Player

			roomID, status, err := h.playerRepo.FindForReconnection(hubCtx, req.Player.ID)
			if err != nil && err != redis.Nil {
				slog.ErrorContext(hubCtx, "Error finding player for reconnection", "player.id", req.Player.ID, "error", err)
				continue
			}

			// Only handle as a reconnection if the player was in a room AND was disconnected.
			if roomID != "" && status == player.StatusDisconnected {
				slog.InfoContext(hubCtx, "Registering reconnected player", "player.id", req.Player.ID, "room.id", roomID)
				h.handleReconnectionRegistration(hubCtx, req.Player, roomID)
				span.End()
			} else {
				// All other cases are treated as a new registration.
				if err := h.playerRepo.SetInitialState(hubCtx, req.Player.ID, h.serverID); err != nil {
					slog.ErrorContext(hubCtx, "Failed to set player info in Redis", "player.id", req.Player.ID, "error", err)
					continue
				}

				if req.Mode == "bot" {
					h.registerBotGame(hubCtx, req)
				} else {
					h.queuePlayerForMatchmaking(hubCtx, req)
				}
				span.End()
			}
			_ = traceCtx

		case p := <-h.unregister:
			hubCtx := context.Background()
			slog.InfoContext(hubCtx, "Player unregistered", "player.id", p.ID)

			delete(h.localPlayers, p.ID)

			if err := h.matchmakingRepo.RemoveFromQueue(hubCtx, p.ID); err != nil {
				slog.WarnContext(hubCtx, "Failed to remove player from matchmaking queue", "player.id", p.ID, "error", err)
			}

			if err := h.playerRepo.SetOffline(hubCtx, p.ID); err != nil {
				slog.ErrorContext(hubCtx, "Failed to set player status to offline", "player.id", p.ID, "error", err)
			}
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
