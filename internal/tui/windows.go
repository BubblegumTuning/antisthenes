package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	openai "github.com/sashabaranov/go-openai"
)

const maxChatWindows = 9

// telegramWindowIndex is slot 2 (Ctrl/Alt+2): configured instant messenger.
const telegramWindowIndex = 1

// ChatWindow is one irssi-style conversation buffer backed by its own session.
// Each window may host at most one /iterative flow (state machine + worker).
type ChatWindow struct {
	Label             string
	SessionID         string
	Messages          []openai.ChatCompletionMessage
	PersistedMsgCount int
	Nudges            []string
	LastNotification  string
	InputHistory      []string
	HistoryIndex      int
	InputDraft        string
	GatewayChatID     string

	// Per-window iterative job (idle when unused).
	iterState           IterativeState
	iterCtx             IterativeContext
	iterGen             int                // invalidates late result/progress after cancel
	iterCancel          context.CancelFunc // cancels in-flight iterative worker
	iterLogOffset       int64              // byte offset into work log for progress tail
	iterProgressSnippet string             // last log line for thinking/status row
}

// GatewayInboundMsg delivers a platform message into the TUI (window 2).
type GatewayInboundMsg struct {
	Platform string
	ChatID   string
	UserID   string
	Text     string
}

// GatewayReplyFunc sends an outbound message to the configured messenger.
type GatewayReplyFunc func(chatID, text string) error

func (m *Model) activeWin() *ChatWindow {
	return &m.windows[m.activeWindow]
}

// iterWin returns the chat window for an iterative job index (clamped to active on bad index).
func (m *Model) iterWin(i int) *ChatWindow {
	if i < 0 || i >= maxChatWindows {
		return m.activeWin()
	}
	return &m.windows[i]
}

// appendMessageTo appends a chat message to a specific window.
func (m *Model) appendMessageTo(winIdx int, msg openai.ChatCompletionMessage) {
	if winIdx < 0 || winIdx >= maxChatWindows {
		winIdx = m.activeWindow
	}
	w := &m.windows[winIdx]
	w.Messages = append(w.Messages, msg)
}

func (m Model) windowOccupied(i int) bool {
	if i < 0 || i >= maxChatWindows {
		return false
	}
	return m.windows[i].SessionID != ""
}

func windowSwitchSlot(msg tea.KeyMsg) int {
	// Irssi uses Meta/Alt+1..9; standard terminals do not emit distinct Ctrl+digit codes.
	if msg.Type == tea.KeyRunes && msg.Alt && len(msg.Runes) == 1 {
		r := msg.Runes[0]
		if r >= '1' && r <= '9' {
			return int(r - '1')
		}
	}
	return -1
}

func (m *Model) switchToWindow(index int) {
	if index < 0 || index >= maxChatWindows {
		return
	}
	if !m.windowOccupied(index) {
		return
	}

	cur := m.activeWin()
	cur.InputDraft = m.textInput.Value()

	m.activeWindow = index
	w := m.activeWin()
	m.textInput.SetValue(w.InputDraft)
	m.textInput.Focus()

	m.viewport.SetContent(m.renderChat())
	if m.cfg.AutoScroll {
		m.viewport.GotoBottom()
	}
	return
}

func (m *Model) spawnNewSession() bool {
	for i := 2; i < maxChatWindows; i++ {
		if m.windows[i].SessionID != "" {
			continue
		}
		if m.store == nil {
			return false
		}
		sid, err := m.store.CreateSession()
		if err != nil {
			return false
		}
		short := sid
		if len(short) > 8 {
			short = short[:8]
		}
		m.windows[i] = ChatWindow{
			Label:     fmt.Sprintf("session-%d", i+1),
			SessionID: sid,
		}
		m.copySharedInputHistory(&m.windows[i])
		m.windows[i].Messages = append(m.windows[i].Messages, openai.ChatCompletionMessage{
			Role:    "assistant",
			Content: fmt.Sprintf("New session %s (window %d). Resume later with: ./antisthenes --resume %s", short, i+1, sid),
		})
		m.windows[i].PersistedMsgCount = 0
		m.persistWindowMessages(i)

		cur := m.activeWin()
		cur.InputDraft = m.textInput.Value()
		m.activeWindow = i
		m.textInput.Reset()
		m.textInput.Focus()
		m.viewport.SetContent(m.renderChat())
		m.viewport.GotoBottom()
		return true
	}
	w := m.activeWin()
	w.Messages = append(w.Messages, openai.ChatCompletionMessage{
		Role:    "assistant",
		Content: "No free windows (slots 3–9 are in use). Clear a window or quit an unused session first.",
	})
	m.persistNewMessages()
	m.viewport.SetContent(m.renderChat())
	return false
}

func (m *Model) loadWindowFromStore(index int) {
	if m.store == nil || index < 0 || index >= maxChatWindows {
		return
	}
	w := &m.windows[index]
	if w.SessionID == "" {
		return
	}
	records, err := m.store.LoadChatMessages(w.SessionID)
	if err == nil && len(records) > 0 {
		msgs := make([]openai.ChatCompletionMessage, 0, len(records))
		for _, rec := range records {
			msg := openai.ChatCompletionMessage{
				Role:    rec.Role,
				Content: rec.Content,
			}
			if rec.ToolCallID != "" {
				msg.ToolCallID = rec.ToolCallID
			}
			msgs = append(msgs, msg)
		}
		w.Messages = msgs
		w.PersistedMsgCount = len(msgs)
	}
	if index != telegramWindowIndex {
		if title, err := m.store.GetSessionTitle(w.SessionID); err == nil && strings.TrimSpace(title) != "" {
			w.Label = title
		}
	}
	if nudges, err := m.store.GetRecentNudges(w.SessionID, 5); err == nil {
		w.Nudges = nudges
	}
}

func (m *Model) persistWindowMessages(index int) {
	if m.store == nil || index < 0 || index >= maxChatWindows {
		return
	}
	w := &m.windows[index]
	if w.SessionID == "" || w.PersistedMsgCount >= len(w.Messages) {
		return
	}
	for i := w.PersistedMsgCount; i < len(w.Messages); i++ {
		msg := w.Messages[i]
		_ = m.store.AddChatMessage(w.SessionID, msg.Role, msg.Content, msg.ToolCallID)
	}
	w.PersistedMsgCount = len(w.Messages)
}

func (m *Model) refreshWindowNudges(index int) {
	if m.store == nil || index < 0 || index >= maxChatWindows {
		return
	}
	w := &m.windows[index]
	if w.SessionID == "" {
		return
	}
	if nudges, err := m.store.GetRecentNudges(w.SessionID, 5); err == nil {
		w.Nudges = nudges
	}
}

func (m Model) windowBarPlain(maxWidth int) string {
	var parts []string
	for i := 0; i < maxChatWindows; i++ {
		slot := fmt.Sprintf("%d", i+1)
		if !m.windowOccupied(i) {
			parts = append(parts, slot+":_")
			continue
		}
		w := m.windows[i]
		label := w.Label
		if label == "" {
			label = w.SessionID
			if len(label) > 8 {
				label = label[:8]
			}
		}
		label = truncateRunes(label, 16)
		text := slot + ":" + label
		if i == m.activeWindow {
			text = "[" + text + "]"
		}
		parts = append(parts, text)
	}
	return truncateDisplayWidth(strings.Join(parts, "  "), maxWidth)
}

func (m Model) renderWindowBarLine(width int, p palette) string {
	if width >= 72 {
		return renderFixedStyledLine(m.renderWindowBar(), width, 1)
	}
	return renderFixedSlot(m.windowBarPlain(width), width, 1, p.dim)
}

func (m Model) renderWindowBar() string {
	p := m.palette()
	var parts []string
	for i := 0; i < maxChatWindows; i++ {
		slot := fmt.Sprintf("%d", i+1)
		if !m.windowOccupied(i) {
			parts = append(parts, p.windowEmpty.Render(slot+":_"))
			continue
		}
		w := m.windows[i]
		label := w.Label
		if label == "" {
			label = w.SessionID
			if len(label) > 8 {
				label = label[:8]
			}
		}
		label = truncateRunes(label, 16)
		text := slot + ":" + label
		if i == m.activeWindow {
			parts = append(parts, p.windowActive.Render("["+text+"]"))
		} else {
			parts = append(parts, p.windowInactive.Render(text))
		}
	}
	return strings.Join(parts, "  ")
}

func extractLastAssistant(messages []openai.ChatCompletionMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "assistant" && strings.TrimSpace(messages[i].Content) != "" {
			return strings.TrimSpace(messages[i].Content)
		}
	}
	return "(no response)"
}
