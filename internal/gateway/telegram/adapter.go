package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/nanami/antisthenes/internal/gateway"
)

// Adapter implements gateway.PlatformAdapter for Telegram using long-polling.
type Adapter struct {
	token    string
	chatID   string
	sender   *gateway.TelegramSender
	incoming chan gateway.MessageEvent
	stopCh   chan struct{}
	wg       sync.WaitGroup
	client   *http.Client
	offset   int64 // last update_id
}

// NewAdapter creates a Telegram adapter with real inbound support.
func NewAdapter(token, chatID string) *Adapter {
	sender := gateway.NewTelegramSender(gateway.TelegramConfig{
		BotToken: token,
		ChatID:   chatID,
	})

	return &Adapter{
		token:    token,
		chatID:   chatID,
		sender:   sender,
		incoming: make(chan gateway.MessageEvent, 64),
		stopCh:   make(chan struct{}),
		client:   &http.Client{Timeout: 60 * time.Second},
	}
}

func (a *Adapter) Name() string { return "telegram" }

func (a *Adapter) Start(ctx context.Context) error {
	a.wg.Add(1)
	go a.pollLoop(ctx)
	return nil
}

func (a *Adapter) pollLoop(ctx context.Context) {
	defer a.wg.Done()

	for {
		select {
		case <-a.stopCh:
			return
		case <-ctx.Done():
			return
		default:
			updates, err := a.getUpdates(ctx)
			if err != nil {
				time.Sleep(2 * time.Second)
				continue
			}

			for _, u := range updates {
				if u.Message != nil && u.Message.Text != "" {
					a.incoming <- gateway.MessageEvent{
						Platform: "telegram",
						ChatID:   strconv.FormatInt(u.Message.Chat.ID, 10),
						UserID:   strconv.FormatInt(u.Message.From.ID, 10),
						Text:     u.Message.Text,
					}
				}
				a.offset = u.UpdateID + 1
			}
		}
	}
}

// getUpdates performs long-polling.
func (a *Adapter) getUpdates(ctx context.Context) ([]Update, error) {
	params := url.Values{}
	params.Set("offset", strconv.FormatInt(a.offset, 10))
	params.Set("timeout", "30")
	params.Set("allowed_updates", `["message"]`)

	reqURL := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?%s", a.token, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("telegram API status %d", resp.StatusCode)
	}

	var result struct {
		OK     bool     `json:"ok"`
		Result []Update `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if !result.OK {
		return nil, fmt.Errorf("telegram API returned not OK")
	}
	return result.Result, nil
}

func (a *Adapter) SendMessage(ctx context.Context, chatID, text string) error {
	return a.sender.SendMessage(ctx, text)
}

func (a *Adapter) Incoming() <-chan gateway.MessageEvent {
	return a.incoming
}

func (a *Adapter) Stop() error {
	close(a.stopCh)
	a.wg.Wait()
	close(a.incoming)
	return nil
}

// Update and Message structs for Telegram API responses (minimal).
type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message"`
}

type Message struct {
	MessageID int64  `json:"message_id"`
	From      User   `json:"from"`
	Chat      Chat   `json:"chat"`
	Text      string `json:"text"`
}

type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
}

type Chat struct {
	ID    int64  `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title"`
}
