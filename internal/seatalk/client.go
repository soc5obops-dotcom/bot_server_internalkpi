package seatalk

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	EventVerification            = "event_verification"
	EventBotAddedToGroupChat     = "bot_added_to_group_chat"
	EventBotRemovedFromGroupChat = "bot_removed_from_group_chat"
	groupMessageEndpoint         = "https://openapi.seatalk.io/messaging/v2/group_chat"
	appAccessTokenEndpoint       = "https://openapi.seatalk.io/auth/app_access_token"
)

type Client struct {
	appID         string
	appSecret     string
	signingSecret string
	httpClient    *http.Client
	mu            sync.Mutex
	token         string
	tokenExpire   time.Time
}

type AlertCard struct {
	UpdatedAt          time.Time
	ControlTowerUpdate string
	OTP1               string
	OTP2               string
	MDT                string
	ReportLink         string
}

func New(appID, appSecret, signingSecret string) *Client {
	return &Client{
		appID:         appID,
		appSecret:     appSecret,
		signingSecret: signingSecret,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) SendInteractiveAlert(ctx context.Context, groupID string, card AlertCard) error {
	description := fmt.Sprintf(
		"Control Tower Latest Update: %s\n----------------------------------\n🏆Month-to-Date\n╰⪼ OTP-1: %s\n╰⪼ OTP-2: %s\n╰⪼ MDT: %s\n----------------------------------",
		blank(card.ControlTowerUpdate),
		blank(card.OTP1),
		blank(card.OTP2),
		blank(card.MDT),
	)
	payload := map[string]any{
		"group_id": groupID,
		"message": map[string]any{
			"tag": "interactive_message",
			"interactive_message": map[string]any{
				"elements": []any{
					map[string]any{
						"element_type": "title",
						"title": map[string]any{
							"text": "SOC 5 KPI Report Update as of " + card.UpdatedAt.Format("3:04PM Jan-02"),
						},
					},
					map[string]any{
						"element_type": "description",
						"description": map[string]any{
							"format": 1,
							"text":   description,
						},
					},
					map[string]any{
						"element_type": "button",
						"button": map[string]any{
							"button_type":  "redirect",
							"text":         "View Report Link",
							"mobile_link":  map[string]any{"type": "web", "path": card.ReportLink},
							"desktop_link": map[string]any{"type": "web", "path": card.ReportLink},
						},
					},
				},
			},
		},
	}
	return c.postAuthed(ctx, groupMessageEndpoint, payload, nil)
}

func blank(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func (c *Client) SendImage(ctx context.Context, groupID string, imageBase64 string) error {
	payload := map[string]any{
		"group_id": groupID,
		"message": map[string]any{
			"tag": "image",
			"image": map[string]any{
				"content": imageBase64,
			},
		},
	}
	return c.postAuthed(ctx, groupMessageEndpoint, payload, nil)
}

func (c *Client) postAuthed(ctx context.Context, url string, payload any, out any) error {
	token, err := c.accessToken(ctx)
	if err != nil {
		return err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("seatalk status %d: %s", resp.StatusCode, string(respBody))
	}
	var apiResp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if out == nil {
		out = &apiResp
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return err
	}
	if apiResp.Code != 0 && out == &apiResp {
		return fmt.Errorf("seatalk code %d: %s", apiResp.Code, apiResp.Msg)
	}
	return nil
}

func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Now().Before(c.tokenExpire.Add(-5*time.Minute)) {
		return c.token, nil
	}
	payload := map[string]string{"app_id": c.appID, "app_secret": c.appSecret}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, appAccessTokenEndpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("token status %d: %s", resp.StatusCode, string(respBody))
	}
	var parsed struct {
		Code           int    `json:"code"`
		AppAccessToken string `json:"app_access_token"`
		Expire         int64  `json:"expire"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", err
	}
	if parsed.Code != 0 || parsed.AppAccessToken == "" {
		return "", fmt.Errorf("token code %d", parsed.Code)
	}
	c.token = parsed.AppAccessToken
	c.tokenExpire = time.Unix(parsed.Expire, 0)
	return c.token, nil
}

func ValidSignature(secret string, body []byte, signature string) bool {
	if secret == "" || signature == "" {
		return false
	}
	sum := sha256.Sum256(append(body, []byte(secret)...))
	return hex.EncodeToString(sum[:]) == signature
}
