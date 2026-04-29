package watcher

import (
	"testing"
	"time"
)

func TestNextScheduledSendSameDay(t *testing.T) {
	location := time.FixedZone("PHT", 8*60*60)
	now := time.Date(2026, time.April, 26, 17, 47, 0, 0, location)

	got := nextScheduledSend(now, scheduledSendHours)
	want := time.Date(2026, time.April, 26, 18, 0, 0, 0, location)
	if !got.Equal(want) {
		t.Fatalf("next scheduled send = %s, want %s", got, want)
	}
}

func TestNextScheduledSendNextDay(t *testing.T) {
	location := time.FixedZone("PHT", 8*60*60)
	now := time.Date(2026, time.April, 26, 21, 0, 0, 0, location)

	got := nextScheduledSend(now, scheduledSendHours)
	want := time.Date(2026, time.April, 27, 0, 0, 0, 0, location)
	if !got.Equal(want) {
		t.Fatalf("next scheduled send = %s, want %s", got, want)
	}
}

func TestNextScheduledSendEarlyMorning(t *testing.T) {
	location := time.FixedZone("PHT", 8*60*60)
	now := time.Date(2026, time.April, 26, 0, 1, 0, 0, location)

	got := nextScheduledSend(now, scheduledSendHours)
	want := time.Date(2026, time.April, 26, 4, 0, 0, 0, location)
	if !got.Equal(want) {
		t.Fatalf("next scheduled send = %s, want %s", got, want)
	}
}

func TestScheduledSendHours(t *testing.T) {
	want := []int{0, 4, 6, 10, 13, 15, 18, 21}
	if len(scheduledSendHours) != len(want) {
		t.Fatalf("scheduled hours = %v, want %v", scheduledSendHours, want)
	}
	for i := range want {
		if scheduledSendHours[i] != want[i] {
			t.Fatalf("scheduled hours = %v, want %v", scheduledSendHours, want)
		}
	}
}
