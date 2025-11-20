package room

import (
	"context"
	"ctchen222/Tic-Tac-Toe/internal/events"
	"ctchen222/Tic-Tac-Toe/internal/game"
	"ctchen222/Tic-Tac-Toe/internal/player"
	"ctchen222/Tic-Tac-Toe/internal/validator"
	"ctchen222/Tic-Tac-Toe/pkg/proto"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/gorilla/websocket"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// HandleMessage handles a message from a player. It acts as a dispatcher.
func (r *Room) HandleMessage(p *player.Player, rawMessage []byte) {
	ctx := context.Background()
	ctx, span := tracer.Start(ctx, "room.HandleMessage", trace.WithAttributes(
		attribute.String("player.id", p.ID),
		attribute.String("room.id", r.ID),
	))
	defer span.End()

	// Lock is now handled within each case to avoid holding it during I/O

	if p.Status == player.StatusDisconnected {
		slog.WarnContext(ctx, "ignoring message from disconnected player", "player.id", p.ID)
		span.SetStatus(codes.Error, "Message from disconnected player")
		return
	}

	var message proto.ClientToServerMessage
	if err := json.Unmarshal(rawMessage, &message); err != nil {
		slog.ErrorContext(ctx, "error unmarshalling message", "error", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "Error unmarshalling message")
		return
	}

	if err := validator.GetValidator().Struct(message); err != nil {
		slog.WarnContext(ctx, "invalid message from player", "player.id", p.ID, "error", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid message format")
		return
	}

	span.SetAttributes(attribute.String("message.type", message.Type))

	switch message.Type {
	case "move":
		r.handleMove(ctx, p, &message)
	case "rematch":
		r.handleRematch(ctx, p, &message)
	case "leave_room":
		go r.closeAndReturnPlayersToLobby(ctx, p)
	}
}

// handleMove processes a player's move.
func (r *Room) handleMove(ctx context.Context, p *player.Player, message *proto.ClientToServerMessage) {
	r.mu.Lock()
	defer r.mu.Unlock()

	ctx, moveSpan := tracer.Start(ctx, "room.handleMove", trace.WithAttributes(
		attribute.String("player.id", p.ID),
		attribute.String("room.id", r.ID),
		attribute.Int("move.row", message.Position[0]),
		attribute.Int("move.col", message.Position[1]),
	))
	defer moveSpan.End()

	gameState, err := r.gameRepo.FindByID(ctx, r.ID)
	if err != nil {
		slog.ErrorContext(ctx, "handleMove could not find game state for room", "room.id", r.ID, "error", err)
		moveSpan.RecordError(err)
		moveSpan.SetStatus(codes.Error, "Could not find game state")
		return
	}

	var playerMark game.PlayerMark
	if p.ID == gameState.PlayerXID {
		playerMark = game.PlayerX
	} else if p.ID == gameState.PlayerOID {
		playerMark = game.PlayerO
	}

	if playerMark == "" {
		slog.WarnContext(ctx, "player is not part of room", "player.id", p.ID, "room.id", r.ID)
		moveSpan.SetStatus(codes.Error, "Player not part of room")
		return
	}

	updatedGameState, err := r.gameRepo.Update(ctx, r.ID, playerMark, message.Position[0], message.Position[1])
	if err != nil {
		slog.WarnContext(ctx, "invalid move from player", "player.id", p.ID, "error", err)
		moveSpan.SetAttributes(attribute.Bool("move.valid", false))
		moveSpan.RecordError(err)
		moveSpan.SetStatus(codes.Error, "Invalid move")
		// Consider sending an error message back to the player
		return
	}
	moveSpan.SetAttributes(attribute.Bool("move.valid", true))

	// Publish the entire updated game state
	payload, err := json.Marshal(updatedGameState)
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal updated game state", "room.id", r.ID, "error", err)
		moveSpan.RecordError(err)
		moveSpan.SetStatus(codes.Error, "Failed to marshal updated game state")
		return
	}

	roomChannel := fmt.Sprintf("channel:room:%s", r.ID)
	if err := r.rdb.Publish(ctx, roomChannel, payload).Err(); err != nil {
		slog.ErrorContext(ctx, "failed to publish update for room", "room.id", r.ID, "error", err)
		moveSpan.RecordError(err)
		moveSpan.SetStatus(codes.Error, "Failed to publish room update")
	}
}

// handleRematch processes a player's rematch request.
func (r *Room) handleRematch(ctx context.Context, p *player.Player, message *proto.ClientToServerMessage) {
	r.mu.Lock()
	defer r.mu.Unlock()

	ctx, span := tracer.Start(ctx, "room.handleRematch", trace.WithAttributes(
		attribute.String("player.id", p.ID),
		attribute.String("room.id", r.ID),
	))
	defer span.End()

	gameState, err := r.gameRepo.FindByID(ctx, r.ID)
	if err != nil {
		slog.ErrorContext(ctx, "could not get game state for rematch vote", "error", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "Could not get game state for rematch vote")
		return
	}

	if gameState.Winner == game.None && !gameState.IsDraw {
		slog.WarnContext(ctx, "Player requested rematch, but game is not over", "player.id", p.ID)
		span.SetStatus(codes.Error, "Rematch requested before game over")
		return
	}

	slog.InfoContext(ctx, "Player voted for a rematch", "player.id", p.ID, "room.id", r.ID)
	if err := r.gameRepo.RecordVote(ctx, r.ID, p.ID); err != nil {
		slog.ErrorContext(ctx, "failed to record rematch vote for player", "player.id", p.ID, "error", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to record rematch vote")
		return
	}

	var otherPlayerIsBot bool
	for _, other := range r.Players {
		if other.ID != p.ID && other.IsBot {
			otherPlayerIsBot = true
			break
		}
	}

	if otherPlayerIsBot {
		slog.InfoContext(ctx, "Bot auto-accepts rematch. Resetting game.", "room.id", r.ID)
		r.resetGameForRematch(ctx)
		return
	}

	allVotes, err := r.gameRepo.GetVotes(ctx, r.ID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get all votes for room", "room.id", r.ID, "error", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to get all votes")
		return
	}

	player1VoteKey := fmt.Sprintf("vote:%s", gameState.PlayerXID)
	player2VoteKey := fmt.Sprintf("vote:%s", gameState.PlayerOID)

	if allVotes[player1VoteKey] == "true" && allVotes[player2VoteKey] == "true" {
		slog.InfoContext(ctx, "All players voted for a rematch. Resetting game.", "room.id", r.ID)
		r.resetGameForRematch(ctx)
	} else {
		payload, _ := json.Marshal(events.RematchRequestedPayload{
			RoomID:   r.ID,
			PlayerID: p.ID,
		})
		event, _ := json.Marshal(events.Event{Type: "rematch_requested", Payload: payload})
		if err := r.rdb.Publish(ctx, events.EventsChannel, event).Err(); err != nil {
			slog.ErrorContext(ctx, "failed to publish rematch_requested event", "room.id", r.ID, "error", err)
			span.RecordError(err)
			span.SetStatus(codes.Error, "Failed to publish rematch_requested event")
		}
	}
}

func (r *Room) closeAndReturnPlayersToLobby(ctx context.Context, leavingPlayer *player.Player) {
	slog.InfoContext(ctx, "Closing room and returning players to lobby", "room.id", r.ID, "leaving_player.id", leavingPlayer.ID)

	r.mu.Lock()
	playersInRoom := make([]*player.Player, len(r.Players))
	copy(playersInRoom, r.Players)
	r.mu.Unlock()

	// Perform blocking operations outside the lock
	for _, p := range playersInRoom {
		// Notify the opponent that the other player has left
		if p.ID != leavingPlayer.ID && !p.IsBot {
			msg := &proto.ServerToClientMessage{Type: "opponent_left"}
			data, _ := json.Marshal(msg)
			if err := p.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
				slog.WarnContext(ctx, "Failed to send opponent_left message", "player.id", p.ID, "error", err)
			}
		}
	}

	// Send all non-bot players back to the lobby
	for _, p := range playersInRoom {
		if !p.IsBot {
			r.returnToLobby <- p
		}
	}

	if err := r.gameRepo.Delete(ctx, r.ID); err != nil {
		slog.ErrorContext(ctx, "Failed to delete game state from Redis", "room.id", r.ID, "error", err)
	}

	// Signal the room to close
	close(r.Done)
}
