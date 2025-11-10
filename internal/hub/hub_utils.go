package hub

import (
	"context"
	"ctchen222/Tic-Tac-Toe/internal/game"
	"ctchen222/Tic-Tac-Toe/internal/player"
	"ctchen222/Tic-Tac-Toe/internal/room"
	"ctchen222/Tic-Tac-Toe/pkg/proto"
	"encoding/json"
	"log/slog"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func (h *Hub) sendInitialRoomState(ctx context.Context, room *room.Room, localPlayers []*player.Player) {
	ctx, span := tracer.Start(ctx, "hub.sendInitialRoomState", trace.WithAttributes(
		attribute.String("room.id", room.ID),
		attribute.Int("local_players.count", len(localPlayers)),
	))
	defer span.End()

	slog.InfoContext(ctx, "Sending initial room state", "room.id", room.ID, "local_players.count", len(localPlayers))

	initialGameState, err := h.gameRepo.FindByID(ctx, room.ID)
	if err != nil {
		slog.ErrorContext(ctx, "Could not get initial game state", "room.id", room.ID, "error", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "Could not get initial game state")
		return
	}

	for _, p := range localPlayers {
		var mark game.PlayerMark
		if p.ID == initialGameState.PlayerXID {
			mark = game.PlayerX
		} else if p.ID == initialGameState.PlayerOID {
			mark = game.PlayerO
		} else {
			continue
		}
		assignmentMessage := &proto.PlayerAssignmentMessage{Type: "assignment", Mark: mark}
		data, _ := json.Marshal(assignmentMessage)
		if p.Conn != nil {
			if err := p.Conn.WriteMessage(1, data); err != nil {
				slog.ErrorContext(ctx, "Error sending assignment to player", "player.id", p.ID, "error", err)
				span.RecordError(err)
				span.SetStatus(codes.Error, "Error sending assignment to player")
			}
		}
	}

	initialUpdate := &proto.ServerToClientMessage{
		Type:   "update",
		Board:  game.BoardArrayToSlice(initialGameState.Board),
		Next:   initialGameState.CurrentTurn,
		Winner: initialGameState.Winner,
	}
	room.Broadcast(initialUpdate)
}
