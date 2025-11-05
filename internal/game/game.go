package game

import (
	"errors"
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

	// Game results
	Draw GameResult = "Draw"

	// Board boundaries
	BorderMin = 0
	BorderMax = 2
)

type Game struct {
	Board       [3][3]PlayerMark
	CurrentTurn PlayerMark
	Winner      PlayerMark
}

func NewGame() *Game {
	return &Game{
		Board:       [3][3]PlayerMark{},
		CurrentTurn: randomlyChooseFirstPlayer(),
		Winner:      None,
	}
}

func (g *Game) Move(row, col int) error {
	if g.Winner != None {
		return errors.New("game already finished")
	}
	if row < BorderMin || row > BorderMax || col < BorderMin || col > BorderMax {
		return errors.New("invalid move")
	}
	if g.Board[row][col] != None {
		return errors.New("cell already occupied")
	}

	g.Board[row][col] = g.CurrentTurn
	if g.CurrentTurn == PlayerX {
		g.CurrentTurn = PlayerO
	} else {
		g.CurrentTurn = PlayerX
	}

	g.Winner = g.checkWinner()
	return nil
}

// BoardAsStrings converts the game board to a dynamic slice of slices of strings.
func (g *Game) BoardAsStrings() [][]PlayerMark {
	board := make([][]PlayerMark, 3)
	for i := range [3]int{} {
		board[i] = make([]PlayerMark, 3)
		for j := range [3]int{} {
			board[i][j] = g.Board[i][j]
		}
	}
	return board
}

func (g *Game) checkWinner() PlayerMark {
	// Check rows
	for i := range [3]int{} {
		if g.Board[i][0] != None && g.Board[i][0] == g.Board[i][1] && g.Board[i][1] == g.Board[i][2] {
			return g.Board[i][0]
		}
	}

	// Check columns
	for i := range [3]int{} {
		if g.Board[0][i] != None && g.Board[0][i] == g.Board[1][i] && g.Board[1][i] == g.Board[2][i] {
			return g.Board[0][i]
		}
	}

	// Check diagonals
	if g.Board[0][0] != None && g.Board[0][0] == g.Board[1][1] && g.Board[1][1] == g.Board[2][2] {
		return g.Board[0][0]
	}
	if g.Board[0][2] != None && g.Board[0][2] == g.Board[1][1] && g.Board[1][1] == g.Board[2][0] {
		return g.Board[0][2]
	}

	if g.IsDraw() {
		return PlayerMark(Draw)
	}

	return None
}

// IsDraw checks if the game is a draw.
func (g *Game) IsDraw() bool {
	// If there is a winner, it's not a draw
	if g.Winner != None {
		return false
	}

	// If there are any empty cells, it's not a draw
	for r := range [3]int{} {
		for c := range [3]int{} {
			if g.Board[r][c] == None {
				return false
			}
		}
	}

	// No winner and no empty cells means it's a draw
	return true
}

func randomlyChooseFirstPlayer() PlayerMark {
	if rand.IntN(2) == 0 {
		return PlayerX
	}
	return PlayerO
}
