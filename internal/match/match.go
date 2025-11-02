package match

import (
	"ctchen222/Tic-Tac-Toe/internal/room"
	"log"
	"sync"
)

type MatchManager struct {
	mu               sync.Mutex
	waitingPlayers   []*room.Player
	addPlayerChan    chan *room.Player
	removePlayerChan chan string
	matchedPairChan  chan [2]*room.Player
}

func NewMatchManager() *MatchManager {
	return &MatchManager{
		waitingPlayers:   make([]*room.Player, 0),
		addPlayerChan:    make(chan *room.Player, 1),
		removePlayerChan: make(chan string),
		matchedPairChan:  make(chan [2]*room.Player, 1),
	}
}

func (m *MatchManager) Run() {
	for {
		select {
		case player := <-m.addPlayerChan:
			m.mu.Lock()
			m.waitingPlayers = append(m.waitingPlayers, player)
			m.mu.Unlock()
			m.tryMatchPlayers()

		case playerID := <-m.removePlayerChan:
			m.mu.Lock()
			for i, p := range m.waitingPlayers {
				if p.ID == playerID {
					m.waitingPlayers = append(m.waitingPlayers[:i], m.waitingPlayers[i+1:]...)
					log.Printf("Matchmaker: Player %s removed from waiting list.", playerID)
					break
				}
			}
			m.mu.Unlock()
		}
	}
}

func (m *MatchManager) AddPlayer(player *room.Player) {
	m.addPlayerChan <- player
}

func (m *MatchManager) RemovePlayer(playerID string) {
	m.removePlayerChan <- playerID
}

func (m *MatchManager) MatchedPair() <-chan [2]*room.Player {
	return m.matchedPairChan
}

func (m *MatchManager) tryMatchPlayers() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for len(m.waitingPlayers) >= 2 {
		player1 := m.waitingPlayers[0]
		player2 := m.waitingPlayers[1]
		m.waitingPlayers = m.waitingPlayers[2:]

		m.matchedPairChan <- [2]*room.Player{player1, player2}
		log.Printf("Matchmaker: Matched players %s and %s", player1.ID, player2.ID)
	}
}
