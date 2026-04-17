// Package push implements Web Push notifications for turn alerts.
package push

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

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

// SubscriptionStore is the subset of store.Store that the Notifier needs.
type SubscriptionStore interface {
	GetPushSubscriptions(ctx context.Context, userID string) ([]Subscription, error)
	RemovePushSubscription(ctx context.Context, userID string, endpoint string) error
}

// Notifier sends Web Push notifications. A nil Notifier is safe to call —
// all methods no-op.
type Notifier struct {
	store        SubscriptionStore
	vapidPublic  string
	vapidPrivate string
	vapidContact string // mailto: URI
}

// NewNotifier creates a Notifier with the given VAPID keys and subscription store.
func NewNotifier(vapidPublic, vapidPrivate, vapidContact string, store SubscriptionStore) *Notifier {
	return &Notifier{
		store:        store,
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

	subs, err := n.store.GetPushSubscriptions(ctx, userID)
	if err != nil {
		log.Printf("push: failed to load subscriptions for %s: %v", userID, err)
		return
	}
	if len(subs) == 0 {
		return
	}

	payload, _ := json.Marshal(notif)

	for _, sub := range subs {
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
			go n.store.RemovePushSubscription(context.Background(), userID, sub.Endpoint)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusGone {
			go n.store.RemovePushSubscription(context.Background(), userID, sub.Endpoint)
		}
	}
}
