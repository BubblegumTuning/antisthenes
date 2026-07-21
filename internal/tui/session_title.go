package tui

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nanami/antisthenes/internal/agent"
)

// sessionTitleMsg updates a window label after async title generation.
type sessionTitleMsg struct {
	windowIndex int
	sessionID   string
	title       string
}

// maybeScheduleSessionTitle returns a Cmd that titles the session once if still untitled.
// Uses aux model role "title" when configured; otherwise a local heuristic. Never blocks Update.
func (m *Model) maybeScheduleSessionTitle(winIdx int, userText string) tea.Cmd {
	if m.store == nil || winIdx < 0 || winIdx >= maxChatWindows {
		return nil
	}
	w := m.windows[winIdx]
	if w.SessionID == "" || strings.TrimSpace(userText) == "" {
		return nil
	}
	// Telegram keeps a fixed channel label.
	if winIdx == telegramWindowIndex {
		return nil
	}
	existing, err := m.store.GetSessionTitle(w.SessionID)
	if err == nil && strings.TrimSpace(existing) != "" {
		return nil
	}
	// Only title on the first user turn (avoid retitling mid-session after clear mishaps).
	userCount := 0
	for _, msg := range w.Messages {
		if msg.Role == "user" && strings.TrimSpace(msg.Content) != "" {
			userCount++
		}
	}
	if userCount != 1 {
		return nil
	}

	sid := w.SessionID
	cfg := m.cfg
	store := m.store
	text := userText
	idx := winIdx
	return func() tea.Msg {
		title := agent.GenerateSessionTitle(context.Background(), cfg, text)
		if title == "" {
			return nil
		}
		_ = store.SetSessionTitle(sid, title)
		return sessionTitleMsg{windowIndex: idx, sessionID: sid, title: title}
	}
}

func (m *Model) handleSessionTitleMsg(msg sessionTitleMsg) {
	if msg.windowIndex < 0 || msg.windowIndex >= maxChatWindows {
		return
	}
	w := &m.windows[msg.windowIndex]
	if w.SessionID == "" || w.SessionID != msg.sessionID {
		return
	}
	if t := strings.TrimSpace(msg.title); t != "" {
		w.Label = t
	}
}
