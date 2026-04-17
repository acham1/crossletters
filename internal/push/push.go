// Package push implements Web Push notifications for turn alerts.
package push

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// Subscription mirrors the browser PushSubscription object.
type Subscription struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

// Notifier sends Web Push notifications. It stores subscriptions in memory
// (keyed by user ID). A nil Notifier is safe to call — all methods no-op.
type Notifier struct {
	mu           sync.RWMutex
	subs         map[string][]Subscription // userId -> subscriptions
	vapidPublic  string
	vapidPrivate string
	vapidContact string // mailto: URI
}

// NewNotifier creates a Notifier with the given VAPID keys.
func NewNotifier(vapidPublic, vapidPrivate, vapidContact string) *Notifier {
	return &Notifier{
		subs:         map[string][]Subscription{},
		vapidPublic:  vapidPublic,
		vapidPrivate: vapidPrivate,
		vapidContact: vapidContact,
	}
}

// VAPIDPublicKey returns the public key for the frontend.
func (n *Notifier) VAPIDPublicKey() string {
	if n == nil {
		return ""
	}
	return n.vapidPublic
}

// Subscribe adds a push subscription for a user.
func (n *Notifier) Subscribe(userID string, sub Subscription) {
	if n == nil {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	// Deduplicate by endpoint.
	for _, s := range n.subs[userID] {
		if s.Endpoint == sub.Endpoint {
			return
		}
	}
	n.subs[userID] = append(n.subs[userID], sub)
}

// Notification payload sent to the browser.
type Notification struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url,omitempty"`
}

// Notify sends a push to all subscriptions for the given user ID. Failures
// are logged but not returned — push is best-effort.
func (n *Notifier) Notify(ctx context.Context, userID string, notif Notification) {
	if n == nil {
		return
	}
	n.mu.RLock()
	subs := append([]Subscription(nil), n.subs[userID]...)
	n.mu.RUnlock()

	if len(subs) == 0 {
		return
	}

	payload, _ := json.Marshal(notif)

	var stale []int
	for i, sub := range subs {
		resp, err := webpush.SendNotificationWithContext(ctx, payload, &webpush.Subscription{
			Endpoint: sub.Endpoint,
			Keys: webpush.Keys{
				P256dh: sub.Keys.P256dh,
				Auth:   sub.Keys.Auth,
			},
		}, &webpush.Options{
			VAPIDPublicKey:  n.vapidPublic,
			VAPIDPrivateKey: n.vapidPrivate,
			Subscriber:      n.vapidContact,
		})
		if err != nil {
			log.Printf("push to %s failed: %v", userID, err)
			stale = append(stale, i)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusGone {
			stale = append(stale, i)
		}
	}

	// Remove stale subscriptions.
	if len(stale) > 0 {
		n.mu.Lock()
		current := n.subs[userID]
		filtered := make([]Subscription, 0, len(current))
		staleSet := map[int]bool{}
		for _, i := range stale {
			staleSet[i] = true
		}
		for i, s := range current {
			if !staleSet[i] {
				filtered = append(filtered, s)
			}
		}
		n.subs[userID] = filtered
		n.mu.Unlock()
	}
}
