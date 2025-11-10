package bot

import (
	"ctchen222/Tic-Tac-Toe/internal/game"
	"ctchen222/Tic-Tac-Toe/internal/hub/types"
	"ctchen222/Tic-Tac-Toe/internal/player"
	"ctchen222/Tic-Tac-Toe/pkg/proto"
	"encoding/json"
	"io"
	"testing"
	"time"
)

func TestNewBotConnection(t *testing.T) {
	playerID := "testBot"
	difficulty := "easy"
	p := &player.Player{ID: playerID}
	incomingMoves := make(chan *types.PlayerMove, 1) // Buffered channel for testing

	bc := NewBotConnection(playerID, difficulty, p, incomingMoves)

	if bc.playerID != playerID {
		t.Errorf("Expected playerID %s, got %s", playerID, bc.playerID)
	}
	if bc.difficulty != difficulty {
		t.Errorf("Expected difficulty %s, got %s", difficulty, bc.difficulty)
	}
	if bc.player != p {
		t.Error("Expected player to be set correctly")
	}
	if bc.incomingMoves != incomingMoves {
		t.Error("Expected incomingMoves channel to be set correctly")
	}
	if bc.mark != "" {
		t.Errorf("Expected initial mark to be empty, got %s", bc.mark)
	}
}

func TestBotConnection_WriteMessage_Assignment(t *testing.T) {
	bc := NewBotConnection("testBot", "easy", &player.Player{}, make(chan *types.PlayerMove, 1))
	assignmentMsg := proto.PlayerAssignmentMessage{
		Type: "assignment",
		Mark: game.PlayerX,
	}
	data, _ := json.Marshal(assignmentMsg)

	err := bc.WriteMessage(1, data)
	if err != nil {
		t.Fatalf("WriteMessage failed: %v", err)
	}

	if bc.mark != game.PlayerX {
		t.Errorf("Expected bot mark to be %s, got %s", game.PlayerX, bc.mark)
	}
}

func TestBotConnection_WriteMessage_Update_BotTurn_MakesMove(t *testing.T) {
	p := &player.Player{ID: "testBot"}
	incomingMoves := make(chan *types.PlayerMove, 1)
	bc := NewBotConnection(p.ID, "easy", p, incomingMoves)
	bc.mark = game.PlayerX // Assign mark first

	updateMsg := proto.ServerToClientMessage{
		Type:   "update",
		Board:  [][]game.PlayerMark{{"", "", ""}, {"", "", ""}, {"", "", ""}},
		Next:   game.PlayerX, // It's bot's turn
		Winner: "",
	}
	data, _ := json.Marshal(updateMsg)

	go func() {
		err := bc.WriteMessage(1, data)
		if err != nil {
			t.Errorf("WriteMessage failed: %v", err)
		}
	}()

	select {
	case moveToSend := <-incomingMoves:
		if moveToSend.Player != p {
			t.Errorf("Expected move from player %v, got %v", p, moveToSend.Player)
		}
		var move proto.ClientToServerMessage
		err := json.Unmarshal(moveToSend.Message, &move)
		if err != nil {
			t.Fatalf("Failed to unmarshal bot's move: %v", err)
		}
		if move.Type != "move" || len(move.Position) != 2 {
			t.Errorf("Invalid move received from bot: %+v", move)
		}
		// Further checks could be added here for the move's validity on the board
	case <-time.After(2 * time.Second): // Allow time for bot's thinking (1s) + processing
		t.Fatal("Bot did not make a move within the expected time")
	}
}

func TestBotConnection_WriteMessage_Update_NotBotTurn_NoMove(t *testing.T) {
	p := &player.Player{ID: "testBot"}
	incomingMoves := make(chan *types.PlayerMove, 1)
	bc := NewBotConnection(p.ID, "easy", p, incomingMoves)
	bc.mark = game.PlayerX

	updateMsg := proto.ServerToClientMessage{
		Type:   "update",
		Board:  [][]game.PlayerMark{{"", "", ""}, {"", "", ""}, {"", "", ""}},
		Next:   game.PlayerO, // Not bot's turn
		Winner: "",
	}
	data, _ := json.Marshal(updateMsg)

	go func() {
		err := bc.WriteMessage(1, data)
		if err != nil {
			t.Errorf("WriteMessage failed: %v", err)
		}
	}()

	select {
	case <-incomingMoves:
		t.Error("Bot made a move when it wasn't its turn")
	case <-time.After(100 * time.Millisecond):
		// Expected behavior: Bot does not make a move
	}
}

func TestBotConnection_WriteMessage_Update_GameEnded_NoMove(t *testing.T) {
	p := &player.Player{ID: "testBot"}
	incomingMoves := make(chan *types.PlayerMove, 1)
	bc := NewBotConnection(p.ID, "easy", p, incomingMoves)
	bc.mark = game.PlayerX

	updateMsg := proto.ServerToClientMessage{
		Type:   "update",
		Board:  [][]game.PlayerMark{{"", "", ""}, {"", "", ""}, {"", "", ""}},
		Next:   game.PlayerX,
		Winner: game.PlayerO, // Game ended
	}
	data, _ := json.Marshal(updateMsg)

	go func() {
		err := bc.WriteMessage(1, data)
		if err != nil {
			t.Errorf("WriteMessage failed: %v", err)
		}
	}()

	select {
	case <-incomingMoves:
		t.Error("Bot made a move when the game had already ended")
	case <-time.After(100 * time.Millisecond):
		// Expected behavior: Bot does not make a move
	}
}

func TestBotConnection_ReadMessage(t *testing.T) {
	bc := NewBotConnection("testBot", "easy", &player.Player{}, make(chan *types.PlayerMove, 1))
	_, _, err := bc.ReadMessage()
	if err != io.EOF {
		t.Errorf("Expected ReadMessage to return io.EOF, got %v", err)
	}
}

func TestBotConnection_Close(t *testing.T) {
	bc := NewBotConnection("testBot", "easy", &player.Player{}, make(chan *types.PlayerMove, 1))
	err := bc.Close()
	if err != nil {
		t.Errorf("Expected Close to return nil, got %v", err)
	}
}

