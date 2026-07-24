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

// prepareIterativeScaffold writes structured design/DoD/log files derived from
// the planning conversation (last assistant plan + user requirements) and returns paths.
func prepareIterativeScaffold(goal, targetDir string, messages []openai.ChatCompletionMessage) (designFile, dodFile, logFile string, err error) {
	fmt.Fprintf(os.Stderr, "[ITER] prepareIterativeScaffold | goal=%q target=%s\n", goal, targetDir)
	timestamp := time.Now().Format("20060102-150405")
	slug := scaffoldSlug(goal, 30, "project")
	designFile, dodFile, logFile = scaffoldFilePaths(targetDir, slug)

	planBody := lastAssistantPlanMessage(messages)
	notes := planningNotesFromMessages(messages)
	reqs := extractRequirementBullets(goal, planBody, messages)
	approach := extractPlanSection(planBody, []string{"approach", "proposed approach", "plan", "implementation", "steps"})
	if strings.TrimSpace(approach) == "" {
		approach = stripPlanFooter(planBody)
	}
	if strings.TrimSpace(approach) == "" {
		approach = "Implement the goal iteratively inside the target directory; re-read design and DoD each major cycle."
	}
	risks := extractPlanSection(planBody, []string{"risks", "risk", "concerns", "tradeoffs", "trade-offs"})
	if strings.TrimSpace(risks) == "" {
		risks = "- Scope drift from the confirmed plan — re-read design and DoD each cycle.\n- Missing edge cases in planning — expand DoD if new acceptance criteria appear."
	}
	dodItems := extractDoDItems(goal, planBody, reqs)

	designContent := formatIterativeDesignContent(timestamp, goal, targetDir, reqs, approach, risks, notes)
	dodContent := formatIterativeDoDContent(timestamp, goal, dodItems)
	logContent := fmt.Sprintf("[%s] Iterative build started (async)\nGoal: %s\nTarget: %s\nDesign: %s\nDoD: %s\n",
		time.Now().Format(time.RFC3339), goal, targetDir, designFile, dodFile)

	if err := writeScaffoldFiles(targetDir, designFile, dodFile, logFile, designContent, dodContent, logContent); err != nil {
		return "", "", "", err
	}
	return designFile, dodFile, logFile, nil
}

// lastAssistantPlanMessage returns the best assistant plan text from planning turns.
// Prefers the latest plan-ready message; falls back to the latest substantive assistant reply.
func lastAssistantPlanMessage(messages []openai.ChatCompletionMessage) string {
	var lastPlan, lastAny string
	for _, msg := range messages {
		if msg.Role != "assistant" {
			continue
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		if skipPlanningScaffoldMessage(content) {
			continue
		}
		lastAny = content
		if isPlanReady(content) {
			lastPlan = content
		}
	}
	if lastPlan != "" {
		return lastPlan
	}
	return lastAny
}

func skipPlanningScaffoldMessage(content string) bool {
	if strings.HasPrefix(content, "[Iterative") || strings.Contains(content, "SHIM_BRIEF") {
		return true
	}
	if strings.Contains(content, "name this project") || strings.Contains(content, "Run in supervised mode?") {
		return true
	}
	if strings.HasPrefix(content, "Sub-agent ") {
		return true
	}
	return false
}

// extractRequirementBullets gathers requirements from plan sections and user turns.
func extractRequirementBullets(goal, planBody string, messages []openai.ChatCompletionMessage) []string {
	var out []string
	seen := map[string]bool{}
	add := func(item string) {
		item = strings.TrimSpace(item)
		item = strings.TrimLeft(item, "-*•")
		item = strings.TrimSpace(item)
		// strip checkbox markers
		item = strings.TrimPrefix(item, "[ ]")
		item = strings.TrimPrefix(item, "[x]")
		item = strings.TrimPrefix(item, "[X]")
		item = strings.TrimSpace(item)
		if item == "" {
			return
		}
		key := strings.ToLower(item)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, item)
	}

	sec := extractPlanSection(planBody, []string{
		"requirements", "clarified requirements", "requirement", "scope", "goals", "objectives",
	})
	for _, line := range strings.Split(sec, "\n") {
		if bulletBody, ok := parseBulletLine(line); ok {
			add(bulletBody)
		}
	}
	// Numbered steps under requirements-like sections already captured; also pull top-level bullets from plan if section empty.
	if len(out) == 0 {
		for _, line := range strings.Split(planBody, "\n") {
			if bulletBody, ok := parseBulletLine(line); ok {
				// skip confirmation boilerplate bullets
				lower := strings.ToLower(bulletBody)
				if strings.Contains(lower, "confirm") || strings.Contains(lower, "reply with") {
					continue
				}
				add(bulletBody)
			}
		}
	}
	for _, msg := range messages {
		if msg.Role != "user" {
			continue
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" || isConfirmation(content) || isCancellation(content) {
			continue
		}
		if _, ok := parseSupervisedChoice(content); ok && isTrivialChoice(content) {
			continue
		}
		if _, ok := parseExecutorChoice(content); ok {
			continue
		}
		// Multi-line user specs: take bullets; else whole message if short enough.
		lines := strings.Split(content, "\n")
		gotBullet := false
		for _, line := range lines {
			if bulletBody, ok := parseBulletLine(line); ok {
				add(bulletBody)
				gotBullet = true
			}
		}
		if !gotBullet && len(content) <= 400 {
			add(content)
		}
	}
	if len(out) == 0 && strings.TrimSpace(goal) != "" {
		add(goal)
	}
	return out
}

func isTrivialChoice(content string) bool {
	c := strings.TrimSpace(strings.ToLower(content))
	switch c {
	case "", "y", "n", "yes", "no":
		return true
	default:
		return false
	}
}

func parseBulletLine(line string) (string, bool) {
	s := strings.TrimSpace(line)
	if s == "" {
		return "", false
	}
	switch {
	case strings.HasPrefix(s, "- "), strings.HasPrefix(s, "* "), strings.HasPrefix(s, "• "):
		return strings.TrimSpace(s[2:]), true
	case strings.HasPrefix(s, "-"), strings.HasPrefix(s, "*"), strings.HasPrefix(s, "•"):
		if len(s) > 1 && (s[1] == ' ' || s[1] == '[' || s[1] == '	') {
			return strings.TrimSpace(s[1:]), true
		}
	}
	// numbered list: "1. item" or "1) item"
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i > 0 && i < len(s) && (s[i] == '.' || s[i] == ')') {
		rest := strings.TrimSpace(s[i+1:])
		if rest != "" {
			return rest, true
		}
	}
	return "", false
}

// extractDoDItems pulls success criteria from the plan; falls back to requirements + defaults.
func extractDoDItems(goal, planBody string, requirements []string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(item string) {
		item = strings.TrimSpace(item)
		item = strings.TrimLeft(item, "-*•")
		item = strings.TrimSpace(item)
		for _, p := range []string{"[ ]", "[x]", "[X]"} {
			item = strings.TrimPrefix(item, p)
			item = strings.TrimSpace(item)
		}
		if item == "" {
			return
		}
		key := strings.ToLower(item)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, item)
	}

	sec := extractPlanSection(planBody, []string{
		"definition of done", "success criteria", "acceptance criteria", "done when", "dod",
	})
	for _, line := range strings.Split(sec, "\n") {
		if body, ok := parseBulletLine(line); ok {
			add(body)
		} else if strings.Contains(line, "[ ]") || strings.Contains(line, "[x]") || strings.Contains(line, "[X]") {
			// bare checkbox line without list marker
			trimmed := strings.TrimSpace(line)
			for _, p := range []string{"- [ ]", "- [x]", "- [X]", "* [ ]", "[ ]", "[x]", "[X]"} {
				if strings.HasPrefix(strings.ToLower(trimmed), strings.ToLower(p)) || strings.HasPrefix(trimmed, p) {
					add(strings.TrimSpace(trimmed[len(p):]))
					break
				}
			}
		}
	}
	// Also harvest any checkbox lines anywhere in the plan.
	if len(out) == 0 {
		for _, line := range strings.Split(planBody, "\n") {
			trimmed := strings.TrimSpace(line)
			lower := strings.ToLower(trimmed)
			if strings.Contains(lower, "[ ]") || strings.HasPrefix(lower, "- [x]") {
				if body, ok := parseBulletLine(trimmed); ok {
					add(body)
				}
			}
		}
	}
	if len(out) == 0 {
		for _, r := range requirements {
			add(r)
		}
	}
	// Stable baseline criteria for the worker.
	add("The project builds cleanly (or equivalent verify step for the stack)")
	add("All stated requirements in the design document are met")
	add("Work log shows completion of the main objective")
	return out
}

// extractPlanSection returns body text under the first matching markdown/plain heading.
func extractPlanSection(planBody string, headings []string) string {
	if strings.TrimSpace(planBody) == "" || len(headings) == 0 {
		return ""
	}
	lines := strings.Split(planBody, "\n")
	want := map[string]bool{}
	for _, h := range headings {
		want[strings.ToLower(strings.TrimSpace(h))] = true
	}
	var b strings.Builder
	capturing := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if headingTitle, ok := parseMarkdownHeading(trimmed); ok {
			if want[strings.ToLower(headingTitle)] {
				capturing = true
				b.Reset()
				continue
			}
			if capturing {
				// next heading ends section
				break
			}
			continue
		}
		// Bold single-line section labels: **Requirements**
		if title, ok := parseBoldLabel(trimmed); ok {
			if want[strings.ToLower(title)] {
				capturing = true
				b.Reset()
				continue
			}
			if capturing {
				break
			}
		}
		if capturing {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return strings.TrimSpace(b.String())
}

func parseMarkdownHeading(line string) (string, bool) {
	s := strings.TrimSpace(line)
	if !strings.HasPrefix(s, "#") {
		return "", false
	}
	i := 0
	for i < len(s) && s[i] == '#' {
		i++
	}
	if i == 0 || i > 6 {
		return "", false
	}
	if i < len(s) && s[i] != ' ' && s[i] != '	' {
		return "", false
	}
	title := strings.TrimSpace(s[i:])
	title = strings.Trim(title, "#")
	title = strings.TrimSpace(title)
	// drop trailing colon
	title = strings.TrimSuffix(title, ":")
	if title == "" {
		return "", false
	}
	return title, true
}

func parseBoldLabel(line string) (string, bool) {
	s := strings.TrimSpace(line)
	if !strings.HasPrefix(s, "**") {
		return "", false
	}
	rest := s[2:]
	end := strings.Index(rest, "**")
	if end <= 0 {
		return "", false
	}
	title := strings.TrimSpace(rest[:end])
	title = strings.TrimSuffix(title, ":")
	after := strings.TrimSpace(rest[end+2:])
	// only treat as a section label when little/no body on the same line
	if after != "" && after != ":" {
		return "", false
	}
	if title == "" {
		return "", false
	}
	return title, true
}

func stripPlanFooter(planBody string) string {
	if strings.TrimSpace(planBody) == "" {
		return ""
	}
	lines := strings.Split(planBody, "\n")
	// drop trailing confirmation boilerplate lines
	for len(lines) > 0 {
		last := strings.TrimSpace(strings.ToLower(lines[len(lines)-1]))
		if last == "" ||
			strings.Contains(last, "confirm") ||
			strings.Contains(last, "ready to proceed") ||
			strings.Contains(last, "awaiting") ||
			strings.Contains(last, "reply with") {
			lines = lines[:len(lines)-1]
			continue
		}
		break
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func formatIterativeDesignContent(timestamp, goal, targetDir string, requirements []string, approach, risks, notes string) string {
	if strings.TrimSpace(notes) == "" {
		notes = "(No planning transcript captured.)"
	}
	var reqBlock strings.Builder
	if len(requirements) == 0 {
		reqBlock.WriteString("- (none extracted — follow the goal and planning transcript)\n")
	} else {
		for _, r := range requirements {
			fmt.Fprintf(&reqBlock, "- %s\n", r)
		}
	}
	return fmt.Sprintf(`# Design Document

**Generated**: %s
**Goal**: %s
**Target Directory**: %s

## Goal

%s

## Requirements

%s
## Approach

%s

## Risks

%s

## Planning Transcript

%s
`, timestamp, goal, targetDir, goal, reqBlock.String(), strings.TrimSpace(approach), strings.TrimSpace(risks), strings.TrimSpace(notes))
}

func formatIterativeDoDContent(timestamp, goal string, items []string) string {
	var b strings.Builder
	if len(items) == 0 {
		b.WriteString("- [ ] The project builds cleanly\n")
		b.WriteString("- [ ] All stated requirements from the design document are met\n")
		b.WriteString("- [ ] Work log shows completion of the main objective\n")
	} else {
		for _, item := range items {
			fmt.Fprintf(&b, "- [ ] %s\n", item)
		}
	}
	return fmt.Sprintf(`# Definition of Done

**Generated**: %s
**Goal**: %s

## Success Criteria

%s`, timestamp, goal, b.String())
}

// buildIterativeWorkerGoal builds the sub-agent seed prompt using config thresholds.
func buildIterativeWorkerGoal(cfg config.Config, targetDir, designFile, dodFile, logFile string) string {
	remind := cfg.IterativeContextRemindPercent()
	summary := cfg.IterativeContextSummaryPercent()
	maxIter := cfg.IterativeMaxIterations()
	return fmt.Sprintf(`AUTONOMOUS ITERATIVE BUILD WORKER

You have been given the following files inside %s:
- %s (design document)
- %s (definition of done)
- %s (work log - append timestamped entries)

Instructions:
1. Read all three files at the start of every major cycle.
2. Work exclusively inside the directory: %s
3. After every significant action (build, edit, test), append a timestamped entry to the work log describing what was done and the result.
4. If context usage exceeds ~%d%%, write a concise summary of progress to the work log, then continue.
5. If context usage exceeds ~%d%%, force a fuller summary into the work log and treat the next cycle as a fresh planning refresh before more edits.
6. Stop only when the Definition of Done is fully satisfied or you have made %d iterations.
7. Use the bash, write_file, patch, and search_files tools as needed.

Begin now.`, targetDir, designFile, dodFile, logFile, targetDir, remind, summary, maxIter)
}

func joinIterPath(dir, name string) string {
	return filepath.Join(dir, name)
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
