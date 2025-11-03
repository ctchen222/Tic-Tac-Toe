package room

import (
	"ctchen222/Tic-Tac-Toe/internal/game"
	"ctchen222/Tic-Tac-Toe/internal/player"
	"ctchen222/Tic-Tac-Toe/internal/validator"
	"ctchen222/Tic-Tac-Toe/pkg/proto"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// MoveCalculator defines an interface for an agent that can calculate a game move.
type MoveCalculator interface {
	CalculateNextMove(board [][]game.PlayerMark, mark game.PlayerMark, difficulty string) (row, col int)
}

// playerMove is a message from a player.
type playerMove struct {
	player  *player.Player
	message []byte
}

// Room represents a game room.
type Room struct {
	ID             string
	Players        []*player.Player
	PlayerMarkMap  map[string]game.PlayerMark
	Game           *game.Game
	mu             sync.Mutex
	incomingMoves  chan *playerMove
	unregister     chan *player.Player
	moveCalculator MoveCalculator
	moveTimeout    time.Duration
	rematchVotes   map[string]bool
	Done           chan struct{} // Channel to signal the room to stop
}

// NewRoom creates a new game room.
func NewRoom(id string, calculator MoveCalculator, timeout time.Duration) *Room {
	return &Room{
		ID:             id,
		Players:        make([]*player.Player, 0, 2),
		PlayerMarkMap:  make(map[string]game.PlayerMark),
		Game:           game.NewGame(),
		incomingMoves:  make(chan *playerMove),
		unregister:     make(chan *player.Player),
		moveCalculator: calculator,
		moveTimeout:    timeout,
		rematchVotes:   make(map[string]bool),
		Done:           make(chan struct{}), // Initialize the done channel
	}
}

// AddPlayer adds a player to the room.
func (r *Room) AddPlayer(p *player.Player) {
	r.Players = append(r.Players, p)
}

// Broadcast sends a message to all players in the room.
func (r *Room) Broadcast(message *proto.ServerToClientMessage) {
	for _, p := range r.Players {
		data, err := json.Marshal(message)
		if err != nil {
			log.Printf("error marshalling message: %v", err)
			continue
		}
		if err := p.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("error writing message to player %s: %v", p.ID, err)
		}
	}
}

// HandleMessage handles a message from a player.
func (r *Room) HandleMessage(p *player.Player, rawMessage []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var message proto.ClientToServerMessage
	if err := json.Unmarshal(rawMessage, &message); err != nil {
		log.Printf("error unmarshalling message: %v", err)
		return
	}

	validate := validator.GetValidator()
	if err := validate.Struct(message); err != nil {
		log.Printf("invalid message from player %s: %v", p.ID, err)
		return
	}

	if message.Type == "move" {
		if r.Game.Winner != game.None || r.Game.IsDraw() {
			log.Printf("Player %s attempted to move, but game is already over.", p.ID)
			return
		}
		playerMark, ok := r.PlayerMarkMap[p.ID]
		if !ok {
			log.Printf("player %s has no assigned mark", p.ID)
			return
		}
		if playerMark != game.PlayerMark(r.Game.CurrentTurn) {
			log.Printf("not player %s's turn", p.ID)
			return
		}

		fmt.Printf("Current board: %v\n", r.Game.BoardAsStrings())
		if err := r.Game.Move(message.Position[0], message.Position[1]); err != nil {
			log.Printf("invalid move from player %s: %v", p.ID, err)
			return
		}

		response := &proto.ServerToClientMessage{
			Type:   "update",
			Board:  r.Game.BoardAsStrings(),
			Next:   r.Game.CurrentTurn,
			Winner: r.Game.Winner,
		}
		r.Broadcast(response)
	} else if message.Type == "rematch" {
		if r.Game.Winner == game.None {
			log.Printf("Player %s requested rematch, but game is not over.", p.ID)
			return
		}

		log.Printf("Player %s voted for a rematch.", p.ID)
		r.rematchVotes[p.ID] = true

		if len(r.rematchVotes) == len(r.Players) && len(r.Players) == 2 {
			log.Printf("All players voted for a rematch in room %s. Resetting game.", r.ID)
			r.resetGameForRematch()
		} else {
			for _, otherPlayer := range r.Players {
				if otherPlayer.ID != p.ID {
					msg := &proto.ServerToClientMessage{Type: "rematch_requested"}
					data, _ := json.Marshal(msg)
					otherPlayer.Conn.WriteMessage(websocket.TextMessage, data)
				}
			}
		}
	}
}

func (r *Room) resetGameForRematch() {
	r.Game = game.NewGame()
	r.rematchVotes = make(map[string]bool)

	p1 := r.Players[0]
	p2 := r.Players[1]
	oldP1Mark := game.PlayerMark(r.PlayerMarkMap[p1.ID])
	r.PlayerMarkMap[p1.ID] = r.PlayerMarkMap[p2.ID]
	r.PlayerMarkMap[p2.ID] = oldP1Mark

	log.Printf("Room %s marks swapped: Player %s is now %s, Player %s is now %s", r.ID, p1.ID, r.PlayerMarkMap[p1.ID], p2.ID, r.PlayerMarkMap[p2.ID])

	for _, p := range r.Players {
		assignmentMessage := &proto.PlayerAssignmentMessage{
			Type: "assignment",
			Mark: game.PlayerMark(r.PlayerMarkMap[p.ID]),
		}
		data, _ := json.Marshal(assignmentMessage)
		p.Conn.WriteMessage(websocket.TextMessage, data)
	}

	initialMessage := &proto.ServerToClientMessage{
		Type:   "update",
		Board:  r.Game.BoardAsStrings(),
		Next:   r.Game.CurrentTurn,
		Winner: r.Game.Winner,
	}
	r.Broadcast(initialMessage)
}

// Start starts the game room, launching the main game loop and listening for player disconnections.
func (r *Room) Start(unregisterPlayer chan<- *player.Player) {
	for _, p := range r.Players {
		go r.readPump(p)
	}

	go r.run()

	for p := range r.unregister {
		unregisterPlayer <- p
	}
}

// readPump pumps messages from the websocket connection to the room's incomingMoves channel.
func (r *Room) readPump(p *player.Player) {
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

	for {
		var currentPlayer *player.Player
		for _, p := range r.Players {
			if game.PlayerMark(r.PlayerMarkMap[p.ID]) == r.Game.CurrentTurn {
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
		case <-r.Done:
			log.Printf("Room %s run goroutine stopping.", r.ID)
			return

		case move := <-r.incomingMoves:
			if !timer.Stop() {
				<-timer.C // Drain the timer
			}
			r.HandleMessage(move.player, move.message)

		case <-timer.C:
			if r.Game.Winner != game.None {
				log.Printf("Game in room %s is over. No proxy move needed.", r.ID)
				continue
			}

			row, col := r.moveCalculator.CalculateNextMove(r.Game.BoardAsStrings(), r.Game.CurrentTurn, "medium")

			if row != -1 && col != -1 {
				log.Printf("Proxy move for player %s: row %d, col %d", currentPlayer.ID, row, col)
				moveMsg := proto.ClientToServerMessage{Type: "move", Position: []int{row, col}}
				moveBytes, _ := json.Marshal(moveMsg)
				r.HandleMessage(currentPlayer, moveBytes)
			}
		}
	}
}
