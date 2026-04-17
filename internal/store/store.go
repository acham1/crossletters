// Package store persists games and users. The interface is designed so an
// in-memory implementation (for local dev) and a future GCS-backed
// implementation can coexist.
package store

import (
	"context"
	"errors"

	"github.com/alan/not-scrabble/internal/game"
	"github.com/alan/not-scrabble/internal/push"
)

// ErrNotFound is returned when a lookup misses.
var ErrNotFound = errors.New("not found")

// ErrConflict is returned when a concurrent write is detected.
var ErrConflict = errors.New("concurrent modification")

// User is the app-level user record.
type User struct {
	ID                string              `json:"id"`    // stable identifier (e.g. Google sub, or dev username)
	Name              string              `json:"name"`
	Email             string              `json:"email,omitempty"`
	GameIDs           []string            `json:"gameIds"`
	PushSubscriptions []push.Subscription `json:"pushSubscriptions,omitempty"`
}

// Store is the persistence contract.
type Store interface {
	// Games
	CreateGame(ctx context.Context, g *game.Game) error
	GetGame(ctx context.Context, id string) (*game.Game, error)
	UpdateGame(ctx context.Context, g *game.Game, mutate func(*game.Game) error) (*game.Game, error)
	FindGameByInvite(ctx context.Context, invite string) (*game.Game, error)

	// Users
	GetOrCreateUser(ctx context.Context, id, name, email string) (*User, error)
	AddGameToUser(ctx context.Context, userID, gameID string) error
	GetUser(ctx context.Context, id string) (*User, error)

	// Push subscriptions
	SavePushSubscription(ctx context.Context, userID string, sub push.Subscription) error
	GetPushSubscriptions(ctx context.Context, userID string) ([]push.Subscription, error)
	RemovePushSubscription(ctx context.Context, userID string, endpoint string) error
}
