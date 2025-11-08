package room

import (
	"ctchen222/Tic-Tac-Toe/internal/hub/types"
	"ctchen222/Tic-Tac-Toe/internal/player"
)

// AddPlayer adds a player to the room.
func (r *Room) AddPlayer(p *player.Player) {
	r.Players = append(r.Players, p)
}

// IncomingMoves returns the channel for incoming player moves.
func (r *Room) IncomingMoves() chan<- *types.PlayerMove {
	return r.incomingMoves
}
