package hub

import (
	"context"
	"ctchen222/Tic-Tac-Toe/internal/bot"
	"ctchen222/Tic-Tac-Toe/internal/hub/types"
	"ctchen222/Tic-Tac-Toe/internal/player"
	"ctchen222/Tic-Tac-Toe/internal/room"
	"log"
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
		log.Printf("Reconnected player %s added back to existing local room %s", p.ID, roomID)
	} else {
		log.Printf("Creating new local room handler for reconnected player %s in room %s", p.ID, roomID)
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

	log.Printf("Creating bot match for player %s with difficulty %s", req.Player.ID, req.Difficulty)

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
		log.Printf("Failed to create new bot game in Redis for room %s: %v", roomID, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to create bot game in Redis")
		return
	}
	log.Printf("Bot game state created in Redis for room %s", roomID)

	newRoom.AddPlayer(player1)
	newRoom.AddPlayer(player2)
	h.localRooms[roomID] = newRoom
	go newRoom.Start(h.unregister)
	go h.runRoomUpdateSubscriber(ctx, newRoom)
	log.Printf("Local room handler created for bot match %s", roomID)

	h.sendInitialRoomState(ctx, newRoom, newRoom.Players)
}

func (h *Hub) queuePlayerForMatchmaking(ctx context.Context, req *types.RegistrationRequest) {
	ctx, span := tracer.Start(ctx, "hub.queuePlayerForMatchmaking", trace.WithAttributes(
		attribute.String("player.id", req.Player.ID),
	))
	defer span.End()

	log.Printf("Player %s added to matchmaking queue.", req.Player.ID)

	if err := h.matchmakingRepo.AddToQueue(ctx, req.Player.ID); err != nil {
		log.Printf("Failed to add player %s to queue: %v", req.Player.ID, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to add player to queue")
	}
}
