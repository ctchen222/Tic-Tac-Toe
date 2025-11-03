package player

// Connection is an interface that abstracts the websocket connection.
type Connection interface {
	WriteMessage(messageType int, data []byte) error
	ReadMessage() (int, []byte, error)
	Close() error
}

// Player represents a player in a room.
type Player struct {
	ID   string
	Conn Connection
}
