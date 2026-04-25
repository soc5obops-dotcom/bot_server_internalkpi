package watcher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"internalkpi-seatalk-bot/internal/seatalk"
)

type Config struct {
	SheetID        string
	TabName        string
	WatchRange     string
	CaptureRange   string
	BotConfigTab   string
	ReportLink     string
	Timezone       string
	PollInterval   time.Duration
	SettleInterval time.Duration
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
	timer    *time.Timer
	alerting bool
}

func New(cfg Config, sheets Sheets, seatalk SeaTalk, renderer Renderer) *Watcher {
	return &Watcher{cfg: cfg, sheets: sheets, seatalk: seatalk, renderer: renderer}
}

func (w *Watcher) Run(ctx context.Context) {
	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	var lastHash string
	var initialized bool
	poll := func() {
		values, err := w.sheets.Values(ctx, w.cfg.TabName, w.cfg.WatchRange)
		if err != nil {
			log.Printf("watch values: %v", err)
			return
		}
		currentHash := hashValues(values)
		if !initialized {
			lastHash = currentHash
			initialized = true
			log.Printf("watch initialized for %s!%s; polling every %s", w.cfg.TabName, w.cfg.WatchRange, w.cfg.PollInterval)
			return
		}
		if currentHash != lastHash {
			log.Printf("change detected in %s!%s; capture scheduled after %s", w.cfg.TabName, w.cfg.WatchRange, w.cfg.SettleInterval)
			w.Trigger(ctx, "sheet-polling")
			lastHash = currentHash
		}
	}

	poll()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			poll()
		}
	}
}

func (w *Watcher) Trigger(ctx context.Context, source string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.timer != nil {
		w.timer.Stop()
	}
	log.Printf("change signal received from %s; capture scheduled after %s", source, w.cfg.SettleInterval)
	w.timer = time.AfterFunc(w.cfg.SettleInterval, func() {
		w.runAlert(ctx)
	})
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
	otp1, err := w.cell(ctx, "S15")
	if err != nil {
		return seatalk.AlertCard{}, err
	}
	otp2, err := w.cell(ctx, "S16")
	if err != nil {
		return seatalk.AlertCard{}, err
	}
	mdt, err := w.cell(ctx, "C21")
	if err != nil {
		return seatalk.AlertCard{}, err
	}
	if otp2 == "" {
		otp2 = "for update"
	}
	return seatalk.AlertCard{
		UpdatedAt:          w.now(),
		ControlTowerUpdate: controlTower,
		OTP1:               otp1,
		OTP2:               otp2,
		MDT:                mdt,
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

func hashValues(values [][]string) string {
	body, _ := json.Marshal(values)
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
