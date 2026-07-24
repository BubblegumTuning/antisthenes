package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nanami/antisthenes/config"
	"github.com/nanami/antisthenes/internal/agent"
)

// PER result kinds carried on iterativeResultMsg.kind.
const (
	perResultKindPlan = "plan" // supervised: plan finished → human executor gate
	perResultKindFull = "full" // unsupervised full loop or supervised execute+review done
)

// PER run modes for the async worker body.
const (
	perModePlanOnly      = "plan_only"
	perModeExecuteReview = "execute_review"
	perModeFull          = "full"
)

// Default inner Execute→Review retries within one cycle (without re-Plan).
// Outer cycles / total Execute attempts are bounded by config iterative.max_iterations.
const perDefaultRetryBudget = 1

const (
	perLogFileName  = "per_log.txt"
	perDoneSignal   = "per_done.signal"
	perPlanFileName = "per_plan.md"
	perStatusDone   = "PER_STATUS: DONE"
	perStatusRetry  = "PER_STATUS: RETRY"
	perStatusFailed = "PER_STATUS: FAILED"
)

// Terminal status from runPERExecuteReview (not LLM markers).
const (
	perTermDone        = "done"
	perTermFailed      = "failed"
	perTermExhausted   = "exhausted"
	perTermError       = "error"
	perTermInterrupted = "interrupted"
)

// perRunOpts configures a multi-phase Plan→Execute→Review run inside a tea.Cmd.
type perRunOpts struct {
	AppCfg      config.Config
	ProjectName string
	TargetDir   string
	Goal        string
	DesignFile  string
	DodFile     string
	LogFile     string
	PlanFile    string
	Executor    string // used for Execute (and retries); Plan/Review use auto/default
	Mode        string // plan_only | execute_review | full
	// RetryBudget: extra Execute→Review rounds within one cycle before re-Plan (full mode).
	// Zero → perDefaultRetryBudget. Negative → treat as zero extra (single attempt per cycle).
	RetryBudget int
	// MaxExecAttempts: hard cap on Execute phases for the whole run (0 → config max_iterations).
	MaxExecAttempts int
}

// runIterativePER runs discrete Plan / Execute / Review delegate calls with
// fresh contexts per phase. Call only from a tea.Cmd goroutine (never Update).
//
// Multi-cycle (full mode): Plan → Execute→Review (up to RetryBudget+1 executes),
// then on RETRY re-Plan and continue until DONE, FAILED, cancel, or MaxExecAttempts.
// execute_review: no re-Plan; Execute→Review up to MaxExecAttempts.
func runIterativePER(ctx context.Context, opts perRunOpts) (summary string, kind string, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	opts = normalizePEROpts(opts)

	var parts []string

	switch opts.Mode {
	case perModePlanOnly:
		planOut, e := runPERPlanPhase(ctx, opts)
		if e != nil {
			return formatPERInterruptOrErr(ctx, "Plan", e), perResultKindPlan, e
		}
		parts = append(parts, "=== Plan phase ===\n"+planOut)
		return strings.Join(parts, "\n\n"), perResultKindPlan, nil

	case perModeExecuteReview:
		planText := readFileString(opts.PlanFile)
		out, term, _, e := runPERExecuteReview(ctx, opts, planText, opts.MaxExecAttempts, 1)
		if e != nil {
			msg := out
			if msg != "" {
				parts = append(parts, msg)
			}
			parts = append(parts, formatPERInterruptOrErr(ctx, "Execute/Review", e))
			return strings.Join(parts, "\n\n"), perResultKindFull, e
		}
		if out != "" {
			parts = append(parts, out)
		}
		if term == perTermExhausted {
			_ = appendPerLog(opts.TargetDir, fmt.Sprintf("PER stopped for %s — max_iterations (%d executes) exhausted", opts.ProjectName, opts.MaxExecAttempts))
			_ = writePerDoneSignal(opts.TargetDir, "stopped:max_iterations exhausted")
		}
		parts = append(parts, formatPERTerminal(term, opts.MaxExecAttempts))
		return strings.Join(parts, "\n\n"), perResultKindFull, nil

	default: // full — multi-cycle Plan→Execute→Review
		return runPERFullMultiCycle(ctx, opts)
	}
}

func normalizePEROpts(opts perRunOpts) perRunOpts {
	if opts.RetryBudget < 0 {
		opts.RetryBudget = 0
	} else if opts.RetryBudget == 0 && opts.Mode != perModePlanOnly {
		// zero means "use default" for execute paths; plan_only ignores retries
		opts.RetryBudget = perDefaultRetryBudget
	}
	if opts.MaxExecAttempts <= 0 {
		opts.MaxExecAttempts = opts.AppCfg.IterativeMaxIterations()
	}
	if opts.MaxExecAttempts < 1 {
		opts.MaxExecAttempts = 1
	}
	if opts.TargetDir == "" {
		opts.TargetDir = "."
	}
	if opts.PlanFile == "" {
		opts.PlanFile = filepath.Join(opts.TargetDir, perPlanFileName)
	}
	return opts
}

func runPERFullMultiCycle(ctx context.Context, opts perRunOpts) (string, string, error) {
	var parts []string
	execUsed := 0
	cycle := 0
	perCycleExec := opts.RetryBudget + 1
	if perCycleExec < 1 {
		perCycleExec = 1
	}

	for execUsed < opts.MaxExecAttempts {
		if err := ctx.Err(); err != nil {
			parts = append(parts, formatPERInterruptOrErr(ctx, "cycle", err))
			return strings.Join(parts, "\n\n"), perResultKindFull, err
		}
		cycle++
		remaining := opts.MaxExecAttempts - execUsed
		cycleMax := perCycleExec
		if cycleMax > remaining {
			cycleMax = remaining
		}

		_ = appendPerLog(opts.TargetDir, fmt.Sprintf("PER outer cycle %d started for %s (exec budget this cycle %d, total remaining %d)", cycle, opts.ProjectName, cycleMax, remaining))
		_ = appendWorkLog(opts.LogFile, fmt.Sprintf("PER outer cycle %d started (cycle_exec_budget=%d remaining_total=%d)", cycle, cycleMax, remaining))
		parts = append(parts, fmt.Sprintf("=== PER outer cycle %d / max_exec %d ===", cycle, opts.MaxExecAttempts))

		planOut, e := runPERPlanPhase(ctx, opts)
		if e != nil {
			parts = append(parts, formatPERInterruptOrErr(ctx, "Plan", e))
			return strings.Join(parts, "\n\n"), perResultKindFull, e
		}
		parts = append(parts, "=== Plan phase ===\n"+planOut)

		out, term, n, e := runPERExecuteReview(ctx, opts, planOut, cycleMax, cycle)
		execUsed += n
		if out != "" {
			parts = append(parts, out)
		}
		if e != nil {
			parts = append(parts, formatPERInterruptOrErr(ctx, "Execute/Review", e))
			return strings.Join(parts, "\n\n"), perResultKindFull, e
		}

		switch term {
		case perTermDone, perTermFailed:
			parts = append(parts, formatPERTerminal(term, opts.MaxExecAttempts))
			return strings.Join(parts, "\n\n"), perResultKindFull, nil
		case perTermInterrupted:
			parts = append(parts, formatPERTerminal(term, opts.MaxExecAttempts))
			return strings.Join(parts, "\n\n"), perResultKindFull, ctx.Err()
		default: // exhausted this cycle — re-Plan if budget remains
			if execUsed >= opts.MaxExecAttempts {
				_ = appendPerLog(opts.TargetDir, fmt.Sprintf("PER stopped for %s — max_iterations (%d executes) exhausted", opts.ProjectName, opts.MaxExecAttempts))
				_ = writePerDoneSignal(opts.TargetDir, "stopped:max_iterations exhausted")
				parts = append(parts, formatPERTerminal(perTermExhausted, opts.MaxExecAttempts))
				return strings.Join(parts, "\n\n"), perResultKindFull, nil
			}
			_ = appendPerLog(opts.TargetDir, fmt.Sprintf("PER re-Plan scheduled for %s after cycle %d (%d executes used, %d left)", opts.ProjectName, cycle, execUsed, opts.MaxExecAttempts-execUsed))
			_ = appendWorkLog(opts.LogFile, fmt.Sprintf("PER scheduling re-Plan (executes used %d / %d)", execUsed, opts.MaxExecAttempts))
			parts = append(parts, fmt.Sprintf("PER: cycle %d exhausted inner retry budget — re-planning (%d executes left).", cycle, opts.MaxExecAttempts-execUsed))
		}
	}

	_ = appendPerLog(opts.TargetDir, fmt.Sprintf("PER stopped for %s — max_iterations exhausted", opts.ProjectName))
	_ = writePerDoneSignal(opts.TargetDir, "stopped:max_iterations exhausted")
	parts = append(parts, formatPERTerminal(perTermExhausted, opts.MaxExecAttempts))
	return strings.Join(parts, "\n\n"), perResultKindFull, nil
}

func runPERPlanPhase(ctx context.Context, opts perRunOpts) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	_ = appendPerLog(opts.TargetDir, fmt.Sprintf("Plan phase started for %s", opts.ProjectName))
	_ = appendWorkLog(opts.LogFile, "PER Plan phase started")

	goal := buildPERPlanGoal(opts)
	out, err := runPERDelegate(ctx, "", goal)
	if err != nil {
		_ = appendPerLog(opts.TargetDir, fmt.Sprintf("Plan phase failed for %s: %v", opts.ProjectName, err))
		return "", err
	}

	if err := os.MkdirAll(opts.TargetDir, 0o755); err == nil {
		_ = os.WriteFile(opts.PlanFile, []byte(out), 0o644)
	}
	_ = appendPerLog(opts.TargetDir, fmt.Sprintf("Plan phase completed for %s", opts.ProjectName))
	_ = appendWorkLog(opts.LogFile, "PER Plan phase completed")
	return out, nil
}

// runPERExecuteReview runs up to maxAttempts Execute→Review pairs.
// attemptBase is the 1-based outer attempt offset label prefix (usually 1, or cycle number context).
// Returns terminal status: done|failed|exhausted|interrupted|error, and number of Execute attempts used.
func runPERExecuteReview(ctx context.Context, opts perRunOpts, planText string, maxAttempts, cycle int) (string, string, int, error) {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	var parts []string
	attempts := 0

	for attempts < maxAttempts {
		if err := ctx.Err(); err != nil {
			return strings.Join(parts, "\n\n"), perTermInterrupted, attempts, err
		}
		attempts++
		label := attempts
		if cycle > 0 {
			label = attempts // still per-cycle attempt index; cycle noted in logs
		}

		// Execute
		_ = appendPerLog(opts.TargetDir, fmt.Sprintf("Execute phase started for %s (cycle %d attempt %d)", opts.ProjectName, cycle, label))
		_ = appendWorkLog(opts.LogFile, fmt.Sprintf("PER Execute phase started (cycle %d attempt %d)", cycle, label))
		execGoal := buildPERExecuteGoal(opts, planText, cycle, label, maxAttempts)
		execOut, err := runPERDelegate(ctx, opts.Executor, execGoal)
		if err != nil {
			_ = appendPerLog(opts.TargetDir, fmt.Sprintf("Execute phase failed for %s: %v", opts.ProjectName, err))
			_ = writePerDoneSignal(opts.TargetDir, "failed:execute")
			parts = append(parts, formatPERInterruptOrErr(ctx, "Execute", err))
			term := perTermError
			if ctx.Err() != nil {
				term = perTermInterrupted
			}
			return strings.Join(parts, "\n\n"), term, attempts, err
		}
		parts = append(parts, fmt.Sprintf("=== Execute phase (cycle %d attempt %d) ===\n%s", cycle, label, execOut))
		_ = appendPerLog(opts.TargetDir, fmt.Sprintf("Execute phase completed for %s (cycle %d attempt %d)", opts.ProjectName, cycle, label))
		_ = appendWorkLog(opts.LogFile, fmt.Sprintf("PER Execute phase completed (cycle %d attempt %d)", cycle, label))

		if err := ctx.Err(); err != nil {
			return strings.Join(parts, "\n\n"), perTermInterrupted, attempts, err
		}

		// Review
		_ = appendPerLog(opts.TargetDir, fmt.Sprintf("Review phase started for %s (cycle %d attempt %d)", opts.ProjectName, cycle, label))
		_ = appendWorkLog(opts.LogFile, fmt.Sprintf("PER Review phase started (cycle %d attempt %d)", cycle, label))
		revGoal := buildPERReviewGoal(opts, execOut, cycle, label, maxAttempts)
		revOut, err := runPERDelegate(ctx, "", revGoal)
		if err != nil {
			_ = appendPerLog(opts.TargetDir, fmt.Sprintf("Review phase failed for %s: %v", opts.ProjectName, err))
			_ = writePerDoneSignal(opts.TargetDir, "failed:review")
			parts = append(parts, formatPERInterruptOrErr(ctx, "Review", err))
			term := perTermError
			if ctx.Err() != nil {
				term = perTermInterrupted
			}
			return strings.Join(parts, "\n\n"), term, attempts, err
		}
		parts = append(parts, fmt.Sprintf("=== Review phase (cycle %d attempt %d) ===\n%s", cycle, label, revOut))
		_ = appendPerLog(opts.TargetDir, fmt.Sprintf("Review phase completed for %s (cycle %d attempt %d)", opts.ProjectName, cycle, label))
		_ = appendWorkLog(opts.LogFile, fmt.Sprintf("PER Review phase completed (cycle %d attempt %d)", cycle, label))

		if reviewIndicatesDone(revOut) {
			_ = appendPerLog(opts.TargetDir, fmt.Sprintf("PER done for %s — DoD satisfied", opts.ProjectName))
			_ = writePerDoneSignal(opts.TargetDir, "done")
			parts = append(parts, "PER: Definition of Done satisfied. Stopping.")
			return strings.Join(parts, "\n\n"), perTermDone, attempts, nil
		}

		if reviewIndicatesFailed(revOut) {
			_ = appendPerLog(opts.TargetDir, fmt.Sprintf("PER stopped for %s — review marked FAILED", opts.ProjectName))
			_ = writePerDoneSignal(opts.TargetDir, "stopped:review marked FAILED")
			parts = append(parts, "PER: stopping — review marked FAILED.")
			return strings.Join(parts, "\n\n"), perTermFailed, attempts, nil
		}

		// RETRY or missing marker → another Execute if attempts remain in this call
		if attempts >= maxAttempts {
			break
		}
		_ = appendPerLog(opts.TargetDir, fmt.Sprintf("PER retry within cycle %d for %s (%d cycle attempts left)", cycle, opts.ProjectName, maxAttempts-attempts))
		_ = appendWorkLog(opts.LogFile, fmt.Sprintf("PER scheduling Execute+Review retry within cycle (%d left in cycle)", maxAttempts-attempts))
		parts = append(parts, fmt.Sprintf("PER: review requests retry (%d left in this cycle).", maxAttempts-attempts))
	}

	return strings.Join(parts, "\n\n"), perTermExhausted, attempts, nil
}

func formatPERTerminal(term string, maxExec int) string {
	switch term {
	case perTermDone:
		return "PER: finished — Definition of Done satisfied."
	case perTermFailed:
		return "PER: finished — review marked FAILED."
	case perTermExhausted:
		return fmt.Sprintf("PER: stopping — max_iterations / cycle budget exhausted (cap %d executes).", maxExec)
	case perTermInterrupted:
		return "PER: interrupted."
	default:
		return "PER: finished — " + term + "."
	}
}

func runPERDelegate(ctx context.Context, executor, goal string) (string, error) {
	cfg := agent.DefaultDelegateConfig()
	cfg.Ctx = ctx
	if executor != "" && !strings.EqualFold(executor, "auto") {
		cfg.ExecutorName = executor
	}
	result := delegateTaskFunc(goal, cfg)
	if result.Error != nil {
		return "", result.Error
	}
	if strings.TrimSpace(result.Result) == "" {
		return "sub-agent completed (no final assistant message)", nil
	}
	return result.Result, nil
}

func buildPERPlanGoal(opts perRunOpts) string {
	return fmt.Sprintf(`PER_PHASE: PLAN — isolated context

Project: %s
Goal: %s
Working directory: %s
Design document: %s
Definition of Done: %s
Work log (append timestamped entries): %s
Plan artifact path: %s

Instructions:
1. Read the design document, definition of done, work log, and any prior plan artifact.
2. Produce a concrete implementation plan only (no code changes yet unless a tiny probe is essential).
3. If this is a re-plan after a prior Execute/Review cycle, incorporate review feedback and remaining DoD gaps.
4. List ordered steps, files to touch, risks, and how you will verify each DoD item.
5. Append a brief timestamped note to the work log that Plan phase finished.
6. Do not claim the DoD is done in this phase.

Return the full plan as your final assistant message.`,
		opts.ProjectName, opts.Goal, opts.TargetDir,
		opts.DesignFile, opts.DodFile, opts.LogFile, opts.PlanFile)
}

func buildPERExecuteGoal(opts perRunOpts, planText string, cycle, attempt, cycleMax int) string {
	if len(planText) > 12000 {
		planText = planText[:12000] + "\n…(truncated)"
	}
	remind := opts.AppCfg.IterativeContextRemindPercent()
	summary := opts.AppCfg.IterativeContextSummaryPercent()
	maxIter := opts.MaxExecAttempts
	if maxIter <= 0 {
		maxIter = opts.AppCfg.IterativeMaxIterations()
	}
	return fmt.Sprintf(`PER_PHASE: EXECUTE — isolated context

Project: %s
Goal: %s
Working directory: %s
Design: %s
Definition of Done: %s
Work log: %s
Outer cycle: %d | attempt in cycle: %d / %d | global execute cap: %d

## Prior plan

%s

Instructions:
1. Work exclusively inside %s.
2. Implement the plan using tools (bash, write_file, patch, search_files, etc.).
3. After significant actions, append timestamped entries to the work log.
4. Context guidance: remind ~%d%%, fuller summary ~%d%%; keep this phase focused.
5. Do not stop early without attempting the main implementation work.
6. Final message: summarise what you changed and what remains.

Begin execution now.`,
		opts.ProjectName, opts.Goal, opts.TargetDir,
		opts.DesignFile, opts.DodFile, opts.LogFile,
		cycle, attempt, cycleMax, maxIter,
		planText, opts.TargetDir, remind, summary)
}

func buildPERReviewGoal(opts perRunOpts, execSummary string, cycle, attempt, cycleMax int) string {
	if len(execSummary) > 8000 {
		execSummary = execSummary[:8000] + "\n…(truncated)"
	}
	return fmt.Sprintf(`PER_PHASE: REVIEW — isolated context

Project: %s
Working directory: %s
Definition of Done file: %s
Design file: %s
Work log: %s
Outer cycle: %d | attempt in cycle: %d / %d

## Prior execute summary

%s

Instructions:
1. Read the Definition of Done and inspect the working directory / work log as needed.
2. Evaluate each success criterion honestly.
3. End your final message with exactly one status line:
   - %s  when all DoD items are satisfied
   - %s when more implementation work would likely finish the DoD (controller may retry Execute or re-Plan within max_iterations)
   - %s when blocked or further retries are pointless
4. Briefly list which criteria pass/fail.
5. Append a short timestamped review note to the work log.

Return your review now.`,
		opts.ProjectName, opts.TargetDir, opts.DodFile, opts.DesignFile, opts.LogFile,
		cycle, attempt, cycleMax,
		execSummary, perStatusDone, perStatusRetry, perStatusFailed)
}

func reviewIndicatesDone(out string) bool {
	u := strings.ToUpper(out)
	if strings.Contains(u, perStatusDone) {
		return true
	}
	// Soft accept common phrasing if explicit marker missing
	if strings.Contains(u, "ALL DOD") && (strings.Contains(u, "SATISFIED") || strings.Contains(u, "MET")) {
		return true
	}
	return false
}

func reviewIndicatesFailed(out string) bool {
	return strings.Contains(strings.ToUpper(out), perStatusFailed)
}

func reviewIndicatesRetry(out string) bool {
	return strings.Contains(strings.ToUpper(out), perStatusRetry)
}

func appendPerLog(targetDir, msg string) error {
	if targetDir == "" {
		targetDir = "."
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(targetDir, perLogFileName)
	line := fmt.Sprintf("%s | %s\n", time.Now().UTC().Format(time.RFC3339), msg)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line)
	return err
}

func writePerDoneSignal(targetDir, status string) error {
	if targetDir == "" {
		targetDir = "."
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}
	body := fmt.Sprintf("%s\n%s\n", time.Now().UTC().Format(time.RFC3339), status)
	return os.WriteFile(filepath.Join(targetDir, perDoneSignal), []byte(body), 0o644)
}

func appendWorkLog(logFile, msg string) error {
	if strings.TrimSpace(logFile) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(logFile), 0o755); err != nil && filepath.Dir(logFile) != "." {
		// best-effort — ignore mkdir error
	}
	line := fmt.Sprintf("%s | %s\n", time.Now().UTC().Format(time.RFC3339), msg)
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line)
	return err
}

func readFileString(path string) string {
	if path == "" {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

func formatPERInterruptOrErr(ctx context.Context, phase string, err error) string {
	if ctx != nil && ctx.Err() != nil {
		return fmt.Sprintf("PER %s interrupted: %v", phase, ctx.Err())
	}
	return fmt.Sprintf("PER %s failed: %v", phase, err)
}
