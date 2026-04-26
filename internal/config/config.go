package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port                  string
	SeaTalkAppID          string
	SeaTalkAppSecret      string
	SeaTalkSigningSecret  string
	AdminToken            string
	GoogleCredentials     string
	GoogleCredentialsJSON string
	SheetID               string
	TabName               string
	WatchRange            string
	CaptureRange          string
	BotConfigTab          string
	ReportLink            string
	Timezone              string
	EnableSheetPolling    bool
	PollInterval          time.Duration
	SettleInterval        time.Duration
	ImageFormat           string
	PNGDPI                int
	PNGMaxWidth           int
	WorkDir               string
}

func Load() (Config, error) {
	cfg := Config{
		Port:               getenv("PORT", "8080"),
		SheetID:            getenv("SHEET_ID", "1pLN46ZKWJIsidswMeoxhZwoacuFMR08sCaTFG6mLytc"),
		TabName:            getenv("TAB_NAME", "revamped_bot_server"),
		WatchRange:         getenv("WATCH_RANGE", "X7:X59"),
		CaptureRange:       getenv("CAPTURE_RANGE", "F1:AD59"),
		BotConfigTab:       getenv("BOT_CONFIG_TAB", "bot_config"),
		ReportLink:         getenv("REPORT_LINK", "https://docs.google.com/spreadsheets/d/1fz0N-8-BWs_6ub4UzfKhBdLIjlRpwc94p4DJHNB6SvU/edit?gid=1887496356#gid=1887496356"),
		Timezone:           getenv("APP_TIMEZONE", "Asia/Manila"),
		ImageFormat:        getenv("IMAGE_FORMAT", "png"),
		PNGDPI:             mustInt("PNG_DPI", 180),
		PNGMaxWidth:        mustInt("PNG_MAX_WIDTH", 1600),
		WorkDir:            getenv("WORK_DIR", "tmp"),
		EnableSheetPolling: getenv("ENABLE_SHEET_POLLING", "true") == "true",
		PollInterval:       mustDuration("POLL_INTERVAL", 5*time.Minute),
		SettleInterval:     mustDuration("SETTLE_INTERVAL", 7*time.Second),
	}
	cfg.SeaTalkAppID = os.Getenv("SEATALK_APP_ID")
	cfg.SeaTalkAppSecret = os.Getenv("SEATALK_APP_SECRET")
	cfg.SeaTalkSigningSecret = os.Getenv("SEATALK_SIGNING_SECRET")
	cfg.AdminToken = os.Getenv("ADMIN_TOKEN")
	cfg.GoogleCredentials = os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	cfg.GoogleCredentialsJSON = os.Getenv("GOOGLE_CREDENTIALS_JSON")

	for name, value := range map[string]string{
		"SEATALK_APP_ID":         cfg.SeaTalkAppID,
		"SEATALK_APP_SECRET":     cfg.SeaTalkAppSecret,
		"SEATALK_SIGNING_SECRET": cfg.SeaTalkSigningSecret,
	} {
		if value == "" {
			return Config{}, fmt.Errorf("%s is required", name)
		}
	}
	if cfg.GoogleCredentials == "" && cfg.GoogleCredentialsJSON == "" {
		return Config{}, fmt.Errorf("GOOGLE_APPLICATION_CREDENTIALS or GOOGLE_CREDENTIALS_JSON is required")
	}

	if cfg.ImageFormat != "png" && cfg.ImageFormat != "jpg" && cfg.ImageFormat != "jpeg" {
		return Config{}, fmt.Errorf("IMAGE_FORMAT must be png or jpg")
	}
	if cfg.PNGDPI <= 0 {
		return Config{}, fmt.Errorf("PNG_DPI must be greater than 0")
	}
	if cfg.PNGMaxWidth <= 0 {
		return Config{}, fmt.Errorf("PNG_MAX_WIDTH must be greater than 0")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func mustDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	if parsed, err := time.ParseDuration(value); err == nil {
		return parsed
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		return time.Duration(seconds) * time.Second
	}
	return fallback
}

func mustInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
