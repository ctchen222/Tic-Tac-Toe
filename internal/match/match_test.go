package match

import (
	"ctchen222/Tic-Tac-Toe/internal/room"
	"testing"
	"time"
)

func TestMatchManager_AddThreePlayers(t *testing.T) {
	mm := NewMatchManager()
	go mm.Run()

	player1 := &room.Player{ID: "player1"}
	player2 := &room.Player{ID: "player2"}
	player3 := &room.Player{ID: "player3"}

	mm.AddPlayer(player1)
	mm.AddPlayer(player2)
	mm.AddPlayer(player3)

	// Check that one pair is matched
	select {
	case pair := <-mm.MatchedPair():
		if (pair[0].ID != "player1" || pair[1].ID != "player2") && (pair[0].ID != "player2" || pair[1].ID != "player1") {
			t.Errorf("Expected players with IDs 'player1' and 'player2', got '%s' and '%s'", pair[0].ID, pair[1].ID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Errorf("Timed out waiting for players to be matched")
	}

	// Check that one player remains
	time.Sleep(10 * time.Millisecond) // allow time for the slice to be updated
	mm.mu.Lock()
	defer mm.mu.Unlock()
	if len(mm.waitingPlayers) != 1 {
		t.Errorf("Expected 1 waiting player, got %d", len(mm.waitingPlayers))
	}
	if mm.waitingPlayers[0].ID != "player3" {
		t.Errorf("Expected player with ID 'player3', got '%s'", mm.waitingPlayers[0].ID)
	}
}

func TestMatchManager_MatchPlayers(t *testing.T) {
	mm := NewMatchManager()
	go mm.Run()

	player1 := &room.Player{ID: "player1"}
	player2 := &room.Player{ID: "player2"}

	mm.AddPlayer(player1)
	mm.AddPlayer(player2)

	select {
	case pair := <-mm.MatchedPair():
		if (pair[0].ID != "player1" || pair[1].ID != "player2") && (pair[0].ID != "player2" || pair[1].ID != "player1") {
			t.Errorf("Expected players with IDs 'player1' and 'player2', got '%s' and '%s'", pair[0].ID, pair[1].ID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Errorf("Timed out waiting for players to be matched")
	}

	mm.mu.Lock()
	defer mm.mu.Unlock()
	if len(mm.waitingPlayers) != 0 {
		t.Errorf("Expected 0 waiting players after matching, got %d", len(mm.waitingPlayers))
	}
}

func TestMatchManager_RemovePlayerFromSingleList(t *testing.T) {
	mm := NewMatchManager()
	go mm.Run()

	player1 := &room.Player{ID: "player1"}

	mm.AddPlayer(player1)
	time.Sleep(10 * time.Millisecond) // allow time for the player to be added

	mm.RemovePlayer("player1")
	time.Sleep(10 * time.Millisecond) // allow time for the player to be removed

	mm.mu.Lock()
	defer mm.mu.Unlock()
	if len(mm.waitingPlayers) != 0 {
		t.Errorf("Expected 0 waiting players, got %d", len(mm.waitingPlayers))
	}
}

func TestMatchManager_RemoveNonExistentPlayer(t *testing.T) {
	mm := NewMatchManager()
	go mm.Run()

	player1 := &room.Player{ID: "player1"}

	mm.AddPlayer(player1)
	time.Sleep(10 * time.Millisecond) // allow time for the player to be added

	// Try to remove a player that doesn't exist
	mm.RemovePlayer("player2")
	time.Sleep(10 * time.Millisecond)

	mm.mu.Lock()
	defer mm.mu.Unlock()
	if len(mm.waitingPlayers) != 1 {
		t.Errorf("Expected 1 waiting player, got %d", len(mm.waitingPlayers))
	}
	if mm.waitingPlayers[0].ID != "player1" {
		t.Errorf("Expected player with ID 'player1', got '%s'", mm.waitingPlayers[0].ID)
	}
}
