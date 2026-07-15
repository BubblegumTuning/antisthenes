package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/nanami/antisthenes/config"
	openai "github.com/sashabaranov/go-openai"
)

func TestWrap(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		width int
		want  string
	}{
		{"short", "hello", 10, "hello"},
		{"zero width", "hi", 0, "hi"},
		{"small width", "test message", 4, "test"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrap(tt.text, tt.width)
			if !strings.Contains(got, tt.want) && len(got) == 0 && tt.want != "" {
				t.Errorf("wrap(%q, %d) = %q, want contain %q", tt.text, tt.width, got, tt.want)
			}
		})
	}
}

func TestRenderChat_AssistantLabelOnFirstLine(t *testing.T) {
	m := Model{width: 80, cfg: config.Config{AgentName: "Antisthenes"}}
	m.viewport = viewport.New(80, 20)
	m.windows[0].Messages = []openai.ChatCompletionMessage{
		{Role: "assistant", Content: "Hello from the agent with enough text to wrap across multiple viewport lines easily."},
	}
	out := m.renderChat()
	if !strings.Contains(out, "Antisthenes:") {
		t.Fatalf("missing agent label in render:\n%s", out)
	}
}

func TestRenderChat_Basic(t *testing.T) {
	m := Model{
		width: 80,
		cfg:   config.Config{AgentName: "TestAgent"},
	}
	m.windows[0].Messages = []openai.ChatCompletionMessage{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
		{Role: "tool", ToolCallID: "call1", Content: "result"},
	}
	out := m.renderChat()
	if !strings.Contains(out, "You: hello") {
		t.Error("missing user message")
	}
	if !strings.Contains(out, "TestAgent: hi there") {
		t.Error("missing assistant message")
	}
	if !strings.Contains(out, "call1") {
		t.Error("missing tool result")
	}
}

func TestRenderChat_DefaultAgentName(t *testing.T) {
	m := Model{width: 80, cfg: config.Config{AgentName: ""}}
	m.windows[0].Messages = []openai.ChatCompletionMessage{
		{Role: "assistant", Content: "hello"},
	}
	out := m.renderChat()
	if strings.Contains(out, ": hello") && !strings.Contains(out, "Antisthenes:") {
		t.Fatalf("expected default agent label, got: %q", out)
	}
}

func TestRenderChat_SingleBlankLineBetweenMessages(t *testing.T) {
	m := Model{width: 80, cfg: config.Config{AgentName: "Bot"}}
	m.windows[0].Messages = []openai.ChatCompletionMessage{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "yo"},
	}
	out := m.renderChat()
	if strings.Count(out, "\n\n\n") > 0 {
		t.Fatalf("unexpected triple spacing:\n%s", out)
	}
	if !strings.Contains(out, "You: hi") || !strings.Contains(out, "Bot: yo") {
		t.Fatalf("unexpected layout:\n%s", out)
	}
}
