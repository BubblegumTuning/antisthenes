package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	openai "github.com/sashabaranov/go-openai"

	"github.com/nanami/antisthenes/internal/agent"
)

// IterativeState drives the /iterative autonomous build flow (DESIGN.md state machine).
type IterativeState int

const (
	IterIdle IterativeState = iota
	IterPlanning
	IterAwaitingSupervised // after project name: Run in supervised mode? (y/N)
	IterAwaitingConfirmation
	IterAwaitingExecutor // supervised: wait for executor name after SHIM brief
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
	case IterAwaitingSupervised:
		return "awaiting_supervised"
	case IterAwaitingConfirmation:
		return "awaiting_confirmation"
	case IterAwaitingExecutor:
		return "awaiting_executor"
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
	Supervised  bool
	Executor    string
	DesignFile  string
	DodFile     string
	LogFile     string
	PlanFile    string // PER plan artifact (per_plan.md)
	PlanText    string // last plan phase output (may be truncated for briefs)
}

const supervisedPrompt = "Run in supervised mode? (y/N) — default is N.\n" +
	"Y: after confirmation a Plan phase runs, then you choose an executor for Execute (auto / coder / deep-thinker / orchestrator).\n" +
	"N: Plan→Execute→Review runs autonomously after you reply confirmed (one review retry budget)."

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

// parseSupervisedChoice returns (supervised, ok). Empty / n / no → false; y / yes → true.
func parseSupervisedChoice(input string) (bool, bool) {
	lower := strings.ToLower(strings.TrimSpace(input))
	switch lower {
	case "", "n", "no", "false", "0":
		return false, true
	case "y", "yes", "true", "1":
		return true, true
	default:
		return false, false
	}
}

// parseExecutorChoice accepts "coder", "executor: auto", etc.
func parseExecutorChoice(input string) (string, bool) {
	lower := strings.ToLower(strings.TrimSpace(input))
	lower = strings.TrimSpace(strings.TrimPrefix(lower, "executor:"))
	lower = strings.TrimSpace(strings.TrimPrefix(lower, "executor"))
	if lower == "" {
		return "", false
	}
	// allow multi-word reply: take first token if needed
	fields := strings.Fields(lower)
	if len(fields) == 0 {
		return "", false
	}
	cand := fields[0]
	for _, name := range agent.MVPExecutorNames {
		if cand == name {
			return name, true
		}
	}
	// full string match for deep-thinker if split wrong
	for _, name := range agent.MVPExecutorNames {
		if lower == name {
			return name, true
		}
	}
	return "", false
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

func (w *ChatWindow) resetIterativeState() {
	w.iterState = IterIdle
	w.iterCtx = IterativeContext{}
	w.iterCancel = nil
	w.iterLogOffset = 0
	w.iterProgressSnippet = ""
}

// interruptIterativeWorker cancels an in-flight worker and invalidates late results.
func (m *Model) interruptIterativeWorker(winIdx int) {
	w := m.iterWin(winIdx)
	if w.iterCancel != nil {
		w.iterCancel()
		w.iterCancel = nil
	}
	w.iterGen++
	if m.thinking && m.thinkingWindow == winIdx {
		m.thinking = false
	}
}

func (m *Model) cancelIterative() tea.Cmd {
	winIdx := m.activeWindow
	w := m.iterWin(winIdx)
	wasExecuting := w.iterState == IterExecuting
	if wasExecuting {
		m.interruptIterativeWorker(winIdx)
	}
	w.iterState = IterCancelled
	msg := "[Iterative build cancelled]"
	if wasExecuting {
		msg = "[Iterative build interrupted — worker cancel requested]"
	}
	m.appendMessageTo(winIdx, openai.ChatCompletionMessage{
		Role:    "assistant",
		Content: msg,
	})
	m.persistWindowMessages(winIdx)
	w.resetIterativeState()
	if winIdx == m.activeWindow && m.ready {
		m.resetViewportHeight()
		m.viewport.SetContent(m.renderChat())
	}
	return nil
}

// handleIterativeSlash starts or rejects /iterative (idle → planning) on the active window.
func (m *Model) handleIterativeSlash() tea.Cmd {
	w := m.activeWin()
	if w.iterState != IterIdle {
		m.appendMessage(openai.ChatCompletionMessage{
			Role:    "assistant",
			Content: fmt.Sprintf("Iterative build already active on this window (state: %s). Say 'cancel' to abort first.", w.iterState),
		})
		m.persistNewMessages()
		m.viewport.SetContent(m.renderChat())
		return nil
	}
	debugLog(m.cfg, "ITER: /iterative invoked win=%d — state idle → planning", m.activeWindow)
	w.iterState = IterPlanning
	w.iterCtx = IterativeContext{}
	m.appendMessage(openai.ChatCompletionMessage{
		Role:    "assistant",
		Content: "What would you like to name this project? (This will become the folder name)",
	})
	m.persistNewMessages()
	m.viewport.SetContent(m.renderChat())
	return nil
}

// handleIterativeInput processes user input while the active window's iterative flow is active.
// Returns handled=true when the input was consumed by the state machine.
func (m *Model) handleIterativeInput(input string) (tea.Cmd, bool) {
	winIdx := m.activeWindow
	w := m.iterWin(winIdx)
	if w.iterState == IterIdle {
		return nil, false
	}

	if isCancellation(input) {
		return m.cancelIterative(), true
	}

	// While executing, only cancel is handled; other input is rejected so the
	// main chat does not interleave with the worker.
	if w.iterState == IterExecuting {
		m.appendMessage(openai.ChatCompletionMessage{
			Role:    "assistant",
			Content: "Iterative build is running. Say 'cancel' or press Ctrl+C once to interrupt.",
		})
		m.persistNewMessages()
		m.viewport.SetContent(m.renderChat())
		return nil, true
	}

	// Collect project name, then ask supervised mode (skills/iterative_per).
	if w.iterState == IterPlanning && w.iterCtx.ProjectName == "" {
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
		w.iterCtx.ProjectName = projectName
		w.iterCtx.TargetDir = projectName
		w.iterCtx.Goal = input
		w.iterState = IterAwaitingSupervised
		debugLog(m.cfg, "ITER: win=%d project %q — awaiting supervised choice", winIdx, projectName)
		m.appendMessage(openai.ChatCompletionMessage{
			Role:    "assistant",
			Content: supervisedPrompt,
		})
		m.persistNewMessages()
		m.viewport.SetContent(m.renderChat())
		return nil, true
	}

	if w.iterState == IterAwaitingSupervised {
		sup, ok := parseSupervisedChoice(input)
		if !ok {
			m.appendMessage(openai.ChatCompletionMessage{
				Role:    "assistant",
				Content: "Please reply y or N (default N).",
			})
			m.persistNewMessages()
			m.viewport.SetContent(m.renderChat())
			return nil, true
		}
		w.iterCtx.Supervised = sup
		w.iterState = IterPlanning
		debugLog(m.cfg, "ITER: win=%d supervised=%v — planning conversation started", winIdx, sup)
		modeNote := "unsupervised (Plan→Execute→Review after confirmation)"
		if sup {
			modeNote = "supervised (Plan phase after confirmation, then executor choice before Execute)"
		}
		instruction := fmt.Sprintf(
			"You are now in ITERATIVE PLANNING MODE for project '%s' (working directory: %s). "+
				"Mode: %s. "+
				"Ask clarifying questions until you have high confidence, then present a plan and wait for the user to reply 'confirmed'.",
			w.iterCtx.ProjectName, w.iterCtx.TargetDir, modeNote,
		)
		m.appendMessage(openai.ChatCompletionMessage{Role: "system", Content: instruction})
		m.appendMessage(openai.ChatCompletionMessage{
			Role:    "assistant",
			Content: fmt.Sprintf("Supervised mode: %v. Describe the goal and constraints for project %q.", sup, w.iterCtx.ProjectName),
		})
		m.persistNewMessages()
		repaint := m.beginThinking(winIdx)
		m.viewport.SetContent(m.renderChat())
		// Kick planning with a synthetic user turn so the agent starts asking questions.
		w.Messages = append(w.Messages, openai.ChatCompletionMessage{
			Role:    "user",
			Content: fmt.Sprintf("Project %q. Begin clarifying questions for the iterative build.", w.iterCtx.ProjectName),
		})
		return agentStartBatch(m, winIdx, repaint), true
	}

	if w.iterState == IterAwaitingExecutor {
		exec, ok := parseExecutorChoice(input)
		if !ok {
			m.appendMessage(openai.ChatCompletionMessage{
				Role:    "assistant",
				Content: "Reply with executor: auto / coder / deep-thinker / orchestrator",
			})
			m.persistNewMessages()
			m.viewport.SetContent(m.renderChat())
			return nil, true
		}
		w.iterCtx.Executor = exec
		debugLog(m.cfg, "ITER: win=%d executor=%s — starting Execute+Review", winIdx, exec)
		return m.startIterativePERAsync(winIdx, exec, perModeExecuteReview), true
	}

	if w.iterState == IterPlanning || w.iterState == IterAwaitingConfirmation {
		if isConfirmation(input) {
			return m.completeIterativeExecution(winIdx), true
		}
		// Normal planning message — track latest user content as goal candidate.
		w.iterCtx.Goal = input
		return nil, false
	}

	return nil, false
}

// completeIterativeExecution writes scaffolds then starts multi-phase PER.
// Unsupervised: full Plan→Execute→Review in one async cmd.
// Supervised: Plan phase async first; executor gate after plan result.
func (m *Model) completeIterativeExecution(winIdx int) tea.Cmd {
	w := m.iterWin(winIdx)
	debugLog(m.cfg, "ITER: win=%d user confirmed — supervised=%v", winIdx, w.iterCtx.Supervised)

	goal := w.iterCtx.Goal
	if goal == "" {
		goal = extractIterativeGoal(w.Messages)
		w.iterCtx.Goal = goal
	}
	targetDir := w.iterCtx.TargetDir
	if targetDir == "" {
		targetDir = "."
		w.iterCtx.TargetDir = targetDir
	}

	designFile, dodFile, logFile, err := prepareIterativeScaffold(goal, targetDir, w.Messages)
	if err != nil {
		w.iterState = IterIdle
		m.appendMessageTo(winIdx, openai.ChatCompletionMessage{
			Role:    "assistant",
			Content: fmt.Sprintf("Failed to write scaffold files: %v", err),
		})
		m.persistWindowMessages(winIdx)
		if winIdx == m.activeWindow {
			m.viewport.SetContent(m.renderChat())
		}
		return nil
	}
	w.iterCtx.DesignFile = designFile
	w.iterCtx.DodFile = dodFile
	w.iterCtx.LogFile = logFile
	w.iterCtx.PlanFile = joinIterPath(targetDir, perPlanFileName)

	if w.iterCtx.Supervised {
		return m.startIterativePERAsync(winIdx, "auto", perModePlanOnly)
	}
	return m.startIterativePERAsync(winIdx, "auto", perModeFull)
}

// presentSupervisedBrief shows the SHIM-tagged brief and waits for executor (skills/iterative_per).
// notes should be the Plan phase output (preferred) or planning transcript.
func (m *Model) presentSupervisedBrief(winIdx int, notes string) tea.Cmd {
	w := m.iterWin(winIdx)
	w.iterState = IterAwaitingExecutor
	if notes == "" {
		notes = w.iterCtx.PlanText
	}
	if notes == "" {
		notes = planningNotesFromMessages(w.Messages)
	}
	brief := buildShimBrief(w.iterCtx, notes)
	// Free main context now that plan + scaffolds exist.
	if len(w.Messages) > 0 {
		if winIdx == m.activeWindow {
			m.clearSessionMemory()
		} else {
			if m.store != nil && w.SessionID != "" {
				_ = m.store.ClearSessionMessages(w.SessionID)
			}
			w.Messages = nil
			w.PersistedMsgCount = 0
		}
	}
	m.appendMessageTo(winIdx, openai.ChatCompletionMessage{
		Role:    "assistant",
		Content: brief,
	})
	m.persistWindowMessages(winIdx)
	if winIdx == m.activeWindow {
		m.viewport.SetContent(m.renderChat())
		if m.cfg.AutoScroll {
			m.viewport.GotoBottom()
		}
	}
	// Optional multi-channel: mirror brief to Telegram when gateway reply is wired.
	if m.gatewayReply != nil {
		tg := m.windows[telegramWindowIndex]
		if tg.GatewayChatID != "" {
			chatID := tg.GatewayChatID
			text := brief
			fn := m.gatewayReply
			go func() { _ = fn(chatID, text) }()
		}
	}
	debugLog(m.cfg, "ITER: win=%d SHIM brief presented — awaiting executor", winIdx)
	return nil
}

func buildShimBrief(ctx IterativeContext, notes string) string {
	if len(notes) > 4000 {
		notes = notes[:4000] + "\n…(truncated)"
	}
	planPath := ctx.PlanFile
	if planPath == "" {
		planPath = "(pending)"
	}
	body := fmt.Sprintf(
		"Project: %s\nTarget: %s\nGoal: %s\n\nDesign: %s\nDoD: %s\nLog: %s\nPlan file: %s\n\n## Plan phase output\n\n%s",
		ctx.ProjectName, ctx.TargetDir, ctx.Goal,
		ctx.DesignFile, ctx.DodFile, ctx.LogFile, planPath,
		notes,
	)
	return fmt.Sprintf(
		"<!--SHIM_BRIEF_START-->\n%s\n<!--SHIM_BRIEF_END-->\n\nBrief ready. Reply with executor: auto / coder / deep-thinker / orchestrator",
		body,
	)
}

// startIterativePERAsync launches Plan / Execute+Review / full PER without blocking Update.
func (m *Model) startIterativePERAsync(winIdx int, executor, mode string) tea.Cmd {
	w := m.iterWin(winIdx)
	w.iterState = IterExecuting
	w.iterCtx.Executor = executor

	ep := m.cfg.GetActiveEndpoint()
	agent.EnsureMVPExecutors(ep.Model, ep.BaseURL, ep.APIKey)

	opts := perRunOpts{
		AppCfg:          m.cfg,
		ProjectName:     w.iterCtx.ProjectName,
		TargetDir:       w.iterCtx.TargetDir,
		Goal:            w.iterCtx.Goal,
		DesignFile:      w.iterCtx.DesignFile,
		DodFile:         w.iterCtx.DodFile,
		LogFile:         w.iterCtx.LogFile,
		PlanFile:        w.iterCtx.PlanFile,
		Executor:        executor,
		Mode:            mode,
		RetryBudget:     perDefaultRetryBudget,
		MaxExecAttempts: m.cfg.IterativeMaxIterations(),
	}
	if opts.PlanFile == "" {
		opts.PlanFile = joinIterPath(opts.TargetDir, perPlanFileName)
		w.iterCtx.PlanFile = opts.PlanFile
	}

	ctx, cancel := context.WithCancel(context.Background())
	w.iterCancel = cancel
	w.iterGen++
	gen := w.iterGen
	w.iterLogOffset = 0
	w.iterProgressSnippet = ""

	// Free main chat context while the isolated worker runs (if not already cleared).
	if len(w.Messages) > 2 {
		if winIdx == m.activeWindow {
			m.clearSessionMemory()
		} else {
			if m.store != nil && w.SessionID != "" {
				_ = m.store.ClearSessionMessages(w.SessionID)
			}
			w.Messages = nil
			w.PersistedMsgCount = 0
		}
	}

	phaseLabel := "PER full multi-cycle (Plan→Execute→Review, re-Plan on retry)"
	switch mode {
	case perModePlanOnly:
		phaseLabel = "PER Plan phase"
	case perModeExecuteReview:
		phaseLabel = "PER Execute→Review multi-cycle"
	}
	m.appendMessageTo(winIdx, openai.ChatCompletionMessage{
		Role: "assistant",
		Content: fmt.Sprintf(
			"[Iterative %s started — async | executor=%s | remind~%d%% summary~%d%% max_exec=%d | inner_retry_budget=%d]\nDesign: %s\nDoD: %s\nLog: %s\nPlan: %s\nWork-log progress streams into chat while running.\nSay 'cancel' or Ctrl+C once to interrupt (active window only).",
			phaseLabel,
			executor,
			m.cfg.IterativeContextRemindPercent(),
			m.cfg.IterativeContextSummaryPercent(),
			opts.MaxExecAttempts,
			perDefaultRetryBudget,
			opts.DesignFile, opts.DodFile, opts.LogFile, opts.PlanFile,
		),
	})
	m.persistWindowMessages(winIdx)

	repaint := m.beginThinking(winIdx)
	if winIdx == m.activeWindow {
		m.viewport.SetContent(m.renderChat())
		if m.cfg.AutoScroll {
			m.viewport.GotoBottom()
		}
	}

	workerCmd := func() tea.Msg {
		summary, kind, err := runIterativePER(ctx, opts)
		if err != nil && summary == "" {
			summary = formatPERInterruptOrErr(ctx, mode, err)
		}
		if kind == "" {
			kind = perResultKindFull
		}
		// Prefix terminal summary for chat clarity
		if kind == perResultKindFull && !strings.HasPrefix(summary, "PER ") && !strings.Contains(summary, "=== ") {
			if err != nil {
				summary = "PER finished with error.\n" + summary
			} else {
				summary = "PER completed.\n" + summary
			}
		}
		return iterativeResultMsg{win: winIdx, gen: gen, result: summary, kind: kind}
	}

	cmds := []tea.Cmd{
		workerCmd,
		m.spinnerTick(),
		m.iterativeLogReadCmd(winIdx, gen),
		m.iterativeLogTickCmd(winIdx, gen),
	}
	if repaint {
		cmds = append([]tea.Cmd{tea.ClearScreen}, cmds...)
	}
	return tea.Batch(cmds...)
}

// handleIterativeResult applies a finished worker result when still current for that window.
// Plan-kind results (supervised) transition to awaiting_executor instead of idle.
// Returns true when the thinking row was active on the active window (repaint).
func (m *Model) handleIterativeResult(msg iterativeResultMsg) bool {
	if msg.win < 0 || msg.win >= maxChatWindows {
		return false
	}
	w := &m.windows[msg.win]
	if msg.gen != w.iterGen {
		debugLog(m.cfg, "ITER: discarding stale result win=%d gen=%d current=%d", msg.win, msg.gen, w.iterGen)
		return false
	}
	ownedThinking := m.thinking && m.thinkingWindow == msg.win
	repaint := ownedThinking && msg.win == m.activeWindow
	if ownedThinking {
		m.thinking = false
	}
	w.iterCancel = nil

	if w.iterState != IterExecuting {
		// Interrupted path already messaged the user.
		debugLog(m.cfg, "ITER: result after non-executing state win=%d %s", msg.win, w.iterState)
		w.resetIterativeState()
		if repaint {
			m.resetViewportHeight()
			m.viewport.SetContent(m.renderChat())
		}
		return repaint
	}

	// Supervised: Plan phase done → human executor gate (do not reset flow).
	if msg.kind == perResultKindPlan && w.iterCtx.Supervised {
		planBody := msg.result
		// Strip phase banner if present for the brief body
		if i := strings.Index(planBody, "=== Plan phase ==="); i >= 0 {
			planBody = strings.TrimSpace(planBody[i+len("=== Plan phase ==="):])
		}
		w.iterCtx.PlanText = planBody
		if w.iterCtx.PlanFile == "" {
			w.iterCtx.PlanFile = joinIterPath(w.iterCtx.TargetDir, perPlanFileName)
		}
		w.iterProgressSnippet = ""
		_ = m.presentSupervisedBrief(msg.win, planBody)
		if repaint {
			m.resetViewportHeight()
		}
		debugLog(m.cfg, "ITER: win=%d plan done — awaiting_executor", msg.win)
		return repaint || msg.win == m.activeWindow
	}

	w.iterState = IterCompleted
	m.appendMessageTo(msg.win, openai.ChatCompletionMessage{
		Role:    "assistant",
		Content: msg.result,
	})
	m.persistWindowMessages(msg.win)
	w.resetIterativeState()
	if msg.win == m.activeWindow {
		if repaint {
			m.resetViewportHeight()
		}
		m.viewport.SetContent(m.renderChat())
		if m.cfg.AutoScroll {
			m.viewport.GotoBottom()
		}
	}
	debugLog(m.cfg, "ITER: PER finished win=%d — state idle", msg.win)
	return repaint
}

func extractIterativeGoal(messages []openai.ChatCompletionMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" && messages[i].Content != "" {
			content := messages[i].Content
			if isConfirmation(content) || isCancellation(content) {
				continue
			}
			if _, ok := parseSupervisedChoice(content); ok && (content == "y" || content == "n" || content == "Y" || content == "N" || content == "" || strings.EqualFold(content, "yes") || strings.EqualFold(content, "no")) {
				continue
			}
			if _, ok := parseExecutorChoice(content); ok {
				continue
			}
			return content
		}
	}
	return "User requested iterative build"
}

// planningNotesFromMessages captures user/assistant planning turns for the design scaffold.
func planningNotesFromMessages(messages []openai.ChatCompletionMessage) string {
	var b strings.Builder
	for _, msg := range messages {
		if msg.Role != "user" && msg.Role != "assistant" {
			continue
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" || isConfirmation(content) || isCancellation(content) {
			continue
		}
		if strings.HasPrefix(content, "[Iterative") {
			continue
		}
		if strings.HasPrefix(content, "[iterative log]") {
			continue
		}
		if strings.Contains(content, "SHIM_BRIEF") {
			continue
		}
		if msg.Role == "assistant" && (strings.Contains(content, "name this project") || strings.Contains(content, "Run in supervised mode?")) {
			continue
		}
		label := "User"
		if msg.Role == "assistant" {
			label = "Assistant"
		}
		fmt.Fprintf(&b, "### %s\n\n%s\n\n", label, content)
	}
	if b.Len() == 0 {
		return "(No planning transcript captured.)\n"
	}
	return b.String()
}

// onIterativeAgentResponse advances planning → awaiting_confirmation when the agent presents a plan.
func (m *Model) onIterativeAgentResponse(winIdx int) {
	if winIdx < 0 || winIdx >= maxChatWindows {
		return
	}
	w := &m.windows[winIdx]
	if w.iterState != IterPlanning {
		return
	}
	for i := len(w.Messages) - 1; i >= 0; i-- {
		if w.Messages[i].Role == "assistant" && w.Messages[i].Content != "" {
			if isPlanReady(w.Messages[i].Content) {
				w.iterState = IterAwaitingConfirmation
				debugLog(m.cfg, "ITER: win=%d plan ready — state awaiting_confirmation", winIdx)
			}
			return
		}
	}
}

// showIterativeStatus returns a status line for the active window's iterative flow.
func (m Model) showIterativeStatus() string {
	w := m.windows[m.activeWindow]
	if w.iterState == IterIdle {
		return ""
	}
	if w.iterState == IterExecuting && w.iterProgressSnippet != "" {
		return fmt.Sprintf("Iterative: executing — %s", w.iterProgressSnippet)
	}
	return fmt.Sprintf("Iterative: %s", w.iterState)
}

// iterativeThinkingLabel is the spinner-row text while the active window's async worker runs.
func (m Model) iterativeThinkingLabel() string {
	w := m.windows[m.activeWindow]
	if w.iterState != IterExecuting {
		return " Thinking..."
	}
	if w.iterProgressSnippet != "" {
		return " Iter: " + w.iterProgressSnippet
	}
	return " Iterative worker..."
}
