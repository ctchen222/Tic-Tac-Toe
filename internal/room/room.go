package room

import (
	"context"
	"ctchen222/Tic-Tac-Toe/internal/events"
	"ctchen222/Tic-Tac-Toe/internal/game"
	"ctchen222/Tic-Tac-Toe/internal/hub/types"
	"ctchen222/Tic-Tac-Toe/internal/player"
	"ctchen222/Tic-Tac-Toe/internal/repository"
	"ctchen222/Tic-Tac-Toe/pkg/proto"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	heartbeatInterval = 30 * time.Second
)

var reconnectionGracePeriod = 1 * time.Minute
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
	returnToLobby  chan<- *player.Player
	moveCalculator MoveCalculator
	moveTimeout    time.Duration
	Done           chan struct{}
}

// NewRoom creates a new game room.
func NewRoom(id string, rdb *redis.Client, gameRepo repository.GameRepository, playerRepo repository.PlayerRepository, calculator MoveCalculator, timeout time.Duration, returnToLobby chan<- *player.Player) *Room {
	return &Room{
		ID:             id,
		rdb:            rdb,
		gameRepo:       gameRepo,
		playerRepo:     playerRepo,
		Players:        make([]*player.Player, 0, 2),
		incomingMoves:  make(chan *types.PlayerMove, 10),
		unregister:     make(chan *player.Player),
		returnToLobby:  returnToLobby,
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
	ctx := context.Background()
	moveTimer := time.NewTimer(r.moveTimeout)
	pingTicker := time.NewTicker(heartbeatInterval)
	cleanupTicker := time.NewTicker(reconnectionGracePeriod)

	defer func() {
		moveTimer.Stop()
		pingTicker.Stop()
		cleanupTicker.Stop()
	}()

	for {
		gameState, err := r.gameRepo.FindByID(ctx, r.ID)
		if err != nil {
			if err := r.gameRepo.RemovePlayersFromGame(ctx, r.Players); err != nil {
				slog.ErrorContext(ctx, "Failed to remove players from game on room closure", "room.id", r.ID, "error", err)
			}
			slog.ErrorContext(ctx, "run loop cannot get game state, closing room", "room.id", r.ID, "error", err)
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
			slog.Info("Room run goroutine stopping.", "room.id", r.ID)
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

			slog.Info("Player timed out", "player.id", currentPlayer.ID, "room.id", r.ID)
			row, col := r.moveCalculator.CalculateNextMove(game.BoardArrayToSlice(gameState.Board), gameState.CurrentTurn, "easy")

			if row != -1 && col != -1 {
				slog.Info("Proxy move for player", "player.id", currentPlayer.ID, "row", row, "col", col)
				moveMsg := proto.ClientToServerMessage{Type: "move", Position: []int{row, col}}
				moveBytes, _ := json.Marshal(moveMsg)
				r.HandleMessage(currentPlayer, moveBytes)
			}

		case <-pingTicker.C:
			for _, p := range r.Players {
				if p.IsBot {
					continue
				}
				if p.Status == player.StatusConnected {
					if err := p.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
						slog.Warn("Failed to send ping to player, assuming disconnect", "player.id", p.ID, "error", err)
						p.Status = player.StatusDisconnected
						p.LastSeen = time.Now()
					}
				}
			}

		case <-cleanupTicker.C:
			r.mu.Lock()
			defer r.mu.Unlock()

			var disconnectedPlayer *player.Player
			var remainingPlayer *player.Player

			for _, p := range r.Players {
				if p.Status == player.StatusDisconnected && time.Since(p.LastSeen) > reconnectionGracePeriod {
					disconnectedPlayer = p
				} else if p.Status == player.StatusConnected {
					remainingPlayer = p
				}
			}

			if disconnectedPlayer != nil {
				slog.Info("Player exceeded reconnection grace period. Declaring forfeit.", "player.id", disconnectedPlayer.ID, "room.id", r.ID)

				// Determine the winner by forfeit
				var winnerMark game.PlayerMark
				if remainingPlayer != nil {
					gameState, err := r.gameRepo.FindByID(ctx, r.ID)
					if err == nil {
						if remainingPlayer.ID == gameState.PlayerXID {
							winnerMark = game.PlayerX
						} else if remainingPlayer.ID == gameState.PlayerOID {
							winnerMark = game.PlayerO
						}
					}
				}

				// Update game state in Redis to reflect forfeit winner
				if winnerMark != game.None {
					roomKey := repository.RoomKeyPrefix + r.ID
					r.rdb.HSet(ctx, roomKey, game.FieldWinner, string(winnerMark)).Err()
					r.rdb.HSet(ctx, roomKey, game.FieldStatus, "finished").Err()

					// Broadcast the final game result to the remaining player
					if remainingPlayer != nil {
						finalMsg := &proto.ServerToClientMessage{
							Type:    "update",
							Winner:  winnerMark,
							Message: fmt.Sprintf("對手 %s 已斷線並棄權，你獲勝！", disconnectedPlayer.ID),
						}
						r.Broadcast(finalMsg)
					}
				} else {
					slog.Warn("Forfeit occurred but winner could not be determined or no remaining player.", "room.id", r.ID)
				}

				// Close the room and return all players (disconnected and remaining) to lobby
				// This will also handle cleaning up the room from localRooms in Hub
				for _, p := range r.Players {
					go r.closeAndReturnPlayersToLobby(ctx, p)
				}
			}
			// close(r.Done)
		}
	}
}

// HandleReconnect handles a player reconnecting to this room.
func (r *Room) HandleReconnect(reconnectingPlayer *player.Player) {
	ctx, span := tracer.Start(context.Background(), "room.HandleReconnect", trace.WithAttributes(
		attribute.String("player.id", reconnectingPlayer.ID),
		attribute.String("room.id", r.ID),
	))
	defer span.End()

	r.mu.Lock()
	defer r.mu.Unlock()

	found := false
	for i, p := range r.Players {
		if p.ID == reconnectingPlayer.ID {
			// Replace the old disconnected player with the new connected one
			r.Players[i] = reconnectingPlayer
			reconnectingPlayer.Status = player.StatusConnected
			reconnectingPlayer.State = player.StateInGame // Ensure state is correct
			reconnectingPlayer.LastSeen = time.Now()
			found = true
			break
		}
	}

	if !found {
		slog.ErrorContext(ctx, "Reconnecting player not found in room's player list", "player.id", reconnectingPlayer.ID, "room.id", r.ID)
		span.SetStatus(codes.Error, "Reconnecting player not found in room")
		// Fallback: send player to lobby if not found in room
		r.returnToLobby <- reconnectingPlayer
		return
	}

	slog.InfoContext(ctx, "Player reconnected to room", "player.id", reconnectingPlayer.ID, "room.id", r.ID)

	// Update Redis status
	if err := r.playerRepo.UpdateConnectionStatus(ctx, reconnectingPlayer.ID, player.StatusConnected); err != nil {
		slog.ErrorContext(ctx, "Failed to update player connection status in Redis on reconnect", "player.id", reconnectingPlayer.ID, "error", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to update Redis status")
	}

	// Start a new ReadPump for the reconnected player
	go r.ReadPump(reconnectingPlayer)

	// Publish player_reconnected global event
	payload, _ := json.Marshal(events.PlayerReconnectedPayload{
		RoomID:   r.ID,
		PlayerID: reconnectingPlayer.ID,
	})
	event, _ := json.Marshal(events.Event{Type: "player_reconnected", Payload: payload})
	if err := r.rdb.Publish(ctx, events.EventsChannel, event).Err(); err != nil {
		slog.ErrorContext(ctx, "Failed to publish player_reconnected event", "player.id", reconnectingPlayer.ID, "error", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to publish player_reconnected event")
	}

	// Send the latest game state to only the reconnected player
	gameState, err := r.gameRepo.FindByID(ctx, r.ID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get game state for reconnected player", "player.id", reconnectingPlayer.ID, "room.id", r.ID, "error", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to get game state for reconnected player")
		return
	}

	initialUpdate := &proto.ServerToClientMessage{
		Type:   "update",
		Board:  game.BoardArrayToSlice(gameState.Board),
		Next:   gameState.CurrentTurn,
		Winner: gameState.Winner,
	}
	data, _ := json.Marshal(initialUpdate)
	if err := reconnectingPlayer.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
		slog.ErrorContext(ctx, "Failed to send initial game state to reconnected player", "player.id", reconnectingPlayer.ID, "error", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to send initial game state")
	}
}
