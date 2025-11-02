package hub

import (
	"ctchen222/Tic-Tac-Toe/internal/bot"
	"ctchen222/Tic-Tac-Toe/internal/game"
	"ctchen222/Tic-Tac-Toe/internal/hub/types"
	"ctchen222/Tic-Tac-Toe/internal/match"
	"ctchen222/Tic-Tac-Toe/internal/room"
	"ctchen222/Tic-Tac-Toe/pkg/proto"
	"encoding/json"
	"log"
	"math/rand/v2"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const moveTimeout = 15 * time.Second

// Hub manages all the rooms and players.
type Hub struct {
	rooms        map[string]*room.Room
	register     chan *types.RegistrationRequest
	unregister   chan *room.Player
	matchManager *match.MatchManager
}

// NewHub creates a new hub.
func NewHub() *Hub {
	return &Hub{
		rooms:        make(map[string]*room.Room),
		register:     make(chan *types.RegistrationRequest),
		unregister:   make(chan *room.Player),
		matchManager: match.NewMatchManager(),
	}
}

// Run starts the hub.
func (h *Hub) Run() {
	go h.matchManager.Run()

	// Create a single move calculator to be used by all rooms.
	moveCalculator := &bot.BotMoveCalculator{}

	for {
		select {
		case req := <-h.register:
			if req.Mode == "bot" {
				// Create a bot match immediately
				log.Printf("Creating bot match for player %s", req.Player.ID)
				player1 := req.Player
				player2 := bot.NewBotPlayer(req.Difficulty)

				roomID := uuid.New().String()
				newRoom := room.NewRoom(roomID, moveCalculator, moveTimeout) // Pass calculator and timeout
				newRoom.AddPlayer(player1)
				newRoom.AddPlayer(player2)

				// Randomly assign marks
				if rand.N(2) == 0 {
					newRoom.PlayerMarkMap[player1.ID] = game.PlayerX
					newRoom.PlayerMarkMap[player2.ID] = game.PlayerO
				} else {
					newRoom.PlayerMarkMap[player1.ID] = game.PlayerO
					newRoom.PlayerMarkMap[player2.ID] = game.PlayerX
				}

				h.rooms[roomID] = newRoom
				go newRoom.Start(h.unregister) // Pass the unregister channel

				log.Printf("Room %s created for bot match with player %s and bot %s", roomID, player1.ID, player2.ID)

				// Notify players that the game has started
				initialMessage := &proto.ServerToClientMessage{
					Type:  "update",
					Board: newRoom.Game.BoardAsStrings(),
					Next:  newRoom.Game.CurrentTurn,
				}
				h.BroadcastPlayerMarkMessage(newRoom)
				newRoom.Broadcast(initialMessage)
			} else {
				// Add player to the regular matchmaking pool
				h.matchManager.AddPlayer(req.Player)
			}

		case player := <-h.unregister:
			h.matchManager.RemovePlayer(player.ID)

			// Remove from active rooms
			for roomID, r := range h.rooms {
				for i, p := range r.Players {
					if p.ID == player.ID {
						r.Players = append(r.Players[:i], r.Players[i+1:]...)
						log.Printf("Player %s removed from room %s", player.ID, roomID)
						if len(r.Players) == 0 {
							delete(h.rooms, roomID) // Close room if empty
							log.Printf("Room %s closed due to no players", roomID)
						}
						break
					}
				}
			}
			log.Printf("Player %s disconnected", player.ID)

		case pair := <-h.matchManager.MatchedPair():
			player1 := pair[0]
			player2 := pair[1]

			roomID := uuid.New().String()
			newRoom := room.NewRoom(roomID, moveCalculator, moveTimeout) // Pass calculator and timeout
			newRoom.AddPlayer(player1)
			newRoom.AddPlayer(player2)

			// Randomly assign marks
			if rand.N(2) == 0 {
				newRoom.PlayerMarkMap[player1.ID] = game.PlayerX
				newRoom.PlayerMarkMap[player2.ID] = game.PlayerO
			} else {
				newRoom.PlayerMarkMap[player1.ID] = game.PlayerO
				newRoom.PlayerMarkMap[player2.ID] = game.PlayerX
			}

			h.rooms[roomID] = newRoom
			go newRoom.Start(h.unregister) // Pass the unregister channel

			log.Printf("Room %s created with players %s and %s", roomID, player1.ID, player2.ID)

			// Notify players that the game has started
			initialMessage := &proto.ServerToClientMessage{
				Type:  "update",
				Board: newRoom.Game.BoardAsStrings(),
				Next:  newRoom.Game.CurrentTurn,
			}
			newRoom.Broadcast(initialMessage)

			h.BroadcastPlayerMarkMessage(newRoom)
		}

	}
}
func (h *Hub) BroadcastPlayerMarkMessage(room *room.Room) {
	for _, player := range room.Players {
		mark, ok := room.PlayerMarkMap[player.ID]
		if !ok {
			log.Printf("No mark assigned for player %s in room %s", player.ID, room.ID)
			continue
		}
		assignmentMessage := &proto.PlayerAssignmentMessage{
			Type: "assignment",
			Mark: mark,
		}

		// Serialize and send the message
		data, err := json.Marshal(assignmentMessage)
		if err != nil {
			log.Printf("Error marshalling assignment message for player %s: %v", player.ID, err)
		}
		if err := player.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
			// TODO: trace | roomID, playerID
			log.Printf("Error sending assignment message to player %s: %v", player.ID, err)
		}
	}
}

// Register returns the register channel.
func (h *Hub) Register() chan<- *types.RegistrationRequest {
	return h.register
}

// Unregister returns the unregister channel.
func (h *Hub) Unregister() chan<- *room.Player {
	return h.unregister
}
