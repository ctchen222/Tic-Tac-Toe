package types

import (
	"context"
	"ctchen222/Tic-Tac-Toe/internal/player"
)

// RegistrationRequest represents a request to register a player.

type RegistrationRequest struct {
	Player     *player.Player
	PlayerID   string // Used for reconnection
	Mode       string // "human" or "bot"
	Difficulty string // "easy", "medium", "hard"
	Ctx        context.Context
}
