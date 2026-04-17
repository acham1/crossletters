package httpapi

import (
	"bufio"
	"context"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"
)

// Allowlist optionally restricts sign-in to a set of email addresses. When
// enabled it wraps the auth middleware: after identity is extracted, the email
// is checked against the list. A nil Allowlist means open access.
type Allowlist struct {
	mu     sync.RWMutex
	emails map[string]bool // lower-cased

	// GCS source for periodic refresh.
	bucket *storage.BucketHandle
	object string
}

// NewAllowlistInline creates an Allowlist from a static comma-separated string
// (e.g. "alice@x.com,bob@y.com").
func NewAllowlistInline(csv string) *Allowlist {
	a := &Allowlist{emails: map[string]bool{}}
	for _, e := range strings.Split(csv, ",") {
		e = strings.TrimSpace(strings.ToLower(e))
		if e != "" {
			a.emails[e] = true
		}
	}
	return a
}

// NewAllowlistGCS creates an Allowlist backed by a GCS object. It loads the
// object immediately and then refreshes every interval.
func NewAllowlistGCS(client *storage.Client, uri string, interval time.Duration) (*Allowlist, error) {
	// Parse gs://bucket/path
	uri = strings.TrimPrefix(uri, "gs://")
	parts := strings.SplitN(uri, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, &InvalidAllowlistURIError{URI: "gs://" + uri}
	}
	a := &Allowlist{
		emails: map[string]bool{},
		bucket: client.Bucket(parts[0]),
		object: parts[1],
	}
	if err := a.refresh(context.Background()); err != nil {
		return nil, err
	}
	go a.poll(interval)
	return a, nil
}

type InvalidAllowlistURIError struct{ URI string }

func (e *InvalidAllowlistURIError) Error() string {
	return "invalid allowlist GCS URI (expected gs://bucket/object): " + e.URI
}

// Contains checks if an email is allowed. Empty email is never allowed.
func (a *Allowlist) Contains(email string) bool {
	if a == nil {
		return true // nil allowlist = open access
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.emails[strings.ToLower(strings.TrimSpace(email))]
}

func (a *Allowlist) refresh(ctx context.Context) error {
	r, err := a.bucket.Object(a.object).NewReader(ctx)
	if err != nil {
		return err
	}
	defer r.Close()
	emails, err := parseEmailList(r)
	if err != nil {
		return err
	}
	a.mu.Lock()
	a.emails = emails
	a.mu.Unlock()
	log.Printf("allowlist: loaded %d emails from gs://%s/%s", len(emails), a.bucket.BucketName(), a.object)
	return nil
}

func (a *Allowlist) poll(interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for range t.C {
		if err := a.refresh(context.Background()); err != nil {
			log.Printf("allowlist refresh error: %v", err)
		}
	}
}

func parseEmailList(r io.Reader) (map[string]bool, error) {
	m := map[string]bool{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		e := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if e != "" && !strings.HasPrefix(e, "#") {
			m[e] = true
		}
	}
	return m, scanner.Err()
}

// AllowlistMiddleware returns an http.Handler that checks the email from the
// authenticated identity against the allowlist. If the email is not on the
// list, it returns 403 before the inner handler runs.
func AllowlistMiddleware(allowlist *Allowlist, next http.Handler) http.Handler {
	if allowlist == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := identityFrom(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "unauthenticated")
			return
		}
		if !allowlist.Contains(id.Email) {
			writeErr(w, http.StatusForbidden, "your account is not on the invite list — please talk to the person who invited you to learn about getting access or self-hosting")
			return
		}
		next.ServeHTTP(w, r)
	})
}
