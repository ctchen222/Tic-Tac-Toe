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
			log.Printf("Received registration request from player %s", req.Player.ID)
			// Handle reconnection
			if req.PlayerID != "" {
				log.Printf("Player %s attempting to reconnect", req.PlayerID)
				room, p := h.findPlayerInRooms(req.PlayerID)
				if room != nil && p != nil && p.Status == player.StatusDisconnected {
					log.Printf("Player %s found in room %s. Re-establishing connection.", p.ID, room.ID)
					p.Conn = req.Player.Conn
					p.Status = player.StatusConnected
					p.LastSeen = time.Now()

					// Start a new readPump for the reconnected player
					go room.ReadPump(p)

					// Send reconnected message and full game state
					reconnectedMsg := &proto.ServerToClientMessage{Type: "reconnected"}
					data, _ := json.Marshal(reconnectedMsg)
					p.Conn.WriteMessage(websocket.TextMessage, data)

					updateMsg := &proto.ServerToClientMessage{
						Type:   "update",
						Board:  room.Game.BoardAsStrings(),
						Next:   room.Game.CurrentTurn,
						Winner: room.Game.Winner,
					}
					data, _ = json.Marshal(updateMsg)
					p.Conn.WriteMessage(websocket.TextMessage, data)

					// Notify opponent
					opponentReconnectedMsg := &proto.ServerToClientMessage{Type: "opponent_reconnected"}
					room.Broadcast(opponentReconnectedMsg)

				} else {
					log.Printf("Could not find disconnected player %s. Rejecting reconnection.", req.PlayerID)
					// Optional: Send a rejection message to the client
					req.Player.Conn.Close()
				}
				continue
			}

			// Handle new player registration
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

				h.BroadcastPlayerMarkMessage(newRoom)
				initialMessage := &proto.ServerToClientMessage{
					Type:  "update",
					Board: newRoom.Game.BoardAsStrings(),
					Next:  newRoom.Game.CurrentTurn,
				}
				newRoom.Broadcast(initialMessage)
			} else {
				h.matchManager.AddPlayer(req.Player)
			}

		case p := <-h.unregister:
			log.Printf("Unregister request for player %s", p.ID)
			h.matchManager.RemovePlayer(p.ID) // Remove from waiting list just in case

			var playerToRequeue *player.Player
			var roomToDeleteID string

			// Find the room the player was in and the other player
			for roomID, r := range h.rooms {
				isPlayerInRoom := false
				var otherPlayer *player.Player

				// Create a copy of the players slice to safely iterate over
				playersInRoom := make([]*player.Player, len(r.Players))
				copy(playersInRoom, r.Players)

				for _, roomPlayer := range playersInRoom {
					if roomPlayer.ID == p.ID {
						isPlayerInRoom = true
					} else {
						otherPlayer = roomPlayer
					}
				}

				if isPlayerInRoom {
					// This unregister was likely triggered by a timeout cleanup.
					// If there were 2 players, the other one should be re-queued.
					if len(playersInRoom) == 2 && otherPlayer != nil {
						playerToRequeue = otherPlayer
						log.Printf("Player %s's opponent timed out. Re-queuing player %s.", otherPlayer.ID, otherPlayer.ID)
					}

					// Mark the room for deletion
					roomToDeleteID = roomID
					break
				}
			}

			// Perform actions outside the loop
			if roomToDeleteID != "" {
				if r, ok := h.rooms[roomToDeleteID]; ok {
					close(r.Done) // Signal the room's goroutines to stop
					delete(h.rooms, roomToDeleteID)
					log.Printf("Room %s closed.", roomToDeleteID)
				}
			}

			if playerToRequeue != nil {
				// Send a message to the player being re-queued
				requeueMsg := &proto.ServerToClientMessage{Type: "requeue", Reason: "opponent_left"}
				data, _ := json.Marshal(requeueMsg)
				if playerToRequeue.Status == player.StatusConnected {
					playerToRequeue.Conn.WriteMessage(websocket.TextMessage, data)
				}

				// Add them back to the matchmaker
				h.matchManager.AddPlayer(playerToRequeue)
			}

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

			h.BroadcastPlayerMarkMessage(newRoom)
			initialMessage := &proto.ServerToClientMessage{
				Type:  "update",
				Board: newRoom.Game.BoardAsStrings(),
				Next:  newRoom.Game.CurrentTurn,
			}
			newRoom.Broadcast(initialMessage)
		}
	}
}

func (h *Hub) findPlayerInRooms(playerID string) (*room.Room, *player.Player) {
	for _, r := range h.rooms {
		for _, p := range r.Players {
			if p.ID == playerID {
				return r, p
			}
		}
	}
	return nil, nil
}

func (h *Hub) BroadcastPlayerMarkMessage(room *room.Room) {
	for _, p := range room.Players {
		if p.Status != player.StatusConnected {
			continue
		}
		mark, ok := room.PlayerMarkMap[p.ID]
		if !ok {
			log.Printf("No mark assigned for player %s in room %s", p.ID, room.ID)
			continue
		}
		assignmentMessage := &proto.PlayerAssignmentMessage{
			Type:     "assignment",
			PlayerID: p.ID,
			Mark:     mark,
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
