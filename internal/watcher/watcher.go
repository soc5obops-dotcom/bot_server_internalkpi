package watcher

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"internalkpi-seatalk-bot/internal/seatalk"
)

type Config struct {
	SheetID      string
	TabName      string
	CaptureRange string
	BotConfigTab string
	ReportLink   string
	Timezone     string
}

type Sheets interface {
	Values(context.Context, string, string) ([][]string, error)
	GroupIDs(context.Context, string) ([]string, error)
	SheetGID(context.Context, string) (int64, error)
	Token(context.Context) (string, error)
}

type SeaTalk interface {
	SendInteractiveAlert(context.Context, string, seatalk.AlertCard, string) error
	SendImage(context.Context, string, string) error
}

type Renderer interface {
	Capture(context.Context, string, int64, string, string) (string, error)
}

type Watcher struct {
	cfg      Config
	sheets   Sheets
	seatalk  SeaTalk
	renderer Renderer
	mu       sync.Mutex
	alerting bool
}

var scheduledSendHours = []int{0, 4, 6, 10, 13, 15, 18, 21}

func New(cfg Config, sheets Sheets, seatalk SeaTalk, renderer Renderer) *Watcher {
	return &Watcher{cfg: cfg, sheets: sheets, seatalk: seatalk, renderer: renderer}
}

func (w *Watcher) RunSchedule(ctx context.Context) {
	location, err := time.LoadLocation(w.cfg.Timezone)
	if err != nil {
		log.Printf("scheduled send timezone %q invalid, using local timezone: %v", w.cfg.Timezone, err)
		location = time.Local
	}
	log.Printf("scheduled sends enabled for %s at 6AM, 10AM, 1PM, 3PM, 6PM, 9PM, 12MN, 4AM", location)

	for {
		now := time.Now().In(location)
		next := nextScheduledSend(now, scheduledSendHours)
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			log.Printf("scheduled send triggered for %s", next.Format("3:04PM Jan-02"))
			w.runAlert(ctx)
		}
	}
}

func nextScheduledSend(now time.Time, hours []int) time.Time {
	for _, hour := range hours {
		next := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())
		if next.After(now) {
			return next
		}
	}
	return time.Date(now.Year(), now.Month(), now.Day()+1, hours[0], 0, 0, 0, now.Location())
}

func (w *Watcher) runAlert(parent context.Context) {
	w.mu.Lock()
	if w.alerting {
		w.mu.Unlock()
		log.Printf("alert already running; skipping overlapping trigger")
		return
	}
	w.alerting = true
	w.mu.Unlock()
	defer func() {
		w.mu.Lock()
		w.alerting = false
		w.mu.Unlock()
	}()

	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	if err := w.alert(ctx); err != nil {
		log.Printf("alert: %v", err)
	}
}

func (w *Watcher) SendNow(ctx context.Context) error {
	w.mu.Lock()
	if w.alerting {
		w.mu.Unlock()
		return fmt.Errorf("alert already running")
	}
	w.alerting = true
	w.mu.Unlock()
	defer func() {
		w.mu.Lock()
		w.alerting = false
		w.mu.Unlock()
	}()

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	return w.alert(ctx)
}

func (w *Watcher) alert(ctx context.Context) error {
	gid, err := w.sheets.SheetGID(ctx, w.cfg.TabName)
	if err != nil {
		return err
	}
	token, err := w.sheets.Token(ctx)
	if err != nil {
		return err
	}
	image, err := w.renderer.Capture(ctx, w.cfg.SheetID, gid, w.cfg.CaptureRange, token)
	if err != nil {
		return err
	}
	groupIDs, err := w.sheets.GroupIDs(ctx, w.cfg.BotConfigTab)
	if err != nil {
		return err
	}
	if len(groupIDs) == 0 {
		return fmt.Errorf("no SeaTalk group IDs found in %s!A2:A", w.cfg.BotConfigTab)
	}
	card, err := w.alertCard(ctx)
	if err != nil {
		return err
	}
	for _, groupID := range groupIDs {
		if err := w.seatalk.SendInteractiveAlert(ctx, groupID, card, image); err != nil {
			log.Printf("send card to %s: %v", groupID, err)
			continue
		}
		log.Printf("sent interactive card with report image to %s", groupID)
	}
	return nil
}

func (w *Watcher) alertCard(ctx context.Context) (seatalk.AlertCard, error) {
	controlTower, err := w.cell(ctx, "E1")
	if err != nil {
		return seatalk.AlertCard{}, err
	}
	return seatalk.AlertCard{
		UpdatedAt:          w.now(),
		ControlTowerUpdate: controlTower,
		ReportLink:         w.cfg.ReportLink,
	}, nil
}

func (w *Watcher) now() time.Time {
	location, err := time.LoadLocation(w.cfg.Timezone)
	if err != nil {
		return time.Now()
	}
	return time.Now().In(location)
}

func (w *Watcher) cell(ctx context.Context, cell string) (string, error) {
	values, err := w.sheets.Values(ctx, w.cfg.TabName, cell)
	if err != nil {
		return "", fmt.Errorf("read %s!%s: %w", w.cfg.TabName, cell, err)
	}
	if len(values) == 0 || len(values[0]) == 0 {
		return "", nil
	}
	return values[0][0], nil
}
