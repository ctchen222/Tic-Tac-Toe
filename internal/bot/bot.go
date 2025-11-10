package bot

import (
	"ctchen222/Tic-Tac-Toe/internal/game"
	"ctchen222/Tic-Tac-Toe/internal/hub/types"
	"ctchen222/Tic-Tac-Toe/internal/player"
	"ctchen222/Tic-Tac-Toe/pkg/proto"
	"encoding/json"
	"io"
	"log/slog"
	"time"
)

// BotConnection simulates a websocket connection for a bot player.
// It implements the player.Connection interface.
type BotConnection struct {
	playerID      string
	player        *player.Player
	incomingMoves chan<- *types.PlayerMove
	mark          game.PlayerMark
	difficulty    string
}

// NewBotConnection creates a new connection for a bot.
func NewBotConnection(playerID string, difficulty string, p *player.Player, incomingMoves chan<- *types.PlayerMove) *BotConnection {
	return &BotConnection{
		playerID:      playerID,
		player:        p,
		incomingMoves: incomingMoves,
		difficulty:    difficulty,
	}
}

// WriteMessage is called by the room to send game state to the bot.
func (bc *BotConnection) WriteMessage(messageType int, data []byte) error {
	// First, try to unmarshal as a generic message to find the type
	var genericMsg map[string]interface{}
	if err := json.Unmarshal(data, &genericMsg); err != nil {
		return err
	}

	msgType, ok := genericMsg["type"].(string)
	if !ok {
		return nil // Not a valid message for the bot
	}

	switch msgType {
	case "assignment":
		var msg proto.PlayerAssignmentMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return err
		}
		bc.mark = msg.Mark
		slog.Info("Bot has been assigned mark", "bot.id", bc.playerID, "mark", bc.mark)

	case "update":
		var msg proto.ServerToClientMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return err
		}

		// The bot only acts if it has a mark, it's its turn, and there is no winner
		if bc.mark != "" && msg.Next == bc.mark && msg.Winner == "" {
			slog.Info("Bot is thinking...", "bot.id", bc.playerID, "mark", bc.mark)
			time.Sleep(1 * time.Second) // Simulate thinking time

			row, col := CalculateNextMove(msg.Board, bc.mark, bc.difficulty)

			if row != -1 {
				slog.Info("Bot calculated move. Injecting into room.", "bot.id", bc.playerID, "row", row, "col", col)
				move := proto.ClientToServerMessage{
					Type:     "move",
					Position: []int{row, col},
				}
				moveBytes, _ := json.Marshal(move)

				// Directly inject the move into the room's incoming channel
				moveToSend := &types.PlayerMove{
					Player:  bc.player,
					Message: moveBytes,
				}
				bc.incomingMoves <- moveToSend

			} else {
				slog.Warn("Bot calculated no valid move.", "bot.id", bc.playerID)
			}
		}
	}

	return nil
}

// ReadMessage is called by the ReadPump. For a bot, we don't read from a real
// connection. We return an EOF error immediately to signal the ReadPump to exit,
// preventing it from blocking forever.
func (bc *BotConnection) ReadMessage() (int, []byte, error) {
	// Returning an error will cause the ReadPump to terminate, which is what we want.
	return 0, nil, io.EOF
}

// Close is a no-op for the bot.
func (bc *BotConnection) Close() error {
	return nil
}