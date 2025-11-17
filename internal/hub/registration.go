package hub

import (
	"context"
	"ctchen222/Tic-Tac-Toe/internal/bot"
	"ctchen222/Tic-Tac-Toe/internal/player"
	"ctchen222/Tic-Tac-Toe/internal/room"
	"ctchen222/Tic-Tac-Toe/pkg/proto"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func (h *Hub) registerBotGame(ctx context.Context, p *player.Player, difficulty string) {
	ctx, span := tracer.Start(ctx, "hub.registerBotGame", trace.WithAttributes(
		attribute.String("player.id", p.ID),
		attribute.String("bot.difficulty", difficulty),
	))
	defer span.End()

	slog.InfoContext(ctx, "Creating bot match", "player.id", p.ID, "difficulty", difficulty)

	var botGameTimeout time.Duration
	switch difficulty {
	case "hard":
		botGameTimeout = 5 * time.Second
	case "easy":
		botGameTimeout = 15 * time.Second
	default:
		botGameTimeout = 10 * time.Second
	}

	roomID := uuid.New().String()
	moveCalculator := &bot.BotMoveCalculator{}
	newRoom := room.NewRoom(roomID, h.rdb, h.gameRepo, h.playerRepo, moveCalculator, botGameTimeout, h.returnToLobby)

	botPlayerID := "bot-" + uuid.New().String()[:8]
	player2 := player.NewPlayer(botPlayerID, nil)
	player2.IsBot = true
	botConn := bot.NewBotConnection(botPlayerID, difficulty, player2, newRoom.IncomingMoves())
	player2.Conn = botConn

	if err := h.gameRepo.Create(ctx, roomID, p.ID, player2.ID); err != nil {
		slog.ErrorContext(ctx, "Failed to create new bot game in Redis", "room.id", roomID, "error", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to create bot game in Redis")
		return
	}
	slog.InfoContext(ctx, "Bot game state created in Redis", "room.id", roomID)

	p.State = player.StateInGame
	player2.State = player.StateInGame

	newRoom.AddPlayer(p)
	newRoom.AddPlayer(player2)
	h.localRooms[roomID] = newRoom
	go newRoom.Start(h.unregister)
	go h.runRoomUpdateSubscriber(ctx, newRoom)
	slog.InfoContext(ctx, "Local room handler created for bot match", "room.id", roomID)

	h.sendInitialRoomState(ctx, newRoom, newRoom.Players)
}

func (h *Hub) queuePlayerForMatchmaking(ctx context.Context, p *player.Player) {
	ctx, span := tracer.Start(ctx, "hub.queuePlayerForMatchmaking", trace.WithAttributes(
		attribute.String("player.id", p.ID),
	))
	defer span.End()

	p.State = player.StateInQueue
	slog.InfoContext(ctx, "Player added to matchmaking queue", "player.id", p.ID)

	if err := h.matchmakingRepo.AddToQueue(ctx, p.ID); err != nil {
		slog.ErrorContext(ctx, "Failed to add player to queue", "player.id", p.ID, "error", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to add player to queue")
		p.State = player.StateLobby
		return
	}

	// Send confirmation to the client
	msg := &proto.ServerToClientMessage{Type: "queue_joined"}
	data, err := json.Marshal(msg)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to marshal queue_joined message", "player.id", p.ID, "error", err)
		return
	}

	if err := p.Conn.WriteMessage(1, data); err != nil {
		slog.ErrorContext(ctx, "Failed to send queue_joined message", "player.id", p.ID, "error", err)
	}
}

