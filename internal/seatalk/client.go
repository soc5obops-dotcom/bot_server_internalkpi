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
	EventVerification                     = "event_verification"
	EventNewBotSubscriber                 = "new_bot_subscriber"
	EventMessageFromBotSubscriber         = "message_from_bot_subscriber"
	EventInteractiveMessageClick          = "interactive_message_click"
	EventBotAddedToGroupChat              = "bot_added_to_group_chat"
	EventBotRemovedFromGroupChat          = "bot_removed_from_group_chat"
	EventNewMentionedMessageFromGroupChat = "new_mentioned_message_received_from_group_chat"
	groupMessageEndpoint                  = "https://openapi.seatalk.io/messaging/v2/group_chat"
	groupTypingEndpoint                   = "https://openapi.seatalk.io/messaging/v2/group_chat_typing"
	serviceNoticeInteractiveCardEndpoint  = "https://openapi.seatalk.io/messaging/v2/single_chat"
	appAccessTokenEndpoint                = "https://openapi.seatalk.io/auth/app_access_token"
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
	ReportLink         string
}

type MessageOptions struct {
	QuotedMessageID string
	ThreadID        string
}

type APIResponse struct {
	Code      int              `json:"code"`
	MessageID string           `json:"message_id"`
	Delivery  []DeliveryStatus `json:"delivery"`
	Msg       string           `json:"msg"`
}

type DeliveryStatus struct {
	Code         int    `json:"code"`
	EmployeeCode string `json:"employee_code"`
	MessageID    string `json:"message_id"`
}

func New(appID, appSecret, signingSecret string) *Client {
	return &Client{
		appID:         appID,
		appSecret:     appSecret,
		signingSecret: signingSecret,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) SendText(ctx context.Context, groupID, content string, format int, opts MessageOptions) (APIResponse, error) {
	if format == 0 {
		format = 1
	}
	message := map[string]any{
		"tag": "text",
		"text": map[string]any{
			"format":  format,
			"content": content,
		},
	}
	applyMessageOptions(message, opts)
	return c.sendGroupMessage(ctx, groupID, message)
}

func (c *Client) SendGroupImage(ctx context.Context, groupID, imageBase64 string, opts MessageOptions) (APIResponse, error) {
	message := map[string]any{
		"tag": "image",
		"image": map[string]any{
			"content": imageBase64,
		},
	}
	applyMessageOptions(message, opts)
	return c.sendGroupMessage(ctx, groupID, message)
}

func (c *Client) SendFile(ctx context.Context, groupID, filename, fileBase64 string, opts MessageOptions) (APIResponse, error) {
	message := map[string]any{
		"tag": "file",
		"file": map[string]any{
			"filename": filename,
			"content":  fileBase64,
		},
	}
	applyMessageOptions(message, opts)
	return c.sendGroupMessage(ctx, groupID, message)
}

func (c *Client) SendInteractive(ctx context.Context, groupID string, elements []any, opts MessageOptions) (APIResponse, error) {
	message := map[string]any{
		"tag": "interactive_message",
		"interactive_message": map[string]any{
			"elements": elements,
		},
	}
	applyMessageOptions(message, opts)
	return c.sendGroupMessage(ctx, groupID, message)
}

func (c *Client) SetGroupTyping(ctx context.Context, groupID, threadID string) (APIResponse, error) {
	payload := map[string]any{"group_id": groupID}
	if threadID != "" {
		payload["thread_id"] = threadID
	}
	var out APIResponse
	err := c.postAuthed(ctx, groupTypingEndpoint, payload, &out)
	return out, err
}

func (c *Client) SendServiceInteractiveCard(ctx context.Context, employeeCodes []string, localizedCards map[string]any, usablePlatform string) (APIResponse, error) {
	payload := map[string]any{
		"tag":                 "interactive_message",
		"interactive_message": localizedCards,
		"employee_codes":      employeeCodes,
	}
	if usablePlatform != "" {
		payload["usable_platform"] = usablePlatform
	}
	var out APIResponse
	err := c.postAuthed(ctx, serviceNoticeInteractiveCardEndpoint, payload, &out)
	return out, err
}

func (c *Client) SendInteractiveAlert(ctx context.Context, groupID string, card AlertCard, imageBase64 string) error {
	description := fmt.Sprintf(
		"----------------------------------\n\nOTP Control Tower\nLatest Update: %s\n----------------------------------",
		blank(card.ControlTowerUpdate),
	)
	elements := []any{
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
			"element_type": "image",
			"image": map[string]any{
				"content": imageBase64,
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
	}
	_, err := c.SendInteractive(ctx, groupID, elements, MessageOptions{})
	return err
}

func (c *Client) sendGroupMessage(ctx context.Context, groupID string, message map[string]any) (APIResponse, error) {
	payload := map[string]any{
		"group_id": groupID,
		"message":  message,
	}
	var out APIResponse
	err := c.postAuthed(ctx, groupMessageEndpoint, payload, &out)
	return out, err
}

func applyMessageOptions(message map[string]any, opts MessageOptions) {
	if opts.QuotedMessageID != "" {
		message["quoted_message_id"] = opts.QuotedMessageID
	}
	if opts.ThreadID != "" {
		message["thread_id"] = opts.ThreadID
	}
}

func blank(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func (c *Client) SendImage(ctx context.Context, groupID string, imageBase64 string) error {
	_, err := c.SendGroupImage(ctx, groupID, imageBase64, MessageOptions{})
	return err
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
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return err
	}
	if apiResp.Code != 0 {
		return fmt.Errorf("seatalk code %d: %s", apiResp.Code, apiResp.Msg)
	}
	if out == nil {
		out = &apiResp
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return err
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
