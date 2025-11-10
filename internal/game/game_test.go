package game

import (
	"testing"
)

func TestCheckWinner(t *testing.T) {
	tests := []struct {
		name  string
		board [3][3]PlayerMark
		want  PlayerMark
	}{
		{
			name:  "No winner - empty board",
			board: [3][3]PlayerMark{},
			want:  None,
		},
		{
			name: "No winner - partial board",
			board: [3][3]PlayerMark{
				{PlayerX, None, None},
				{None, PlayerO, None},
				{None, None, None},
			},
			want: None,
		},
		{
			name: "X wins - first row",
			board: [3][3]PlayerMark{
				{PlayerX, PlayerX, PlayerX},
				{None, PlayerO, None},
				{None, None, PlayerO},
			},
			want: PlayerX,
		},
		{
			name: "O wins - second column",
			board: [3][3]PlayerMark{
				{PlayerX, PlayerO, None},
				{PlayerX, PlayerO, None},
				{None, PlayerO, None},
			},
			want: PlayerO,
		},
		{
			name: "X wins - main diagonal",
			board: [3][3]PlayerMark{
				{PlayerX, None, None},
				{None, PlayerX, None},
				{None, None, PlayerX},
			},
			want: PlayerX,
		},
		{
			name: "O wins - anti-diagonal",
			board: [3][3]PlayerMark{
				{None, None, PlayerO},
				{None, PlayerO, None},
				{PlayerO, None, None},
			},
			want: PlayerO,
		},
		{
			name: "No winner - full board (draw)",
			board: [3][3]PlayerMark{
				{PlayerX, PlayerO, PlayerX},
				{PlayerX, PlayerO, PlayerO},
				{PlayerO, PlayerX, PlayerX},
			},
			want: PlayerMark(Draw),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CheckWinner(tt.board); got != tt.want {
				t.Errorf("CheckWinner() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsBoardFull(t *testing.T) {
	tests := []struct {
		name  string
		board [3][3]PlayerMark
		want  bool
	}{
		{
			name:  "Empty board is not full",
			board: [3][3]PlayerMark{},
			want:  false,
		},
		{
			name: "Partial board is not full",
			board: [3][3]PlayerMark{
				{PlayerX, None, None},
				{None, PlayerO, None},
				{None, None, None},
			},
			want: false,
		},
		{
			name: "Full board is full",
			board: [3][3]PlayerMark{
				{PlayerX, PlayerO, PlayerX},
				{PlayerX, PlayerO, PlayerO},
				{PlayerO, PlayerX, PlayerX},
			},
			want: true,
		},
		{
			name: "Full board with winner is full",
			board: [3][3]PlayerMark{
				{PlayerX, PlayerX, PlayerX},
				{PlayerO, PlayerO, PlayerX},
				{PlayerO, PlayerX, PlayerO},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsBoardFull(tt.board); got != tt.want {
				t.Errorf("IsBoardFull() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRandomlyChooseFirstPlayer(t *testing.T) {
	// Run multiple times to ensure randomness, but check for valid output
	// This is not a statistical test for perfect randomness, just for valid values
	seenX := false
	seenO := false
	for i := 0; i < 100; i++ {
		player := RandomlyChooseFirstPlayer()
		if player != PlayerX && player != PlayerO {
			t.Errorf("RandomlyChooseFirstPlayer() returned invalid player: %v", player)
		}
		if player == PlayerX {
			seenX = true
		}
		if player == PlayerO {
			seenO = true
		}
	}

	if !seenX || !seenO {
		t.Errorf("RandomlyChooseFirstPlayer() did not return both PlayerX and PlayerO over 100 runs. Seen X: %v, Seen O: %v", seenX, seenO)
	}
}
