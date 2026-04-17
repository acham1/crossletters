package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/alan/not-scrabble/internal/game"
	"github.com/alan/not-scrabble/internal/push"
)

// GCS is a Store backed by Google Cloud Storage. Games, users, and invite
// pointers are stored as JSON objects. UpdateGame uses if-generation-match for
// optimistic concurrency.
type GCS struct {
	bucket *storage.BucketHandle
}

// NewGCS returns a new GCS-backed store.
func NewGCS(client *storage.Client, bucketName string) *GCS {
	return &GCS{bucket: client.Bucket(bucketName)}
}

// Object layout:
//   games/{id}.json
//   users/{id}.json
//   invites/{code}.json   → {"gameId": "..."}

func gameKey(id string) string    { return "games/" + id + ".json" }
func userKey(id string) string    { return "users/" + id + ".json" }
func inviteKey(code string) string { return "invites/" + code + ".json" }

// --- games ---

func (s *GCS) CreateGame(ctx context.Context, g *game.Game) error {
	b, err := json.Marshal(g)
	if err != nil {
		return err
	}
	// Write game object; DoesNotExist prevents overwriting.
	obj := s.bucket.Object(gameKey(g.ID))
	w := obj.If(storage.Conditions{DoesNotExist: true}).NewWriter(ctx)
	w.ContentType = "application/json"
	if _, err := w.Write(b); err != nil {
		w.Close()
		return fmt.Errorf("write game: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close game write: %w", gcsErr(err))
	}
	// Write invite pointer.
	inv, _ := json.Marshal(map[string]string{"gameId": g.ID})
	iw := s.bucket.Object(inviteKey(g.InviteCode)).NewWriter(ctx)
	iw.ContentType = "application/json"
	iw.Write(inv)
	if err := iw.Close(); err != nil {
		return fmt.Errorf("write invite: %w", err)
	}
	return nil
}

func (s *GCS) GetGame(ctx context.Context, id string) (*game.Game, error) {
	g, _, err := s.getGameWithGen(ctx, id)
	return g, err
}

// getGameWithGen returns the game and the GCS object generation for
// if-generation-match on subsequent writes.
func (s *GCS) getGameWithGen(ctx context.Context, id string) (*game.Game, int64, error) {
	obj := s.bucket.Object(gameKey(id))
	r, err := obj.NewReader(ctx)
	if err != nil {
		return nil, 0, gcsErr(err)
	}
	defer r.Close()
	gen := r.Attrs.Generation
	var g game.Game
	if err := json.NewDecoder(r).Decode(&g); err != nil {
		return nil, 0, fmt.Errorf("decode game: %w", err)
	}
	return &g, gen, nil
}

func (s *GCS) UpdateGame(ctx context.Context, in *game.Game, mutate func(*game.Game) error) (*game.Game, error) {
	// The `in` parameter carries the game ID but we re-read from GCS to get
	// the freshest state and its generation number.
	id := in.ID
	const maxRetries = 3
	for attempt := range maxRetries {
		g, gen, err := s.getGameWithGen(ctx, id)
		if err != nil {
			return nil, err
		}
		if err := mutate(g); err != nil {
			return nil, err
		}
		b, err := json.Marshal(g)
		if err != nil {
			return nil, err
		}
		obj := s.bucket.Object(gameKey(id))
		w := obj.If(storage.Conditions{GenerationMatch: gen}).NewWriter(ctx)
		w.ContentType = "application/json"
		if _, err := w.Write(b); err != nil {
			w.Close()
			return nil, fmt.Errorf("write game: %w", err)
		}
		if err := w.Close(); err != nil {
			if errors.Is(gcsErr(err), ErrConflict) && attempt < maxRetries-1 {
				continue // retry
			}
			return nil, gcsErr(err)
		}
		return g, nil
	}
	return nil, ErrConflict
}

func (s *GCS) FindGameByInvite(ctx context.Context, invite string) (*game.Game, error) {
	r, err := s.bucket.Object(inviteKey(invite)).NewReader(ctx)
	if err != nil {
		return nil, gcsErr(err)
	}
	defer r.Close()
	var ptr struct {
		GameID string `json:"gameId"`
	}
	if err := json.NewDecoder(r).Decode(&ptr); err != nil {
		return nil, fmt.Errorf("decode invite: %w", err)
	}
	return s.GetGame(ctx, ptr.GameID)
}

// --- users ---

func (s *GCS) GetOrCreateUser(ctx context.Context, id, name, email string) (*User, error) {
	u, gen, err := s.getUserWithGen(ctx, id)
	if err == nil {
		changed := false
		if name != "" && u.Name == "" {
			u.Name = name
			changed = true
		}
		if email != "" && u.Email == "" {
			u.Email = email
			changed = true
		}
		if changed {
			s.putUser(ctx, u, gen)
		}
		return u, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	u = &User{ID: id, Name: name, Email: email, GameIDs: []string{}}
	if err := s.putUser(ctx, u, -1); err != nil {
		// Race: another request created it first. Re-read.
		u2, _, err2 := s.getUserWithGen(ctx, id)
		if err2 != nil {
			return nil, err
		}
		return u2, nil
	}
	return u, nil
}

func (s *GCS) AddGameToUser(ctx context.Context, userID, gameID string) error {
	const maxRetries = 3
	for attempt := range maxRetries {
		u, gen, err := s.getUserWithGen(ctx, userID)
		if err != nil {
			return err
		}
		if slices.Contains(u.GameIDs, gameID) {
			return nil
		}
		u.GameIDs = append(u.GameIDs, gameID)
		if err := s.putUser(ctx, u, gen); err != nil {
			if errors.Is(err, ErrConflict) && attempt < maxRetries-1 {
				continue
			}
			return err
		}
		return nil
	}
	return ErrConflict
}

func (s *GCS) GetUser(ctx context.Context, id string) (*User, error) {
	u, _, err := s.getUserWithGen(ctx, id)
	return u, err
}

func (s *GCS) getUserWithGen(ctx context.Context, id string) (*User, int64, error) {
	r, err := s.bucket.Object(userKey(id)).NewReader(ctx)
	if err != nil {
		return nil, 0, gcsErr(err)
	}
	defer r.Close()
	gen := r.Attrs.Generation
	var u User
	if err := json.NewDecoder(r).Decode(&u); err != nil {
		return nil, 0, fmt.Errorf("decode user: %w", err)
	}
	return &u, gen, nil
}

// putUser writes a user object. gen=-1 means create (DoesNotExist), otherwise
// uses if-generation-match.
func (s *GCS) putUser(ctx context.Context, u *User, gen int64) error {
	b, err := json.Marshal(u)
	if err != nil {
		return err
	}
	obj := s.bucket.Object(userKey(u.ID))
	if gen >= 0 {
		obj = obj.If(storage.Conditions{GenerationMatch: gen})
	} else {
		obj = obj.If(storage.Conditions{DoesNotExist: true})
	}
	w := obj.NewWriter(ctx)
	w.ContentType = "application/json"
	if _, err := w.Write(b); err != nil {
		w.Close()
		return fmt.Errorf("write user: %w", err)
	}
	if err := w.Close(); err != nil {
		return gcsErr(err)
	}
	return nil
}

// --- push subscriptions ---

func (s *GCS) SavePushSubscription(ctx context.Context, userID string, sub push.Subscription) error {
	const maxRetries = 3
	for attempt := range maxRetries {
		u, gen, err := s.getUserWithGen(ctx, userID)
		if err != nil {
			return err
		}
		// Deduplicate by endpoint.
		for _, existing := range u.PushSubscriptions {
			if existing.Endpoint == sub.Endpoint {
				return nil
			}
		}
		u.PushSubscriptions = append(u.PushSubscriptions, sub)
		if err := s.putUser(ctx, u, gen); err != nil {
			if errors.Is(err, ErrConflict) && attempt < maxRetries-1 {
				continue
			}
			return err
		}
		return nil
	}
	return ErrConflict
}

func (s *GCS) GetPushSubscriptions(ctx context.Context, userID string) ([]push.Subscription, error) {
	u, _, err := s.getUserWithGen(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return u.PushSubscriptions, nil
}

func (s *GCS) RemovePushSubscription(ctx context.Context, userID string, endpoint string) error {
	const maxRetries = 3
	for attempt := range maxRetries {
		u, gen, err := s.getUserWithGen(ctx, userID)
		if err != nil {
			return err
		}
		filtered := make([]push.Subscription, 0, len(u.PushSubscriptions))
		for _, s := range u.PushSubscriptions {
			if s.Endpoint != endpoint {
				filtered = append(filtered, s)
			}
		}
		u.PushSubscriptions = filtered
		if err := s.putUser(ctx, u, gen); err != nil {
			if errors.Is(err, ErrConflict) && attempt < maxRetries-1 {
				continue
			}
			return err
		}
		return nil
	}
	return ErrConflict
}

// --- helpers ---

// gcsErr maps GCS errors to store-level errors.
func gcsErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, storage.ErrObjectNotExist) {
		return ErrNotFound
	}
	// 412 Precondition Failed → concurrent modification.
	s := err.Error()
	if strings.Contains(s, "conditionNotMet") || strings.Contains(s, "412") {
		return ErrConflict
	}
	return err
}

