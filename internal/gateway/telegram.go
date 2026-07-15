package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// TelegramConfig holds minimal settings for outbound Telegram delivery.
type TelegramConfig struct {
	BotToken string
	ChatID   string // can be user or group
}

// TelegramSender implements a minimal outbound-only Telegram channel.
type TelegramSender struct {
	cfg    TelegramConfig
	client *http.Client
}

// NewTelegramSender creates a Telegram sender.
func NewTelegramSender(cfg TelegramConfig) *TelegramSender {
	return &TelegramSender{
		cfg: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SendMessage sends a plain text message via Telegram Bot API.
func (t *TelegramSender) SendMessage(ctx context.Context, text string) error {
	if t.cfg.BotToken == "" || t.cfg.ChatID == "" {
		return fmt.Errorf("telegram not configured")
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.cfg.BotToken)

	payload := map[string]string{
		"chat_id": t.cfg.ChatID,
		"text":    text,
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("telegram send failed: status %d", resp.StatusCode)
	}
	return nil
}
