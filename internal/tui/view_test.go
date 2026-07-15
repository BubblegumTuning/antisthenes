package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/nanami/antisthenes/config"
	openai "github.com/sashabaranov/go-openai"
)

func TestView_SingleStatusBarWithSlashHint(t *testing.T) {
	m := Model{
		textInput: textarea.New(),
		cfg:       config.Config{AgentName: "Test", MaxTokens: 160000, EditHeight: 3},
	}
	mp := &m
	mp.handleWindowSize(tea.WindowSizeMsg{Width: 80, Height: 24})
	mp.textInput.SetValue("/hel")
	out := mp.View()
	if strings.Count(out, "tokens |") != 1 {
		t.Fatalf("slash hint must not duplicate status bar, got %d token bars", strings.Count(out, "tokens |"))
	}
	if !strings.Contains(out, "/help") && !strings.Contains(out, "Slash:") {
		t.Fatalf("slash hint missing from status row:\n%s", out)
	}
	lines := strings.Count(out, "\n") + 1
	if lines > m.height {
		t.Fatalf("view height %d exceeds terminal %d with slash hint active", lines, m.height)
	}
}

func TestView_SingleStatusBarAndInput(t *testing.T) {
	m := Model{
		textInput: textarea.New(),
		cfg:       config.Config{AgentName: "Test", MaxTokens: 160000, EditHeight: 3},
	}
	mp := &m
	mp.handleWindowSize(tea.WindowSizeMsg{Width: 80, Height: 24})
	mp.windows[0].Messages = []openai.ChatCompletionMessage{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
	}
	mp.syncViewport()
	out := mp.View()
	tokenBar := strings.Count(out, "tokens |")
	if tokenBar != 1 {
		t.Fatalf("expected exactly one token status bar, got %d", tokenBar)
	}
	lines := strings.Count(out, "\n") + 1
	if lines > m.height {
		t.Fatalf("view height %d exceeds terminal height %d (layout overflow can duplicate chrome)", lines, m.height)
	}
}

func TestView_States(t *testing.T) {
	base := Model{
		ready:     true,
		width:     80,
		height:    24,
		viewport:  viewport.New(80, 18),
		textInput: textarea.New(),
		cfg:       config.Config{AgentName: "Test", MaxTokens: 1000},
	}
	m1 := base
	m1.thinking = true
	v1 := (&m1).View()
	if !strings.Contains(v1, "Thinking") {
		t.Error("no thinking in view")
	}
	m2 := base
	m2.lastError = "boom"
	v2 := (&m2).View()
	if !strings.Contains(v2, "Error: boom") {
		t.Error("no error in view")
	}
	m3 := base
	m3.confirmCommand = "/clear"
	v3 := (&m3).View()
	if !strings.Contains(v3, "Confirm Action") || !strings.Contains(v3, "/clear") {
		t.Error("no confirm modal in view")
	}
	m4 := Model{ready: false}
	v4 := (&m4).View()
	if !strings.Contains(v4, "Initializing") {
		t.Error("not initializing message")
	}
}
