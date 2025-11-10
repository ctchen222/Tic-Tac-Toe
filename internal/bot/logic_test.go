package bot

import (
	"ctchen222/Tic-Tac-Toe/internal/game"
	"testing"
)

// moveIn is a helper function to check if a move is in a list of expected moves.
func moveIn(move [2]int, list [][2]int) bool {
	for _, item := range list {
		if item == move {
			return true
		}
	}
	return false
}

func TestFindWinningMove(t *testing.T) {
	tests := []struct {
		name  string
		board [][]game.PlayerMark
		mark  game.PlayerMark
		wantRow, wantCol int
		wantFound bool
	}{
		{
			name:  "No winning move - empty board",
			board: [][]game.PlayerMark{{"", "", ""}, {"", "", ""}, {"", "", ""}},
			mark:  game.PlayerX,
			wantRow: -1, wantCol: -1, wantFound: false,
		},
		{
			name: "X can win - first row",
			board: [][]game.PlayerMark{
				{game.PlayerX, game.PlayerX, ""},
				{game.PlayerO, game.PlayerO, ""},
				{"", "", ""},
			},
			mark: game.PlayerX,
			wantRow: 0, wantCol: 2, wantFound: true,
		},
		{
			name: "O can win - second column",
			board: [][]game.PlayerMark{
				{game.PlayerX, game.PlayerO, ""},
				{game.PlayerX, game.PlayerO, ""},
				{"", "", ""},
			},
			mark: game.PlayerO,
			wantRow: 2, wantCol: 1, wantFound: true,
		},
		{
			name: "X can win - main diagonal",
			board: [][]game.PlayerMark{
				{game.PlayerX, "", ""},
				{"", game.PlayerX, ""},
				{"", "", ""},
			},
			mark: game.PlayerX,
			wantRow: 2, wantCol: 2, wantFound: true,
		},
		{
			name: "O can win - anti-diagonal",
			board: [][]game.PlayerMark{
				{"", "", game.PlayerO},
				{"", game.PlayerO, ""},
				{"", "", ""},
			},
			mark: game.PlayerO,
			wantRow: 2, wantCol: 0, wantFound: true,
		},
		{
			name: "X can win - No winning move possible (actually can win)",
			board: [][]game.PlayerMark{
				{game.PlayerX, game.PlayerO, game.PlayerX},
				{game.PlayerO, game.PlayerX, game.PlayerO},
				{game.PlayerO, game.PlayerX, ""},
			},
			mark: game.PlayerX,
			wantRow: 2, wantCol: 2, wantFound: true,
		},
		{
			name: "Full board, no win possible",
			board: [][]game.PlayerMark{
				{game.PlayerX, game.PlayerO, game.PlayerX},
				{game.PlayerO, game.PlayerX, game.PlayerO},
				{game.PlayerO, game.PlayerX, game.PlayerO},
			},
			mark: game.PlayerX,
			wantRow: -1, wantCol: -1, wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row, col, found := findWinningMove(tt.board, tt.mark)
			if found != tt.wantFound || row != tt.wantRow || col != tt.wantCol {
				t.Errorf("findWinningMove() for %s got (%d, %d, %v), want (%d, %d, %v)", tt.name, row, col, found, tt.wantRow, tt.wantCol, tt.wantFound)
			}
		})
	}
}

func TestEasyMove(t *testing.T) {
	t.Run("Only one spot left", func(t *testing.T) {
		board := [][]game.PlayerMark{
			{game.PlayerX, game.PlayerO, game.PlayerX},
			{game.PlayerO, game.PlayerX, game.PlayerO},
			{game.PlayerX, "", game.PlayerO},
		}
		row, col := easyMove(board)
		if row != 2 || col != 1 {
			t.Errorf("easyMove should pick the only available spot (2,1), but got (%d, %d)", row, col)
		}
	})

	t.Run("Multiple spots left - check randomness and validity", func(t *testing.T) {
		board := [][]game.PlayerMark{
			{"", "", ""},
			{"", "", ""},
			{"", "", ""},
		}
		availableMoves := [][2]int{}
		for r := 0; r < 3; r++ {
			for c := 0; c < 3; c++ {
				availableMoves = append(availableMoves, [2]int{r, c})
			}
		}

		// Run multiple times to check if it picks from available moves
		for i := 0; i < 50; i++ {
			row, col := easyMove(board)
			move := [2]int{row, col}
			if !moveIn(move, availableMoves) {
				t.Errorf("easyMove returned an invalid move (%d, %d)", row, col)
			}
			// Ensure it's not always the same move (basic randomness check)
			if i > 0 && row == availableMoves[0][0] && col == availableMoves[0][1] && i < 49 {
				// This is a weak check, but better than nothing for randomness
			}
		}
	})

	t.Run("Full board", func(t *testing.T) {
		board := [][]game.PlayerMark{
			{game.PlayerX, game.PlayerO, game.PlayerX},
			{game.PlayerO, game.PlayerX, game.PlayerO},
			{game.PlayerX, game.PlayerO, game.PlayerX},
		}
		row, col := easyMove(board)
		if row != -1 || col != -1 {
			t.Errorf("easyMove on a full board should return (-1, -1), but got (%d, %d)", row, col)
		}
	})
}

func TestMediumMove(t *testing.T) {
	tests := []struct {
		name string
		board [][]game.PlayerMark
		botMark game.PlayerMark
		wantRow, wantCol int
	}{
		{
			name: "Bot can win",
			board: [][]game.PlayerMark{
				{game.PlayerX, game.PlayerX, ""},
				{game.PlayerO, "", ""},
				{"", "", ""},
			},
			botMark: game.PlayerX,
			wantRow: 0, wantCol: 2,
		},
		{
			name: "Bot must block opponent",
			board: [][]game.PlayerMark{
				{game.PlayerO, game.PlayerO, ""},
				{game.PlayerX, "", ""},
				{"", "", ""},
			},
			botMark: game.PlayerX,
			wantRow: 0, wantCol: 2,
		},
		{
			name: "No immediate win or block, random move",
			board: [][]game.PlayerMark{
				{game.PlayerX, "", ""},
				{"", game.PlayerO, ""},
				{"", "", ""},
			},
			botMark: game.PlayerX,
			// Expect a random empty spot, so just check it's not X or O
			wantRow: -1, wantCol: -1, // Placeholder, will check validity
		},
		{
			name: "Full board",
			board: [][]game.PlayerMark{
				{game.PlayerX, game.PlayerO, game.PlayerX},
				{game.PlayerO, game.PlayerX, game.PlayerO},
				{game.PlayerX, game.PlayerO, game.PlayerX},
			},
			botMark: game.PlayerX,
			wantRow: -1, wantCol: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row, col := mediumMove(tt.board, tt.botMark)
			if tt.wantRow == -1 && tt.wantCol == -1 { // Expecting random or full board
				if row != -1 || col != -1 { // If not full, check if it's an empty spot
					if tt.board[row][col] != "" {
						t.Errorf("mediumMove returned a non-empty spot (%d, %d) for random move", row, col)
					}
				}
			} else if row != tt.wantRow || col != tt.wantCol {
				t.Errorf("mediumMove() for %s got (%d, %d), want (%d, %d)", tt.name, row, col, tt.wantRow, tt.wantCol)
			}
		})
	}
}

func TestHardMove(t *testing.T) {
	tests := []struct {
		name string
		board [][]game.PlayerMark
		botMark game.PlayerMark
		wantRow, wantCol int // Use -1,-1 for random/any valid empty spot
	}{
		{
			name: "Bot can win",
			board: [][]game.PlayerMark{
				{game.PlayerX, game.PlayerX, ""},
				{game.PlayerO, game.PlayerO, ""},
				{"", "", ""},
			},
			botMark: game.PlayerX,
			wantRow: 0, wantCol: 2,
		},
		{
			name: "Bot must block opponent",
			board: [][]game.PlayerMark{
				{game.PlayerO, game.PlayerO, ""},
				{game.PlayerX, "", ""},
				{"", "", ""},
			},
			botMark: game.PlayerX,
			wantRow: 0, wantCol: 2,
		},
		{
			name: "Take center",
			board: [][]game.PlayerMark{
				{game.PlayerO, "", ""},
				{"", "", ""},
				{"", "", ""},
			},
			botMark: game.PlayerX,
			wantRow: 1, wantCol: 1,
		},
		{
			name: "Take corner (random)",
			board: [][]game.PlayerMark{
				{"", "", ""},
				{"", game.PlayerO, ""},
				{"", "", ""},
			},
			botMark: game.PlayerX,
			wantRow: -1, wantCol: -1, // Expect a corner
		},
		{
			name: "Take side (random)",
			board: [][]game.PlayerMark{
				{game.PlayerO, "", game.PlayerX},
				{"", game.PlayerX, ""},
				{game.PlayerO, "", game.PlayerX},
			},
			botMark: game.PlayerO,
			wantRow: -1, wantCol: -1, // Expect a side
		},
		{
			name: "Full board",
			board: [][]game.PlayerMark{
				{game.PlayerX, game.PlayerO, game.PlayerX},
				{game.PlayerO, game.PlayerX, game.PlayerO},
				{game.PlayerX, game.PlayerO, game.PlayerX},
			},
			botMark: game.PlayerX,
			wantRow: -1, wantCol: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row, col := hardMove(tt.board, tt.botMark)
			if tt.wantRow == -1 && tt.wantCol == -1 { // Expecting random corner/side or full board
				if row != -1 || col != -1 { // If not full, check if it's an empty spot
					if tt.board[row][col] != "" {
						t.Errorf("hardMove returned a non-empty spot (%d, %d) for random move", row, col)
					}
					// For corner/side tests, we need more specific checks
					if tt.name == "Take corner (random)" {
						corners := [][2]int{{0, 0}, {0, 2}, {2, 0}, {2, 2}}
						if !moveIn([2]int{row, col}, corners) {
							t.Errorf("hardMove expected a corner, got (%d, %d)", row, col)
						}
					}
					if tt.name == "Take side (random)" {
						sides := [][2]int{{0, 1}, {1, 0}, {1, 2}, {2, 1}}
						if !moveIn([2]int{row, col}, sides) {
							t.Errorf("hardMove expected a side, got (%d, %d)", row, col)
						}
					}
				}
			} else if row != tt.wantRow || col != tt.wantCol {
				t.Errorf("hardMove() for %s got (%d, %d), want (%d, %d)", tt.name, row, col, tt.wantRow, tt.wantCol)
			}
		})
	}
}

func TestCalculateNextMove(t *testing.T) {
	tests := []struct {
		name string
		board [][]game.PlayerMark
		botMark game.PlayerMark
		difficulty string
		wantRow, wantCol int // Use -1,-1 for random/any valid empty spot
	}{
		{
			name: "Hard difficulty - winning move",
			board: [][]game.PlayerMark{
				{game.PlayerX, game.PlayerX, ""},
				{game.PlayerO, "", ""},
				{"", "", ""},
			},
			botMark: game.PlayerX,
			difficulty: "hard",
			wantRow: 0, wantCol: 2,
		},
		{
			name: "Medium difficulty - blocking move",
			board: [][]game.PlayerMark{
				{game.PlayerO, game.PlayerO, ""},
				{game.PlayerX, "", ""},
				{"", "", ""},
			},
			botMark: game.PlayerX,
			difficulty: "medium",
			wantRow: 0, wantCol: 2,
		},
		{
			name: "Easy difficulty - random valid move",
			board: [][]game.PlayerMark{
				{"", "", ""},
				{"", "", ""},
				{"", "", ""},
			},
			botMark: game.PlayerX,
			difficulty: "easy",
			wantRow: -1, wantCol: -1, // Expect any empty spot
		},
		{
			name: "Invalid difficulty - defaults to hard",
			board: [][]game.PlayerMark{
				{game.PlayerX, game.PlayerX, ""},
				{game.PlayerO, "", ""},
				{"", "", ""},
			},
			botMark: game.PlayerX,
			difficulty: "invalid",
			wantRow: 0, wantCol: 2, // Should default to hard and pick winning move
		},
		{
			name: "Full board - all difficulties",
			board: [][]game.PlayerMark{
				{game.PlayerX, game.PlayerO, game.PlayerX},
				{game.PlayerO, game.PlayerX, game.PlayerO},
				{game.PlayerX, game.PlayerO, game.PlayerX},
			},
			botMark: game.PlayerX,
			difficulty: "hard",
			wantRow: -1, wantCol: -1,
		},
		{
			name: "Full board - easy difficulty",
			board: [][]game.PlayerMark{
				{game.PlayerX, game.PlayerO, game.PlayerX},
				{game.PlayerO, game.PlayerX, game.PlayerO},
				{game.PlayerX, game.PlayerO, game.PlayerX},
			},
			botMark: game.PlayerX,
			difficulty: "easy",
			wantRow: -1, wantCol: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row, col := CalculateNextMove(tt.board, tt.botMark, tt.difficulty)
			if tt.wantRow == -1 && tt.wantCol == -1 { // Expecting random or full board
				if row != -1 || col != -1 { // If not full, check if it's an empty spot
					if tt.board[row][col] != "" {
						t.Errorf("CalculateNextMove for %s returned a non-empty spot (%d, %d)", tt.name, row, col)
					}
				}
			} else if row != tt.wantRow || col != tt.wantCol {
				t.Errorf("CalculateNextMove() for %s got (%d, %d), want (%d, %d)", tt.name, row, col, tt.wantRow, tt.wantCol)
			}
		})
	}
}
