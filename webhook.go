package unionewrapper

import (
	"context"
	"io"
	"net/http"

	"github.com/unione-repo/unione-go-sdk/api/webhook"
)

// MaxWebhookBody is the largest webhook request body accepted, bytes.
const MaxWebhookBody = 16 << 20

// Handlers receive the webhook notifications with a valid signature. Both are optional; an error answers
// 500, so the API repeats the notification later. They must be fast: the API waits for 200 OK for 3 seconds.
type Handlers struct {
	// OnNotification gets the whole notification.
	OnNotification func(ctx context.Context, notification *webhook.WebhookCallback) error
	// OnEvent gets each event of the notification (email status, spam block...) with its user/project.
	OnEvent func(ctx context.Context, event *webhook.WebhookCallbackEventsByUserItemEventsItem, user *webhook.WebhookCallbackEventsByUserItem) error
}

// WebhookHandler is the webhook endpoint: GET/HEAD answer 200 (the URL check of webhook/set), POST checks
// the signature with key (401 when wrong), parses the notification and calls the handlers.
func WebhookHandler(key string, h Handlers) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead:
			w.WriteHeader(http.StatusOK)
			return
		case http.MethodPost:
		default:
			w.Header().Set("Allow", "GET, HEAD, POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxWebhookBody))
		if err != nil {
			http.Error(w, "can not read the body", http.StatusBadRequest)
			return
		}
		if !VerifySignature(body, key) {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
		notification, err := webhook.ParseCallback(body)
		if err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		if err := dispatch(r.Context(), notification, h); err != nil {
			http.Error(w, "webhook handler failed", http.StatusInternalServerError)
			return
		}
		_, _ = io.WriteString(w, "OK")
	})
}

func dispatch(ctx context.Context, n *webhook.WebhookCallback, h Handlers) error {
	if h.OnNotification != nil {
		if err := h.OnNotification(ctx, n); err != nil {
			return err
		}
	}
	if h.OnEvent == nil {
		return nil
	}
	for i := range n.EventsByUser {
		user := &n.EventsByUser[i]
		for j := range user.Events {
			if err := h.OnEvent(ctx, &user.Events[j], user); err != nil {
				return err
			}
		}
	}
	return nil
}
