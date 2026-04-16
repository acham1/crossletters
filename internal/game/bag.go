package game

import "math/rand"

// NewBag returns a fresh 100-tile bag shuffled deterministically from seed.
// The returned slice is the draw order; draws pop from the end.
func NewBag(seed int64) []Letter {
	tiles := make([]Letter, 0, 100)
	for l, n := range LetterCounts {
		for range n {
			tiles = append(tiles, l)
		}
	}
	// Sort deterministically before shuffling so iteration order of the map
	// doesn't leak into the randomness.
	sortLetters(tiles)
	r := rand.New(rand.NewSource(seed))
	r.Shuffle(len(tiles), func(i, j int) {
		tiles[i], tiles[j] = tiles[j], tiles[i]
	})
	return tiles
}

func sortLetters(ts []Letter) {
	// Simple insertion sort: tiny input (100), avoids pulling sort package.
	for i := 1; i < len(ts); i++ {
		v := ts[i]
		j := i
		for j > 0 && ts[j-1] > v {
			ts[j] = ts[j-1]
			j--
		}
		ts[j] = v
	}
}

// DrawN removes up to n tiles from the end of the bag and returns them.
func DrawN(bag []Letter, n int) (drawn []Letter, remaining []Letter) {
	if n > len(bag) {
		n = len(bag)
	}
	cut := len(bag) - n
	drawn = append([]Letter(nil), bag[cut:]...)
	remaining = bag[:cut]
	return
}

// ReturnAndReshuffle puts the given tiles back into the bag and reshuffles
// with the given seed (used for the exchange action; callers derive a fresh
// seed from e.g. turn index to keep the game reproducible).
func ReturnAndReshuffle(bag []Letter, returned []Letter, seed int64) []Letter {
	bag = append(bag, returned...)
	sortLetters(bag)
	r := rand.New(rand.NewSource(seed))
	r.Shuffle(len(bag), func(i, j int) { bag[i], bag[j] = bag[j], bag[i] })
	return bag
}
