package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/x/ansi"
	"github.com/nanami/antisthenes/config"
	openai "github.com/sashabaranov/go-openai"
)

func TestRenderChat_AssistantLabelVisible(t *testing.T) {
	data, err := os.ReadFile("/tmp/antisthenes-copy-20260709-191959.txt")
	if err != nil {
		t.Skip("no fixture")
	}
	idx := strings.Index(string(data), "Antisthenes:")
	end := strings.Index(string(data), "\n\nYou: tell me about helsinki")
	content := strings.TrimSpace(string(data)[idx+len("Antisthenes:") : end])
	m := Model{width: 80, cfg: config.Config{AgentName: "Antisthenes"}}
	m.viewport = viewport.New(80, 24)
	m.windows[0].Messages = []openai.ChatCompletionMessage{{Role: "assistant", Content: content}}
	out := m.renderChat()
	plain := ansi.Strip(out)
	if !strings.Contains(plain, "Antisthenes:") {
		t.Fatalf("render missing agent label; first 200 chars: %q", plain[:min(200, len(plain))])
	}
	vp := viewport.New(80, 24)
	vp.SetContent(out)
	view := ansi.Strip(vp.View())
	if !strings.Contains(view, "Antisthenes:") {
		t.Fatalf("viewport missing agent label; sample: %q", view[:min(300, len(view))])
	}
}
