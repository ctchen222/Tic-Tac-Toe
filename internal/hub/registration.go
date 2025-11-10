package hub

import (
	"context"
	"ctchen222/Tic-Tac-Toe/internal/bot"
	"ctchen222/Tic-Tac-Toe/internal/hub/types"
	"ctchen222/Tic-Tac-Toe/internal/player"
	"ctchen222/Tic-Tac-Toe/internal/room"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func (h *Hub) handleReconnectionRegistration(ctx context.Context, p *player.Player, roomID string) {
	ctx, span := tracer.Start(ctx, "hub.handleReconnectionRegistration", trace.WithAttributes(
		attribute.String("player.id", p.ID),
		attribute.String("room.id", roomID),
	))
	defer span.End()

	if existingRoom, ok := h.localRooms[roomID]; ok {
		existingRoom.AddPlayer(p)
		go existingRoom.ReadPump(p)
		slog.InfoContext(ctx, "Reconnected player added back to existing local room", "player.id", p.ID, "room.id", roomID)
	} else {
		slog.InfoContext(ctx, "Creating new local room handler for reconnected player", "player.id", p.ID, "room.id", roomID)
		moveCalculator := &bot.BotMoveCalculator{}
		newRoom := room.NewRoom(roomID, h.rdb, h.gameRepo, h.playerRepo, moveCalculator, moveTimeout)
		newRoom.AddPlayer(p)
		h.localRooms[roomID] = newRoom
		go newRoom.Start(h.unregister)
		go h.runRoomUpdateSubscriber(ctx, newRoom)
	}

	h.sendInitialRoomState(ctx, h.localRooms[roomID], []*player.Player{p})
}

func (h *Hub) registerBotGame(ctx context.Context, req *types.RegistrationRequest) {
	ctx, span := tracer.Start(ctx, "hub.registerBotGame", trace.WithAttributes(
		attribute.String("player.id", req.Player.ID),
		attribute.String("bot.difficulty", req.Difficulty),
	))
	defer span.End()

	slog.InfoContext(ctx, "Creating bot match", "player.id", req.Player.ID, "difficulty", req.Difficulty)

	var botGameTimeout time.Duration
	switch req.Difficulty {
	case "hard":
		botGameTimeout = 5 * time.Second
	case "easy":
		botGameTimeout = 15 * time.Second
	default:
		botGameTimeout = 10 * time.Second
	}

	roomID := uuid.New().String()
	moveCalculator := &bot.BotMoveCalculator{}
	newRoom := room.NewRoom(roomID, h.rdb, h.gameRepo, h.playerRepo, moveCalculator, botGameTimeout)

	player1 := req.Player
	botPlayerID := "bot-" + uuid.New().String()[:8]
	player2 := player.NewPlayer(botPlayerID, nil)
	player2.IsBot = true
	botConn := bot.NewBotConnection(botPlayerID, req.Difficulty, player2, newRoom.IncomingMoves())
	player2.Conn = botConn

	if err := h.gameRepo.Create(ctx, roomID, player1.ID, player2.ID); err != nil {
		slog.ErrorContext(ctx, "Failed to create new bot game in Redis", "room.id", roomID, "error", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to create bot game in Redis")
		return
	}
	slog.InfoContext(ctx, "Bot game state created in Redis", "room.id", roomID)

	newRoom.AddPlayer(player1)
	newRoom.AddPlayer(player2)
	h.localRooms[roomID] = newRoom
	go newRoom.Start(h.unregister)
	go h.runRoomUpdateSubscriber(ctx, newRoom)
	slog.InfoContext(ctx, "Local room handler created for bot match", "room.id", roomID)

	h.sendInitialRoomState(ctx, newRoom, newRoom.Players)
}

func (h *Hub) queuePlayerForMatchmaking(ctx context.Context, req *types.RegistrationRequest) {
	ctx, span := tracer.Start(ctx, "hub.queuePlayerForMatchmaking", trace.WithAttributes(
		attribute.String("player.id", req.Player.ID),
	))
	defer span.End()

	slog.InfoContext(ctx, "Player added to matchmaking queue", "player.id", req.Player.ID)

	if err := h.matchmakingRepo.AddToQueue(ctx, req.Player.ID); err != nil {
		slog.ErrorContext(ctx, "Failed to add player to queue", "player.id", req.Player.ID, "error", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to add player to queue")
	}
}
