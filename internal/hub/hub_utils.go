package hub

import (
	"context"
	"ctchen222/Tic-Tac-Toe/internal/game"
	"ctchen222/Tic-Tac-Toe/internal/player"
	"ctchen222/Tic-Tac-Toe/internal/room"
	"ctchen222/Tic-Tac-Toe/pkg/proto"
	"encoding/json"
	"log"

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

	log.Printf("Sending assignments/state for room %s to %d local players.", room.ID, len(localPlayers))

	initialGameState, err := h.gameRepo.FindByID(ctx, room.ID)
	if err != nil {
		log.Printf("Could not get initial game state for room %s: %v", room.ID, err)
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
				log.Printf("Error sending assignment to player %s: %v", p.ID, err)
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
