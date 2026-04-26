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
