package game

import (
	"errors"
)

type Game struct {
	Board       [3][3]string
	CurrentTurn string
	Winner      string
}

func NewGame() *Game {
	return &Game{
		Board:       [3][3]string{},
		CurrentTurn: randomlyChooseFirstPlayer(),
	}
}

func (g *Game) Move(row, col int) error {
	if g.Winner != "" {
		return errors.New("game already finished")
	}
	if row < BorderMin || row > BorderMax || col < BorderMin || col > BorderMax {
		return errors.New("invalid move")
	}
	if g.Board[row][col] != "" {
		return errors.New("cell already occupied")
	}

	g.Board[row][col] = g.CurrentTurn
	if g.CurrentTurn == PlayerX {
		g.CurrentTurn = PlayerO
	} else {
		g.CurrentTurn = PlayerX
	}

	g.checkWinner()
	return nil
}

// ToArray converts the game board to a dynamic slice of slices.
func (g *Game) BoardAsStrings() [][]string {
	board := make([][]string, 3)
	for i := range [3]int{} {
		board[i] = make([]string, 3)
		for j := range [3]int{} {
			board[i][j] = g.Board[i][j]
		}
	}
	return board
}

// ToArray converts the game board to a slice of slices.
func (g *Game) ToArray() [][]string {
	board := make([][]string, 3)
	for i := 0; i < 3; i++ {
		board[i] = make([]string, 3)
		for j := 0; j < 3; j++ {
			board[i][j] = g.Board[i][j]
		}
	}
	return board
}

// checkWinner checks if there is a winner.
func (g *Game) checkWinner() {
	// Check rows
	for i := range [3]int{} {
		if g.Board[i][0] == g.Board[i][1] && g.Board[i][1] == g.Board[i][2] && g.Board[i][0] != "" {
			g.Winner = g.Board[i][0]
			return
		}
	}

	// Check columns
	for i := range [3]int{} {
		if g.Board[0][i] == g.Board[1][i] && g.Board[1][i] == g.Board[2][i] && g.Board[0][i] != "" {
			g.Winner = g.Board[0][i]
			return
		}
	}

	// Check diagonals
	if g.Board[0][0] == g.Board[1][1] && g.Board[1][1] == g.Board[2][2] && g.Board[0][0] != "" {
		g.Winner = g.Board[0][0]
		return
	}
	if g.Board[0][2] == g.Board[1][1] && g.Board[1][1] == g.Board[2][0] && g.Board[0][2] != "" {
		g.Winner = g.Board[0][2]
		return
	}

	// Check for draw
	draw := true
	for i := range [3]int{} {
		for j := range [3]int{} {
			if g.Board[i][j] == "" {
				draw = false
				break
			}
		}
	}
	if draw {
		g.Winner = "draw"
	}
}
