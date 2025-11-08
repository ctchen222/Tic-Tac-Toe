package room

import (
	"context"
	"ctchen222/Tic-Tac-Toe/internal/game"
	"ctchen222/Tic-Tac-Toe/internal/hub/types"
	"ctchen222/Tic-Tac-Toe/internal/player"
	"ctchen222/Tic-Tac-Toe/internal/repository"
	"ctchen222/Tic-Tac-Toe/pkg/proto"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
	"go.opentelemetry.io/otel"
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

// Room represents a game room.
type Room struct {
	ID             string
	rdb            *redis.Client
	gameRepo       repository.GameRepository
	playerRepo     repository.PlayerRepository
	Players        []*player.Player
	mu             sync.Mutex
	incomingMoves  chan *types.PlayerMove
	unregister     chan *player.Player
	moveCalculator MoveCalculator
	moveTimeout    time.Duration
	Done           chan struct{}
}

// NewRoom creates a new game room.
func NewRoom(id string, rdb *redis.Client, gameRepo repository.GameRepository, playerRepo repository.PlayerRepository, calculator MoveCalculator, timeout time.Duration) *Room {
	return &Room{
		ID:             id,
		rdb:            rdb,
		gameRepo:       gameRepo,
		playerRepo:     playerRepo,
		Players:        make([]*player.Player, 0, 2),
		incomingMoves:  make(chan *types.PlayerMove, 10),
		unregister:     make(chan *player.Player),
		moveCalculator: calculator,
		moveTimeout:    timeout,
		Done:           make(chan struct{}),
	}
}

// Start starts the game room, launching the main game loop and listening for player disconnections.
func (r *Room) Start(unregisterPlayer chan<- *player.Player) {
	for _, p := range r.Players {
		if !p.IsBot {
			go r.ReadPump(p)
		}
	}
	go r.run()

	for p := range r.unregister {
		unregisterPlayer <- p
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
		ctx := context.Background()
		gameState, err := r.gameRepo.FindByID(ctx, r.ID)
		if err != nil {
			log.Printf("run loop cannot get game state for room %s: %v. Closing room.", r.ID, err)
			if len(r.Players) > 0 {
				r.unregister <- r.Players[0]
			}
			return
		}

		var currentPlayer *player.Player
		for _, p := range r.Players {
			var mark game.PlayerMark
			if p.ID == gameState.PlayerXID {
				mark = game.PlayerX
			} else if p.ID == gameState.PlayerOID {
				mark = game.PlayerO
			}

			if mark == gameState.CurrentTurn {
				currentPlayer = p
				break
			}
		}

		isLocalTurn := currentPlayer != nil

		if isLocalTurn {
			if currentPlayer.Status == player.StatusConnected {
				moveTimer.Reset(r.moveTimeout)
			} else {
				moveTimer.Reset(1 * time.Second)
			}
		} else {
			moveTimer.Stop()
		}

		select {
		case <-r.Done:
			log.Printf("Room %s run goroutine stopping.", r.ID)
			return

		case move := <-r.incomingMoves:
			if !moveTimer.Stop() {
				select {
				case <-moveTimer.C:
				default:
				}
			}
			r.HandleMessage(move.Player, move.Message)

		case <-moveTimer.C:
			if !isLocalTurn {
				continue
			}

			if gameState.Winner != game.None || gameState.IsDraw {
				continue
			}

			log.Printf("Player %s timed out.", currentPlayer.ID)
			row, col := r.moveCalculator.CalculateNextMove(game.BoardArrayToSlice(gameState.Board), gameState.CurrentTurn, "medium")

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
