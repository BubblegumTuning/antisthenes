package gateway

import (
	"context"
	"fmt"
	"strings"

	"github.com/nanami/antisthenes/internal/agent"
	openai "github.com/sashabaranov/go-openai"
)

// BridgeOptions configures adapter→agent→reply routing.
type BridgeOptions struct {
	// Notify delivers human-readable status to frontends (e.g. TUI right slot).
	Notify func(text string)
	// OnInbound routes inbound platform messages to a frontend (e.g. TUI window 2).
	// When set, the bridge does not run the agent loop itself.
	OnInbound func(event MessageEvent)
}

// StartBridge starts registered adapters and routes incoming messages through the agent loop.
// Returns after handlers are launched; runs until ctx is cancelled.
func (g *Gateway) StartBridge(ctx context.Context, loop *agent.Loop, opts BridgeOptions) error {
	if err := g.StartAllAdapters(ctx); err != nil {
		return err
	}

	g.adapterMu.RLock()
	adapters := make(map[string]PlatformAdapter, len(g.adapters))
	for name, adapter := range g.adapters {
		adapters[name] = adapter
	}
	g.adapterMu.RUnlock()

	for name, adapter := range adapters {
		go g.handleAdapter(ctx, name, adapter, loop, opts)
	}
	return nil
}

// RunWithAdapters starts the bridge and blocks until ctx is done.
func (g *Gateway) RunWithAdapters(ctx context.Context, loop *agent.Loop) error {
	if err := g.StartBridge(ctx, loop, BridgeOptions{}); err != nil {
		return err
	}
	<-ctx.Done()
	return ctx.Err()
}

// StopAllAdapters stops every registered adapter.
func (g *Gateway) StopAllAdapters() {
	g.adapterMu.RLock()
	defer g.adapterMu.RUnlock()
	for _, adapter := range g.adapters {
		_ = adapter.Stop()
	}
}

func (g *Gateway) handleAdapter(ctx context.Context, name string, adapter PlatformAdapter, loop *agent.Loop, opts BridgeOptions) {
	for event := range adapter.Incoming() {
		if opts.OnInbound != nil {
			if opts.Notify != nil {
				preview := event.Text
				if len(preview) > 60 {
					preview = preview[:60] + "..."
				}
				opts.Notify(fmt.Sprintf("Gateway [%s] <- %s: %s", name, event.UserID, preview))
			}
			opts.OnInbound(event)
			continue
		}

		if opts.Notify != nil {
			preview := event.Text
			if len(preview) > 60 {
				preview = preview[:60] + "..."
			}
			opts.Notify(fmt.Sprintf("Gateway [%s] <- %s: %s", name, event.UserID, preview))
		}

		messages := []openai.ChatCompletionMessage{
			{Role: "user", Content: event.Text},
		}

		reply, err := loop.RunWithTools(ctx, messages)
		if err != nil {
			if opts.Notify != nil {
				opts.Notify(fmt.Sprintf("Gateway [%s]: agent error: %v", name, err))
			}
			_ = adapter.SendMessage(ctx, event.ChatID, "Sorry, I encountered an error.")
			continue
		}

		responseText := extractAssistantReply(reply)
		if err := adapter.SendMessage(ctx, event.ChatID, responseText); err != nil {
			if opts.Notify != nil {
				opts.Notify(fmt.Sprintf("Gateway [%s]: send error: %v", name, err))
			}
			continue
		}

		if opts.Notify != nil {
			summary := responseText
			if len(summary) > 60 {
				summary = summary[:60] + "..."
			}
			opts.Notify(fmt.Sprintf("Gateway [%s] -> chat %s: %s", name, event.ChatID, summary))
		}
	}
}

func extractAssistantReply(reply []openai.ChatCompletionMessage) string {
	for i := len(reply) - 1; i >= 0; i-- {
		if reply[i].Role == "assistant" && strings.TrimSpace(reply[i].Content) != "" {
			return strings.TrimSpace(reply[i].Content)
		}
	}
	return "(no response)"
}
