package app

import (
	"context"
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
			log.Printf("bot_added_to_group_chat received without group_id")
			return nil
		}
		log.Printf("bot added to group %s (%s)", event.Event.Group.GroupID, event.Event.Group.GroupName)
		if err := store.UpsertGroupID(ctx, botConfigTab, event.Event.Group.GroupID); err != nil {
			log.Printf("store group id %s in %s failed: %v", event.Event.Group.GroupID, botConfigTab, err)
			return err
		}
		log.Printf("stored group id %s in %s", event.Event.Group.GroupID, botConfigTab)
		return nil
	case seatalk.EventBotRemovedFromGroupChat:
		if event.Event.Group.GroupID == "" {
			log.Printf("bot_removed_from_group_chat received without group_id")
			return nil
		}
		log.Printf("bot removed from group %s (%s)", event.Event.Group.GroupID, event.Event.Group.GroupName)
		if err := store.RemoveGroupID(ctx, botConfigTab, event.Event.Group.GroupID); err != nil {
			log.Printf("remove group id %s from %s failed: %v", event.Event.Group.GroupID, botConfigTab, err)
			return err
		}
		log.Printf("removed group id %s from %s", event.Event.Group.GroupID, botConfigTab)
		return nil
	default:
		log.Printf("ignored seatalk event type %s", event.EventType)
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
