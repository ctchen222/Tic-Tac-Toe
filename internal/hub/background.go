package hub

import (
	"context"
	"ctchen222/Tic-Tac-Toe/internal/events"
	"ctchen222/Tic-Tac-Toe/internal/game"
	"ctchen222/Tic-Tac-Toe/internal/room"
	"ctchen222/Tic-Tac-Toe/pkg/proto"
	"encoding/json"
	"fmt"
	"log/slog"
	"time" // Added for time.Sleep

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func (h *Hub) runRoomUpdateSubscriber(ctx context.Context, room *room.Room) {
	ctx, span := tracer.Start(ctx, "hub.runRoomUpdateSubscriber", trace.WithAttributes(
		attribute.String("room.id", room.ID),
	))
	defer span.End()

	roomChannel := fmt.Sprintf("channel:room:%s", room.ID)
	slog.InfoContext(ctx, "Starting room subscriber", "room.id", room.ID, "channel", roomChannel)
	pubsub := h.rdb.Subscribe(ctx, roomChannel)
	defer pubsub.Close()

	ch := pubsub.Channel()
	for msg := range ch {
		updateCtx, updateSpan := tracer.Start(ctx, "hub.handleRoomUpdate", trace.WithAttributes(
			attribute.String("room.id", room.ID),
			attribute.String("redis.payload", msg.Payload),
		))
		defer updateSpan.End()

		slog.InfoContext(updateCtx, "Received room update", "room.id", room.ID, "payload", msg.Payload)
		gameState, err := h.gameRepo.FindByID(updateCtx, room.ID)
		if err != nil {
			slog.ErrorContext(updateCtx, "Room subscriber could not get game state", "room.id", room.ID, "error", err)
			updateSpan.RecordError(err)
			updateSpan.SetStatus(codes.Error, "Could not get game state")
			continue
		}
		updateMsg := &proto.ServerToClientMessage{
			Type:   "update",
			Board:  game.BoardArrayToSlice(gameState.Board),
			Next:   gameState.CurrentTurn,
			Winner: gameState.Winner,
		}
		room.Broadcast(updateMsg)
	}
	slog.InfoContext(ctx, "Stopping room subscriber", "room.id", room.ID)
}

func (h *Hub) runMatcher(ctx context.Context) {
	slog.InfoContext(ctx, "Redis-based matcher started")
	for {
		//TODO: refactor span error handling
		matchCtx, matchSpan := tracer.Start(ctx, "hub.runMatcher.matchAttempt")

		player1ID, player2ID, err := h.matchmakingRepo.GetPlayersFromQueue(matchCtx)
		if err != nil {
			slog.ErrorContext(matchCtx, "Error getting players from queue", "error", err)
			matchSpan.RecordError(err)
			matchSpan.SetStatus(codes.Error, "Error getting players from queue")
			matchSpan.End()
			time.Sleep(1 * time.Second)
			continue
		}
		matchSpan.SetAttributes(attribute.String("player1.id", player1ID), attribute.String("player2.id", player2ID))

		roomID := uuid.New().String()
		matchSpan.SetAttributes(attribute.String("room.id", roomID))

		if err := h.gameRepo.Create(matchCtx, roomID, player1ID, player2ID); err != nil {
			slog.ErrorContext(matchCtx, "Failed to create new game in Redis", "room.id", roomID, "error", err)
			slog.InfoContext(matchCtx, "Re-queuing players")
			matchSpan.RecordError(err)
			matchSpan.SetStatus(codes.Error, "Failed to create game in Redis")
			if err := h.matchmakingRepo.AddToQueue(matchCtx, player1ID); err != nil {
				slog.ErrorContext(matchCtx, "FATAL: Failed to re-queue player", "player.id", player1ID, "error", err)
				matchSpan.RecordError(err)
				matchSpan.SetStatus(codes.Error, "FATAL: Failed to re-queue player1")
			}
			if err := h.matchmakingRepo.AddToQueue(matchCtx, player2ID); err != nil {
				slog.ErrorContext(matchCtx, "FATAL: Failed to re-queue player", "player.id", player2ID, "error", err)
				matchSpan.RecordError(err)
				matchSpan.SetStatus(codes.Error, "FATAL: Failed to re-queue player2")
			}
			matchSpan.End()
			continue
		}

		if err := h.playerRepo.UpdateForMatch(matchCtx, player1ID, roomID); err != nil {
			slog.ErrorContext(matchCtx, "Failed to update player state for match", "player.id", player1ID, "error", err)
			matchSpan.RecordError(err)
			matchSpan.SetStatus(codes.Error, "Failed to update player1 for match")
		}
		if err := h.playerRepo.UpdateForMatch(matchCtx, player2ID, roomID); err != nil {
			slog.ErrorContext(matchCtx, "Failed to update player state for match", "player.id", player2ID, "error", err)
			matchSpan.RecordError(err)
			matchSpan.SetStatus(codes.Error, "Failed to update player2 for match")
		}

		payload, err := json.Marshal(events.MatchMadePayload{RoomID: roomID, PlayerIDs: []string{player1ID, player2ID}})
		if err != nil {
			slog.ErrorContext(matchCtx, "Failed to marshal match_made payload", "error", err)
			matchSpan.RecordError(err)
			matchSpan.SetStatus(codes.Error, "Failed to marshal match_made payload")
			matchSpan.End()
			continue
		}
		event, err := json.Marshal(events.Event{Type: "match_made", Payload: payload})
		if err != nil {
			slog.ErrorContext(matchCtx, "Failed to marshal event", "error", err)
			matchSpan.RecordError(err)
			matchSpan.SetStatus(codes.Error, "Failed to marshal event")
			matchSpan.End()
			continue
		}

		if err := h.rdb.Publish(matchCtx, events.EventsChannel, event).Err(); err != nil {
			slog.ErrorContext(matchCtx, "Failed to publish match_made event", "error", err)
			matchSpan.RecordError(err)
			matchSpan.SetStatus(codes.Error, "Failed to publish match_made event")
			matchSpan.End()
			continue
		}

		slog.InfoContext(matchCtx, "Room created and event published", "room.id", roomID, "player1.id", player1ID, "player2.id", player2ID)
		matchSpan.End()
	}
}
