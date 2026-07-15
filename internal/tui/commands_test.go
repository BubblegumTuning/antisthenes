package tui

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	openai "github.com/sashabaranov/go-openai"

	"github.com/nanami/antisthenes/config"
	"github.com/nanami/antisthenes/internal/agent"
)

func TestCallAgentCmd_ReturnsImmediately(t *testing.T) {
	loop := agent.NewLoop("", "test", "")
	m := Model{
		loop:     loop,
		viewport: viewport.New(80, 10),
		cfg:      config.Config{AgentName: "Test"},
	}
	m.windows[0].Messages = []openai.ChatCompletionMessage{{Role: "user", Content: "ping"}}

	var notified atomic.Bool
	m.SetNotify(func(tea.Msg) { notified.Store(true) })

	cmd := (&m).callAgentCmd()
	if cmd == nil {
		t.Fatal("expected cmd")
	}
	start := time.Now()
	if msg := cmd(); msg != nil {
		t.Fatalf("async cmd should return nil immediately, got %T", msg)
	}
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("cmd blocked for %v", elapsed)
	}

	deadline := time.After(3 * time.Second)
	for !notified.Load() {
		select {
		case <-deadline:
			t.Fatal("agent goroutine did not notify within timeout")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}
