package game

// Premium represents a premium-square multiplier on the board.
type Premium int

const (
	PremNone Premium = iota
	PremDL           // double letter
	PremTL           // triple letter
	PremDW           // double word
	PremTW           // triple word
)

// PlacedTile is a tile committed to the board.
type PlacedTile struct {
	Letter Letter `json:"letter"`
	Blank  bool   `json:"blank"`
}

// Value returns the tile's point value (blanks score 0).
func (t PlacedTile) Value() int {
	if t.Blank {
		return 0
	}
	return LetterValues[t.Letter]
}

// Board is a 15x15 grid. A nil entry means empty.
type Board struct {
	Squares [BoardSize][BoardSize]*PlacedTile `json:"squares"`
}

func NewBoard() *Board { return &Board{} }

// Center returns the coordinates of the center (star) square.
func Center() (int, int) { return BoardSize / 2, BoardSize / 2 }

// InBounds reports whether (row, col) is on the board.
func InBounds(row, col int) bool {
	return row >= 0 && row < BoardSize && col >= 0 && col < BoardSize
}

// At returns the tile at (row, col) or nil if empty.
func (b *Board) At(row, col int) *PlacedTile {
	if !InBounds(row, col) {
		return nil
	}
	return b.Squares[row][col]
}

// IsEmpty reports whether the board has no tiles placed.
func (b *Board) IsEmpty() bool {
	for r := range BoardSize {
		for c := range BoardSize {
			if b.Squares[r][c] != nil {
				return false
			}
		}
	}
	return true
}

// PremiumAt returns the premium multiplier type for the square.
func PremiumAt(row, col int) Premium { return premiumLayout[row][col] }

// premiumLayout is the standard English Scrabble premium-square grid.
// See en.wikipedia.org/wiki/Scrabble#Board for the canonical layout.
var premiumLayout = func() [BoardSize][BoardSize]Premium {
	var p [BoardSize][BoardSize]Premium
	set := func(prem Premium, coords ...[2]int) {
		for _, c := range coords {
			p[c[0]][c[1]] = prem
		}
	}
	set(PremTW,
		[2]int{0, 0}, [2]int{0, 7}, [2]int{0, 14},
		[2]int{7, 0}, [2]int{7, 14},
		[2]int{14, 0}, [2]int{14, 7}, [2]int{14, 14},
	)
	set(PremDW,
		[2]int{1, 1}, [2]int{2, 2}, [2]int{3, 3}, [2]int{4, 4},
		[2]int{1, 13}, [2]int{2, 12}, [2]int{3, 11}, [2]int{4, 10},
		[2]int{13, 1}, [2]int{12, 2}, [2]int{11, 3}, [2]int{10, 4},
		[2]int{13, 13}, [2]int{12, 12}, [2]int{11, 11}, [2]int{10, 10},
		[2]int{7, 7},
	)
	set(PremTL,
		[2]int{1, 5}, [2]int{1, 9},
		[2]int{5, 1}, [2]int{5, 5}, [2]int{5, 9}, [2]int{5, 13},
		[2]int{9, 1}, [2]int{9, 5}, [2]int{9, 9}, [2]int{9, 13},
		[2]int{13, 5}, [2]int{13, 9},
	)
	set(PremDL,
		[2]int{0, 3}, [2]int{0, 11},
		[2]int{2, 6}, [2]int{2, 8},
		[2]int{3, 0}, [2]int{3, 7}, [2]int{3, 14},
		[2]int{6, 2}, [2]int{6, 6}, [2]int{6, 8}, [2]int{6, 12},
		[2]int{7, 3}, [2]int{7, 11},
		[2]int{8, 2}, [2]int{8, 6}, [2]int{8, 8}, [2]int{8, 12},
		[2]int{11, 0}, [2]int{11, 7}, [2]int{11, 14},
		[2]int{12, 6}, [2]int{12, 8},
		[2]int{14, 3}, [2]int{14, 11},
	)
	return p
}()
