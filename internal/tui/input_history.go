package tui

import (
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

func (m *Model) handleClearHistorySlash() (tea.Cmd, bool) {
	w := m.activeWin()
	w.clearInputHistory()
	m.appendMessage(openai.ChatCompletionMessage{
		Role:    "assistant",
		Content: "[Input history cleared for this window]",
	})
	m.persistNewMessages()
	m.viewport.SetContent(m.renderChat())
	m.viewport.GotoBottom()
	m.textInput.Reset()
	return nil, true
}
