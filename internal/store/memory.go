package store

import (
	"context"
	"encoding/json"
	"slices"
	"sync"

	"github.com/alan/not-scrabble/internal/game"
)

// Memory is an in-process Store for local dev and tests. Safe for concurrent
// use. UpdateGame takes the per-game mutex to serialize mutations.
type Memory struct {
	mu      sync.Mutex
	games   map[string]*gameEntry
	users   map[string]*User
	invites map[string]string // inviteCode -> gameID
}

type gameEntry struct {
	mu      sync.Mutex
	payload []byte // JSON-serialized so callers never mutate stored state
}

func NewMemory() *Memory {
	return &Memory{
		games:   map[string]*gameEntry{},
		users:   map[string]*User{},
		invites: map[string]string{},
	}
}

func (m *Memory) CreateGame(_ context.Context, g *game.Game) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, err := json.Marshal(g)
	if err != nil {
		return err
	}
	m.games[g.ID] = &gameEntry{payload: b}
	m.invites[g.InviteCode] = g.ID
	return nil
}

func (m *Memory) GetGame(_ context.Context, id string) (*game.Game, error) {
	m.mu.Lock()
	e, ok := m.games[id]
	m.mu.Unlock()
	if !ok {
		return nil, ErrNotFound
	}
	e.mu.Lock()
	payload := append([]byte(nil), e.payload...)
	e.mu.Unlock()
	var g game.Game
	if err := json.Unmarshal(payload, &g); err != nil {
		return nil, err
	}
	return &g, nil
}

// UpdateGame runs mutate under the per-game lock, then persists the result.
// Returns the post-mutation game (re-unmarshaled so callers can't affect state).
func (m *Memory) UpdateGame(_ context.Context, in *game.Game, mutate func(*game.Game) error) (*game.Game, error) {
	m.mu.Lock()
	e, ok := m.games[in.ID]
	m.mu.Unlock()
	if !ok {
		return nil, ErrNotFound
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	var g game.Game
	if err := json.Unmarshal(e.payload, &g); err != nil {
		return nil, err
	}
	if err := mutate(&g); err != nil {
		return nil, err
	}
	b, err := json.Marshal(&g)
	if err != nil {
		return nil, err
	}
	e.payload = b
	// Re-unmarshal to return an independent copy.
	var out game.Game
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (m *Memory) FindGameByInvite(ctx context.Context, invite string) (*game.Game, error) {
	m.mu.Lock()
	id, ok := m.invites[invite]
	m.mu.Unlock()
	if !ok {
		return nil, ErrNotFound
	}
	return m.GetGame(ctx, id)
}

func (m *Memory) GetOrCreateUser(_ context.Context, id, name, email string) (*User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u, ok := m.users[id]; ok {
		if name != "" && u.Name == "" {
			u.Name = name
		}
		if email != "" && u.Email == "" {
			u.Email = email
		}
		return cloneUser(u), nil
	}
	u := &User{ID: id, Name: name, Email: email, GameIDs: []string{}}
	m.users[id] = u
	return cloneUser(u), nil
}

func (m *Memory) AddGameToUser(_ context.Context, userID, gameID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[userID]
	if !ok {
		return ErrNotFound
	}
	if slices.Contains(u.GameIDs, gameID) {
		return nil
	}
	u.GameIDs = append(u.GameIDs, gameID)
	return nil
}

func (m *Memory) GetUser(_ context.Context, id string) (*User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneUser(u), nil
}

func cloneUser(u *User) *User {
	cp := *u
	cp.GameIDs = append([]string(nil), u.GameIDs...)
	return &cp
}
