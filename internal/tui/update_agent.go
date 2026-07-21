package tui

import (
	"fmt"
	"os"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// handleResponseMsg applies an agent result. The returned bool requests a full
// terminal repaint when the visible thinking row was active (clears ghost borders).
func (m *Model) handleResponseMsg(msg responseMsg) bool {
	fmt.Fprintf(os.Stderr, "[AGENT] response received window %d (err=%v)\n", msg.windowIndex+1, msg.err)
	repaint := m.thinking && m.thinkingWindow == m.activeWindow
	winIdx := msg.windowIndex
	if winIdx < 0 || winIdx >= maxChatWindows {
		winIdx = m.activeWindow
	}
	w := &m.windows[winIdx]
	m.thinking = false
	if msg.err != nil {
		if winIdx == m.activeWindow {
			m.lastError = msg.err.Error()
		}
	} else {
		w.Messages = msg.messages
		m.onIterativeAgentResponse(winIdx)
		m.persistWindowMessages(winIdx)
		m.refreshWindowNudges(winIdx)
		if winIdx == telegramWindowIndex && m.gatewayReply != nil && w.GatewayChatID != "" {
			reply := extractLastAssistant(w.Messages)
			chatID := w.GatewayChatID
			if fn := m.gatewayReply; fn != nil {
				go func() { _ = fn(chatID, reply) }()
			}
			preview := reply
			if len(preview) > 60 {
				preview = preview[:60] + "..."
			}
			w.LastNotification = "Gateway -> " + preview
		}
	}
	dumpWin := m.pendingDumpWindow
	if m.pendingDumpSummary && dumpWin == winIdx && len(w.Messages) > 0 {
		last := w.Messages[len(w.Messages)-1]
		if last.Role == "assistant" {
			filename := strings.TrimSpace(last.Content)
			if winIdx == m.activeWindow {
				m.clearSessionMemory()
			} else {
				if m.store != nil && w.SessionID != "" {
					_ = m.store.ClearSessionMessages(w.SessionID)
				}
				w.Messages = nil
				w.PersistedMsgCount = 0
			}
			continuation := "The previous work summary has been written to " + filename + ". Please read that file and continue the task from where we left off."
			w.Messages = append(w.Messages, openai.ChatCompletionMessage{Role: "user", Content: continuation})
			m.persistWindowMessages(winIdx)
		}
		m.pendingDumpSummary = false
	}

	if winIdx == m.activeWindow {
		if repaint {
			m.resetViewportHeight()
		}
		m.syncViewport()
		if m.ready {
			m.clearTextInput()
		}
	}
	return repaint
}
