package bot

import (
	"ctchen222/Tic-Tac-Toe/internal/game"
	"math/rand/v2"
)

// BotMoveCalculator implements the room.MoveCalculator interface.
type BotMoveCalculator struct{}

// CalculateNextMove calls the package-level function to satisfy the interface.
func (c *BotMoveCalculator) CalculateNextMove(board [][]game.PlayerMark, mark game.PlayerMark, difficulty string) (row, col int) {
	return CalculateNextMove(board, mark, difficulty)
}

// CalculateNextMove determines the bot's next move based on the specified difficulty.
func CalculateNextMove(board [][]game.PlayerMark, botMark game.PlayerMark, difficulty string) (row, col int) {
	switch difficulty {
	case "easy":
		return easyMove(board)
	case "medium":
		return mediumMove(board, botMark)
	case "hard":
		return hardMove(board, botMark)
	default:
		return hardMove(board, botMark)
	}
}

// easyMove makes a completely random move.
func easyMove(board [][]game.PlayerMark) (row, col int) {
	var availableMoves [][2]int
	for r, rowData := range board {
		for c, cell := range rowData {
			if cell == "" {
				availableMoves = append(availableMoves, [2]int{r, c})
			}
		}
	}

	if len(availableMoves) == 0 {
		return -1, -1 // No moves left
	}

	randomMove := availableMoves[rand.IntN(len(availableMoves))]
	return randomMove[0], randomMove[1]
}

// mediumMove will win if it can, block if it must, otherwise move randomly.
func mediumMove(board [][]game.PlayerMark, botMark game.PlayerMark) (row, col int) {
	opponentMark := game.PlayerX
	if botMark == game.PlayerX {
		opponentMark = game.PlayerO
	}

	// 1. Win: Check if the bot can win in the next move
	nextRow, nextCol, canWin := findWinningMove(board, botMark)
	if canWin {
		return nextRow, nextCol
	}

	// 2. Block: Check if the opponent is about to win and block them
	nextRow, nextCol, canBlock := findWinningMove(board, opponentMark)
	if canBlock {
		return nextRow, nextCol
	}

	// 3. Random: Otherwise, make a random move
	return easyMove(board)
}

// hardMove implements the optimal strategy.
func hardMove(board [][]game.PlayerMark, botMark game.PlayerMark) (row, col int) {
	opponentMark := game.PlayerX
	if botMark == game.PlayerX {
		opponentMark = game.PlayerO
	}

	// 1. Win: Check if the bot can win in the next move
	nextRow, nextCol, canWin := findWinningMove(board, botMark)
	if canWin {
		return nextRow, nextCol
	}

	// 2. Block: Check if the opponent is about to win and block them
	nextRow, nextCol, canBlock := findWinningMove(board, opponentMark)
	if canBlock {
		return nextRow, nextCol
	}

	// 3. Center: Take the center if it's available
	if board[1][1] == "" {
		return 1, 1
	}

	// 4. Corners: Take an available corner randomly
	availableCorners := [][2]int{}
	corners := [][2]int{{0, 0}, {0, 2}, {2, 0}, {2, 2}}
	for _, corner := range corners {
		if board[corner[0]][corner[1]] == "" {
			availableCorners = append(availableCorners, corner)
		}
	}
	if len(availableCorners) > 0 {
		randomCorner := availableCorners[rand.IntN(len(availableCorners))]
		return randomCorner[0], randomCorner[1]
	}

	// 5. Sides: Take any available side randomly
	availableSides := [][2]int{}
	sides := [][2]int{{0, 1}, {1, 0}, {1, 2}, {2, 1}}
	for _, side := range sides {
		if board[side[0]][side[1]] == "" {
			availableSides = append(availableSides, side)
		}
	}
	if len(availableSides) > 0 {
		randomSide := availableSides[rand.IntN(len(availableSides))]
		return randomSide[0], randomSide[1]
	}

	// Should not happen in a normal game, but as a fallback
	return -1, -1
}

// findWinningMove checks if a player has a potential winning move (two in a row with an empty third).
func findWinningMove(board [][]game.PlayerMark, mark game.PlayerMark) (row, col int, found bool) {
	// Check rows
	for r := range [3]int{} {
		if board[r][0] == mark && board[r][1] == mark && board[r][2] == "" {
			return r, 2, true
		}
		if board[r][0] == mark && board[r][2] == mark && board[r][1] == "" {
			return r, 1, true
		}
		if board[r][1] == mark && board[r][2] == mark && board[r][0] == "" {
			return r, 0, true
		}
	}

	// Check columns
	for c := range [3]int{} {
		if board[0][c] == mark && board[1][c] == mark && board[2][c] == "" {
			return 2, c, true
		}
		if board[0][c] == mark && board[2][c] == mark && board[1][c] == "" {
			return 1, c, true
		}
		if board[1][c] == mark && board[2][c] == mark && board[0][c] == "" {
			return 0, c, true
		}
	}

	// Check diagonals
	// Top-left to bottom-right
	if board[0][0] == mark && board[1][1] == mark && board[2][2] == "" {
		return 2, 2, true
	}
	if board[0][0] == mark && board[2][2] == mark && board[1][1] == "" {
		return 1, 1, true
	}
	if board[1][1] == mark && board[2][2] == mark && board[0][0] == "" {
		return 0, 0, true
	}

	// Top-right to bottom-left
	if board[0][2] == mark && board[1][1] == mark && board[2][0] == "" {
		return 2, 0, true
	}
	if board[0][2] == mark && board[2][0] == mark && board[1][1] == "" {
		return 1, 1, true
	}
	if board[1][1] == mark && board[2][0] == mark && board[0][2] == "" {
		return 0, 2, true
	}

	return -1, -1, false
}
