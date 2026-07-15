package tui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/nanami/antisthenes/config"
)

// palette holds lipgloss styles built from config.TUIColors at render time.
type palette struct {
	input          lipgloss.Style
	title          lipgloss.Style
	nudge          lipgloss.Style
	errorStyle     lipgloss.Style
	thinkingBox    lipgloss.Style
	thinkingText   lipgloss.Style
	status         lipgloss.Style
	compression    lipgloss.Style
	modal          lipgloss.Style
	dim            lipgloss.Style
	user           lipgloss.Style
	assistant      lipgloss.Style
	toolCall       lipgloss.Style
	toolResult     lipgloss.Style
	emptyChat      lipgloss.Style
	windowActive   lipgloss.Style
	windowInactive lipgloss.Style
	windowEmpty    lipgloss.Style
}

func (m Model) palette() palette {
	return buildPalette(m.cfg.Colors)
}

func buildPalette(c config.TUIColors) palette {
	colors := c
	colors.ApplyDefaults()
	return palette{
		input: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colors.InputBorder)).
			Padding(0, 1),
		title: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Title)).
			Bold(true),
		nudge: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Nudge)).
			Italic(true),
		errorStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Error)).
			Bold(true),
		thinkingBox: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(colors.ThinkingBorder)).
			Padding(0, 1),
		thinkingText: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.AssistantThinking)),
		status: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Status)),
		compression: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Compression)),
		modal: lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color(colors.ModalBorder)).
			Padding(1, 2),
		dim: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Dim)),
		user: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.User)),
		assistant: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.Assistant)),
		toolCall: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.ToolCall)),
		toolResult: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.ToolResult)),
		emptyChat: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.EmptyChat)).
			Italic(true),
		windowActive: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.WindowActive)).
			Bold(true),
		windowInactive: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.WindowInactive)),
		windowEmpty: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors.WindowEmpty)),
	}
}
