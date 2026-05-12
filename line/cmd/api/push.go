package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	webpush "github.com/SherClockHolmes/webpush-go"

	"github.com/solanyn/mono/line/internal/db"
)

func (s *server) loadPushSubscriptions(ctx context.Context) {
	if s.database == nil {
		return
	}
	subs, err := s.database.ListPushSubscriptions(ctx)
	if err != nil {
		slog.Warn("failed to load push subscriptions from DB", "err", err)
		return
	}
	s.pushMu.Lock()
	for _, sub := range subs {
		s.pushSubs = append(s.pushSubs, webpush.Subscription{
			Endpoint: sub.Endpoint,
			Keys: webpush.Keys{
				P256dh: sub.P256dh,
				Auth:   sub.Auth,
			},
		})
	}
	s.pushMu.Unlock()
	slog.Info("loaded push subscriptions from DB", "count", len(subs))
}

func (s *server) handleVAPIDKey(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"public_key": s.vapidPublicKey})
}

func (s *server) handlePushSubscribe(w http.ResponseWriter, r *http.Request) {
	var sub webpush.Subscription
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		http.Error(w, "invalid subscription", http.StatusBadRequest)
		return
	}
	s.pushMu.Lock()
	s.pushSubs = append(s.pushSubs, sub)
	s.pushMu.Unlock()
	if s.database != nil {
		s.database.SavePushSubscription(r.Context(), &db.PushSubscription{
			Endpoint: sub.Endpoint,
			P256dh:   sub.Keys.P256dh,
			Auth:     sub.Keys.Auth,
		})
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handlePushUnsubscribe(w http.ResponseWriter, r *http.Request) {
	var sub webpush.Subscription
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		http.Error(w, "invalid subscription", http.StatusBadRequest)
		return
	}
	s.pushMu.Lock()
	for i, existing := range s.pushSubs {
		if existing.Endpoint == sub.Endpoint {
			s.pushSubs = append(s.pushSubs[:i], s.pushSubs[i+1:]...)
			break
		}
	}
	s.pushMu.Unlock()
	if s.database != nil {
		s.database.DeletePushSubscription(r.Context(), sub.Endpoint)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) sendPushNotification(title, body string) {
	if s.vapidPublicKey == "" || s.vapidPrivate == "" {
		return
	}
	payload, _ := json.Marshal(map[string]string{"title": title, "body": body})
	s.pushMu.RLock()
	subs := make([]webpush.Subscription, len(s.pushSubs))
	copy(subs, s.pushSubs)
	s.pushMu.RUnlock()

	for _, sub := range subs {
		resp, err := webpush.SendNotification(payload, &sub, &webpush.Options{
			VAPIDPublicKey:  s.vapidPublicKey,
			VAPIDPrivateKey: s.vapidPrivate,
			Subscriber:      "mailto:line@goyangi.io",
		})
		if err != nil {
			slog.Debug("push notification failed", "endpoint", sub.Endpoint, "err", err)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusGone {
			s.pushMu.Lock()
			for i, existing := range s.pushSubs {
				if existing.Endpoint == sub.Endpoint {
					s.pushSubs = append(s.pushSubs[:i], s.pushSubs[i+1:]...)
					break
				}
			}
			s.pushMu.Unlock()
			if s.database != nil {
				s.database.DeletePushSubscription(context.Background(), sub.Endpoint)
			}
		}
	}
}


