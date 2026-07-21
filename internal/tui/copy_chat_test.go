package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/nanami/antisthenes/config"
	openai "github.com/sashabaranov/go-openai"
)

func copyTestModel() Model {
	return Model{textInput: textarea.New()}
}

func TestPlainChatText_Basic(t *testing.T) {
	m := Model{
		width: 80,
		cfg:   config.Config{AgentName: "TestAgent"},
	}
	m.windows[0].Messages = []openai.ChatCompletionMessage{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
		{Role: "tool", ToolCallID: "call1", Content: "result"},
	}
	out := m.plainChatText()
	if strings.Contains(out, "\x1b[") {
		t.Fatal("plain chat should not contain ANSI escapes")
	}
	if !strings.Contains(out, "You: hello") {
		t.Error("missing user message")
	}
	if !strings.Contains(out, "TestAgent: hi there") {
		t.Error("missing assistant message")
	}
	if !strings.Contains(out, "← call1: result [OK]") {
		t.Error("missing tool result")
	}
}

func TestPlainVisibleChatText(t *testing.T) {
	m := Model{
		width:    80,
		cfg:      config.Config{AgentName: "Bot"},
		viewport: viewport.New(80, 2),
	}
	m.windows[0].Messages = []openai.ChatCompletionMessage{
		{Role: "user", Content: "line1"},
		{Role: "assistant", Content: "line2"},
		{Role: "assistant", Content: "line3"},
	}
	m.viewport.SetContent(m.plainChatText())
	m.viewport.YOffset = 1

	out := m.plainVisibleChatText()
	lines := strings.Split(out, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 visible lines, got %d: %q", len(lines), out)
	}
}

func TestHandleCopySlash_Empty(t *testing.T) {
	m := copyTestModel()
	cmd, handled := (&m).handleCopySlash("/copy")
	if !handled || cmd != nil {
		t.Fatalf("expected handled copy with no cmd, got handled=%v cmd=%v", handled, cmd)
	}
	if m.windows[0].LastNotification != "Copy: no chat content" {
		t.Fatalf("unexpected notification: %q", m.windows[0].LastNotification)
	}
}

func TestHandleCopyKey(t *testing.T) {
	m := copyTestModel()
	m.windows[0].Messages = []openai.ChatCompletionMessage{
		{Role: "user", Content: "copy me"},
	}
	handled := (&m).handleCopyKey(tea.KeyMsg{Type: tea.KeyCtrlY})
	if !handled {
		t.Fatal("expected ctrl+y handled")
	}
	note := m.windows[0].LastNotification
	if !strings.Contains(note, "Copied chat") &&
		!strings.Contains(note, "Saved chat") &&
		!strings.Contains(note, "Copy failed") {
		t.Fatalf("unexpected notification: %q", note)
	}
}

func TestWriteCopyDestination_FileFallback(t *testing.T) {
	dest, err := writeCopyDestination("hello from antisthenes copy test")
	if err != nil {
		t.Fatalf("writeCopyDestination: %v", err)
	}
	if dest.method != "file" && dest.method != "clipboard" && dest.method != "osc52" {
		t.Fatalf("unexpected method: %q", dest.method)
	}
	if dest.method == "file" {
		if dest.path == "" {
			t.Fatal("expected file path")
		}
		data, err := os.ReadFile(dest.path)
		if err != nil {
			t.Fatalf("read copy file: %v", err)
		}
		if string(data) != "hello from antisthenes copy test" {
			t.Fatalf("unexpected file contents: %q", string(data))
		}
		_ = os.Remove(dest.path)
	}
}
