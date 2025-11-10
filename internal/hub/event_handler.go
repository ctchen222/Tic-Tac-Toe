package hub

import (
	"context"
	"ctchen222/Tic-Tac-Toe/internal/bot"
	"ctchen222/Tic-Tac-Toe/internal/events"
	"ctchen222/Tic-Tac-Toe/internal/player"
	"ctchen222/Tic-Tac-Toe/internal/room"
	"ctchen222/Tic-Tac-Toe/pkg/proto"
	"encoding/json"
	"log/slog"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func (h *Hub) runEventSubscriber(ctx context.Context) {
	slog.InfoContext(ctx, "Event subscriber started", "channel", events.EventsChannel)
	pubsub := h.rdb.Subscribe(ctx, events.EventsChannel)
	defer pubsub.Close()

	ch := pubsub.Channel()

	for msg := range ch {
		eventCtx, eventSpan := tracer.Start(ctx, "hub.handleEvent", trace.WithAttributes(
			attribute.String("event.channel", events.EventsChannel),
			attribute.String("event.payload", msg.Payload),
		))
		defer eventSpan.End()

		var event events.Event
		if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
			var oldFormat map[string]string
			if errOld := json.Unmarshal([]byte(msg.Payload), &oldFormat); errOld == nil {
				if oldFormat["event"] == "player_disconnected" {
					h.handlePlayerDisconnected(eventCtx, &events.PlayerDisconnectedPayload{
						RoomID:   oldFormat["room_id"],
						PlayerID: oldFormat["player_id"],
					})
				}
			} else {
				slog.ErrorContext(eventCtx, "Could not unmarshal global event", "error", err)
				eventSpan.RecordError(err)
				eventSpan.SetStatus(codes.Error, "Could not unmarshal global event")
			}
			continue
		}
		eventSpan.SetAttributes(attribute.String("event.type", event.Type))

		switch event.Type {
		case "match_made":
			var payload events.MatchMadePayload
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				slog.ErrorContext(eventCtx, "Could not unmarshal match_made payload", "error", err)
				eventSpan.RecordError(err)
				eventSpan.SetStatus(codes.Error, "Could not unmarshal match_made payload")
				continue
			}
			h.handleMatchMade(eventCtx, &payload)

		case "player_disconnected":
			var payload events.PlayerDisconnectedPayload
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				slog.ErrorContext(eventCtx, "Could not unmarshal player_disconnected payload", "error", err)
				eventSpan.RecordError(err)
				eventSpan.SetStatus(codes.Error, "Could not unmarshal player_disconnected payload")
				continue
			}
			h.handlePlayerDisconnected(eventCtx, &payload)

		case "player_reconnected":
			var payload events.PlayerReconnectedPayload
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				slog.ErrorContext(eventCtx, "Could not unmarshal player_reconnected payload", "error", err)
				eventSpan.RecordError(err)
				eventSpan.SetStatus(codes.Error, "Could not unmarshal player_reconnected payload")
				continue
			}
			h.handlePlayerReconnected(eventCtx, &payload)

		case "rematch_requested":
			var payload events.RematchRequestedPayload
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				slog.ErrorContext(eventCtx, "Could not unmarshal rematch_requested payload", "error", err)
				eventSpan.RecordError(err)
				eventSpan.SetStatus(codes.Error, "Could not unmarshal rematch_requested payload")
				continue
			}
			h.handleRematchRequested(eventCtx, &payload)

		case "rematch_successful":
			var payload events.RematchSuccessfulPayload
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				slog.ErrorContext(eventCtx, "Could not unmarshal rematch_successful payload", "error", err)
				eventSpan.RecordError(err)
				eventSpan.SetStatus(codes.Error, "Could not unmarshal rematch_successful payload")
				continue
			}
			h.handleRematchSuccessful(eventCtx, &payload)
		}
	}
}

func (h *Hub) handleMatchMade(ctx context.Context, payload *events.MatchMadePayload) {
	ctx, span := tracer.Start(ctx, "hub.handleMatchMade", trace.WithAttributes(
		attribute.String("room.id", payload.RoomID),
		attribute.Int("player.count", len(payload.PlayerIDs)),
	))
	defer span.End()

	slog.InfoContext(ctx, "Received match_made event", "room.id", payload.RoomID)

	var localPlayersInRoom []*player.Player
	for _, playerID := range payload.PlayerIDs {
		if p, isLocal := h.localPlayers[playerID]; isLocal {
			localPlayersInRoom = append(localPlayersInRoom, p)
		}
	}

	if len(localPlayersInRoom) > 0 {
		slog.InfoContext(ctx, "Found local players for room, creating handler", "local_players.count", len(localPlayersInRoom), "room.id", payload.RoomID)
		h.createAndStartRoom(ctx, payload.RoomID, localPlayersInRoom)
	}
}

func (h *Hub) handlePlayerDisconnected(ctx context.Context, payload *events.PlayerDisconnectedPayload) {
	ctx, span := tracer.Start(ctx, "hub.handlePlayerDisconnected", trace.WithAttributes(
		attribute.String("room.id", payload.RoomID),
		attribute.String("player.id", payload.PlayerID),
	))
	defer span.End()

	slog.InfoContext(ctx, "Received player_disconnected event", "player.id", payload.PlayerID, "room.id", payload.RoomID)

	if room, ok := h.localRooms[payload.RoomID]; ok {
		room.HandleOpponentDisconnected()
	}
}

func (h *Hub) handlePlayerReconnected(ctx context.Context, payload *events.PlayerReconnectedPayload) {
	ctx, span := tracer.Start(ctx, "hub.handlePlayerReconnected", trace.WithAttributes(
		attribute.String("room.id", payload.RoomID),
		attribute.String("player.id", payload.PlayerID),
	))
	defer span.End()

	slog.InfoContext(ctx, "Received player_reconnected event", "player.id", payload.PlayerID, "room.id", payload.RoomID)

	if room, ok := h.localRooms[payload.RoomID]; ok {
		room.HandleOpponentReconnected()
	}
}

func (h *Hub) handleRematchSuccessful(ctx context.Context, payload *events.RematchSuccessfulPayload) {
	ctx, span := tracer.Start(ctx, "hub.handleRematchSuccessful", trace.WithAttributes(
		attribute.String("room.id", payload.RoomID),
	))
	defer span.End()

	slog.InfoContext(ctx, "Received rematch_successful event", "room.id", payload.RoomID)

	if room, ok := h.localRooms[payload.RoomID]; ok {
		// Resend assignments and initial state to all players in the room
		h.sendInitialRoomState(ctx, room, room.Players)
	}
}

func (h *Hub) handleRematchRequested(ctx context.Context, payload *events.RematchRequestedPayload) {
	ctx, span := tracer.Start(ctx, "hub.handleRematchRequested", trace.WithAttributes(
		attribute.String("room.id", payload.RoomID),
		attribute.String("player.id", payload.PlayerID),
	))
	defer span.End()

	slog.InfoContext(ctx, "Received rematch_requested event", "player.id", payload.PlayerID, "room.id", payload.RoomID)

	if room, ok := h.localRooms[payload.RoomID]; ok {
		for _, p := range room.Players {
			if p.ID != payload.PlayerID {
				msg := &proto.ServerToClientMessage{Type: "rematch_requested"}
				data, _ := json.Marshal(msg)
				if p.Conn != nil {
					if err := p.Conn.WriteMessage(1, data); err != nil {
						slog.ErrorContext(ctx, "Error sending rematch_requested to player", "player.id", p.ID, "error", err)
						span.RecordError(err)
						span.SetStatus(codes.Error, "Error sending rematch_requested")
					}
				}
			}
		}
	}
}

// createAndStartRoom is a helper to create a room and start its goroutines.
func (h *Hub) createAndStartRoom(ctx context.Context, roomID string, localPlayers []*player.Player) {
	ctx, span := tracer.Start(ctx, "hub.createAndStartRoom", trace.WithAttributes(
		attribute.String("room.id", roomID),
		attribute.Int("local_players.count", len(localPlayers)),
	))
	defer span.End()

	moveCalculator := &bot.BotMoveCalculator{}
	newRoom := room.NewRoom(roomID, h.rdb, h.gameRepo, h.playerRepo, moveCalculator, moveTimeout)
	for _, p := range localPlayers {
		newRoom.AddPlayer(p)
	}
	h.localRooms[roomID] = newRoom

	go newRoom.Start(h.unregister)
	go h.runRoomUpdateSubscriber(ctx, newRoom)
	h.sendInitialRoomState(ctx, newRoom, localPlayers)
}
