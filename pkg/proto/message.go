package proto

import "ctchen222/Tic-Tac-Toe/internal/game"

// ClientToServerMessage represents a message from the client to the server.
type ClientToServerMessage struct {
	Type     string `json:"type" validate:"required"`
	Position []int  `json:"position,omitempty"`
}

// ServerToClientMessage represents a message from the server to the client.
type ServerToClientMessage struct {
	Type   string              `json:"type"`
	Board  [][]game.PlayerMark `json:"board"`
	Next   game.PlayerMark     `json:"next"`
	Winner game.PlayerMark     `json:"winner,omitempty"`
}

// PlayerAssignmentMessage informs a player of their assigned mark.
type PlayerAssignmentMessage struct {
	Type string          `json:"type"`
	Mark game.PlayerMark `json:"mark"`
}
