package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	osc52 "github.com/aymanbagabas/go-osc52/v2"
	tea "github.com/charmbracelet/bubbletea"
	openai "github.com/sashabaranov/go-openai"
)

func (m Model) plainChatText() string {
	w := m.activeWin()
	if len(w.Messages) == 0 {
		return ""
	}
	var parts []string
	for _, msg := range w.Messages {
		if block := m.plainMessageBlock(msg); block != "" {
			parts = append(parts, block)
		}
	}
	return strings.Join(parts, "\n\n")
}

func (m Model) plainMessageBlock(msg openai.ChatCompletionMessage) string {
	var lines []string
	switch msg.Role {
	case "user":
		lines = append(lines, "You: "+msg.Content)
	case "assistant":
		if msg.Content != "" {
			lines = append(lines, m.agentLabel()+msg.Content)
		}
		for _, tc := range msg.ToolCalls {
			args := tc.Function.Arguments
			if !m.cfg.ShowFullToolDumps && len(args) > 80 {
				args = args[:77] + "..."
			}
			lines = append(lines, fmt.Sprintf("→ Tool: %s(%s)", tc.Function.Name, args))
		}
		if m.showThinking && msg.ReasoningContent != "" {
			lines = append(lines, "Thinking: "+msg.ReasoningContent)
		}
	case "tool":
		content := msg.Content
		if !m.cfg.ShowFullToolDumps && len(content) > 200 {
			content = content[:197] + "..."
		}
		status := toolResultStatus(msg.Content)
		lines = append(lines, fmt.Sprintf("← %s: %s [%s]", msg.ToolCallID, content, status))
	case "system":
		if strings.TrimSpace(msg.Content) != "" {
			lines = append(lines, "[system] "+msg.Content)
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

func (m Model) plainVisibleChatText() string {
	full := m.plainChatText()
	if full == "" {
		return ""
	}
	lines := strings.Split(full, "\n")
	start := m.viewport.YOffset
	if start >= len(lines) {
		return ""
	}
	end := start + m.viewport.Height
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[start:end], "\n")
}

type copyDestination struct {
	method string // "clipboard" or "file"
	path   string // set when method == "file"
}

func emitOSC52(text string) {
	seq := osc52.New(text)
	switch {
	case os.Getenv("TMUX") != "":
		seq = seq.Tmux()
	case os.Getenv("STY") != "":
		seq = seq.Screen()
	}
	_, _ = fmt.Fprint(os.Stderr, seq)
}

func writeCopyFile(text string) (string, error) {
	name := fmt.Sprintf("antisthenes-copy-%s.txt", time.Now().Format("20060102-150405"))
	path := filepath.Join(os.TempDir(), name)
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func writeCopyDestination(text string) (copyDestination, error) {
	if err := clipboard.WriteAll(text); err == nil {
		return copyDestination{method: "clipboard"}, nil
	}
	emitOSC52(text)
	path, err := writeCopyFile(text)
	if err != nil {
		return copyDestination{}, fmt.Errorf("clipboard unavailable and file write failed: %w", err)
	}
	return copyDestination{method: "file", path: path}, nil
}

func (m *Model) copyChat(scope string) {
	var text string
	switch scope {
	case "visible":
		text = m.plainVisibleChatText()
	default:
		text = m.plainChatText()
	}
	w := m.activeWin()
	if text == "" {
		w.LastNotification = "Copy: no chat content"
		return
	}
	dest, err := writeCopyDestination(text)
	if err != nil {
		w.LastNotification = fmt.Sprintf("Copy failed: %v", err)
		return
	}
	lines := strings.Count(text, "\n") + 1
	switch dest.method {
	case "clipboard":
		if scope == "visible" {
			w.LastNotification = fmt.Sprintf("Copied %d visible lines to clipboard", lines)
		} else {
			w.LastNotification = fmt.Sprintf("Copied chat (%d lines) to clipboard", lines)
		}
	default:
		if scope == "visible" {
			w.LastNotification = fmt.Sprintf("Saved %d visible lines to %s", lines, dest.path)
		} else {
			w.LastNotification = fmt.Sprintf("Saved chat (%d lines) to %s", lines, dest.path)
		}
	}
}

func (m *Model) handleCopySlash(input string) (tea.Cmd, bool) {
	scope := "all"
	switch input {
	case "/copy":
		// full chat
	case "/copy visible":
		scope = "visible"
	default:
		return nil, false
	}
	m.copyChat(scope)
	m.textInput.Reset()
	return nil, true
}

func (m *Model) handleCopyKey(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "ctrl+y", "ctrl+shift+c":
		m.copyChat("all")
		return true
	}
	return false
}
