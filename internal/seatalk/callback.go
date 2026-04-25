package seatalk

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
)

type CallbackEvent struct {
	EventID   string `json:"event_id"`
	EventType string `json:"event_type"`
	Timestamp int64  `json:"timestamp"`
	AppID     string `json:"app_id"`
	Event     struct {
		SeaTalkChallenge string `json:"seatalk_challenge"`
		Group            struct {
			GroupID   string `json:"group_id"`
			GroupName string `json:"group_name"`
		} `json:"group"`
	} `json:"event"`
}

func CallbackHandler(signingSecret string, handle func(context.Context, CallbackEvent) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if !ValidSignature(signingSecret, body, r.Header.Get("Signature")) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var event CallbackEvent
		if err := json.Unmarshal(body, &event); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if event.EventType == EventVerification {
			_ = json.NewEncoder(w).Encode(map[string]string{
				"seatalk_challenge": event.Event.SeaTalkChallenge,
			})
			return
		}
		if err := handle(r.Context(), event); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}
