package room

import (
	"context"
	"ctchen222/Tic-Tac-Toe/internal/game"
	"ctchen222/Tic-Tac-Toe/internal/player"
	"ctchen222/Tic-Tac-Toe/internal/validator"
	"ctchen222/Tic-Tac-Toe/pkg/proto"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	heartbeatInterval = 10 * time.Second
)

var reconnectionGracePeriod = 60 * time.Second
var tracer = otel.Tracer("room")

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
	ID                 string
	Players            []*player.Player
	PlayerMarkMap      map[string]game.PlayerMark
	Game               *game.Game
	mu                 sync.Mutex
	incomingMoves      chan *playerMove
	disconnectedPlayer chan *player.Player
	unregister         chan *player.Player
	moveCalculator     MoveCalculator
	moveTimeout        time.Duration
	rematchVotes       map[string]bool
	Done               chan struct{} // Channel to signal the room to stop
}

// NewRoom creates a new game room.
func NewRoom(id string, calculator MoveCalculator, timeout time.Duration) *Room {
	return &Room{
		ID:                 id,
		Players:            make([]*player.Player, 0, 2),
		PlayerMarkMap:      make(map[string]game.PlayerMark),
		Game:               game.NewGame(),
		incomingMoves:      make(chan *playerMove),
		disconnectedPlayer: make(chan *player.Player),
		unregister:         make(chan *player.Player),
		moveCalculator:     calculator,
		moveTimeout:        timeout,
		rematchVotes:       make(map[string]bool),
		Done:               make(chan struct{}), // Initialize the done channel
	}
}

// AddPlayer adds a player to the room.
func (r *Room) AddPlayer(p *player.Player) {
	r.Players = append(r.Players, p)
}

// Broadcast sends a message to all connected players in the room.
func (r *Room) Broadcast(message *proto.ServerToClientMessage) {
	data, err := json.Marshal(message)
	if err != nil {
		log.Printf("error marshalling message: %v", err)
		return
	}

	for _, p := range r.Players {
		if p.Status == player.StatusConnected {
			if err := p.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Printf("error writing message to player %s: %v", p.ID, err)
			}
		}
	}
}

// HandleMessage handles a message from a player.
func (r *Room) HandleMessage(p *player.Player, rawMessage []byte) {
	ctx := context.Background()
	ctx, span := tracer.Start(ctx, "room.handle_message")
	defer span.End()

	r.mu.Lock()
	defer r.mu.Unlock()

	if p.Status == player.StatusDisconnected {
		log.Printf("ignoring message from disconnected player %s", p.ID)
		return
	}

	var message proto.ClientToServerMessage
	if err := json.Unmarshal(rawMessage, &message); err != nil {
		log.Printf("error unmarshalling message: %v", err)
		return
	}

	if err := validator.GetValidator().Struct(message); err != nil {
		log.Printf("invalid message from player %s: %v", p.ID, err)
		return
	}

	span.SetAttributes(attribute.String("message.type", message.Type))

	if message.Type == "move" {
		_, moveSpan := tracer.Start(ctx, "room.move", trace.WithAttributes(
			attribute.String("player.id", p.ID),
			attribute.String("room.id", r.ID),
			attribute.Int("move.row", message.Position[0]),
			attribute.Int("move.col", message.Position[1]),
		))
		defer moveSpan.End()

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

		if err := r.Game.Move(message.Position[0], message.Position[1]); err != nil {
			log.Printf("invalid move from player %s: %v", p.ID, err)
			moveSpan.SetAttributes(attribute.Bool("move.valid", false))
			return
		}
		moveSpan.SetAttributes(attribute.Bool("move.valid", true))

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

		// Check if the other player is a bot, if so, auto-accept rematch
		var otherPlayerIsBot bool
		for _, other := range r.Players {
			if other.ID != p.ID && other.IsBot {
				otherPlayerIsBot = true
				break
			}
		}

		if otherPlayerIsBot || (len(r.rematchVotes) == len(r.Players) && len(r.Players) == 2) {
			log.Printf("All players voted for a rematch in room %s (or bot auto-accepted). Resetting game.", r.ID)
			r.resetGameForRematch()
		} else {
			// Notify human opponent that a rematch has been requested
			for _, otherPlayer := range r.Players {
				if otherPlayer.ID != p.ID {
					msg := &proto.ServerToClientMessage{Type: "rematch_requested"}
					data, _ := json.Marshal(msg)
					if otherPlayer.Status == player.StatusConnected {
						otherPlayer.Conn.WriteMessage(websocket.TextMessage, data)
					}
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
		if p.Status == player.StatusConnected {
			p.Conn.WriteMessage(websocket.TextMessage, data)
		}
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
		go r.ReadPump(p)
	}

	go r.run()

	for p := range r.unregister {
		unregisterPlayer <- p
	}
}

// ReadPump pumps messages from the websocket connection to the room's incomingMoves channel.
func (r *Room) ReadPump(p *player.Player) {
	defer func() {
		p.Conn.Close()
		r.mu.Lock()
		p.Status = player.StatusDisconnected
		p.LastSeen = time.Now()
		r.mu.Unlock()
		r.disconnectedPlayer <- p
	}()

	for {
		_, msg, err := p.Conn.ReadMessage()
		if err != nil {
			log.Printf("Player %s connection error in room %s: %v", p.ID, r.ID, err)
			return
		}
		r.incomingMoves <- &playerMove{player: p, message: msg}
	}
}

// run is the main game loop for the room.
func (r *Room) run() {
	moveTimer := time.NewTimer(r.moveTimeout)
	pingTicker := time.NewTicker(heartbeatInterval)
	cleanupTicker := time.NewTicker(reconnectionGracePeriod)

	defer func() {
		moveTimer.Stop()
		pingTicker.Stop()
		cleanupTicker.Stop()
	}()

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

		if currentPlayer.Status == player.StatusConnected {
			moveTimer.Reset(r.moveTimeout)
		} else {
			moveTimer.Reset(1 * time.Second) // shorter timeout for disconnected players
		}

		select {
		case <-r.Done:
			log.Printf("Room %s run goroutine stopping.", r.ID)
			return

		case move := <-r.incomingMoves:
			if !moveTimer.Stop() {
				<-moveTimer.C // Drain the timer
			}
			r.HandleMessage(move.player, move.message)

		case p := <-r.disconnectedPlayer:
			log.Printf("Player %s marked as disconnected in room %s.", p.ID, r.ID)
			// Notify other players
			for _, other := range r.Players {
				if other.ID != p.ID && other.Status == player.StatusConnected {
					msg := &proto.ServerToClientMessage{Type: "opponent_disconnected"}
					data, _ := json.Marshal(msg)
					other.Conn.WriteMessage(websocket.TextMessage, data)
				}
			}

		case <-moveTimer.C:
			if r.Game.Winner != game.None {
				continue
			}

			log.Printf("Player %s timed out.", currentPlayer.ID)
			row, col := r.moveCalculator.CalculateNextMove(r.Game.BoardAsStrings(), r.Game.CurrentTurn, "medium")

			if row != -1 && col != -1 {
				log.Printf("Proxy move for player %s: row %d, col %d", currentPlayer.ID, row, col)
				moveMsg := proto.ClientToServerMessage{Type: "move", Position: []int{row, col}}
				moveBytes, _ := json.Marshal(moveMsg)
				r.HandleMessage(currentPlayer, moveBytes)
			}

		case <-pingTicker.C:
			for _, p := range r.Players {
				if p.Status == player.StatusConnected {
					if err := p.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
						log.Printf("Failed to send ping to player %s, assuming disconnect: %v", p.ID, err)
						// The readPump will catch the resulting connection error
					}
				}
			}

		case <-cleanupTicker.C:
			r.mu.Lock()
			for _, p := range r.Players {
				if p.Status == player.StatusDisconnected && time.Since(p.LastSeen) > reconnectionGracePeriod {
					log.Printf("Player %s exceeded reconnection grace period. Removing from room %s.", p.ID, r.ID)
					r.unregister <- p
				}
			}
			r.mu.Unlock()
		}
	}
}
