package types

import "ctchen222/Tic-Tac-Toe/internal/player"

// RegistrationRequest bundles a player and their desired game mode.
type RegistrationRequest struct {
	Player     *player.Player
	PlayerID   string // Used for reconnection
	Mode       string // e.g., "human", "bot"
	Difficulty string // e.g., "easy", "medium", "hard"
}

func NewRegistrationRequest(p *player.Player, mode string, difficulty string) *RegistrationRequest {
	return &RegistrationRequest{
		Player:     p,
		Mode:       mode,
		Difficulty: difficulty,
	}
}