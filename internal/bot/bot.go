package bot

import (
	"ctchen222/Tic-Tac-Toe/internal/game"
	"ctchen222/Tic-Tac-Toe/internal/player"
	"ctchen222/Tic-Tac-Toe/pkg/proto"
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"
)

// BotConnection simulates a websocket connection for a bot player.
// It implements the player.Connection interface.
type BotConnection struct {
	playerID   string
	moveChan   chan []byte
	mark       game.PlayerMark // Stores the bot's mark ('X' or 'O')
	difficulty string          // Stores the bot's difficulty
}

// NewBotConnection creates a new connection for a bot.
func NewBotConnection(playerID string, difficulty string) *BotConnection {
	return &BotConnection{
		playerID:   playerID,
		moveChan:   make(chan []byte, 1),
		difficulty: difficulty,
	}
}

// WriteMessage is called by the room to send game state to the bot.
func (bc *BotConnection) WriteMessage(messageType int, data []byte) error {
	// First, try to unmarshal as a generic message to find the type
	var genericMsg map[string]any
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
		log.Printf("Bot %s assigned mark: %s", bc.playerID, bc.mark)

	case "update":
		var msg proto.ServerToClientMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return err
		}

		// The bot only acts if it has a mark, it's its turn, and there is no winner
		if bc.mark != "" && msg.Next == bc.mark && msg.Winner == "" {
			log.Printf("Bot %s (as %s) is thinking...", bc.playerID, bc.mark)
			time.Sleep(1 * time.Second) // Simulate thinking time

			row, col := CalculateNextMove(msg.Board, bc.mark, bc.difficulty)

			if row != -1 {
				move := proto.ClientToServerMessage{
					Type:     "move",
					Position: []int{row, col},
				}
				moveBytes, _ := json.Marshal(move)
				bc.moveChan <- moveBytes
			}
		}
	}

	return nil
}

// ReadMessage is called by the room to get the bot's next move.
func (bc *BotConnection) ReadMessage() (int, []byte, error) {
	// Block until the bot's logic (in WriteMessage) produces a move.
	move := <-bc.moveChan
	return 1, move, nil // 1 = TextMessage
}

// Close is a no-op for the bot.
func (bc *BotConnection) Close() error {
	return nil
}

// NewBotPlayer creates a new player instance that is a bot.
func NewBotPlayer(difficulty string) *player.Player {
	botID := "bot-" + uuid.New().String()[:8]
	botConn := NewBotConnection(botID, difficulty)
	p := player.NewPlayer(botID, botConn)
	p.IsBot = true
	return p
}

