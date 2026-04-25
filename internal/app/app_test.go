package app

import (
	"context"
	"testing"

	"internalkpi-seatalk-bot/internal/seatalk"
)

type fakeGroupIDStore struct {
	upsertTab string
	upsertID  string
	removeTab string
	removeID  string
}

func (f *fakeGroupIDStore) UpsertGroupID(_ context.Context, tab, groupID string) error {
	f.upsertTab = tab
	f.upsertID = groupID
	return nil
}

func (f *fakeGroupIDStore) RemoveGroupID(_ context.Context, tab, groupID string) error {
	f.removeTab = tab
	f.removeID = groupID
	return nil
}

func TestHandleSeaTalkEventStoresJoinedGroupID(t *testing.T) {
	var event seatalk.CallbackEvent
	event.EventType = seatalk.EventBotAddedToGroupChat
	event.Event.Group.GroupID = "group-123"
	event.Event.Group.GroupName = "SOC Alerts"

	store := &fakeGroupIDStore{}
	if err := handleSeaTalkEvent(context.Background(), event, "bot_config", store); err != nil {
		t.Fatalf("handle event: %v", err)
	}

	if store.upsertTab != "bot_config" {
		t.Fatalf("upsert tab = %q, want bot_config", store.upsertTab)
	}
	if store.upsertID != "group-123" {
		t.Fatalf("upsert group ID = %q, want group-123", store.upsertID)
	}
}

func TestHandleSeaTalkEventRemovesGroupID(t *testing.T) {
	var event seatalk.CallbackEvent
	event.EventType = seatalk.EventBotRemovedFromGroupChat
	event.Event.Group.GroupID = "group-123"

	store := &fakeGroupIDStore{}
	if err := handleSeaTalkEvent(context.Background(), event, "bot_config", store); err != nil {
		t.Fatalf("handle event: %v", err)
	}

	if store.removeTab != "bot_config" {
		t.Fatalf("remove tab = %q, want bot_config", store.removeTab)
	}
	if store.removeID != "group-123" {
		t.Fatalf("remove group ID = %q, want group-123", store.removeID)
	}
}
