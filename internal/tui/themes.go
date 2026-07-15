package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nanami/antisthenes/config"
)

func (m *Model) handleThemeSlash(input string) (tea.Cmd, bool) {
	name := strings.TrimSpace(strings.TrimPrefix(input, "/theme"))
	w := m.activeWin()

	if name == "" {
		names := config.ThemeNames()
		w.LastNotification = "Themes: " + strings.Join(names, ", ") + " — usage: /theme <name>"
		m.textInput.Reset()
		return nil, true
	}

	colors, ok := config.ThemeColors(strings.ToLower(name))
	if !ok {
		w.LastNotification = fmt.Sprintf("Unknown theme %q — available: %s", name, strings.Join(config.ThemeNames(), ", "))
		m.textInput.Reset()
		return nil, true
	}

	m.cfg.Colors = colors
	if err := config.Save(m.cfg); err != nil {
		w.LastNotification = fmt.Sprintf("Theme %q applied (could not save config.json: %v)", name, err)
	} else {
		w.LastNotification = fmt.Sprintf("Theme %q applied", name)
	}
	m.textInput.Reset()
	return nil, true
}
