package match

import (
	"ctchen222/Tic-Tac-Toe/internal/player"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type MatchManager struct {
	mu               sync.Mutex
	waitingPlayers   []*player.Player
	addPlayerChan    chan *player.Player
	removePlayerChan chan string
	matchedPairChan  chan [2]*player.Player
}

func NewMatchManager() *MatchManager {
	return &MatchManager{
		waitingPlayers:   make([]*player.Player, 0),
		addPlayerChan:    make(chan *player.Player, 1),
		removePlayerChan: make(chan string),
		matchedPairChan:  make(chan [2]*player.Player, 1),
	}
}

func (m *MatchManager) Run() {
	for {
		select {
		case p := <-m.addPlayerChan:
			m.mu.Lock()
			m.waitingPlayers = append(m.waitingPlayers, p)
			m.mu.Unlock()
			m.tryMatchPlayers()

		case playerID := <-m.removePlayerChan:
			m.mu.Lock()
			for i, p := range m.waitingPlayers {
				if p.ID == playerID {
					m.waitingPlayers = append(m.waitingPlayers[:i], m.waitingPlayers[i+1:]...)
					slog.Info("Matchmaker: Player removed from waiting list", "player.id", playerID)
					break
				}
			}
			m.mu.Unlock()
		}
	}
}

func (m *MatchManager) AddPlayer(p *player.Player) {
	m.addPlayerChan <- p
}

func (m *MatchManager) RemovePlayer(playerID string) {
	m.removePlayerChan <- playerID
}

func (m *MatchManager) MatchedPair() <-chan [2]*player.Player {
	return m.matchedPairChan
}

func (m *MatchManager) tryMatchPlayers() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for len(m.waitingPlayers) >= 2 {
		player1 := m.waitingPlayers[0]
		player2 := m.waitingPlayers[1]
		m.waitingPlayers = m.waitingPlayers[2:]

		m.matchedPairChan <- [2]*player.Player{player1, player2}
		slog.Info("Matchmaker: Matched players", "player1.id", player1.ID, "player2.id", player2.ID)
	}
}

// GetWaitingPlayersCount returns the current number of players waiting for a match.
func (m *MatchManager) GetWaitingPlayersCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.waitingPlayers)
}

// WaitForWaitingPlayers polls the waitingPlayers count until it matches the expected count or a timeout occurs.
func (m *MatchManager) WaitForWaitingPlayers(expectedCount int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if m.GetWaitingPlayersCount() == expectedCount {
			return nil
		}
		time.Sleep(10 * time.Millisecond) // Poll every 10ms
	}
	return fmt.Errorf("timed out waiting for waitingPlayers count to be %d", expectedCount)
}