package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nanami/antisthenes/config"
	"github.com/nanami/antisthenes/internal/agent"
	openai "github.com/sashabaranov/go-openai"
)

// delegateTaskFunc is swappable in tests to avoid live sub-agent calls.
var delegateTaskFunc = agent.DelegateTaskWithConfig

// callAgentCmd starts the agent on a background goroutine so the UI can redraw immediately
// (cleared input, thinking spinner) while RunAgent blocks. Results arrive via NotifyFunc/p.Send.
func (m *Model) callAgentCmd() tea.Cmd {
	return m.callAgentCmdForWindow(m.activeWindow)
}

func (m *Model) callAgentCmdForWindow(winIdx int) tea.Cmd {
	return func() tea.Msg {
		go m.dispatchAgentForWindow(winIdx)
		return nil
	}
}

func (m *Model) dispatchAgentForWindow(winIdx int) {
	msg := m.callAgentForWindow(winIdx)
	if m.notify != nil {
		m.notify(msg)
	}
}

func (m *Model) callAgentForWindow(winIdx int) responseMsg {
	m.wireApprovalHandler()
	w := m.windows[winIdx]
	fmt.Fprintf(os.Stderr, "[AGENT] callAgent window %d: sending %d messages to model\n", winIdx+1, len(w.Messages))
	// Copy so RunStream tool recursion cannot mutate the TUI's slice in place.
	history := append([]openai.ChatCompletionMessage(nil), w.Messages...)
	updated, err := m.loop.RunAgent(context.Background(), history)
	if err != nil {
		return responseMsg{windowIndex: winIdx, messages: history, err: err}
	}
	return responseMsg{windowIndex: winIdx, messages: updated, err: nil}
}

func (m Model) spinnerTick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
}

// beginThinking activates the spinner for winIdx. Returns true when the thinking
// row is visible on the active window; callers should batch tea.ClearScreen to
// flush Bubble Tea's line cache (prevents ghost/duplicated status chrome).
func (m *Model) beginThinking(winIdx int) bool {
	m.thinking = true
	m.thinkingWindow = winIdx
	m.lastError = ""
	if winIdx == m.activeWindow {
		m.resetViewportHeight()
		return true
	}
	return false
}

func agentStartBatch(m *Model, winIdx int, repaint bool) tea.Cmd {
	var agentCmd tea.Cmd
	if winIdx == m.activeWindow {
		agentCmd = m.callAgentCmd()
	} else {
		agentCmd = m.callAgentCmdForWindow(winIdx)
	}
	cmds := []tea.Cmd{agentCmd, m.spinnerTick()}
	if repaint {
		cmds = append([]tea.Cmd{tea.ClearScreen}, cmds...)
	}
	return tea.Batch(cmds...)
}

// buildScaffold holds paths for design, definition-of-done, and work-log files.
type buildScaffold struct {
	DesignFile string
	DodFile    string
	LogFile    string
}

func scaffoldSlug(text string, maxLen int, defaultSlug string) string {
	if len(text) > maxLen {
		return strings.ToLower(strings.ReplaceAll(text[:maxLen], " ", "_"))
	}
	return defaultSlug
}

func scaffoldFilePaths(targetDir, slug string) (designFile, dodFile, logFile string) {
	timestamp := time.Now().Format("20060102-150405")
	designFile = fmt.Sprintf("%s/%s_%s_design.md", targetDir, timestamp, slug)
	dodFile = fmt.Sprintf("%s/%s_%s_definition_of_done.md", targetDir, timestamp, slug)
	logFile = fmt.Sprintf("%s/%s_%s.log", targetDir, timestamp, slug)
	return designFile, dodFile, logFile
}

func writeScaffoldFiles(targetDir string, designFile, dodFile, logFile, designContent, dodContent, logContent string) error {
	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(designFile, []byte(designContent), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(dodFile, []byte(dodContent), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(logFile, []byte(logContent), 0o644); err != nil {
		return err
	}
	return nil
}

func buildFastPathInstruction(designFile, dodFile, logFile string) string {
	return fmt.Sprintf(`Read the following files and proceed to work autonomously:

- %s
- %s
- %s (append timestamped progress here)

Work inside the current directory. After each significant step, update the work log. If context gets high, summarize progress into the log and continue.`, designFile, dodFile, logFile)
}

func writeBuildFastScaffold(task, targetDir string) (buildScaffold, error) {
	slug := scaffoldSlug(task, 25, "build")
	designFile, dodFile, logFile := scaffoldFilePaths(targetDir, slug)
	timestamp := time.Now().Format("20060102-150405")

	designContent := fmt.Sprintf(`# Design Document (from /build)

**Generated**: %s
**Goal**: %s

## Requirements
%s
`, timestamp, task, task)

	dodContent := fmt.Sprintf(`# Definition of Done (from /build)

**Generated**: %s
**Goal**: %s

- Project builds cleanly
- Stated goal is achieved
`, timestamp, task)

	logContent := fmt.Sprintf("[%s] /build started\nGoal: %s\n", time.Now().Format(time.RFC3339), task)

	if err := writeScaffoldFiles(targetDir, designFile, dodFile, logFile, designContent, dodContent, logContent); err != nil {
		return buildScaffold{}, err
	}
	return buildScaffold{DesignFile: designFile, DodFile: dodFile, LogFile: logFile}, nil
}

func startIterativeBuild(goal string, targetDir string, m Model) string {
	fmt.Fprintf(os.Stderr, "[ITER] startIterativeBuild called | goal=%q target=%s\n", goal, targetDir)
	timestamp := time.Now().Format("20060102-150405")
	slug := scaffoldSlug(goal, 30, "project")
	designFile, dodFile, logFile := scaffoldFilePaths(targetDir, slug)

	designContent := fmt.Sprintf(`# Design Document

**Generated**: %s
**Goal**: %s
**Target Directory**: %s

## Clarified Requirements

(Agent should fill this section during planning phase)

## Proposed Approach

(Agent should fill this section)
`, timestamp, goal, targetDir)

	dodContent := fmt.Sprintf(`# Definition of Done

**Generated**: %s
**Goal**: %s

## Success Criteria

- [ ] The project builds cleanly
- [ ] All stated requirements from the design document are met
- [ ] Work log shows completion of the main objective
`, timestamp, goal)

	logContent := fmt.Sprintf("[%s] Iterative build started\nGoal: %s\nTarget: %s\nDesign: %s\nDoD: %s\n",
		time.Now().Format(time.RFC3339), goal, targetDir, designFile, dodFile)

	if err := writeScaffoldFiles(targetDir, designFile, dodFile, logFile, designContent, dodContent, logContent); err != nil {
		return fmt.Sprintf("Failed to write scaffold files: %v", err)
	}

	return startIterativeWorker(goal, targetDir, designFile, dodFile, logFile)
}

func startIterativeWorker(goal, targetDir, designFile, dodFile, logFile string) string {
	workerGoal := fmt.Sprintf(`AUTONOMOUS ITERATIVE BUILD WORKER

You have been given the following files inside %s:
- %s (design document)
- %s (definition of done)
- %s (work log - append timestamped entries)

Instructions:
1. Read all three files at the start of every major cycle.
2. Work exclusively inside the directory: %s
3. After every significant action (build, edit, test), append a timestamped entry to the work log describing what was done and the result.
4. If context usage exceeds ~55%%, write a concise summary of progress to the work log, then continue.
5. Stop only when the Definition of Done is fully satisfied or you have made 40 iterations.
6. Use the bash, write_file, patch, and search_files tools as needed.

Begin now.`, targetDir, designFile, dodFile, logFile, targetDir)

	cfg := agent.DefaultDelegateConfig()
	result := delegateTaskFunc(workerGoal, cfg)

	if result.Error != nil {
		return fmt.Sprintf("Sub-agent failed: %v", result.Error)
	}
	return fmt.Sprintf("Sub-agent completed.\nFinal output:\n%s", result.Result)
}

// debugLog writes timestamped entries to log/debug.log when cfg.DebugLogging is true.
// The log/ folder is created on first use. Suitable for iterative diagnostics and future general logging.
func debugLog(cfg config.Config, format string, a ...interface{}) {
	if !cfg.DebugLogging {
		return
	}
	if err := os.MkdirAll("log", 0o700); err != nil {
		return
	}
	logPath := filepath.Join("log", "debug.log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	ts := time.Now().Format(time.RFC3339)
	fmt.Fprintf(f, "[%s] "+format+"\n", append([]interface{}{ts}, a...)...)
}
