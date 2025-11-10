package game

import (
	"math/rand/v2"
)

// PlayerMark represents the mark of a player (X, O) or an empty cell.
type PlayerMark string
type GameResult string

const (
	// Player marks
	None    PlayerMark = ""
	PlayerX PlayerMark = "X"
	PlayerO PlayerMark = "O"
	DRAW    PlayerMark = "Draw"

	// Game results
	Draw GameResult = "Draw"

	// Board boundaries
	BorderMin = 0
	BorderMax = 2

	// Redis hash fields
	FieldBoard    = "board"
	FieldPlayerX  = "player_x"
	FieldPlayerO  = "player_o"
	FieldNextTurn = "next_turn"
	FieldWinner   = "winner"
	FieldStatus   = "status"
)

// GameStateDTO is a Data Transfer Object for game state.
// It's used to pass game state information around without holding state in memory.
type GameStateDTO struct {
	Board       [3][3]PlayerMark
	CurrentTurn PlayerMark
	Winner      PlayerMark
	IsDraw      bool
	PlayerXID   string
	PlayerOID   string
}

// RandomlyChooseFirstPlayer randomly selects who goes first.
func RandomlyChooseFirstPlayer() PlayerMark {
	if rand.IntN(2) == 0 {
		return PlayerX
	}
	return PlayerO
}

// BoardArrayToSlice converts a 3x3 array to a slice of slices.
func BoardArrayToSlice(board [3][3]PlayerMark) [][]PlayerMark {
	slice := make([][]PlayerMark, 3)
	for i := 0; i < 3; i++ {
		slice[i] = make([]PlayerMark, 3)
		copy(slice[i], board[i][:])
	}
	return slice
}

// CheckWinner checks if there is a winner on the board. Returns the winner's mark or None.
func CheckWinner(board [3][3]PlayerMark) PlayerMark {
	// Check rows
	for i := range [3]int{} {
		if board[i][0] != None && board[i][0] == board[i][1] && board[i][1] == board[i][2] {
			return board[i][0]
		}
	}

	// Check columns
	for i := range [3]int{} {
		if board[0][i] != None && board[0][i] == board[1][i] && board[1][i] == board[2][i] {
			return board[0][i]
		}
	}

	// Check diagonals
	if board[0][0] != None && board[0][0] == board[1][1] && board[1][1] == board[2][2] {
		return board[0][0]
	}
	if board[0][2] != None && board[0][2] == board[1][1] && board[1][1] == board[2][0] {
		return board[0][2]
	}

	isBoardFull := IsBoardFull(board)
	if isBoardFull {
		return DRAW
	}

	return None
}

// IsBoardFull checks if the board has any empty cells left.
func IsBoardFull(board [3][3]PlayerMark) bool {
	for r := range [3]int{} {
		for c := range [3]int{} {
			if board[r][c] == None {
				return false
			}
		}
	}
	return true
}

