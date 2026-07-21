package tui

import (
	"encoding/json"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	openai "github.com/sashabaranov/go-openai"

	"github.com/nanami/antisthenes/config"
)

func (w *ChatWindow) appendInputHistory(input string, cfg config.Config) {
	if !cfg.InputHistoryOn() || input == "" {
		return
	}
	if len(w.InputHistory) > 0 && w.InputHistory[len(w.InputHistory)-1] == input {
		w.HistoryIndex = len(w.InputHistory)
		return
	}
	w.InputHistory = append(w.InputHistory, input)
	max := cfg.InputHistoryMax()
	if len(w.InputHistory) > max {
		w.InputHistory = w.InputHistory[len(w.InputHistory)-max:]
	}
	w.HistoryIndex = len(w.InputHistory)
}

func (w *ChatWindow) clearInputHistory() {
	w.InputHistory = nil
	w.HistoryIndex = 0
}

func (w *ChatWindow) setInputHistory(hist []string) {
	if len(hist) == 0 {
		w.InputHistory = nil
		w.HistoryIndex = 0
		return
	}
	w.InputHistory = append([]string(nil), hist...)
	w.HistoryIndex = len(w.InputHistory)
}

// loadInputHistoryFile reads bash-style JSON history (array of strings). Missing file → empty.
func loadInputHistoryFile(cfg config.Config) []string {
	if !cfg.InputHistoryOn() {
		return nil
	}
	path := cfg.InputHistoryFile()
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return nil
	}
	var hist []string
	if err := json.Unmarshal(data, &hist); err != nil {
		return nil
	}
	return capHistory(hist, cfg.InputHistoryMax())
}

// saveInputHistoryFile writes the capped history list (0600). No-op when history disabled.
func saveInputHistoryFile(cfg config.Config, hist []string) error {
	if !cfg.InputHistoryOn() {
		return nil
	}
	path := cfg.InputHistoryFile()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	hist = capHistory(hist, cfg.InputHistoryMax())
	if hist == nil {
		hist = []string{}
	}
	data, err := json.MarshalIndent(hist, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func capHistory(hist []string, max int) []string {
	if max <= 0 || len(hist) == 0 {
		return hist
	}
	if len(hist) > max {
		return append([]string(nil), hist[len(hist)-max:]...)
	}
	return hist
}

// seedInputHistoryFromDisk loads shared file history into every occupied window.
func (m *Model) seedInputHistoryFromDisk() {
	hist := loadInputHistoryFile(m.cfg)
	for i := range m.windows {
		if i != 0 && m.windows[i].SessionID == "" {
			continue
		}
		m.windows[i].setInputHistory(hist)
	}
}

// recordInputHistory appends to the active window, persists the file, and mirrors
// the list to other occupied windows (shared bash-style history).
func (m *Model) recordInputHistory(input string) {
	w := m.activeWin()
	w.appendInputHistory(input, m.cfg)
	_ = saveInputHistoryFile(m.cfg, w.InputHistory)
	m.mirrorInputHistory(w.InputHistory, m.activeWindow)
}

func (m *Model) mirrorInputHistory(hist []string, except int) {
	for i := range m.windows {
		if i == except {
			continue
		}
		if i != 0 && m.windows[i].SessionID == "" {
			continue
		}
		// Keep browsing index only if still in range; otherwise snap to end.
		idx := m.windows[i].HistoryIndex
		m.windows[i].setInputHistory(hist)
		if idx >= 0 && idx < len(m.windows[i].InputHistory) {
			m.windows[i].HistoryIndex = idx
		}
	}
}

func (m *Model) clearAllInputHistory() {
	for i := range m.windows {
		m.windows[i].clearInputHistory()
	}
	_ = saveInputHistoryFile(m.cfg, nil)
}

func (m *Model) handleClearHistorySlash() (tea.Cmd, bool) {
	m.clearAllInputHistory()
	m.appendMessage(openai.ChatCompletionMessage{
		Role:    "assistant",
		Content: "[Input history cleared]",
	})
	m.persistNewMessages()
	m.viewport.SetContent(m.renderChat())
	m.viewport.GotoBottom()
	m.textInput.Reset()
	return nil, true
}

// copySharedInputHistory seeds a newly spawned window from window 0 (already file-backed).
func (m *Model) copySharedInputHistory(dst *ChatWindow) {
	src := m.windows[0].InputHistory
	if len(src) == 0 {
		// Fallback to disk if window 0 empty but file exists.
		src = loadInputHistoryFile(m.cfg)
	}
	dst.setInputHistory(src)
}
