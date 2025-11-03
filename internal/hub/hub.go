package hub

import (
	"ctchen222/Tic-Tac-Toe/internal/bot"
	"ctchen222/Tic-Tac-Toe/internal/game"
	"ctchen222/Tic-Tac-Toe/internal/hub/types"
	"ctchen222/Tic-Tac-Toe/internal/match"
	"ctchen222/Tic-Tac-Toe/internal/player"
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
	unregister   chan *player.Player
	matchManager *match.MatchManager
}

// NewHub creates a new hub.
func NewHub() *Hub {
	return &Hub{
		rooms:        make(map[string]*room.Room),
		register:     make(chan *types.RegistrationRequest),
		unregister:   make(chan *player.Player),
		matchManager: match.NewMatchManager(),
	}
}

// Run starts the hub.
func (h *Hub) Run() {
	go h.matchManager.Run()

	moveCalculator := &bot.BotMoveCalculator{}

	for {
		select {
		case req := <-h.register:
			if req.Mode == "bot" {
				log.Printf("Creating bot match for player %s", req.Player.ID)
				player1 := req.Player
				player2 := bot.NewBotPlayer(req.Difficulty)

				roomID := uuid.New().String()
				newRoom := room.NewRoom(roomID, moveCalculator, moveTimeout)
				newRoom.AddPlayer(player1)
				newRoom.AddPlayer(player2)

				if rand.N(2) == 0 {
					newRoom.PlayerMarkMap[player1.ID] = game.PlayerX
					newRoom.PlayerMarkMap[player2.ID] = game.PlayerO
				} else {
					newRoom.PlayerMarkMap[player1.ID] = game.PlayerO
					newRoom.PlayerMarkMap[player2.ID] = game.PlayerX
				}

				h.rooms[roomID] = newRoom
				go newRoom.Start(h.unregister)

				log.Printf("Room %s created for bot match with player %s and bot %s", roomID, player1.ID, player2.ID)

				initialMessage := &proto.ServerToClientMessage{
					Type:  "update",
					Board: newRoom.Game.BoardAsStrings(),
					Next:  newRoom.Game.CurrentTurn,
				}
				h.BroadcastPlayerMarkMessage(newRoom)
				newRoom.Broadcast(initialMessage)
			} else {
				h.matchManager.AddPlayer(req.Player)
			}

		case p := <-h.unregister:
			h.matchManager.RemovePlayer(p.ID)

			for roomID, r := range h.rooms {
				for i, roomPlayer := range r.Players {
					if roomPlayer.ID == p.ID {
						r.Players = append(r.Players[:i], r.Players[i+1:]...)
						log.Printf("Player %s removed from room %s", p.ID, roomID)
						if len(r.Players) == 0 {
							close(r.Done)
							delete(h.rooms, roomID)
							log.Printf("Room %s closed due to no players", roomID)
						}
						break
					}
				}
			}
			log.Printf("Player %s disconnected", p.ID)

		case pair := <-h.matchManager.MatchedPair():
			player1 := pair[0]
			player2 := pair[1]

			roomID := uuid.New().String()
			newRoom := room.NewRoom(roomID, moveCalculator, moveTimeout)
			newRoom.AddPlayer(player1)
			newRoom.AddPlayer(player2)

			if rand.N(2) == 0 {
				newRoom.PlayerMarkMap[player1.ID] = game.PlayerX
				newRoom.PlayerMarkMap[player2.ID] = game.PlayerO
			} else {
				newRoom.PlayerMarkMap[player1.ID] = game.PlayerO
				newRoom.PlayerMarkMap[player2.ID] = game.PlayerX
			}

			h.rooms[roomID] = newRoom
			go newRoom.Start(h.unregister)

			log.Printf("Room %s created with players %s and %s", roomID, player1.ID, player2.ID)

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
	for _, p := range room.Players {
		mark, ok := room.PlayerMarkMap[p.ID]
		if !ok {
			log.Printf("No mark assigned for player %s in room %s", p.ID, room.ID)
			continue
		}
		assignmentMessage := &proto.PlayerAssignmentMessage{
			Type: "assignment",
			Mark: mark,
		}

		data, err := json.Marshal(assignmentMessage)
		if err != nil {
			log.Printf("Error marshalling assignment message for player %s: %v", p.ID, err)
		}
		if err := p.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("Error sending assignment message to player %s: %v", p.ID, err)
		}
	}
}

// Register returns the register channel.
func (h *Hub) Register() chan<- *types.RegistrationRequest {
	return h.register
}

// Unregister returns the unregister channel.
func (h *Hub) Unregister() chan<- *player.Player {
	return h.unregister
}

