package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/lipgloss"
)

// newTextInput builds a textarea whose Enter key submits via handleKeyMsg, not newline.
// Alt+Enter inserts a newline (DESIGN-TUI.md multi-line input).
func newTextInput() textarea.Model {
	ta := textarea.New()
	ta.Placeholder = "Type a message..."
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline = key.NewBinding(key.WithKeys("alt+enter"))
	// Avoid full-line background highlight; it makes the box appear to resize on first input.
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ta.Focus()
	return ta
}

func (m *Model) inputBoxWidth() int {
	return inputBoxWidth(m.width)
}

func (m *Model) inputEditHeight() int {
	editH := m.textInput.Height()
	if editH <= 0 {
		editH = m.cfg.EditHeight
		if editH <= 0 {
			editH = 3
		}
	}
	return editH
}

func blankInputInner(width, lines int) string {
	if lines < 1 {
		lines = 1
	}
	if width < 1 {
		width = 1
	}
	line := strings.Repeat(" ", width)
	if lines == 1 {
		return line
	}
	rows := make([]string, lines)
	for i := range rows {
		rows[i] = line
	}
	return strings.Join(rows, "\n")
}

// padInputInner expands textarea output to exactly lines rows so the lipgloss
// border always paints over the same number of terminal cells.
func padInputInner(inner string, lines, width int) string {
	if lines < 1 {
		lines = 1
	}
	if width < 1 {
		width = 1
	}
	blank := strings.Repeat(" ", width)
	split := strings.Split(inner, "\n")
	if n := len(split); n > 0 && split[n-1] == "" {
		split = split[:n-1]
	}
	out := make([]string, lines)
	for i := 0; i < lines; i++ {
		if i < len(split) {
			line := split[i]
			if lipgloss.Width(line) > width {
				line = truncateDisplayWidth(line, width)
			} else if pad := width - lipgloss.Width(line); pad > 0 {
				line += strings.Repeat(" ", pad)
			}
			out[i] = line
			continue
		}
		out[i] = blank
	}
	return strings.Join(out, "\n")
}

func (m *Model) renderInputInner(editH, width int, p palette) string {
	if m.thinking && m.thinkingWindow == m.activeWindow {
		return blankInputInner(width, editH)
	}
	return padInputInner(m.textInput.View(), editH, width)
}

func (m *Model) renderInputBox(p palette) string {
	editH := m.inputEditHeight()
	w := m.inputBoxWidth()
	inner := m.renderInputInner(editH, w, p)
	return p.input.Width(w).Height(editH).Render(inner)
}

func (m *Model) clearTextInput() {
	w := m.inputBoxWidth()
	h := m.textInput.Height()
	if h <= 0 {
		h = 3
		if m.cfg.EditHeight > 0 {
			h = m.cfg.EditHeight
		}
	}
	m.textInput = newTextInput()
	m.textInput.SetWidth(w)
	m.textInput.SetHeight(h)
}
