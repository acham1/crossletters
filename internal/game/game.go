package game

import (
	"errors"
	"fmt"
	"time"
)

type Status string

const (
	StatusWaiting   Status = "waiting"
	StatusActive    Status = "active"
	StatusCompleted Status = "completed"
)

// Player is one seat in a game.
type Player struct {
	UserID string   `json:"userId"`
	Name   string   `json:"name"`
	Score  int      `json:"score"`
	Rack   []Letter `json:"rack"`
}

type TurnType string

const (
	TurnPlay     TurnType = "play"
	TurnExchange TurnType = "exchange"
	TurnPass     TurnType = "pass"
)

// TurnRecord records one turn of play for replay/history.
type TurnRecord struct {
	PlayerIdx  int          `json:"playerIdx"`
	Type       TurnType     `json:"type"`
	Placements []Placement  `json:"placements,omitempty"`
	Words      []ScoredWord `json:"words,omitempty"`
	Score      int          `json:"score"`
	Bingo      bool         `json:"bingo,omitempty"`
	Exchanged  int          `json:"exchanged,omitempty"`
	At         time.Time    `json:"at"`
}

// Game is the full state of a single game.
type Game struct {
	ID         string       `json:"id"`
	CreatorID  string       `json:"creatorId"`
	InviteCode string       `json:"inviteCode"`
	Status     Status       `json:"status"`
	CreatedAt  time.Time    `json:"createdAt"`
	StartedAt  *time.Time   `json:"startedAt,omitempty"`
	EndedAt    *time.Time   `json:"endedAt,omitempty"`
	Players    []*Player    `json:"players"`
	Turn       int          `json:"turn"` // index of the current player, also equals len(History) when no one has passed and the game is active
	Board      *Board       `json:"board"`
	Bag        []Letter     `json:"bag"`
	BagSeed    int64        `json:"bagSeed"`
	History    []TurnRecord `json:"history"`
	PassStreak int          `json:"passStreak"` // consecutive scoreless turns
	Winners    []int        `json:"winners,omitempty"`
}

// NewGame creates a waiting game with the creator as the first player.
func NewGame(id, creatorID, creatorName, inviteCode string, seed int64, now time.Time) *Game {
	return &Game{
		ID:         id,
		CreatorID:  creatorID,
		InviteCode: inviteCode,
		Status:     StatusWaiting,
		CreatedAt:  now,
		Players: []*Player{{
			UserID: creatorID,
			Name:   creatorName,
			Rack:   []Letter{},
		}},
		Board:   NewBoard(),
		Bag:     NewBag(seed),
		BagSeed: seed,
		History: []TurnRecord{},
	}
}

// AddPlayer joins a waiting game. Returns an error if not in waiting state,
// the game is full, or the user is already a player.
func (g *Game) AddPlayer(userID, name string) error {
	if g.Status != StatusWaiting {
		return errors.New("game is not accepting new players")
	}
	if len(g.Players) >= 4 {
		return errors.New("game is full (max 4 players)")
	}
	for _, p := range g.Players {
		if p.UserID == userID {
			return errors.New("player already in game")
		}
	}
	g.Players = append(g.Players, &Player{UserID: userID, Name: name, Rack: []Letter{}})
	return nil
}

// Start deals initial racks and marks the game active.
func (g *Game) Start(now time.Time) error {
	if g.Status != StatusWaiting {
		return errors.New("game already started or ended")
	}
	if len(g.Players) < 2 {
		return errors.New("need at least 2 players to start")
	}
	for _, p := range g.Players {
		drawn, rem := DrawN(g.Bag, RackSize)
		g.Bag = rem
		p.Rack = drawn
	}
	g.Status = StatusActive
	g.StartedAt = &now
	return nil
}

// CurrentPlayer returns the player whose turn it is, or nil if game not active.
func (g *Game) CurrentPlayer() *Player {
	if g.Status != StatusActive {
		return nil
	}
	return g.Players[g.Turn%len(g.Players)]
}

// requireActive returns an error unless it's userID's turn.
func (g *Game) requireTurn(userID string) (int, error) {
	if g.Status != StatusActive {
		return 0, errors.New("game is not active")
	}
	idx := g.Turn % len(g.Players)
	if g.Players[idx].UserID != userID {
		return 0, errors.New("not your turn")
	}
	return idx, nil
}

// Play validates and applies a move. Returns the scoring breakdown.
func (g *Game) Play(userID string, placements []Placement, dict WordSet, now time.Time) (*PlayResult, error) {
	idx, err := g.requireTurn(userID)
	if err != nil {
		return nil, err
	}
	player := g.Players[idx]

	res, err := ValidateAndScore(g.Board, player.Rack, placements, dict)
	if err != nil {
		return nil, err
	}

	// Apply to board.
	g.Board.Apply(placements)

	// Remove used tiles from rack.
	player.Rack = removeTiles(player.Rack, res.UsedRack)

	// Refill rack from bag.
	needed := RackSize - len(player.Rack)
	if needed > 0 {
		drawn, rem := DrawN(g.Bag, needed)
		g.Bag = rem
		player.Rack = append(player.Rack, drawn...)
	}

	player.Score += res.Score
	g.History = append(g.History, TurnRecord{
		PlayerIdx:  idx,
		Type:       TurnPlay,
		Placements: placements,
		Words:      res.Words,
		Score:      res.Score,
		Bingo:      res.Bingo,
		At:         now,
	})
	g.PassStreak = 0
	g.Turn++

	g.maybeEnd(now, idx)
	return res, nil
}

// Exchange swaps the given rack tiles back into the bag (requires bag >= 7).
func (g *Game) Exchange(userID string, tiles []Letter, now time.Time) error {
	idx, err := g.requireTurn(userID)
	if err != nil {
		return err
	}
	if len(tiles) == 0 {
		return errors.New("must exchange at least one tile")
	}
	if len(g.Bag) < RackSize {
		return errors.New("cannot exchange when bag has fewer than 7 tiles")
	}
	player := g.Players[idx]
	newRack, err := removeExact(player.Rack, tiles)
	if err != nil {
		return err
	}
	drawn, rem := DrawN(g.Bag, len(tiles))
	// Return the exchanged tiles and reshuffle deterministically using the
	// current turn number so replays stay reproducible.
	rem = ReturnAndReshuffle(rem, tiles, g.BagSeed+int64(g.Turn+1))
	g.Bag = rem
	player.Rack = append(newRack, drawn...)

	g.History = append(g.History, TurnRecord{
		PlayerIdx: idx,
		Type:      TurnExchange,
		Exchanged: len(tiles),
		At:        now,
	})
	g.PassStreak++
	g.Turn++
	g.maybeEnd(now, idx)
	return nil
}

// Pass advances the turn without playing.
func (g *Game) Pass(userID string, now time.Time) error {
	idx, err := g.requireTurn(userID)
	if err != nil {
		return err
	}
	g.History = append(g.History, TurnRecord{
		PlayerIdx: idx, Type: TurnPass, At: now,
	})
	g.PassStreak++
	g.Turn++
	g.maybeEnd(now, idx)
	return nil
}

// maybeEnd ends the game if the bag is empty and the just-played player has an
// empty rack, or if PassStreak reaches 2*len(players) (everyone passed twice).
// Applies end-of-game scoring adjustments.
func (g *Game) maybeEnd(now time.Time, lastIdx int) {
	emptyRack := len(g.Players[lastIdx].Rack) == 0
	bagEmpty := len(g.Bag) == 0

	endByGoingOut := emptyRack && bagEmpty
	endByPasses := g.PassStreak >= 2*len(g.Players)

	if !endByGoingOut && !endByPasses {
		return
	}

	// Subtract remaining rack values from each player; if a player went out,
	// they also gain the sum of opponents' remaining values.
	outIdx := -1
	if endByGoingOut {
		outIdx = lastIdx
	}
	sumOthers := 0
	for i, p := range g.Players {
		rackVal := 0
		for _, l := range p.Rack {
			rackVal += LetterValues[l]
		}
		p.Score -= rackVal
		if i != outIdx {
			sumOthers += rackVal
		}
	}
	if outIdx >= 0 {
		g.Players[outIdx].Score += sumOthers
	}

	g.Status = StatusCompleted
	g.EndedAt = &now

	// Determine winner(s) — ties share.
	best := g.Players[0].Score
	for _, p := range g.Players[1:] {
		if p.Score > best {
			best = p.Score
		}
	}
	for i, p := range g.Players {
		if p.Score == best {
			g.Winners = append(g.Winners, i)
		}
	}
}

func removeTiles(rack, used []Letter) []Letter {
	out := append([]Letter(nil), rack...)
	for _, u := range used {
		for i, l := range out {
			if l == u {
				out = append(out[:i], out[i+1:]...)
				break
			}
		}
	}
	return out
}

// removeExact removes each tile in `tiles` from `rack`. Fails if the rack
// doesn't have enough of a given letter.
func removeExact(rack, tiles []Letter) ([]Letter, error) {
	out := append([]Letter(nil), rack...)
	for _, t := range tiles {
		idx := -1
		for i, l := range out {
			if l == t {
				idx = i
				break
			}
		}
		if idx == -1 {
			return nil, fmt.Errorf("rack does not contain %q", string(t))
		}
		out = append(out[:idx], out[idx+1:]...)
	}
	return out, nil
}
