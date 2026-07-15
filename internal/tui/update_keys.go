package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	openai "github.com/sashabaranov/go-openai"
)

func (m *Model) handleKeyMsg(msg tea.KeyMsg) (tea.Cmd, bool) {
	if cmd, handled := m.handleModalKey(msg); handled {
		return cmd, true
	}
	if slot := windowSwitchSlot(msg); slot >= 0 {
		m.switchToWindow(slot)
		return nil, true
	}
	if handled := m.handleCopyKey(msg); handled {
		return nil, true
	}

	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		return tea.Quit, true
	case tea.KeyUp:
		if !m.cfg.InputHistoryOn() {
			return nil, false
		}
		w := m.activeWin()
		if len(w.InputHistory) > 0 && w.HistoryIndex > 0 {
			w.HistoryIndex--
			m.textInput.SetValue(w.InputHistory[w.HistoryIndex])
		}
		return nil, true
	case tea.KeyDown:
		if !m.cfg.InputHistoryOn() {
			return nil, false
		}
		w := m.activeWin()
		if len(w.InputHistory) > 0 {
			if w.HistoryIndex < len(w.InputHistory)-1 {
				w.HistoryIndex++
				m.textInput.SetValue(w.InputHistory[w.HistoryIndex])
			} else {
				w.HistoryIndex = len(w.InputHistory)
				m.clearTextInput()
			}
		}
		return nil, true
	case tea.KeyEnter:
		return m.handleSubmitKey()
	case tea.KeyRunes:
		if len(msg.Runes) == 1 && (msg.Runes[0] == '\n' || msg.Runes[0] == '\r') {
			return m.handleSubmitKey()
		}
	}
	return nil, false
}

func (m *Model) handleSubmitKey() (tea.Cmd, bool) {
	input := strings.TrimSpace(m.textInput.Value())
	if input == "" {
		m.clearTextInput()
		return nil, true
	}
	if cmd, handled := m.handleSlashCommand(input); handled {
		m.clearTextInput()
		return cmd, true
	}
	if cmd, handled := m.handleIterativeInput(input); handled {
		m.clearTextInput()
		return cmd, true
	}
	repaint := m.submitUserMessage(input)
	return agentStartBatch(m, m.activeWindow, repaint), true
}

func (m *Model) submitUserMessage(input string) bool {
	w := m.activeWin()
	w.Messages = append(w.Messages, openai.ChatCompletionMessage{Role: "user", Content: input})
	w.appendInputHistory(input, m.cfg)
	repaint := m.beginThinking(m.activeWindow)
	m.clearTextInput()
	m.syncViewport()
	return repaint
}
