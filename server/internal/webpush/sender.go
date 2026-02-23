package webpush

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	wp "github.com/SherClockHolmes/webpush-go"
	"go.uber.org/zap"
)

// pushPayload is the JSON sent to the browser's service worker.
type pushPayload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Tag   string `json:"tag,omitempty"`
	URL   string `json:"url,omitempty"`
}

// Sender delivers Web Push notifications to subscribed browsers.
type Sender struct {
	store      *Store
	privateKey string
	publicKey  string
	logger     *zap.Logger
}

// NewSender creates a new Web Push sender.
func NewSender(privateKey, publicKey string, store *Store, logger *zap.Logger) *Sender {
	return &Sender{
		store:      store,
		privateKey: privateKey,
		publicKey:  publicKey,
		logger:     logger.Named("webpush"),
	}
}

// SendToUser sends a push notification to all of a user's subscribed browsers.
func (s *Sender) SendToUser(ctx context.Context, userID, title, body string) error {
	subs, err := s.store.ListByUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("list subscriptions: %w", err)
	}
	if len(subs) == 0 {
		return nil
	}

	payload, _ := json.Marshal(pushPayload{
		Title: title,
		Body:  body,
		Tag:   "blue-reminder",
		URL:   "/",
	})

	var lastErr error
	for _, sub := range subs {
		wpSub := &wp.Subscription{
			Endpoint: sub.Endpoint,
			Keys: wp.Keys{
				P256dh: sub.KeyP256dh,
				Auth:   sub.KeyAuth,
			},
		}

		resp, err := wp.SendNotification(payload, wpSub, &wp.Options{
			VAPIDPublicKey:  s.publicKey,
			VAPIDPrivateKey: s.privateKey,
			Subscriber:      "mailto:noreply@zimaos.local",
		})
		if err != nil {
			s.logger.Warn("push send failed", zap.String("endpoint", sub.Endpoint), zap.Error(err))
			lastErr = err
			continue
		}
		resp.Body.Close()

		// 410 Gone = subscription expired, clean up
		if resp.StatusCode == http.StatusGone {
			s.store.DeleteByEndpoint(ctx, sub.Endpoint)
			s.logger.Info("removed stale push subscription", zap.String("endpoint", sub.Endpoint))
		}
	}
	return lastErr
}
