package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/cellbuf"
	openai "github.com/sashabaranov/go-openai"
)

func (m Model) agentLabel() string {
	name := strings.TrimSpace(m.cfg.AgentName)
	if name == "" {
		name = "Antisthenes"
	}
	return name + ": "
}

func (m Model) renderChat() string {
	p := m.palette()
	w := m.activeWin()
	if len(w.Messages) == 0 {
		return p.emptyChat.Render("No messages yet. Type a message to begin.")
	}

	var parts []string
	for _, msg := range w.Messages {
		if block := m.renderMessageBlock(msg, p); block != "" {
			parts = append(parts, block)
		}
	}
	return strings.Join(parts, "\n")
}

func (m Model) chatWrapWidth() int {
	w := m.viewport.Width
	if w > 0 {
		return w
	}
	if m.width > 4 {
		return m.width
	}
	return 76
}

func (m Model) renderMessageBlock(msg openai.ChatCompletionMessage, p palette) string {
	width := m.chatWrapWidth()
	var lines []string
	switch msg.Role {
	case "user":
		lines = append(lines, m.prefixStyledLabel(p.user, "You: ", msg.Content, width, false))
	case "assistant":
		if msg.Content != "" {
			lines = append(lines, m.prefixStyledLabel(p.assistant, m.agentLabel(), msg.Content, width, true))
		}
		for _, tc := range msg.ToolCalls {
			args := tc.Function.Arguments
			if !m.cfg.ShowFullToolDumps && len(args) > 80 {
				args = args[:77] + "..."
			}
			toolCall := p.toolCall.Render(fmt.Sprintf("→ Tool: %s(%s)", tc.Function.Name, args))
			lines = append(lines, wrap(toolCall, width))
		}
		if m.showThinking && msg.ReasoningContent != "" {
			text := p.thinkingBox.Width(width).Render(
				p.thinkingText.Render("Thinking: " + msg.ReasoningContent))
			lines = append(lines, text)
		}
	case "tool":
		content := msg.Content
		if !m.cfg.ShowFullToolDumps && len(content) > 200 {
			content = content[:197] + "..."
		}
		status := toolResultStatus(msg.Content)
		toolResult := p.toolResult.Render(fmt.Sprintf("← %s: %s [%s]", msg.ToolCallID, content, status))
		lines = append(lines, wrap(toolResult, width))
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

// toolResultStatus classifies tool output for [OK]/[ERROR] display (DESIGN-TUI.md phase 8).
func toolResultStatus(content string) string {
	lower := strings.ToLower(strings.TrimSpace(content))
	if strings.HasPrefix(lower, "error:") {
		return "ERROR"
	}
	if strings.Contains(lower, "denied") || strings.Contains(lower, "failed") || strings.Contains(lower, " error") {
		return "ERROR"
	}
	return "OK"
}

// prefixStyledLabel wraps body text, then prepends a styled role label on the
// first line only. Assistant bodies may use inline markdown when enabled.
func (m Model) prefixStyledLabel(style lipgloss.Style, label, body string, width int, markdown bool) string {
	var wrapped string
	if markdown && m.markdownEnabled() {
		wrapped = renderInlineMarkdown(body, width)
	} else {
		wrapped = wrap(body, width)
	}
	if wrapped == "" {
		return style.Render(label)
	}
	lines := strings.Split(wrapped, "\n")
	lines[0] = style.Render(label) + lines[0]
	return strings.Join(lines, "\n")
}

// wrap word-wraps chat text without lipgloss width padding. Padding each line to
// full width caused the viewport to reflow and visually duplicate lines.
func wrap(text string, width int) string {
	if width <= 4 {
		return text
	}
	return cellbuf.Wrap(text, width, "")
}
