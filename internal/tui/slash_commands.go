package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	openai "github.com/sashabaranov/go-openai"
)

func isClearContextCommand(cmd string) bool {
	return cmd == "/clear" || cmd == "/new"
}

func (m *Model) handleSlashCommand(input string) (tea.Cmd, bool) {
	switch {
	case isClearContextCommand(input):
		if m.cfg.ClearWithoutConfirm {
			m.executeClearContext()
			m.textInput.Reset()
			return nil, true
		}
		m.confirmCommand = input
		m.textInput.Reset()
		return nil, true

	case input == "/new_session":
		m.spawnNewSession()
		m.textInput.Reset()
		return nil, true

	case input == "/clear-history":
		return m.handleClearHistorySlash()

	case input == "/tools":
		return m.handleToolsSlash()

	case input == "/tmux" || strings.HasPrefix(input, "/tmux "):
		return m.handleTmuxSlash(input)

	case input == "/copy", input == "/copy visible":
		return m.handleCopySlash(input)

	case input == "/help":
		help := "Available slash commands:\n" +
			"/clear         - Clear conversation context (y/n; use a/always to skip future prompts)\n" +
			"/new           - Alias for /clear\n" +
			"/compress      - Compress history: keep first message, stub all tool results, retain last 20 messages\n" +
			"/dump-summary  - Ask agent to write a work summary (with file paths) to dump-*.md in /tmp, then auto-clear context and inject reload prompt\n" +
			"/iterative      - Start clarification conversation for a complex autonomous build/design task. Agent will ask questions, propose a plan, then (on confirmation) spawn a sub-agent to execute.\n" +
			"/build <task>  - Enter forced autonomous iterative build mode. Provide goal + definition of done. Runs until DoD met or 40 iterations.\n" +
			"/new_session   - Open a new chat window in the next free slot (3–9)\n" +
			"/theme <name>  - Switch color theme (green, amber)\n" +
			"/clear-history - Wipe Up/Down input history for this window\n" +
			"/copy          - Copy full chat history (clipboard, or /tmp file if unavailable)\n" +
			"/copy visible  - Copy only the lines visible in the chat viewport\n" +
			"/tools         - List registered agent tools (names + descriptions)\n" +
			"/tmux          - Chat-area tmux pane (above thinking): on|off|refresh|host|session|status\n" +
			"/help          - Show this help message\n\n" +
			"Copy shortcuts: Ctrl+Y or Ctrl+Shift+C (full chat).\n\n" +
			"Windows (irssi-style): Alt+1..9 switch buffers. 1=main chat, 2=instant messenger, 3–9=spawned sessions."
		m.appendMessage(openai.ChatCompletionMessage{Role: "assistant", Content: help})
		m.persistNewMessages()
		m.viewport.SetContent(m.renderChat())
		m.viewport.GotoBottom()
		m.textInput.Reset()
		return nil, true

	case input == "/compress":
		w := m.activeWin()
		if len(w.Messages) > 0 {
			compressed := []openai.ChatCompletionMessage{w.Messages[0]}
			for i := 1; i < len(w.Messages); i++ {
				msg := w.Messages[i]
				if msg.Role == "tool" {
					msg.Content = "[tool result stubbed]"
				}
				compressed = append(compressed, msg)
			}
			if len(compressed) > 21 {
				compressed = append(compressed[:1], compressed[len(compressed)-20:]...)
			}
			w.Messages = compressed
		}
		m.appendMessage(openai.ChatCompletionMessage{
			Role:    "assistant",
			Content: "[Context compressed: first kept, tools stubbed, last 20 retained]",
		})
		m.repersistAllMessages()
		m.viewport.SetContent(m.renderChat())
		m.viewport.GotoBottom()
		m.textInput.Reset()
		return nil, true

	case input == "/dump-summary":
		instruction := "Please create a concise summary of all work done so far. For every file you created, modified, or referenced, include its full path. Write the summary to the system temp directory as a file named dump- followed by the current date-time and a short hash (e.g. dump-20250702-1845-a3f2.md). After writing the file, reply with ONLY the filename of the summary you just created."
		m.appendMessage(openai.ChatCompletionMessage{Role: "system", Content: instruction})
		m.pendingDumpSummary = true
		m.pendingDumpWindow = m.activeWindow
		repaint := m.beginThinking(m.activeWindow)
		m.textInput.Reset()
		m.viewport.SetContent(m.renderChat())
		return agentStartBatch(m, m.activeWindow, repaint), true

	case input == "/exit":
		fmt.Printf("To resume this session: ./antisthenes --resume %s\n", m.activeWin().SessionID)
		return tea.Quit, true

	case strings.HasPrefix(input, "/iterative"):
		m.textInput.Reset()
		return m.handleIterativeSlash(), true

	case strings.HasPrefix(input, "/build "):
		return m.handleBuildSlash(input)

	case input == "/theme", strings.HasPrefix(input, "/theme "):
		return m.handleThemeSlash(input)

	default:
		return nil, false
	}
}

func (m *Model) handleBuildSlash(input string) (tea.Cmd, bool) {
	task := strings.TrimSpace(strings.TrimPrefix(input, "/build "))
	if task == "" {
		task = "Build and verify the current project until it compiles and basic tests pass."
	}

	sc, err := writeBuildFastScaffold(task, ".")
	if err != nil {
		m.lastError = err.Error()
		m.textInput.Reset()
		return nil, true
	}

	m.clearSessionMemory()
	instruction := buildFastPathInstruction(sc.DesignFile, sc.DodFile, sc.LogFile)
	m.appendMessage(openai.ChatCompletionMessage{Role: "user", Content: instruction})
	repaint := m.beginThinking(m.activeWindow)
	m.textInput.Reset()
	m.viewport.SetContent(m.renderChat())
	return agentStartBatch(m, m.activeWindow, repaint), true
}
