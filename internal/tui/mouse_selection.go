package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// selPos is a cell within the chat viewport content (absolute line + display column).
type selPos struct {
	line int
	col  int
}

// chatViewportTopRow is the first terminal row of the chat viewport (0-based).
// View layout: title (1) + window bar (1) + viewport...
func chatViewportTopRow() int { return 2 }

func orderSel(a, b selPos) (selPos, selPos) {
	if a.line < b.line || (a.line == b.line && a.col <= b.col) {
		return a, b
	}
	return b, a
}

func (m *Model) selectionActive() bool {
	return m.selSelecting || m.selHasRange
}

func (m *Model) clearSelection() {
	was := m.selectionActive()
	m.selSelecting = false
	m.selHasRange = false
	m.selAnchor = selPos{}
	m.selEnd = selPos{}
	if was {
		m.refreshViewportContent()
	}
}

// refreshViewportContent reloads chat into the viewport, applying selection highlight when active.
func (m *Model) refreshViewportContent() {
	base := m.renderChat()
	if m.selectionActive() {
		base = applySelectionHighlight(base, m.selAnchor, m.selEnd)
	}
	off := m.viewport.YOffset
	m.viewport.SetContent(base)
	m.viewport.SetYOffset(off)
}

func applySelectionHighlight(content string, a, b selPos) string {
	a, b = orderSel(a, b)
	if a.line == b.line && a.col == b.col {
		return content
	}
	lines := strings.Split(content, "\n")
	rev := lipgloss.NewStyle().Reverse(true)
	for i := a.line; i <= b.line && i < len(lines); i++ {
		plainW := ansi.StringWidth(ansi.Strip(lines[i]))
		start, end := 0, plainW
		if i == a.line {
			start = a.col
		}
		if i == b.line {
			end = b.col
		}
		if start < 0 {
			start = 0
		}
		if end > plainW {
			end = plainW
		}
		if start >= end {
			continue
		}
		lines[i] = lipgloss.StyleRanges(lines[i], lipgloss.NewRange(start, end, rev))
	}
	return strings.Join(lines, "\n")
}

func extractSelectionText(content string, a, b selPos) string {
	a, b = orderSel(a, b)
	if a.line == b.line && a.col == b.col {
		return ""
	}
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return ""
	}
	if a.line < 0 {
		a.line = 0
	}
	if a.line >= len(lines) {
		return ""
	}
	if b.line >= len(lines) {
		b.line = len(lines) - 1
		b.col = ansi.StringWidth(ansi.Strip(lines[b.line]))
	}
	var parts []string
	for i := a.line; i <= b.line; i++ {
		plain := ansi.Strip(lines[i])
		plainW := ansi.StringWidth(plain)
		start, end := 0, plainW
		if i == a.line {
			start = a.col
		}
		if i == b.line {
			end = b.col
		}
		if start < 0 {
			start = 0
		}
		if end > plainW {
			end = plainW
		}
		if start > end {
			start, end = end, start
		}
		if start >= end {
			parts = append(parts, "")
			continue
		}
		parts = append(parts, ansi.Cut(plain, start, end))
	}
	return strings.Join(parts, "\n")
}

func (m *Model) clampSelPos(p selPos, content string) selPos {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return selPos{}
	}
	if p.line < 0 {
		p.line = 0
	}
	if p.line >= len(lines) {
		p.line = len(lines) - 1
	}
	w := ansi.StringWidth(ansi.Strip(lines[p.line]))
	if p.col < 0 {
		p.col = 0
	}
	if p.col > w {
		p.col = w
	}
	return p
}

// mouseToChatPos maps a terminal mouse event to chat content coordinates.
func (m *Model) mouseToChatPos(msg tea.MouseMsg) (selPos, bool) {
	if m.viewport.Height <= 0 {
		return selPos{}, false
	}
	top := chatViewportTopRow()
	if msg.Y < top || msg.Y >= top+m.viewport.Height {
		return selPos{}, false
	}
	line := m.viewport.YOffset + (msg.Y - top)
	col := msg.X
	if col < 0 {
		col = 0
	}
	content := m.renderChat()
	return m.clampSelPos(selPos{line: line, col: col}, content), true
}

// handleMouseMsg processes wheel scroll and left-drag selection when mouse mode is on.
// Returns handled=true when the event should not fall through to textarea.
func (m *Model) handleMouseMsg(msg tea.MouseMsg) (tea.Cmd, bool) {
	if !m.mouseEnabled {
		return nil, false
	}
	if m.modalActive() {
		return nil, false
	}

	if tea.MouseEvent(msg).IsWheel() {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		// Keep highlight painted if a selection exists while scrolling.
		if m.selectionActive() {
			m.refreshViewportContent()
		}
		return cmd, true
	}

	switch msg.Button {
	case tea.MouseButtonLeft:
		switch msg.Action {
		case tea.MouseActionPress:
			pos, ok := m.mouseToChatPos(msg)
			if !ok {
				// Click outside chat clears any standing highlight.
				m.clearSelection()
				return nil, true
			}
			m.selSelecting = true
			m.selHasRange = true
			m.selAnchor = pos
			m.selEnd = pos
			m.refreshViewportContent()
			return nil, true

		case tea.MouseActionMotion:
			if !m.selSelecting {
				return nil, false
			}
			if pos, ok := m.mouseToChatPos(msg); ok {
				m.selEnd = pos
			} else {
				// Drag outside: clamp to nearest chat edge row.
				m.selEnd = m.clampDragOutside(msg)
			}
			m.refreshViewportContent()
			return nil, true

		case tea.MouseActionRelease:
			if !m.selSelecting {
				return nil, false
			}
			m.selSelecting = false
			if pos, ok := m.mouseToChatPos(msg); ok {
				m.selEnd = pos
			}
			m.finishSelectionCopy()
			return nil, true
		}
	default:
		// Ignore other buttons; do not leak into textarea.
		if msg.Action == tea.MouseActionPress || msg.Action == tea.MouseActionRelease {
			return nil, true
		}
	}
	return nil, false
}

func (m *Model) clampDragOutside(msg tea.MouseMsg) selPos {
	content := m.renderChat()
	top := chatViewportTopRow()
	var screenLine int
	switch {
	case msg.Y < top:
		screenLine = 0
	case msg.Y >= top+m.viewport.Height:
		screenLine = m.viewport.Height - 1
		if screenLine < 0 {
			screenLine = 0
		}
	default:
		screenLine = msg.Y - top
	}
	line := m.viewport.YOffset + screenLine
	col := msg.X
	if col < 0 {
		col = 0
	}
	return m.clampSelPos(selPos{line: line, col: col}, content)
}

func (m *Model) finishSelectionCopy() {
	content := m.renderChat()
	text := extractSelectionText(content, m.selAnchor, m.selEnd)
	text = strings.TrimRight(text, "\n")
	if strings.TrimSpace(text) == "" {
		m.clearSelection()
		return
	}
	// Keep highlight briefly; clear after successful copy paint.
	m.refreshViewportContent()
	dest, err := writeCopyDestination(text)
	w := m.activeWin()
	if err != nil {
		w.LastNotification = "Selection copy failed: " + err.Error()
		m.clearSelection()
		return
	}
	lines := strings.Count(text, "\n") + 1
	switch dest.method {
	case "clipboard":
		w.LastNotification = formatCopyOK(lines, "clipboard")
	case "osc52":
		w.LastNotification = formatCopyOK(lines, "clipboard (terminal/SSH)")
	default:
		w.LastNotification = formatCopyFile(lines, dest.path)
	}
	// Drop highlight so subsequent views are clean; text is already on clipboard.
	m.clearSelection()
}

func formatCopyOK(lines int, dest string) string {
	if lines == 1 {
		return "Copied selection to " + dest
	}
	return fmt.Sprintf("Copied selection (%d lines) to %s", lines, dest)
}

func formatCopyFile(lines int, path string) string {
	if lines == 1 {
		return "Saved selection to " + path
	}
	return fmt.Sprintf("Saved selection (%d lines) to %s", lines, path)
}

func (m *Model) handleMouseSlash(input string) (tea.Cmd, bool) {
	switch strings.TrimSpace(input) {
	case "/mouse", "/mouse status":
		state := "off"
		if m.mouseEnabled {
			state = "on"
		}
		m.activeWin().LastNotification = "Mouse mode: " + state + " (wheel scroll + drag-copy when on; native select when off)"
		m.textInput.Reset()
		return nil, true
	case "/mouse on":
		m.mouseEnabled = true
		m.activeWin().LastNotification = "Mouse on: wheel scrolls chat; drag selects and copies"
		m.textInput.Reset()
		return tea.EnableMouseCellMotion, true
	case "/mouse off":
		m.mouseEnabled = false
		m.clearSelection()
		m.activeWin().LastNotification = "Mouse off: keyboard scroll; terminal native drag-select"
		m.textInput.Reset()
		return tea.DisableMouse, true
	default:
		if strings.HasPrefix(input, "/mouse ") {
			m.activeWin().LastNotification = "Usage: /mouse on | /mouse off"
			m.textInput.Reset()
			return nil, true
		}
		return nil, false
	}
}
