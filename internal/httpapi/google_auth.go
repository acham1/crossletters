package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"google.golang.org/api/idtoken"
)

// GoogleAuth verifies Google ID tokens (JWTs) and manages signed session
// cookies so the user doesn't re-authenticate on every request.
type GoogleAuth struct {
	clientID  string
	cookieKey []byte // HMAC-SHA256 key for signing session cookies
	secure    bool   // set Secure flag on cookies (true in prod)
}

// NewGoogleAuth creates a GoogleAuth. sessionSecret can be empty; if so, a
// random key is generated (sessions won't survive restarts).
func NewGoogleAuth(clientID, sessionSecret string, secureCookie bool) *GoogleAuth {
	var key []byte
	if sessionSecret != "" {
		key = []byte(sessionSecret)
	} else {
		key = make([]byte, 32)
		rand.Read(key)
	}
	return &GoogleAuth{clientID: clientID, cookieKey: key, secure: secureCookie}
}

// ClientID returns the Google client ID for the frontend to load GIS.
func (g *GoogleAuth) ClientID() string { return g.clientID }

type sessionPayload struct {
	UserID string `json:"sub"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Exp    int64  `json:"exp"`
}

const sessionCookieName = "session"
const sessionDuration = 30 * 24 * time.Hour

// Authenticate reads the session cookie and returns the identity.
func (g *GoogleAuth) Authenticate(r *http.Request) (Identity, error) {
	c, err := r.Cookie(sessionCookieName)
	if err != nil || c.Value == "" {
		return Identity{}, ErrUnauthenticated
	}
	payload, err := g.verifySessionCookie(c.Value)
	if err != nil {
		return Identity{}, ErrUnauthenticated
	}
	return Identity{
		UserID: payload.UserID,
		Name:   payload.Name,
		Email:  payload.Email,
	}, nil
}

// HandleCallback is the handler for POST /api/auth/google/callback.
// Body: {"credential": "<Google ID token>"}
func (g *GoogleAuth) HandleCallback(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Credential string `json:"credential"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Credential == "" {
		writeErr(w, http.StatusBadRequest, "credential required")
		return
	}

	payload, err := idtoken.Validate(r.Context(), req.Credential, g.clientID)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "invalid Google token")
		return
	}

	sub := payload.Subject
	email, _ := payload.Claims["email"].(string)
	name, _ := payload.Claims["name"].(string)
	if name == "" {
		name = email
	}

	cookie, err := g.signSessionCookie(sessionPayload{
		UserID: sub,
		Name:   name,
		Email:  email,
		Exp:    time.Now().Add(sessionDuration).Unix(),
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "session error")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    cookie,
		Path:     "/",
		HttpOnly: true,
		Secure:   g.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionDuration.Seconds()),
	})
	writeJSON(w, http.StatusOK, UserSummary{UserID: sub, Name: name, Email: email})
}

// HandleLogout clears the session cookie.
func (g *GoogleAuth) HandleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:   sessionCookieName,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

// signSessionCookie produces "base64(json).base64(hmac)".
func (g *GoogleAuth) signSessionCookie(p sessionPayload) (string, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	data := base64.RawURLEncoding.EncodeToString(b)
	mac := hmac.New(sha256.New, g.cookieKey)
	mac.Write([]byte(data))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return data + "." + sig, nil
}

func (g *GoogleAuth) verifySessionCookie(cookie string) (sessionPayload, error) {
	parts := strings.SplitN(cookie, ".", 2)
	if len(parts) != 2 {
		return sessionPayload{}, errors.New("malformed session cookie")
	}
	data, sigStr := parts[0], parts[1]
	mac := hmac.New(sha256.New, g.cookieKey)
	mac.Write([]byte(data))
	expected := mac.Sum(nil)
	sig, err := base64.RawURLEncoding.DecodeString(sigStr)
	if err != nil || !hmac.Equal(sig, expected) {
		return sessionPayload{}, errors.New("invalid signature")
	}
	b, err := base64.RawURLEncoding.DecodeString(data)
	if err != nil {
		return sessionPayload{}, err
	}
	var p sessionPayload
	if err := json.Unmarshal(b, &p); err != nil {
		return sessionPayload{}, err
	}
	if time.Now().Unix() > p.Exp {
		return sessionPayload{}, errors.New("session expired")
	}
	return p, nil
}

// VerifyEmail checks if a Google ID token's email matches. Used externally
// by the allowlist middleware, not by Authenticate (which uses the session
// cookie).
func ExtractEmailFromContext(ctx context.Context) string {
	id, ok := identityFrom(ctx)
	if !ok {
		return ""
	}
	return id.Email
}

// ChainAuth tries multiple authenticators in order, returning the first success.
type ChainAuth []Authenticator

func (c ChainAuth) Authenticate(r *http.Request) (Identity, error) {
	for _, a := range c {
		id, err := a.Authenticate(r)
		if err == nil {
			return id, nil
		}
	}
	return Identity{}, ErrUnauthenticated
}

