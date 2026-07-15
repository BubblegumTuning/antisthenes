package tui

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	minViewportHeight = 5
	statusRowLines    = 3
	// statusRowBorderedContentLines is lipgloss content height for NormalBorder (adds 2 chrome lines).
	statusRowBorderedContentLines = 1
	// RoundedBorder on the input lipgloss style adds two lines not counted in edit_height.
	inputChromeLines = 2
)

// layoutReservedLines returns fixed chrome height excluding the edit box (phase 7 resize hardening).
func layoutReservedLines() int {
	// title, window bar, blank after chat, separator, thinking/status row, token bar, input border.
	return 1 + 1 + 1 + 1 + statusRowLines + 1 + inputChromeLines
}

// layoutReservedLinesWithTmux adds the chat-area tmux pane when Phase 3 pane is enabled.
// The pane splits the text region above the thinking/status chrome (not under the status bar).
func (m *Model) layoutReservedLinesWithTmux() int {
	n := layoutReservedLines()
	if m.tmuxPaneVisible() {
		n += m.tmuxChromeLines()
	}
	return n
}

func clampEditHeight(requested, terminalHeight int) int {
	editH := requested
	if editH <= 0 {
		editH = 3
	}
	fixed := layoutReservedLines()
	maxEdit := terminalHeight - fixed - minViewportHeight
	if maxEdit < 1 {
		maxEdit = 1
	}
	if editH > maxEdit {
		editH = maxEdit
	}
	return editH
}

func inputBoxWidth(terminalWidth int) int {
	w := terminalWidth - 6
	if w < 10 {
		w = 10
	}
	return w
}

// truncateStatusPair shortens status texts for narrow terminals; right side yields first (DESIGN-TUI.md).
func truncateStatusPair(width int, left, right string) (string, string) {
	if width < 1 {
		width = 1
	}
	lw := lipgloss.Width(left)
	rw := lipgloss.Width(right)
	if lw+rw <= width {
		return left, right
	}
	if right != "" {
		availRight := width - lw
		if availRight < 0 {
			availRight = 0
		}
		if rw > availRight {
			right = truncateDisplayWidth(right, availRight)
			rw = lipgloss.Width(right)
		}
	}
	if lw+rw > width {
		availLeft := width - rw
		if availLeft < 1 {
			availLeft = 1
		}
		if lw > availLeft {
			left = truncateDisplayWidth(left, availLeft)
		}
	}
	return left, right
}

func truncateDisplayWidth(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	if maxWidth <= 1 {
		return "…"
	}
	if maxWidth <= 3 {
		return strings.Repeat(".", maxWidth)
	}
	target := maxWidth - 3
	var b strings.Builder
	used := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if used+rw > target {
			break
		}
		b.WriteRune(r)
		used += rw
	}
	if b.Len() == 0 && len(s) > 0 {
		_, size := utf8.DecodeRuneInString(s)
		return s[:size] + "..."
	}
	return b.String() + "..."
}

func (m *Model) handleWindowSize(msg tea.WindowSizeMsg) {
	// Height calc per DESIGN-TUI.md: use config-driven edit_height + fixed regions accounting (title, blanks, separator, status/thinking row, stats, padding, input box).
	// AltScreen in use for clean redraws.
	m.width = msg.Width
	m.height = msg.Height
	editH := clampEditHeight(m.cfg.EditHeight, msg.Height)
	vH := msg.Height - (m.layoutReservedLinesWithTmux() + editH)
	if vH < 1 {
		vH = 1
	}
	boxW := inputBoxWidth(msg.Width)
	if !m.ready {
		m.viewport = viewport.New(msg.Width, vH)
		m.ready = true
	} else {
		m.viewport.Width = msg.Width
		m.viewport.Height = vH
	}
	m.textInput.SetWidth(boxW)
	m.textInput.SetHeight(editH)
	if m.tmuxPaneVisible() {
		m.ensureTmuxViewport()
	}
	m.fitViewportForTerminal()
	m.viewport.SetContent(m.renderChat())
	if m.cfg.AutoScroll {
		m.viewport.GotoBottom()
	}
}

// resetViewportHeight restores the chat viewport to its layout target.
// View() can shrink the viewport while fitting content; without a WindowSizeMsg
// that height is never restored, which leaves ghost thinking chrome on screen.
func (m *Model) resetViewportHeight() {
	if m.height <= 0 {
		return
	}
	editH := clampEditHeight(m.cfg.EditHeight, m.height)
	vH := m.height - (m.layoutReservedLinesWithTmux() + editH)
	if vH < 1 {
		vH = 1
	}
	m.viewport.Height = vH
	m.fitViewportForTerminal()
}
