package tui

import (
	openai "github.com/sashabaranov/go-openai"
)

// loadSessionFromStore restores the active window from SQLite (compat wrapper).
func (m *Model) loadSessionFromStore() {
	m.loadWindowFromStore(m.activeWindow)
}

// persistNewMessages writes messages added since the last persist checkpoint for the active window.
func (m *Model) persistNewMessages() {
	m.persistWindowMessages(m.activeWindow)
}

// clearSessionMemory wipes in-memory and persisted messages for the active window.
func (m *Model) clearSessionMemory() {
	w := m.activeWin()
	if m.store != nil && w.SessionID != "" {
		_ = m.store.ClearSessionMessages(w.SessionID)
	}
	w.Messages = nil
	w.PersistedMsgCount = 0
}

// refreshNudges reloads recent nudges for the active window.
func (m *Model) refreshNudges() {
	m.refreshWindowNudges(m.activeWindow)
}

// repersistAllMessages replaces stored session history with current in-memory messages.
func (m *Model) repersistAllMessages() {
	w := m.activeWin()
	if m.store == nil || w.SessionID == "" {
		return
	}
	_ = m.store.ClearSessionMessages(w.SessionID)
	w.PersistedMsgCount = 0
	m.persistNewMessages()
}

// appendMessage appends a chat message to the active window.
func (m *Model) appendMessage(msg openai.ChatCompletionMessage) {
	w := m.activeWin()
	w.Messages = append(w.Messages, msg)
}

// syncViewport refreshes scrollable chat content from the active window messages.
func (m *Model) syncViewport() {
	m.viewport.SetContent(m.renderChat())
	if m.cfg.AutoScroll {
		m.viewport.GotoBottom()
	}
}
