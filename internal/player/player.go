package player

import "time"

// Connection is an interface that abstracts the websocket connection.
type Connection interface {
	WriteMessage(messageType int, data []byte) error
	ReadMessage() (int, []byte, error)
	Close() error
}

// PlayerStatus represents the connection status of a player.
type PlayerStatus string

const (
	StatusConnected    PlayerStatus = "connected"
	StatusDisconnected PlayerStatus = "disconnected"
)

// Player represents a player in a room.
type Player struct {
	ID       string
	Conn     Connection
	Status   PlayerStatus
	LastSeen time.Time
	IsBot    bool
}

// NewPlayer creates a new player instance.
func NewPlayer(id string, conn Connection) *Player {
	return &Player{
		ID:       id,
		Conn:     conn,
		Status:   StatusConnected,
		LastSeen: time.Now(),
		IsBot:    false, // Defaults to human player
	}
}
