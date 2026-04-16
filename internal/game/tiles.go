package game

import (
	"encoding/json"
	"fmt"
)

// Letter is a single tile character. 'A'..'Z' for lettered tiles and '?' for an
// unplayed blank (in a bag or rack). Placed tiles always carry 'A'..'Z'; the
// Blank flag on PlacedTile records that the placement came from a blank.
type Letter byte

const Blank Letter = '?'

// LetterValues gives point value per tile in the bag. Blanks score 0.
var LetterValues = map[Letter]int{
	'A': 1, 'B': 3, 'C': 3, 'D': 2, 'E': 1, 'F': 4, 'G': 2, 'H': 4,
	'I': 1, 'J': 8, 'K': 5, 'L': 1, 'M': 3, 'N': 1, 'O': 1, 'P': 3,
	'Q': 10, 'R': 1, 'S': 1, 'T': 1, 'U': 1, 'V': 4, 'W': 4, 'X': 8,
	'Y': 4, 'Z': 10,
	Blank: 0,
}

// LetterCounts is the standard English Scrabble letter distribution (100 tiles).
var LetterCounts = map[Letter]int{
	'A': 9, 'B': 2, 'C': 2, 'D': 4, 'E': 12, 'F': 2, 'G': 3, 'H': 2,
	'I': 9, 'J': 1, 'K': 1, 'L': 4, 'M': 2, 'N': 6, 'O': 8, 'P': 2,
	'Q': 1, 'R': 6, 'S': 4, 'T': 6, 'U': 4, 'V': 2, 'W': 2, 'X': 1,
	'Y': 2, 'Z': 1,
	Blank: 2,
}

// RackSize is the number of tiles a player holds at a time.
const RackSize = 7

// BoardSize is 15x15.
const BoardSize = 15

func (l Letter) IsLetter() bool { return l >= 'A' && l <= 'Z' }
func (l Letter) IsBlank() bool  { return l == Blank }

func (l Letter) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(l))
}

func (l *Letter) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	if len(s) != 1 {
		return fmt.Errorf("letter must be a single character, got %q", s)
	}
	c := Letter(s[0])
	if !c.IsLetter() && !c.IsBlank() {
		return fmt.Errorf("invalid letter %q", s)
	}
	*l = c
	return nil
}
