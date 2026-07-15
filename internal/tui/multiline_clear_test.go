package tui

import (
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/nanami/antisthenes/config"
	"testing"
)

func TestUpdate_KeyEnter_ClearsMultilineTextInput(t *testing.T) {
	ti := textarea.New()
	ti.SetValue("line one\nline two\nline three")
	m := Model{
		textInput: ti,
		ready:     true,
		width:     80,
		viewport:  viewport.New(80, 10),
		cfg:       config.Config{AgentName: "Test"},
	}
	updated, _ := modelFromUpdate(&m, tea.KeyMsg{Type: tea.KeyEnter})
	if updated.textInput.Value() != "" {
		t.Fatalf("multiline textInput not cleared: %q", updated.textInput.Value())
	}
}
