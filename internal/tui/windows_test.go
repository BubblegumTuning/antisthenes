package tui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/nanami/antisthenes/internal/memory"
	openai "github.com/sashabaranov/go-openai"
)

func TestWindowSwitchSlot(t *testing.T) {
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}, Alt: true}
	if got := windowSwitchSlot(msg); got != 2 {
		t.Fatalf("slot = %d, want 2", got)
	}
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}, Alt: true}
	if got := windowSwitchSlot(msg); got != -1 {
		t.Fatalf("non-digit alt = %d, want -1", got)
	}
}

func TestSwitchToWindow(t *testing.T) {
	m := Model{
		windows: [maxChatWindows]ChatWindow{
			{Label: "Chat", SessionID: "s1", Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "one"}}},
			{Label: "Telegram", SessionID: "s2", Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "two"}}},
		},
		activeWindow: 0,
		textInput:    textarea.New(),
		viewport:     viewport.New(80, 10),
		ready:        true,
		width:        80,
	}
	m.textInput.SetValue("draft-main")

	(&m).switchToWindow(1)
	if m.activeWindow != 1 {
		t.Fatalf("activeWindow = %d, want 1", m.activeWindow)
	}
	if m.windows[0].InputDraft != "draft-main" {
		t.Fatalf("draft not saved: %q", m.windows[0].InputDraft)
	}
	if len(m.windows[1].Messages) != 1 || m.windows[1].Messages[0].Content != "two" {
		t.Fatalf("wrong window messages: %+v", m.windows[1].Messages)
	}
}

func TestSpawnNewSession(t *testing.T) {
	tmp := t.TempDir()
	store, err := memory.NewStore(filepath.Join(tmp, "win.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	m := NewModel(nil, store, "main-session", false)
	m.ready = true
	m.viewport = viewport.New(80, 10)
	m.width = 80

	if !m.spawnNewSession() {
		t.Fatal("expected spawn to succeed")
	}
	if m.activeWindow != 2 {
		t.Fatalf("activeWindow = %d, want 2", m.activeWindow)
	}
	if m.windows[2].SessionID == "" {
		t.Fatal("window 3 has no session")
	}
	if len(m.windows[2].Messages) == 0 {
		t.Fatal("expected welcome message")
	}
}

func TestNewSessionSlashCommand(t *testing.T) {
	tmp := t.TempDir()
	store, err := memory.NewStore(filepath.Join(tmp, "slash.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ti := textarea.New()
	ti.SetValue("/new_session")
	m := NewModel(nil, store, "main", false)
	m.textInput = ti
	m.ready = true
	m.width = 80
	m.viewport = viewport.New(80, 10)

	updated, _ := modelFromUpdate(&m, tea.KeyMsg{Type: tea.KeyEnter})
	if updated.activeWindow != 2 {
		t.Fatalf("activeWindow = %d, want 2", updated.activeWindow)
	}
}

func TestRenderWindowBar(t *testing.T) {
	m := Model{
		windows: [maxChatWindows]ChatWindow{
			{Label: "Chat", SessionID: "abc"},
			{Label: "Telegram", SessionID: "tg"},
		},
		activeWindow: 0,
		width:        120,
		ready:        true,
	}
	bar := m.renderWindowBar()
	if !strings.Contains(bar, "[1:Chat]") {
		t.Fatalf("missing active marker: %s", bar)
	}
	if !strings.Contains(bar, "2:Telegram") {
		t.Fatalf("missing telegram window: %s", bar)
	}
}
