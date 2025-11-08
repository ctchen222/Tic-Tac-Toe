package types

import (
	"context"
	"ctchen222/Tic-Tac-Toe/internal/player"
)

// RegistrationRequest is a request from a player to register with the hub.
type RegistrationRequest struct {
	Player     *player.Player
	PlayerID   string
	Mode       string
	Difficulty string
	Ctx        context.Context
}

// PlayerMove is a message from a player, bundled with the player object.
type PlayerMove struct {
	Player  *player.Player
	Message []byte
}
