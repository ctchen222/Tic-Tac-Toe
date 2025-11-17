package hub

import (
	"context"
	"ctchen222/Tic-Tac-Toe/internal/player"
	"ctchen222/Tic-Tac-Toe/pkg/proto"
	"encoding/json"
	"log/slog"

	"github.com/gorilla/websocket"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// LobbyReadPump pumps messages from the websocket connection for a player in the lobby.
// It ensures that a player is unregistered from the hub when their connection is closed.
func (h *Hub) LobbyReadPump(p *player.Player) {
	defer func() {
		slog.Info("LobbyReadPump stopped for player, unregistering", "player.id", p.ID)
		h.unregister <- p
	}()

	for {
		_, msg, err := p.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Warn("LobbyReadPump connection error", "player.id", p.ID, "error", err)
			} else {
				slog.Info("Lobby player disconnected", "player.id", p.ID)
			}
			break
		}
		h.HandleLobbyMessage(p, msg)
	}
}

// HandleLobbyMessage handles messages from a player who is in the lobby.
func (h *Hub) HandleLobbyMessage(p *player.Player, rawMessage []byte) {
	ctx, span := tracer.Start(context.Background(), "hub.HandleLobbyMessage", trace.WithAttributes(
		attribute.String("player.id", p.ID),
	))
	defer span.End()

	var msg proto.ClientToServerMessage
	if err := json.Unmarshal(rawMessage, &msg); err != nil {
		slog.ErrorContext(ctx, "Failed to unmarshal lobby message", "error", err, "raw_message", string(rawMessage))
		return
	}

	slog.InfoContext(ctx, "Received message from lobby player", "player.id", p.ID, "message_type", msg.Type)

	switch msg.Type {
	case "join_queue":
		if p.State != player.StateLobby {
			slog.WarnContext(ctx, "Player tried to join queue but was not in lobby", "player.id", p.ID, "current_state", p.State)
			return
		}
		h.queuePlayerForMatchmaking(ctx, p)

	case "start_bot_game":
		if p.State != player.StateLobby {
			slog.WarnContext(ctx, "Player tried to start bot game but was not in lobby", "player.id", p.ID, "current_state", p.State)
			return
		}
		difficulty := "medium" // Default difficulty
		if msg.Difficulty != "" {
			difficulty = msg.Difficulty
		}
		h.registerBotGame(ctx, p, difficulty)

	default:
		slog.WarnContext(ctx, "Unknown message type from lobby player", "message_type", msg.Type)
		// Optionally send an error message back to the client
		response := &proto.ServerToClientMessage{Type: "error", Message: "Unknown command"}
		data, _ := json.Marshal(response)
		if err := p.Conn.WriteMessage(1, data); err != nil {
			slog.ErrorContext(ctx, "Failed to write error message to lobby player", "player.id", p.ID, "error", err)
		}
	}
}
