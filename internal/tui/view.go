package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	ctx "github.com/nanami/antisthenes/internal/context"
)

const compressionWarningText = "⚠ Context high — run: dump_work_summary with a summary of current work"

func (m *Model) View() string {
	if !m.ready {
		return "Initializing TUI..."
	}

	m.fitViewportForTerminal()

	p := m.palette()
	w := m.activeWin()
	barWidth := m.width
	if barWidth < 20 {
		barWidth = 80
	}

	used := ctx.EstimateTokens(w.Messages)
	total := m.cfg.MaxTokens
	if total == 0 {
		total = 160000
	}
	pct := 0
	if total > 0 {
		pct = used * 100 / total
	}

	var usedStr string
	if used < 1000 {
		usedStr = fmt.Sprintf("%d", used)
	} else {
		usedStr = fmt.Sprintf("%dk", used/1000)
	}
	leftPlain := fmt.Sprintf(
		"%s | %d%% %s/%dk tokens | %s",
		m.cfg.GetActiveEndpoint().Model,
		pct,
		usedStr,
		total/1000,
		time.Now().Format("15:04:05"),
	)
	rightPlain := w.LastNotification
	if rightPlain == "" {
		rightPlain = m.tmuxStatusSnippet()
	} else if snip := m.tmuxStatusSnippet(); snip != "" {
		rightPlain = snip + " | " + rightPlain
	}
	statusWidth := m.width
	if statusWidth < 0 {
		statusWidth = 0
	}
	leftPlain, rightPlain = truncateStatusPair(statusWidth, leftPlain, rightPlain)
	leftStatus := p.status.Render(leftPlain)
	rightStatus := p.status.Render(rightPlain)
	leftW := lipgloss.Width(leftStatus)
	rightW := lipgloss.Width(rightStatus)
	pad := statusWidth - leftW - rightW
	if pad < 0 {
		pad = 0
	}
	statusBar := renderStatusBarSlot(leftStatus+strings.Repeat(" ", pad)+rightStatus, barWidth)

	active := m.windows[m.activeWindow]
	headerPlain := fmt.Sprintf("Antisthenes — [%d:%s] %s", m.activeWindow+1, active.Label, active.SessionID)
	header := renderFixedSlot(headerPlain, barWidth, 1, p.title)
	windowBar := m.renderWindowBarLine(barWidth, p)

	var sections []string
	sections = append(sections, header)
	sections = append(sections, windowBar)
	if m.viewport.Height > 0 {
		sections = append(sections, m.viewport.View())
	}
	// Tmux splits the chat text area (above thinking/status), not the chrome under the status bar.
	if m.tmuxPaneVisible() {
		sections = append(sections, m.renderTmuxPane(p))
	}
	sections = append(sections, "")

	sepWidth := m.width
	if sepWidth < 1 {
		sepWidth = 1
	}
	sep := strings.Repeat("─", sepWidth)

	sections = append(sections, sep)

	sections = append(sections, m.renderStatusRowSlot(p))
	sections = append(sections, statusBar)
	sections = append(sections, m.renderInputBox(p))

	base := strings.Join(sections, "\n")
	if viewDisplayLines(base) > m.height && m.viewport.Height > 0 {
		m.viewport.Height--
		return m.View()
	}
	base = fitViewToTerminal(base, m.width, m.height)
	if m.modalActive() {
		return m.renderModalOverlay(base)
	}
	return base
}
