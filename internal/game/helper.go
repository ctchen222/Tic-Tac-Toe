package game

import "math/rand/v2"

func randomlyChooseFirstPlayer() string {
	if rand.IntN(2) == 0 {
		return PlayerX
	}
	return PlayerO
}

const (
	PlayerX string = "X"
	PlayerO string = "O"
	None    string = ""
)

// Border
const (
	BorderMin = 0 // First index of the board
	BorderMax = 2 // Last index of the board
)
