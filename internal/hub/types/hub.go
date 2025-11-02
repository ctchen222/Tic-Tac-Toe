package types

import "ctchen222/Tic-Tac-Toe/internal/room"

// RegistrationRequest bundles a player and their desired game mode.
type RegistrationRequest struct {
	Player     *room.Player
	Mode       string // e.g., "human", "bot"
	Difficulty string // e.g., "easy", "medium", "hard"
}

func NewRegistrationRequest(player *room.Player, mode string, difficulty string) *RegistrationRequest {
	return &RegistrationRequest{
		Player:     player,
		Mode:       mode,
		Difficulty: difficulty,
	}
}
