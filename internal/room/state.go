package room

import (
	"context"
	"ctchen222/Tic-Tac-Toe/internal/events"
	"ctchen222/Tic-Tac-Toe/pkg/proto"
	"encoding/json"
	"log"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// resetGameForRematch resets the game state and clears rematch votes.
func (r *Room) resetGameForRematch(ctx context.Context) {
	ctx, span := tracer.Start(ctx, "room.resetGameForRematch", trace.WithAttributes(
		attribute.String("room.id", r.ID),
	))
	defer span.End()

	oldGameState, err := r.gameRepo.FindByID(ctx, r.ID)
	if err != nil {
		log.Printf("failed to get old game state before rematch reset: %v", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to get old game state before rematch reset")
		return
	}

	err = r.gameRepo.Create(ctx, r.ID, oldGameState.PlayerOID, oldGameState.PlayerXID)
	if err != nil {
		log.Printf("failed to reset game for rematch in redis: %v", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to reset game for rematch in redis")
		return
	}

	if err := r.gameRepo.ClearVotes(ctx, r.ID, oldGameState.PlayerXID, oldGameState.PlayerOID); err != nil {
		log.Printf("failed to clean up votes for room %s: %v", r.ID, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to clean up votes for room")
	}

	log.Printf("Room %s game reset in Redis for rematch and votes cleaned.", r.ID)

	// Publish a global event to notify hubs to resend assignments and state
	payload, _ := json.Marshal(events.RematchSuccessfulPayload{
		RoomID: r.ID,
	})
	event, _ := json.Marshal(events.Event{Type: "rematch_successful", Payload: payload})
	if err := r.rdb.Publish(ctx, events.EventsChannel, event).Err(); err != nil {
		log.Printf("failed to publish rematch_successful event for room %s: %v", r.ID, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to publish rematch_successful event")
	}
}

// HandleOpponentDisconnected broadcasts a message to local players that the opponent has disconnected.
func (r *Room) HandleOpponentDisconnected() {
	_, span := tracer.Start(context.Background(), "room.HandleOpponentDisconnected", trace.WithAttributes(
		attribute.String("room.id", r.ID),
	))
	defer span.End()

	msg := &proto.ServerToClientMessage{Type: "opponent_disconnected"}
	r.Broadcast(msg)
}

// HandleOpponentReconnected broadcasts a message to local players that the opponent has reconnected.
func (r *Room) HandleOpponentReconnected() {
	_, span := tracer.Start(context.Background(), "room.HandleOpponentReconnected", trace.WithAttributes(
		attribute.String("room.id", r.ID),
	))
	defer span.End()

	msg := &proto.ServerToClientMessage{Type: "opponent_reconnected"}
	r.Broadcast(msg)
}
