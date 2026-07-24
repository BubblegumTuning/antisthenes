package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	ctx "github.com/nanami/antisthenes/internal/context"
	openai "github.com/sashabaranov/go-openai"
)

// contentWidth is the full terminal content width for non-bordered chrome
// (status text, blank status rows, separators' companion slots).
func contentWidth(m Model) int {
	w := m.width
	if w < 20 {
		w = 76
	}
	return w
}

// borderedContentWidth is lipgloss Width for NormalBorder/RoundedBorder boxes.
// Borders sit outside Width (+2 cols), so term-2 hugs both edges.
func borderedContentWidth(m Model) int {
	w := contentWidth(m) - 2
	if w < 10 {
		w = 10
	}
	return w
}

// viewDisplayLines counts terminal rows after lipgloss block heights are applied.
func viewDisplayLines(s string) int {
	n := 0
	for _, line := range strings.Split(s, "\n") {
		h := lipgloss.Height(line)
		if h < 1 {
			h = 1
		}
		n += h
	}
	return n
}

// chromeLineBudget is the non-viewport line count: fixed chrome + textarea inner lines.
func chromeLineBudget(editH int) int {
	if editH <= 0 {
		editH = 3
	}
	return layoutReservedLines() + editH
}

// measureStaticChromeLines renders every non-viewport region to count real row usage.
func (m *Model) measureStaticChromeLines() int {
	if m.height <= 0 || m.width <= 0 {
		return chromeLineBudget(3)
	}
	p := m.palette()
	barWidth := m.width
	if barWidth < 20 {
		barWidth = 80
	}
	sepWidth := m.width
	if sepWidth < 1 {
		sepWidth = 1
	}
	sep := strings.Repeat("─", sepWidth)
	parts := []string{
		renderFixedSlot("Antisthenes", barWidth, 1, p.title),
		m.renderWindowBarLine(barWidth, p),
	}
	// Match View(): tmux sits under chat viewport, above thinking/status chrome.
	if m.tmuxPaneVisible() {
		parts = append(parts, m.renderTmuxPane(p))
	}
	parts = append(parts,
		"",
		sep,
		m.renderStatusRowSlot(p),
		renderStatusBarSlot(p.status.Render("status"), barWidth),
		m.renderInputBox(p),
	)
	chrome := strings.Join(parts, "\n")
	return viewDisplayLines(chrome)
}

// fitViewportForTerminal shrinks the chat viewport until chrome + viewport fits m.height.
func (m *Model) fitViewportForTerminal() {
	if m.height <= 0 {
		return
	}
	for m.viewport.Height > 0 && m.measureStaticChromeLines()+m.viewport.Height > m.height {
		m.viewport.Height--
	}
	if m.viewport.Height < 0 {
		m.viewport.Height = 0
	}
}

// padViewToHeight appends blank rows so AltScreen redraws never leave ghost chrome behind.
func padViewToHeight(s string, height int) string {
	for viewDisplayLines(s) < height {
		s += "\n"
	}
	return s
}

// fitViewToTerminal pads or clips the frame to exactly height rows and width columns.
// AltScreen keeps prior frames in scrollback when output overflows; clipping prevents
// duplicate frozen status bars and other ghost chrome.
func fitViewToTerminal(s string, width, height int) string {
	if height < 1 {
		return s
	}
	if width < 1 {
		width = 1
	}
	s = padViewToHeight(s, height)
	return lipgloss.Place(width, height, lipgloss.Left, lipgloss.Top, s)
}

// renderFixedStyledLine constrains already-styled content to an exact terminal row count.
func renderFixedStyledLine(content string, width, height int) string {
	return lipgloss.NewStyle().Width(width).Height(height).Render(content)
}

// renderFixedSlot renders exactly height terminal rows; embedded newlines are flattened.
func renderFixedSlot(content string, width, height int, style lipgloss.Style) string {
	content = strings.ReplaceAll(strings.TrimSpace(content), "\n", " ")
	if width > 0 && lipgloss.Width(content) > width {
		content = truncateDisplayWidth(content, width)
	}
	return style.Width(width).Height(height).Render(content)
}

// renderStatusRowBlank fills the status slot with width-stable empty rows so
// bordered thinking chrome is fully overwritten when the spinner clears.
func renderStatusRowBlank(width int) string {
	line := lipgloss.NewStyle().Width(width).MaxHeight(1).Render("")
	lines := make([]string, statusRowLines)
	for i := range lines {
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}

// renderStatusRowSlot renders the dedicated thinking/status area as a fixed 3-line block.
func (m *Model) renderStatusRowSlot(p palette) string {
	w := contentWidth(*m)

	if m.thinking && m.thinkingWindow == m.activeWindow {
		// Bordered lipgloss styles use Height for content rows only; 1 content + 2 border = statusRowLines.
		// Width is inner; borders add +2 so borderedContentWidth hugs the terminal edges.
		return renderFixedSlot(OrbitFrames[m.spinnerFrame]+m.iterativeThinkingLabel(), borderedContentWidth(*m), statusRowBorderedContentLines, p.thinkingBox)
	}
	if m.lastError != "" {
		return renderFixedSlot("Error: "+m.lastError, w, statusRowLines, p.errorStyle)
	}
	if iterStatus := m.showIterativeStatus(); iterStatus != "" {
		return renderFixedSlot(iterStatus, w, statusRowLines, p.status)
	}
	if plain := m.statusRowAuxPlain(m.activeWin()); plain != "" {
		return renderFixedSlot(plain, w, statusRowLines, m.statusRowAuxStyle(p))
	}
	return renderStatusRowBlank(w)
}

// renderStatusBarSlot renders the split token/time bar as exactly one terminal row.
func renderStatusBarSlot(bar string, width int) string {
	return lipgloss.NewStyle().Width(width).Height(1).Render(bar)
}

// statusRowAuxPlain picks slash hints, compression warnings, and nudges for the thinking row.
func (m *Model) statusRowAuxPlain(w *ChatWindow) string {
	slotW := contentWidth(*m)
	if val := m.textInput.Value(); len(val) > 0 && val[0] == '/' {
		return formatSlashHintSlot(val, slotW)
	}
	if m.shouldWarnCompression(w) {
		return truncateDisplayWidth(compressionWarningText, slotW)
	}
	if len(w.Nudges) > 0 {
		return truncateDisplayWidth("Nudges: "+strings.Join(w.Nudges, " | "), slotW)
	}
	return ""
}

func (m *Model) statusRowAuxStyle(p palette) lipgloss.Style {
	w := m.activeWin()
	if val := m.textInput.Value(); len(val) > 0 && val[0] == '/' {
		return p.nudge
	}
	if m.shouldWarnCompression(w) {
		return p.compression
	}
	return p.nudge
}

func (m *Model) shouldWarnCompression(w *ChatWindow) bool {
	var tools []openai.Tool
	sys := ""
	maxTok := m.cfg.MaxTokens
	if maxTok == 0 {
		maxTok = 160000
	}
	if m.loop != nil {
		if reg := m.loop.Registry(); reg != nil {
			tools = reg.ToOpenAITools()
		}
		if b := m.loop.Builder(); b != nil {
			sys = b.SystemPrompt
			if b.MaxTokens > 0 {
				maxTok = b.MaxTokens
			}
		}
	}
	// Prefer live estimate of current full request; if API last prompt is higher, use that.
	used := ctx.EstimateRequestTokens(sys, w.Messages, tools)
	if m.loop != nil {
		if lp := m.loop.Usage().LastPromptTokens; lp > used {
			used = lp
		}
	}
	threshold := int(float64(maxTok) * 0.75)
	return used > threshold
}
