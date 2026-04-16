package game

import (
	"errors"
	"fmt"
	"strings"
)

// Placement is one new tile being played on the board.
type Placement struct {
	Row    int    `json:"row"`
	Col    int    `json:"col"`
	Letter Letter `json:"letter"` // the letter as it appears on the board ('A'..'Z')
	Blank  bool   `json:"blank"`  // true if this tile came from a blank rack tile
}

// WordSet is the contract the engine needs from a dictionary.
type WordSet interface {
	Contains(word string) bool
}

// ScoredWord is a single word formed by a play.
type ScoredWord struct {
	Word  string `json:"word"`
	Score int    `json:"score"`
}

// PlayResult describes the outcome of a successful play.
type PlayResult struct {
	Words    []ScoredWord `json:"words"`
	Score    int          `json:"score"` // total including bingo bonus
	Bingo    bool         `json:"bingo"`
	UsedRack []Letter     `json:"usedRack"` // rack tiles consumed, blanks as '?'
}

// InvalidWordsError is returned when one or more formed words are not in the
// dictionary. The caller can surface the words to highlight them in the UI.
type InvalidWordsError struct{ Words []string }

func (e *InvalidWordsError) Error() string {
	return "invalid word(s): " + strings.Join(e.Words, ", ")
}

// ValidateAndScore checks a proposed play against the current board, rack,
// and dictionary. On success it returns the score breakdown; the caller is
// responsible for applying the placements to the board and updating the rack.
func ValidateAndScore(board *Board, rack []Letter, placements []Placement, dict WordSet) (*PlayResult, error) {
	if len(placements) == 0 {
		return nil, errors.New("no tiles placed")
	}
	if len(placements) > RackSize {
		return nil, fmt.Errorf("cannot place more than %d tiles in one turn", RackSize)
	}

	placedSet := map[[2]int]bool{}
	for _, p := range placements {
		if !InBounds(p.Row, p.Col) {
			return nil, fmt.Errorf("placement out of bounds: (%d,%d)", p.Row, p.Col)
		}
		if !p.Letter.IsLetter() {
			return nil, fmt.Errorf("placement letter must be A-Z, got %q", string(p.Letter))
		}
		if board.At(p.Row, p.Col) != nil {
			return nil, fmt.Errorf("square (%d,%d) already occupied", p.Row, p.Col)
		}
		if placedSet[[2]int{p.Row, p.Col}] {
			return nil, fmt.Errorf("duplicate placement at (%d,%d)", p.Row, p.Col)
		}
		placedSet[[2]int{p.Row, p.Col}] = true
	}

	usedRack, err := consumeRack(rack, placements)
	if err != nil {
		return nil, err
	}

	horizOK, vertOK := true, true
	row0, col0 := placements[0].Row, placements[0].Col
	for _, p := range placements[1:] {
		if p.Row != row0 {
			horizOK = false
		}
		if p.Col != col0 {
			vertOK = false
		}
	}
	if !horizOK && !vertOK {
		return nil, errors.New("all placed tiles must be in the same row or column")
	}

	if board.IsEmpty() {
		cr, cc := Center()
		if !placedSet[[2]int{cr, cc}] {
			return nil, errors.New("first move must cover the center square")
		}
	} else {
		connected := false
		for _, p := range placements {
			if hasAdjacentExistingTile(board, p.Row, p.Col) {
				connected = true
				break
			}
		}
		if !connected {
			return nil, errors.New("play must connect to an existing tile")
		}
	}

	preview := boardWithPlacements(board, placements)

	// Decide main direction. Multi-tile plays are unambiguous. Single-tile
	// plays pick whichever direction produces a >=2-letter run; if both do,
	// horizontal is main and vertical becomes a cross-word.
	mainHoriz := horizOK
	if horizOK && vertOK {
		p := placements[0]
		hr := extractRun(preview, p.Row, p.Col, true)
		vr := extractRun(preview, p.Row, p.Col, false)
		switch {
		case len(hr.cells) >= 2:
			mainHoriz = true
		case len(vr.cells) >= 2:
			mainHoriz = false
		default:
			return nil, errors.New("play does not form a word of length >= 2")
		}
	}

	if err := checkContiguity(preview, placements, mainHoriz); err != nil {
		return nil, err
	}

	words := collectWords(preview, placements, mainHoriz)
	if len(words) == 0 {
		return nil, errors.New("play does not form a word of length >= 2")
	}

	var invalid []string
	for _, w := range words {
		if !dict.Contains(w.word) {
			invalid = append(invalid, w.word)
		}
	}
	if len(invalid) > 0 {
		return nil, &InvalidWordsError{Words: invalid}
	}

	res := &PlayResult{UsedRack: usedRack}
	for _, w := range words {
		score := scoreWord(preview, w, placedSet)
		res.Words = append(res.Words, ScoredWord{Word: w.word, Score: score})
		res.Score += score
	}
	if len(placements) == RackSize {
		res.Bingo = true
		res.Score += 50
	}
	return res, nil
}

// Apply commits placements to the board in-place. Call only after a successful
// ValidateAndScore.
func (b *Board) Apply(placements []Placement) {
	for _, p := range placements {
		b.Squares[p.Row][p.Col] = &PlacedTile{Letter: p.Letter, Blank: p.Blank}
	}
}

func boardWithPlacements(b *Board, placements []Placement) *Board {
	cp := *b
	for _, p := range placements {
		cp.Squares[p.Row][p.Col] = &PlacedTile{Letter: p.Letter, Blank: p.Blank}
	}
	return &cp
}

func hasAdjacentExistingTile(b *Board, row, col int) bool {
	for _, d := range [4][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
		if b.At(row+d[0], col+d[1]) != nil {
			return true
		}
	}
	return false
}

type wordRun struct {
	word    string
	cells   [][2]int // coordinates in order
	isHoriz bool
}

// collectWords returns the main word (in the direction mainHoriz) plus any
// cross-words formed perpendicular through each placement. Words of length 1
// are ignored. Duplicates (same start + direction + length) are de-duplicated.
func collectWords(preview *Board, placements []Placement, mainHoriz bool) []wordRun {
	var words []wordRun
	seen := map[string]bool{}

	add := func(run wordRun) {
		if len(run.cells) < 2 {
			return
		}
		key := runKey(run)
		if seen[key] {
			return
		}
		seen[key] = true
		words = append(words, run)
	}

	main := extractRun(preview, placements[0].Row, placements[0].Col, mainHoriz)
	main.isHoriz = mainHoriz
	add(main)

	for _, p := range placements {
		cross := extractRun(preview, p.Row, p.Col, !mainHoriz)
		cross.isHoriz = !mainHoriz
		add(cross)
	}
	return words
}

func runKey(r wordRun) string {
	if len(r.cells) == 0 {
		return ""
	}
	dir := "v"
	if r.isHoriz {
		dir = "h"
	}
	return fmt.Sprintf("%s:%d:%d:%d", dir, r.cells[0][0], r.cells[0][1], len(r.cells))
}

// extractRun walks outward from (row, col) in the given direction to find the
// contiguous run of tiles containing that square.
func extractRun(preview *Board, row, col int, horiz bool) wordRun {
	dr, dc := 0, 1
	if !horiz {
		dr, dc = 1, 0
	}
	r, c := row, col
	for InBounds(r-dr, c-dc) && preview.At(r-dr, c-dc) != nil {
		r -= dr
		c -= dc
	}
	var run wordRun
	var sb strings.Builder
	for InBounds(r, c) && preview.At(r, c) != nil {
		sb.WriteByte(byte(preview.At(r, c).Letter))
		run.cells = append(run.cells, [2]int{r, c})
		r += dr
		c += dc
	}
	run.word = sb.String()
	return run
}

// checkContiguity ensures no gaps between the min and max placed coordinate
// along the main direction.
func checkContiguity(preview *Board, placements []Placement, horiz bool) error {
	if len(placements) < 2 {
		return nil
	}
	var line, minI, maxI int
	if horiz {
		line = placements[0].Row
		minI, maxI = placements[0].Col, placements[0].Col
		for _, p := range placements[1:] {
			if p.Col < minI {
				minI = p.Col
			}
			if p.Col > maxI {
				maxI = p.Col
			}
		}
	} else {
		line = placements[0].Col
		minI, maxI = placements[0].Row, placements[0].Row
		for _, p := range placements[1:] {
			if p.Row < minI {
				minI = p.Row
			}
			if p.Row > maxI {
				maxI = p.Row
			}
		}
	}
	for i := minI; i <= maxI; i++ {
		var sq *PlacedTile
		if horiz {
			sq = preview.At(line, i)
		} else {
			sq = preview.At(i, line)
		}
		if sq == nil {
			return errors.New("placements leave a gap in the main word")
		}
	}
	return nil
}

func consumeRack(rack []Letter, placements []Placement) ([]Letter, error) {
	rackCopy := append([]Letter(nil), rack...)
	used := make([]Letter, 0, len(placements))
	for _, p := range placements {
		need := p.Letter
		if p.Blank {
			need = Blank
		}
		idx := -1
		for i, l := range rackCopy {
			if l == need {
				idx = i
				break
			}
		}
		if idx == -1 {
			return nil, fmt.Errorf("rack does not contain tile %q", string(need))
		}
		rackCopy = append(rackCopy[:idx], rackCopy[idx+1:]...)
		used = append(used, need)
	}
	return used, nil
}

// scoreWord computes the score of a single word; premium squares only apply to
// newly placed tiles (tracked via newPlacements).
func scoreWord(preview *Board, w wordRun, newPlacements map[[2]int]bool) int {
	sum := 0
	wordMult := 1
	for _, cell := range w.cells {
		tile := preview.At(cell[0], cell[1])
		letterVal := tile.Value()
		if newPlacements[[2]int{cell[0], cell[1]}] {
			switch PremiumAt(cell[0], cell[1]) {
			case PremDL:
				letterVal *= 2
			case PremTL:
				letterVal *= 3
			case PremDW:
				wordMult *= 2
			case PremTW:
				wordMult *= 3
			}
		}
		sum += letterVal
	}
	return sum * wordMult
}
