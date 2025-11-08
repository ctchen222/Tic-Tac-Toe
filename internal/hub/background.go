package hub

import (
	"context"
	"ctchen222/Tic-Tac-Toe/internal/events"
	"ctchen222/Tic-Tac-Toe/internal/game"
	"ctchen222/Tic-Tac-Toe/internal/room"
	"ctchen222/Tic-Tac-Toe/pkg/proto"
	"encoding/json"
	"fmt"
	"log"
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
	log.Printf("Starting subscriber for room %s on channel %s", room.ID, roomChannel)
	pubsub := h.rdb.Subscribe(ctx, roomChannel)
	defer pubsub.Close()

	ch := pubsub.Channel()
	for msg := range ch {
		updateCtx, updateSpan := tracer.Start(ctx, "hub.handleRoomUpdate", trace.WithAttributes(
			attribute.String("room.id", room.ID),
			attribute.String("redis.payload", msg.Payload),
		))
		defer updateSpan.End()

		log.Printf("Received update for room %s, payload: %s", room.ID, msg.Payload)
		gameState, err := h.gameRepo.FindByID(updateCtx, room.ID)
		if err != nil {
			log.Printf("Room subscriber for %s could not get game state: %v", room.ID, err)
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
	log.Printf("Stopping subscriber for room %s", room.ID)
}

func (h *Hub) runMatcher(ctx context.Context) {
	log.Println("Redis-based matcher started")
	for {
		//TODO: refactor span error handling
		matchCtx, matchSpan := tracer.Start(ctx, "hub.runMatcher.matchAttempt")

		player1ID, player2ID, err := h.matchmakingRepo.GetPlayersFromQueue(matchCtx)
		if err != nil {
			log.Printf("Error getting players from queue: %v", err)
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
			log.Printf("Failed to create new game in Redis for room %s: %v", roomID, err)
			log.Println("Re-queuing players...")
			matchSpan.RecordError(err)
			matchSpan.SetStatus(codes.Error, "Failed to create game in Redis")
			if err := h.matchmakingRepo.AddToQueue(matchCtx, player1ID); err != nil {
				log.Printf("FATAL: Failed to re-queue player %s: %v", player1ID, err)
				matchSpan.RecordError(err)
				matchSpan.SetStatus(codes.Error, "FATAL: Failed to re-queue player1")
			}
			if err := h.matchmakingRepo.AddToQueue(matchCtx, player2ID); err != nil {
				log.Printf("FATAL: Failed to re-queue player %s: %v", player2ID, err)
				matchSpan.RecordError(err)
				matchSpan.SetStatus(codes.Error, "FATAL: Failed to re-queue player2")
			}
			matchSpan.End()
			continue
		}

		if err := h.playerRepo.UpdateForMatch(matchCtx, player1ID, roomID); err != nil {
			log.Printf("Failed to update player %s state for match: %v", player1ID, err)
			matchSpan.RecordError(err)
			matchSpan.SetStatus(codes.Error, "Failed to update player1 for match")
		}
		if err := h.playerRepo.UpdateForMatch(matchCtx, player2ID, roomID); err != nil {
			log.Printf("Failed to update player %s state for match: %v", player2ID, err)
			matchSpan.RecordError(err)
			matchSpan.SetStatus(codes.Error, "Failed to update player2 for match")
		}

		payload, err := json.Marshal(events.MatchMadePayload{RoomID: roomID, PlayerIDs: []string{player1ID, player2ID}})
		if err != nil {
			log.Printf("Failed to marshal match_made payload: %v", err)
			matchSpan.RecordError(err)
			matchSpan.SetStatus(codes.Error, "Failed to marshal match_made payload")
			matchSpan.End()
			continue
		}
		event, err := json.Marshal(events.Event{Type: "match_made", Payload: payload})
		if err != nil {
			log.Printf("Failed to marshal event: %v", err)
			matchSpan.RecordError(err)
			matchSpan.SetStatus(codes.Error, "Failed to marshal event")
			matchSpan.End()
			continue
		}

		if err := h.rdb.Publish(matchCtx, events.EventsChannel, event).Err(); err != nil {
			log.Printf("Failed to publish match_made event: %v", err)
			matchSpan.RecordError(err)
			matchSpan.SetStatus(codes.Error, "Failed to publish match_made event")
			matchSpan.End()
			continue
		}

		log.Printf("Room %s created for players %s and %s. Event published.", roomID, player1ID, player2ID)
		matchSpan.End()
	}
}
