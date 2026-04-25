package app

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"internalkpi-seatalk-bot/internal/config"
	"internalkpi-seatalk-bot/internal/render"
	"internalkpi-seatalk-bot/internal/seatalk"
	"internalkpi-seatalk-bot/internal/sheets"
	"internalkpi-seatalk-bot/internal/watcher"
)

type App struct {
	cfg      config.Config
	ctx      context.Context
	sheets   *sheets.Client
	seatalk  *seatalk.Client
	renderer *render.Renderer
	watcher  *watcher.Watcher
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	sheetsClient, err := sheets.New(ctx, cfg.GoogleCredentials, cfg.GoogleCredentialsJSON, cfg.SheetID)
	if err != nil {
		return nil, err
	}
	seatalkClient := seatalk.New(cfg.SeaTalkAppID, cfg.SeaTalkAppSecret, cfg.SeaTalkSigningSecret)
	renderer := render.New(cfg.WorkDir, cfg.ImageFormat, cfg.PNGDPI, cfg.PNGMaxWidth)

	a := &App{
		cfg:      cfg,
		ctx:      ctx,
		sheets:   sheetsClient,
		seatalk:  seatalkClient,
		renderer: renderer,
	}
	a.watcher = watcher.New(watcher.Config{
		SheetID:        cfg.SheetID,
		TabName:        cfg.TabName,
		WatchRange:     cfg.WatchRange,
		CaptureRange:   cfg.CaptureRange,
		BotConfigTab:   cfg.BotConfigTab,
		ReportLink:     cfg.ReportLink,
		Timezone:       cfg.Timezone,
		PollInterval:   cfg.PollInterval,
		SettleInterval: cfg.SettleInterval,
	}, sheetsClient, seatalkClient, renderer)
	return a, nil
}

func (a *App) StartBackground(ctx context.Context) {
	if a.cfg.EnableSheetPolling {
		go a.watcher.Run(ctx)
	}
	go a.runDailyGroupSync(ctx)
}

func (a *App) KPIChangeHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		secret := r.Header.Get("X-KPI-Webhook-Secret")
		if secret == "" {
			secret = r.URL.Query().Get("secret")
		}
		if secret != a.cfg.KPIWebhookSecret {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		a.watcher.Trigger(a.ctx, "apps-script")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "scheduled"})
	})
}

func (a *App) SeaTalkCallbackHandler() http.Handler {
	return seatalk.CallbackHandler(a.cfg.SeaTalkSigningSecret, func(ctx context.Context, event seatalk.CallbackEvent) error {
		return handleSeaTalkEvent(ctx, event, a.cfg.BotConfigTab, a.sheets)
	})
}

type groupIDStore interface {
	UpsertGroupID(context.Context, string, string) error
	RemoveGroupID(context.Context, string, string) error
}

func handleSeaTalkEvent(ctx context.Context, event seatalk.CallbackEvent, botConfigTab string, store groupIDStore) error {
	switch event.EventType {
	case seatalk.EventBotAddedToGroupChat:
		if event.Event.Group.GroupID == "" {
			return nil
		}
		log.Printf("bot added to group %s (%s)", event.Event.Group.GroupID, event.Event.Group.GroupName)
		return store.UpsertGroupID(ctx, botConfigTab, event.Event.Group.GroupID)
	case seatalk.EventBotRemovedFromGroupChat:
		if event.Event.Group.GroupID == "" {
			return nil
		}
		log.Printf("bot removed from group %s (%s)", event.Event.Group.GroupID, event.Event.Group.GroupName)
		return store.RemoveGroupID(ctx, botConfigTab, event.Event.Group.GroupID)
	default:
		return nil
	}
}

func (a *App) runDailyGroupSync(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := a.sheets.NormalizeGroupIDs(ctx, a.cfg.BotConfigTab); err != nil {
				log.Printf("daily group sync: %v", err)
			}
		}
	}
}

func (a *App) Close() {
	a.renderer.Cleanup()
}
