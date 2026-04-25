package seatalk

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCallbackHandlerVerificationReturnsChallenge(t *testing.T) {
	body := []byte(`{"event_id":"evt-1","event_type":"event_verification","timestamp":1611220944,"app_id":"app-1","event":{"seatalk_challenge":"challenge-123"}}`)
	req := httptest.NewRequest(http.MethodPost, "/seatalk/callback", bytes.NewReader(body))
	req.Header.Set("Signature", signatureForTest("secret", body))
	rec := httptest.NewRecorder()

	CallbackHandler("secret", func(context.Context, CallbackEvent) error {
		t.Fatal("handler should not be called for verification events")
		return nil
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var response map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["seatalk_challenge"] != "challenge-123" {
		t.Fatalf("challenge = %q, want challenge-123", response["seatalk_challenge"])
	}
}

func TestCallbackHandlerParsesBotAddedEvent(t *testing.T) {
	body := []byte(`{"event_id":"evt-2","event_type":"bot_added_to_group_chat","timestamp":1687764109,"app_id":"app-1","event":{"group":{"group_id":"group-123","group_name":"SOC Alerts"}}}`)
	req := httptest.NewRequest(http.MethodPost, "/seatalk/callback", bytes.NewReader(body))
	req.Header.Set("Signature", signatureForTest("secret", body))
	rec := httptest.NewRecorder()

	var got CallbackEvent
	CallbackHandler("secret", func(_ context.Context, event CallbackEvent) error {
		got = event
		return nil
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got.EventType != EventBotAddedToGroupChat {
		t.Fatalf("event type = %q, want %q", got.EventType, EventBotAddedToGroupChat)
	}
	if got.Event.Group.GroupID != "group-123" {
		t.Fatalf("group ID = %q, want group-123", got.Event.Group.GroupID)
	}
	if got.Event.Group.GroupName != "SOC Alerts" {
		t.Fatalf("group name = %q, want SOC Alerts", got.Event.Group.GroupName)
	}
}

func TestCallbackHandlerRejectsInvalidSignature(t *testing.T) {
	body := []byte(`{"event_id":"evt-3","event_type":"bot_added_to_group_chat","event":{"group":{"group_id":"group-123"}}}`)
	req := httptest.NewRequest(http.MethodPost, "/seatalk/callback", bytes.NewReader(body))
	req.Header.Set("Signature", "invalid")
	rec := httptest.NewRecorder()

	CallbackHandler("secret", func(context.Context, CallbackEvent) error {
		t.Fatal("handler should not be called with invalid signature")
		return nil
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func signatureForTest(secret string, body []byte) string {
	sum := sha256.Sum256(append(body, []byte(secret)...))
	return hex.EncodeToString(sum[:])
}
