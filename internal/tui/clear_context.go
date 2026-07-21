package tui

import (
	"fmt"

	"github.com/nanami/antisthenes/config"
)

func (m *Model) executeClearContext() {
	m.clearSessionMemory()
	m.viewport.SetContent(m.renderChat())
}

func (m *Model) persistClearWithoutConfirm() {
	m.cfg.ClearWithoutConfirm = true
	w := m.activeWin()
	if err := config.Save(m.cfg); err != nil {
		w.LastNotification = fmt.Sprintf("Clear always enabled (could not save config.json: %v)", err)
		return
	}
	w.LastNotification = "Clear always enabled — set clear_without_confirm to false in config.json to restore prompts"
}
