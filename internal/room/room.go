package room

import (
	"ctchen222/Tic-Tac-Toe/internal/game"
	"ctchen222/Tic-Tac-Toe/internal/validator"
	"ctchen222/Tic-Tac-Toe/pkg/proto"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// MoveCalculator defines an interface for an agent that can calculate a game move.
type MoveCalculator interface {
	CalculateNextMove(board [][]string, mark string, difficulty string) (row, col int)
}

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

// playerMove is a message from a player.
type playerMove struct {
	player  *Player
	message []byte
}

// Room represents a game room.
type Room struct {
	ID             string
	Players        []*Player
	PlayerMarkMap  map[string]string
	Game           *game.Game
	mu             sync.Mutex
	incomingMoves  chan *playerMove
	unregister     chan *Player
	moveCalculator MoveCalculator
	moveTimeout    time.Duration
}

// NewRoom creates a new game room.
func NewRoom(id string, calculator MoveCalculator, timeout time.Duration) *Room {
	return &Room{
		ID:             id,
		Players:        make([]*Player, 0, 2),
		PlayerMarkMap:  make(map[string]string),
		Game:           game.NewGame(),
		incomingMoves:  make(chan *playerMove),
		unregister:     make(chan *Player),
		moveCalculator: calculator,
		moveTimeout:    timeout,
	}
}

// AddPlayer adds a player to the room.
func (r *Room) AddPlayer(player *Player) {
	r.Players = append(r.Players, player)
}

// Broadcast sends a message to all players in the room.
func (r *Room) Broadcast(message *proto.ServerToClientMessage) {
	for _, player := range r.Players {
		data, err := json.Marshal(message)
		if err != nil {
			log.Printf("error marshalling message: %v", err)
			continue
		}
		if err := player.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("error writing message to player %s: %v", player.ID, err)
		}
	}
}

// HandleMessage handles a message from a player.
func (r *Room) HandleMessage(player *Player, rawMessage []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var message proto.ClientToServerMessage
	if err := json.Unmarshal(rawMessage, &message); err != nil {
		log.Printf("error unmarshalling message: %v", err)
		return
	}

	validate := validator.GetValidator()
	if err := validate.Struct(message); err != nil {
		log.Printf("invalid message from player %s: %v", player.ID, err)
		return
	}

	if message.Type == "move" {
		playerMark, ok := r.PlayerMarkMap[player.ID]
		if !ok {
			log.Printf("player %s has no assigned mark", player.ID)
			return
		}
		if playerMark != r.Game.CurrentTurn {
			log.Printf("not player %s's turn", player.ID)
			return
		}

		if err := r.Game.Move(message.Position[0], message.Position[1]); err != nil {
			log.Printf("invalid move from player %s: %v", player.ID, err)
			return
		}

		response := &proto.ServerToClientMessage{
			Type:   "update",
			Board:  r.Game.BoardAsStrings(),
			Next:   string(r.Game.CurrentTurn),
			Winner: string(r.Game.Winner),
		}
		r.Broadcast(response)
	}
}

// Start starts the game room, launching the main game loop and listening for player disconnections.
func (r *Room) Start(unregisterPlayer chan<- *Player) {
	// Start a read pump for each player
	for _, player := range r.Players {
		go r.readPump(player)
	}

	// Start the main game loop
	go r.run()

	// Forward unregister events to the hub
	for p := range r.unregister {
		unregisterPlayer <- p
	}
}

// readPump pumps messages from the websocket connection to the room's incomingMoves channel.
func (r *Room) readPump(p *Player) {
	defer func() {
		r.unregister <- p
		p.Conn.Close()
	}()

	for {
		_, msg, err := p.Conn.ReadMessage()
		if err != nil {
			log.Printf("Player %s disconnected from room %s: %v", p.ID, r.ID, err)
			return
		}
		r.incomingMoves <- &playerMove{player: p, message: msg}
	}
}

// run is the main game loop for the room.
func (r *Room) run() {
	timer := time.NewTimer(r.moveTimeout)
	defer timer.Stop()

	for r.Game.Winner == "" && len(r.Players) == 2 {
		var currentPlayer *Player
		for _, p := range r.Players {
			if r.PlayerMarkMap[p.ID] == r.Game.CurrentTurn {
				currentPlayer = p
				break
			}
		}

		if currentPlayer == nil {
			log.Printf("Could not find current player in room %s", r.ID)
			return // End game if something is wrong
		}

		timer.Reset(r.moveTimeout)

		select {
		case move := <-r.incomingMoves:
			if !timer.Stop() {
				<-timer.C // Drain the timer
			}
			if move.player.ID != currentPlayer.ID {
				log.Printf("Ignoring move from player %s, it is %s's turn", move.player.ID, currentPlayer.ID)
				continue
			}
			r.HandleMessage(move.player, move.message)

		case <-timer.C:
			log.Printf("Player %s timed out. Making a proxy move.", currentPlayer.ID)

			// Use bot logic to make a move
			row, col := r.moveCalculator.CalculateNextMove(r.Game.BoardAsStrings(), r.Game.CurrentTurn, "medium")
			if row != -1 {
				moveMsg := proto.ClientToServerMessage{Type: "move", Position: []int{row, col}}
				moveBytes, _ := json.Marshal(moveMsg)
				r.HandleMessage(currentPlayer, moveBytes)
			}
		}
	}
}
