package room

import (
	"context"
	"ctchen222/Tic-Tac-Toe/internal/events"
	"ctchen222/Tic-Tac-Toe/internal/hub/types"
	"ctchen222/Tic-Tac-Toe/internal/player"
	"ctchen222/Tic-Tac-Toe/pkg/proto"
	"encoding/json"
	"log"

	"github.com/gorilla/websocket"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Broadcast sends a message to all connected players in the room.
func (r *Room) Broadcast(message *proto.ServerToClientMessage) {
	_, span := tracer.Start(context.Background(), "room.Broadcast", trace.WithAttributes(
		attribute.String("room.id", r.ID),
		attribute.String("message.type", message.Type),
	))
	defer span.End()

	data, err := json.Marshal(message)
	if err != nil {
		log.Printf("error marshalling message: %v", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "Error marshalling message")
		return
	}

	for _, p := range r.Players {
		if p.Status == player.StatusConnected {
			if err := p.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Printf("error writing message to player %s: %v", p.ID, err)
				span.RecordError(err)
				span.SetStatus(codes.Error, "Error writing message to player")
			}
		}
	}
}

// ReadPump pumps messages from the websocket connection to the room's incomingMoves channel.
func (r *Room) ReadPump(p *player.Player) {
	ctx, span := tracer.Start(context.Background(), "room.ReadPump", trace.WithAttributes(
		attribute.String("player.id", p.ID),
		attribute.String("room.id", r.ID),
	))
	defer span.End()

	defer func() {
		p.Conn.Close()
		disconnectCtx, disconnectSpan := tracer.Start(ctx, "room.ReadPump.disconnectHandler", trace.WithAttributes(
			attribute.String("player.id", p.ID),
			attribute.String("room.id", r.ID),
		))
		defer disconnectSpan.End()

		if err := r.playerRepo.UpdateConnectionStatus(disconnectCtx, p.ID, player.StatusDisconnected); err != nil {
			log.Printf("Failed to set player %s status to disconnected: %v", p.ID, err)
			disconnectSpan.RecordError(err)
			disconnectSpan.SetStatus(codes.Error, "Failed to set player status to disconnected")
		}

		payload, _ := json.Marshal(events.PlayerDisconnectedPayload{
			RoomID:   r.ID,
			PlayerID: p.ID,
		})
		event, _ := json.Marshal(events.Event{Type: "player_disconnected", Payload: payload})
		if err := r.rdb.Publish(disconnectCtx, events.EventsChannel, event).Err(); err != nil {
			log.Printf("Failed to publish player_disconnected event for player %s: %v", p.ID, err)
			disconnectSpan.RecordError(err)
			disconnectSpan.SetStatus(codes.Error, "Failed to publish player_disconnected event")
		}
		log.Printf("Player %s disconnected. Updated status and published event.", p.ID)
	}()

	for {
		_, msg, err := p.Conn.ReadMessage()
		if err != nil {
			log.Printf("Player %s connection error in room %s: %v", p.ID, r.ID, err)
			span.RecordError(err)
			span.SetStatus(codes.Error, "Player connection error")
			return
		}
		r.incomingMoves <- &types.PlayerMove{Player: p, Message: msg}
	}
}
