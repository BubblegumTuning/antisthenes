package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/nanami/antisthenes/config"
)

func TestAppendInputHistory_CapAndDedup(t *testing.T) {
	w := ChatWindow{}
	cfg := config.Config{InputHistoryEnabled: true, InputHistorySize: 3}

	w.appendInputHistory("a", cfg)
	w.appendInputHistory("b", cfg)
	w.appendInputHistory("c", cfg)
	w.appendInputHistory("d", cfg)

	if len(w.InputHistory) != 3 {
		t.Fatalf("history len = %d, want 3", len(w.InputHistory))
	}
	if w.InputHistory[0] != "b" || w.InputHistory[2] != "d" {
		t.Fatalf("unexpected cap order: %v", w.InputHistory)
	}

	w.appendInputHistory("d", cfg)
	if len(w.InputHistory) != 3 || w.HistoryIndex != 3 {
		t.Fatalf("dedup failed: %v idx=%d", w.InputHistory, w.HistoryIndex)
	}
}

func TestAppendInputHistory_Disabled(t *testing.T) {
	w := ChatWindow{}
	cfg := config.Config{InputHistoryEnabled: false, InputHistorySize: 50}
	w.appendInputHistory("hello", cfg)
	if len(w.InputHistory) != 0 {
		t.Error("history should stay empty when disabled")
	}
}

func TestUpdate_ClearHistorySlash(t *testing.T) {
	ti := textarea.New()
	ti.SetValue("/clear-history")
	m := Model{
		textInput: ti,
		ready:     true,
		width:     80,
		viewport:  viewport.New(80, 20),
		cfg:       config.Config{AgentName: "Test"},
	}
	m.windows[0].InputHistory = []string{"one", "two"}
	m.windows[0].HistoryIndex = 2

	updated, _ := modelFromUpdate(&m, tea.KeyMsg{Type: tea.KeyEnter})
	if len(updated.windows[0].InputHistory) != 0 {
		t.Error("history should be cleared")
	}
	last := updated.windows[0].Messages[len(updated.windows[0].Messages)-1]
	if !strings.Contains(last.Content, "Input history cleared") {
		t.Errorf("unexpected ack: %q", last.Content)
	}
}

func TestUpdate_KeyUp_DisabledByConfig(t *testing.T) {
	w := ChatWindow{InputHistory: []string{"prev"}, HistoryIndex: 1}
	m := Model{
		cfg:       config.Config{InputHistoryEnabled: false},
		textInput: textarea.New(),
		ready:     true,
	}
	m.windows[0] = w
	m.textInput.SetValue("")

	_, handled := (&m).handleKeyMsg(tea.KeyMsg{Type: tea.KeyUp})
	if handled {
		t.Error("Up should not be handled when history disabled")
	}
}
