package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

// Identity identifies the authenticated user for a request.
type Identity struct {
	UserID string
	Name   string
	Email  string
}

// Authenticator extracts an Identity from an HTTP request.
type Authenticator interface {
	Authenticate(r *http.Request) (Identity, error)
}

// ErrUnauthenticated is returned when there is no valid identity.
var ErrUnauthenticated = errors.New("unauthenticated")

type ctxKey int

const identityKey ctxKey = 1

// identityFrom returns the Identity stored on the request context, or an empty
// value.
func identityFrom(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityKey).(Identity)
	return id, ok
}

// withIdentity adds an Identity to the request context.
func withIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityKey, id)
}

// DevAuth is a dev-only authenticator that reads the user identity from a
// cookie named "dev_user". Use behind an inaccessible endpoint for local
// development only.
type DevAuth struct{}

func (DevAuth) Authenticate(r *http.Request) (Identity, error) {
	c, err := r.Cookie("dev_user")
	if err != nil || c.Value == "" {
		return Identity{}, ErrUnauthenticated
	}
	user := strings.TrimSpace(c.Value)
	if user == "" {
		return Identity{}, ErrUnauthenticated
	}
	// Display name defaults to the cookie value.
	name := user
	if n, err := r.Cookie("dev_name"); err == nil && n.Value != "" {
		name = n.Value
	}
	return Identity{UserID: user, Name: name}, nil
}
