package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	openai "github.com/sashabaranov/go-openai"
)

func (m Model) formatToolsListing() string {
	if m.loop == nil || m.loop.Registry() == nil {
		return "Tools unavailable (no registry)."
	}
	tools := m.loop.Registry().ToOpenAITools()
	if len(tools) == 0 {
		return "No tools registered."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Registered tools (%d):\n", len(tools))
	for _, t := range tools {
		if t.Function == nil {
			continue
		}
		desc := strings.TrimSpace(t.Function.Description)
		if desc == "" {
			desc = "(no description)"
		}
		// Keep lines readable in the chat viewport.
		if len(desc) > 120 {
			desc = desc[:117] + "..."
		}
		fmt.Fprintf(&b, "  • %s — %s\n", t.Function.Name, desc)
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m *Model) handleToolsSlash() (tea.Cmd, bool) {
	m.appendMessage(openai.ChatCompletionMessage{
		Role:    "assistant",
		Content: m.formatToolsListing(),
	})
	m.persistNewMessages()
	m.viewport.SetContent(m.renderChat())
	m.viewport.GotoBottom()
	m.textInput.Reset()
	return nil, true
}
