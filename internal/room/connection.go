package room

import (
	"context"
	"ctchen222/Tic-Tac-Toe/internal/events"
	"ctchen222/Tic-Tac-Toe/internal/hub/types"
	"ctchen222/Tic-Tac-Toe/internal/player"
	"ctchen222/Tic-Tac-Toe/pkg/proto"
	"encoding/json"
	"log/slog"

	"github.com/gorilla/websocket"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Broadcast sends a message to all connected players in the room.
func (r *Room) Broadcast(message *proto.ServerToClientMessage) {
	ctx := context.Background()
	_, span := tracer.Start(ctx, "room.Broadcast", trace.WithAttributes(
		attribute.String("room.id", r.ID),
		attribute.String("message.type", message.Type),
	))
	defer span.End()

	data, err := json.Marshal(message)
	if err != nil {
		slog.ErrorContext(ctx, "error marshalling message", "error", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "Error marshalling message")
		return
	}

	for _, p := range r.Players {
		if p.Status == player.StatusConnected {
			if err := p.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
				slog.ErrorContext(ctx, "error writing message to player", "player.id", p.ID, "error", err)
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
			slog.ErrorContext(disconnectCtx, "Failed to set player status to disconnected", "player.id", p.ID, "error", err)
			disconnectSpan.RecordError(err)
			disconnectSpan.SetStatus(codes.Error, "Failed to set player status to disconnected")
		}

		payload, _ := json.Marshal(events.PlayerDisconnectedPayload{
			RoomID:   r.ID,
			PlayerID: p.ID,
		})
		event, _ := json.Marshal(events.Event{Type: "player_disconnected", Payload: payload})
		if err := r.rdb.Publish(disconnectCtx, events.EventsChannel, event).Err(); err != nil {
			slog.ErrorContext(disconnectCtx, "Failed to publish player_disconnected event", "player.id", p.ID, "error", err)
			disconnectSpan.RecordError(err)
			disconnectSpan.SetStatus(codes.Error, "Failed to publish player_disconnected event")
		}
		slog.InfoContext(disconnectCtx, "Player disconnected. Updated status and published event.", "player.id", p.ID)
	}()

	for {
		_, msg, err := p.Conn.ReadMessage()
		if err != nil {
			slog.WarnContext(ctx, "Player connection error", "player.id", p.ID, "room.id", r.ID, "error", err)
			span.RecordError(err)
			span.SetStatus(codes.Error, "Player connection error")
			return
		}
		r.incomingMoves <- &types.PlayerMove{Player: p, Message: msg}
	}
}
