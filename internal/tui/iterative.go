package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	openai "github.com/sashabaranov/go-openai"
)

// IterativeState drives the /iterative autonomous build flow (DESIGN.md state machine).
type IterativeState int

const (
	IterIdle IterativeState = iota
	IterPlanning
	IterAwaitingConfirmation
	IterExecuting
	IterCompleted
	IterCancelled
)

func (s IterativeState) String() string {
	switch s {
	case IterIdle:
		return "idle"
	case IterPlanning:
		return "planning"
	case IterAwaitingConfirmation:
		return "awaiting_confirmation"
	case IterExecuting:
		return "executing"
	case IterCompleted:
		return "completed"
	case IterCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

// IterativeContext holds per-flow data across state transitions.
type IterativeContext struct {
	ProjectName string
	TargetDir   string
	Goal        string
}

func isConfirmation(input string) bool {
	lower := strings.ToLower(strings.TrimSpace(input))
	if lower == "confirmed" || lower == "approved" {
		return true
	}
	return strings.Contains(lower, "confirmed") || strings.Contains(lower, "approved")
}

func isCancellation(input string) bool {
	lower := strings.ToLower(strings.TrimSpace(input))
	return lower == "cancel" || lower == "cancelled" || lower == "abort"
}

func isPlanReady(content string) bool {
	lower := strings.ToLower(content)
	for _, ind := range []string{
		"reply with 'confirmed'",
		"reply with \"confirmed\"",
		"say 'confirmed'",
		"awaiting your confirmation",
		"awaiting confirmation",
		"please confirm",
		"ready to proceed",
		"definition of done",
	} {
		if strings.Contains(lower, ind) {
			return true
		}
	}
	return false
}

func sanitizeProjectName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "..", "")
	return name
}

func (m *Model) resetIterativeState() {
	m.iterState = IterIdle
	m.iterCtx = IterativeContext{}
}

func (m *Model) cancelIterative() tea.Cmd {
	m.iterState = IterCancelled
	m.appendMessage(openai.ChatCompletionMessage{
		Role:    "assistant",
		Content: "[Iterative build cancelled]",
	})
	m.persistNewMessages()
	m.resetIterativeState()
	m.viewport.SetContent(m.renderChat())
	return nil
}

// handleIterativeSlash starts or rejects /iterative (idle → planning).
func (m *Model) handleIterativeSlash() tea.Cmd {
	if m.iterState != IterIdle {
		m.appendMessage(openai.ChatCompletionMessage{
			Role:    "assistant",
			Content: fmt.Sprintf("Iterative build already active (state: %s). Say 'cancel' to abort first.", m.iterState),
		})
		m.persistNewMessages()
		m.viewport.SetContent(m.renderChat())
		return nil
	}
	debugLog(m.cfg, "ITER: /iterative invoked — state idle → planning")
	m.iterState = IterPlanning
	m.iterCtx = IterativeContext{}
	m.appendMessage(openai.ChatCompletionMessage{
		Role:    "assistant",
		Content: "What would you like to name this project? (This will become the folder name)",
	})
	m.persistNewMessages()
	m.viewport.SetContent(m.renderChat())
	return nil
}

// handleIterativeInput processes user input while an iterative flow is active.
// Returns handled=true when the input was consumed by the state machine.
func (m *Model) handleIterativeInput(input string) (tea.Cmd, bool) {
	if m.iterState == IterIdle {
		return nil, false
	}

	if isCancellation(input) {
		return m.cancelIterative(), true
	}

	// Collect project name before entering agent-driven planning.
	if m.iterState == IterPlanning && m.iterCtx.ProjectName == "" {
		projectName := sanitizeProjectName(input)
		if projectName == "" {
			m.appendMessage(openai.ChatCompletionMessage{
				Role:    "assistant",
				Content: "Project name cannot be empty.",
			})
			m.persistNewMessages()
			m.viewport.SetContent(m.renderChat())
			return nil, true
		}
		m.iterCtx.ProjectName = projectName
		m.iterCtx.TargetDir = projectName
		m.iterCtx.Goal = input
		debugLog(m.cfg, "ITER: project %q — planning conversation started", projectName)
		instruction := fmt.Sprintf(
			"You are now in ITERATIVE PLANNING MODE for project '%s' (working directory: %s). "+
				"Ask clarifying questions until you have high confidence, then present a plan and wait for the user to reply 'confirmed'.",
			projectName, projectName,
		)
		m.appendMessage(openai.ChatCompletionMessage{Role: "system", Content: instruction})
		repaint := m.beginThinking(m.activeWindow)
		m.viewport.SetContent(m.renderChat())
		return agentStartBatch(m, m.activeWindow, repaint), true
	}

	if m.iterState == IterPlanning || m.iterState == IterAwaitingConfirmation {
		if isConfirmation(input) {
			return m.completeIterativeExecution(), true
		}
		// Normal planning message — track latest user content as goal candidate.
		m.iterCtx.Goal = input
		return nil, false
	}

	return nil, false
}

func (m *Model) completeIterativeExecution() tea.Cmd {
	m.iterState = IterExecuting
	debugLog(m.cfg, "ITER: user confirmed — state executing")

	w := m.activeWin()
	goal := m.iterCtx.Goal
	if goal == "" {
		goal = extractIterativeGoal(w.Messages)
	}
	targetDir := m.iterCtx.TargetDir
	if targetDir == "" {
		targetDir = "."
	}

	result := startIterativeBuild(goal, targetDir, *m)
	m.iterState = IterCompleted

	m.clearSessionMemory()
	m.appendMessage(openai.ChatCompletionMessage{
		Role:    "assistant",
		Content: result,
	})
	m.persistNewMessages()
	m.resetIterativeState()
	m.viewport.SetContent(m.renderChat())
	return nil
}

func extractIterativeGoal(messages []openai.ChatCompletionMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" && messages[i].Content != "" {
			content := messages[i].Content
			if isConfirmation(content) || isCancellation(content) {
				continue
			}
			return content
		}
	}
	return "User requested iterative build"
}

// onIterativeAgentResponse advances planning → awaiting_confirmation when the agent presents a plan.
func (m *Model) onIterativeAgentResponse() {
	if m.iterState != IterPlanning {
		return
	}
	w := m.activeWin()
	for i := len(w.Messages) - 1; i >= 0; i-- {
		if w.Messages[i].Role == "assistant" && w.Messages[i].Content != "" {
			if isPlanReady(w.Messages[i].Content) {
				m.iterState = IterAwaitingConfirmation
				debugLog(m.cfg, "ITER: plan ready — state awaiting_confirmation")
			}
			return
		}
	}
}

// showIterativeStatus returns a status line for the thinking row during iterative flows.
func (m Model) showIterativeStatus() string {
	if m.iterState == IterIdle {
		return ""
	}
	return fmt.Sprintf("Iterative: %s", m.iterState)
}
