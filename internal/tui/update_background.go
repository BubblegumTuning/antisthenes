package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	openai "github.com/sashabaranov/go-openai"
)

func (m *Model) handleGatewayInbound(msg GatewayInboundMsg) tea.Cmd {
	w := &m.windows[telegramWindowIndex]
	if w.SessionID == "" {
		return nil
	}
	w.GatewayChatID = msg.ChatID
	content := msg.Text
	if msg.UserID != "" {
		content = fmt.Sprintf("[%s] %s", msg.UserID, msg.Text)
	}
	w.Messages = append(w.Messages, openai.ChatCompletionMessage{Role: "user", Content: content})
	m.persistWindowMessages(telegramWindowIndex)
	preview := msg.Text
	if len(preview) > 60 {
		preview = preview[:60] + "..."
	}
	w.LastNotification = fmt.Sprintf("Telegram <- %s", preview)
	if m.store != nil {
		_ = m.store.AddNudge(w.SessionID, "gateway", w.LastNotification)
		m.refreshWindowNudges(telegramWindowIndex)
	}
	winIdx := telegramWindowIndex
	repaint := m.beginThinking(winIdx)
	if m.activeWindow == telegramWindowIndex {
		m.viewport.SetContent(m.renderChat())
		if m.cfg.AutoScroll {
			m.viewport.GotoBottom()
		}
	}
	return agentStartBatch(m, winIdx, repaint)
}

func (m *Model) handleCronResult(msg CronResultMsg) {
	w := &m.windows[0]
	w.LastNotification = msg.Text
	if m.store != nil && w.SessionID != "" {
		_ = m.store.AddNudge(w.SessionID, "cron", msg.Text)
		m.refreshWindowNudges(0)
	}
}

func (m *Model) handleGatewayNotify(msg GatewayMsg) {
	winIdx := telegramWindowIndex
	if m.windows[winIdx].SessionID == "" {
		winIdx = 0
	}
	w := &m.windows[winIdx]
	w.LastNotification = msg.Text
	if m.store != nil && w.SessionID != "" {
		_ = m.store.AddNudge(w.SessionID, "gateway", msg.Text)
		m.refreshWindowNudges(winIdx)
	}
}

func (m *Model) handleSpinnerTick() (tea.Model, tea.Cmd) {
	if m.thinking {
		m.spinnerFrame = (m.spinnerFrame + 1) % len(OrbitFrames)
		return m, m.spinnerTick()
	}
	return m, nil
}
