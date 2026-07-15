package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/nanami/antisthenes/config"
)

func TestHandleThemeSlash_ApplyGreen(t *testing.T) {
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	ti := textarea.New()
	ti.SetValue("/theme green")
	m := Model{
		textInput: ti,
		ready:     true,
		width:     80,
		viewport:  viewport.New(80, 20),
		cfg:       config.DefaultConfig(),
	}
	cmd, _ := (&m).handleThemeSlash("/theme green")
	if cmd != nil {
		t.Error("theme apply should not return cmd")
	}
	if m.cfg.Colors.User != config.GreenPhosphorColors().User {
		t.Errorf("user color = %q, want %q", m.cfg.Colors.User, config.GreenPhosphorColors().User)
	}
	if len(m.windows[0].Messages) != 0 {
		t.Errorf("theme changes should not be stored in chat history, got %d messages", len(m.windows[0].Messages))
	}
	if !strings.Contains(m.windows[0].LastNotification, "green") {
		t.Errorf("expected status notification mentioning green, got %q", m.windows[0].LastNotification)
	}
}

func TestHandleThemeSlash_ListThemes(t *testing.T) {
	m := Model{
		textInput: textarea.New(),
		ready:     true,
		width:     80,
		viewport:  viewport.New(80, 20),
		cfg:       config.DefaultConfig(),
	}
	_, _ = (&m).handleThemeSlash("/theme")
	if len(m.windows[0].Messages) != 0 {
		t.Error("theme list should not be stored in chat history")
	}
	note := m.windows[0].LastNotification
	if !strings.Contains(note, "green") || !strings.Contains(note, "amber") {
		t.Errorf("theme list should mention green and amber: %q", note)
	}
}

func TestUpdate_ThemeSlashViaEnter(t *testing.T) {
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	ti := textarea.New()
	ti.SetValue("/theme amber")
	m := Model{
		textInput: ti,
		ready:     true,
		width:     80,
		viewport:  viewport.New(80, 20),
		cfg:       config.DefaultConfig(),
	}
	updated, _ := modelFromUpdate(&m, tea.KeyMsg{Type: tea.KeyEnter})
	if updated.cfg.Colors.User != config.AmberPhosphorColors().User {
		t.Errorf("user color = %q, want amber phosphor %q", updated.cfg.Colors.User, config.AmberPhosphorColors().User)
	}
}
