package gateway

import (
	"context"
	"sync"

	openai "github.com/sashabaranov/go-openai"
)

// MessageHandler is the interface that any frontend (TUI, HTTP, MCP, platform adapters)
// must implement to drive the agent loop.
type MessageHandler interface {
	HandleMessage(ctx context.Context, messages []openai.ChatCompletionMessage) (openai.ChatCompletionMessage, error)
}

// Gateway provides a narrow interface for different delivery channels.
type Gateway struct {
	handler   MessageHandler
	adapters  map[string]PlatformAdapter
	adapterMu sync.RWMutex
}

// NewGateway creates a new gateway with the given handler.
func NewGateway(h MessageHandler) *Gateway {
	return &Gateway{
		handler:  h,
		adapters: make(map[string]PlatformAdapter),
	}
}

// Send forwards a message through the primary handler.
func (g *Gateway) Send(ctx context.Context, messages []openai.ChatCompletionMessage) (openai.ChatCompletionMessage, error) {
	return g.handler.HandleMessage(ctx, messages)
}

// RegisterAdapter adds a platform adapter (Telegram, WhatsApp, etc.).
func (g *Gateway) RegisterAdapter(adapter PlatformAdapter) {
	g.adapterMu.Lock()
	defer g.adapterMu.Unlock()
	g.adapters[adapter.Name()] = adapter
}

// GetAdapter returns a registered adapter by name.
func (g *Gateway) GetAdapter(name string) PlatformAdapter {
	g.adapterMu.RLock()
	defer g.adapterMu.RUnlock()
	return g.adapters[name]
}

// StartAllAdapters starts all registered adapters.
func (g *Gateway) StartAllAdapters(ctx context.Context) error {
	g.adapterMu.RLock()
	defer g.adapterMu.RUnlock()

	for _, adapter := range g.adapters {
		if err := adapter.Start(ctx); err != nil {
			return err
		}
	}
	return nil
}
