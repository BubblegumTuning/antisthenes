package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/viewport"

	"github.com/nanami/antisthenes/config"
	"github.com/nanami/antisthenes/internal/agent"
)

func TestRunIterativePER_Full_PhasesAndDone(t *testing.T) {
	orig := delegateTaskFunc
	var mu sync.Mutex
	var phases []string
	delegateTaskFunc = func(goal string, _ agent.DelegateConfig) agent.SubAgentResult {
		mu.Lock()
		defer mu.Unlock()
		g := strings.ToUpper(goal)
		switch {
		case strings.Contains(g, "PER_PHASE: PLAN"):
			phases = append(phases, "plan")
			return agent.SubAgentResult{Result: "plan text"}
		case strings.Contains(g, "PER_PHASE: EXECUTE"):
			phases = append(phases, "execute")
			return agent.SubAgentResult{Result: "exec text"}
		case strings.Contains(g, "PER_PHASE: REVIEW"):
			phases = append(phases, "review")
			return agent.SubAgentResult{Result: "looks good\nPER_STATUS: DONE"}
		default:
			phases = append(phases, "unknown")
			return agent.SubAgentResult{Result: "x"}
		}
	}
	defer func() { delegateTaskFunc = orig }()

	tmp := t.TempDir()
	design := filepath.Join(tmp, "d.md")
	dod := filepath.Join(tmp, "o.md")
	logf := filepath.Join(tmp, "w.log")
	_ = os.WriteFile(design, []byte("design"), 0o644)
	_ = os.WriteFile(dod, []byte("- [ ] a"), 0o644)
	_ = os.WriteFile(logf, []byte(""), 0o644)

	summary, kind, err := runIterativePER(context.Background(), perRunOpts{
		AppCfg:          config.Config{},
		ProjectName:     "p",
		TargetDir:       tmp,
		Goal:            "g",
		DesignFile:      design,
		DodFile:         dod,
		LogFile:         logf,
		Mode:            perModeFull,
		RetryBudget:     1,
		MaxExecAttempts: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if kind != perResultKindFull {
		t.Fatalf("kind=%s", kind)
	}
	mu.Lock()
	got := strings.Join(phases, ",")
	mu.Unlock()
	if got != "plan,execute,review" {
		t.Fatalf("phases=%s want plan,execute,review", got)
	}
	if !strings.Contains(summary, "Plan phase") || !strings.Contains(summary, "satisfied") {
		t.Fatalf("summary=%s", summary)
	}
	perLog, _ := os.ReadFile(filepath.Join(tmp, perLogFileName))
	if !strings.Contains(string(perLog), "Plan phase completed") {
		t.Fatalf("per_log: %s", perLog)
	}
	if _, err := os.Stat(filepath.Join(tmp, perDoneSignal)); err != nil {
		t.Fatal(err)
	}
	plan, err := os.ReadFile(filepath.Join(tmp, perPlanFileName))
	if err != nil || !strings.Contains(string(plan), "plan text") {
		t.Fatalf("plan file: %s err=%v", plan, err)
	}
}

func TestRunIterativePER_InnerRetryThenStopAtMaxExec(t *testing.T) {
	// MaxExecAttempts=2, RetryBudget=1 → one cycle with 2 executes, then stop (no re-plan).
	orig := delegateTaskFunc
	var reviews int
	var plans int
	delegateTaskFunc = func(goal string, _ agent.DelegateConfig) agent.SubAgentResult {
		g := strings.ToUpper(goal)
		switch {
		case strings.Contains(g, "PER_PHASE: PLAN"):
			plans++
			return agent.SubAgentResult{Result: "p"}
		case strings.Contains(g, "PER_PHASE: EXECUTE"):
			return agent.SubAgentResult{Result: "e"}
		case strings.Contains(g, "PER_PHASE: REVIEW"):
			reviews++
			return agent.SubAgentResult{Result: "not yet\nPER_STATUS: RETRY"}
		default:
			return agent.SubAgentResult{Result: "x"}
		}
	}
	defer func() { delegateTaskFunc = orig }()

	tmp := t.TempDir()
	summary, _, err := runIterativePER(context.Background(), perRunOpts{
		AppCfg:          config.Config{},
		ProjectName:     "p",
		TargetDir:       tmp,
		Goal:            "g",
		DesignFile:      filepath.Join(tmp, "d"),
		DodFile:         filepath.Join(tmp, "o"),
		LogFile:         filepath.Join(tmp, "l"),
		Mode:            perModeFull,
		RetryBudget:     1,
		MaxExecAttempts: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if plans != 1 {
		t.Fatalf("plans=%d want 1", plans)
	}
	if reviews != 2 {
		t.Fatalf("reviews=%d want 2 (initial + one inner retry)", reviews)
	}
	if !strings.Contains(summary, "max_iterations") && !strings.Contains(summary, "exhausted") {
		t.Fatalf("summary=%s", summary)
	}
	sig, _ := os.ReadFile(filepath.Join(tmp, perDoneSignal))
	if !strings.Contains(string(sig), "max_iterations") {
		t.Fatalf("signal=%s", sig)
	}
}

func TestRunIterativePER_MultiCycleRePlan(t *testing.T) {
	// RetryBudget=1 → 2 execs/cycle. MaxExec=5 → cycles get 2+2+1 execs, 3 plans.
	orig := delegateTaskFunc
	var mu sync.Mutex
	var plans, executes, reviews int
	delegateTaskFunc = func(goal string, _ agent.DelegateConfig) agent.SubAgentResult {
		mu.Lock()
		defer mu.Unlock()
		g := strings.ToUpper(goal)
		switch {
		case strings.Contains(g, "PER_PHASE: PLAN"):
			plans++
			return agent.SubAgentResult{Result: "plan"}
		case strings.Contains(g, "PER_PHASE: EXECUTE"):
			executes++
			return agent.SubAgentResult{Result: "exec"}
		case strings.Contains(g, "PER_PHASE: REVIEW"):
			reviews++
			return agent.SubAgentResult{Result: "keep going\nPER_STATUS: RETRY"}
		default:
			return agent.SubAgentResult{Result: "x"}
		}
	}
	defer func() { delegateTaskFunc = orig }()

	tmp := t.TempDir()
	summary, _, err := runIterativePER(context.Background(), perRunOpts{
		AppCfg:          config.Config{},
		ProjectName:     "p",
		TargetDir:       tmp,
		Goal:            "g",
		DesignFile:      filepath.Join(tmp, "d"),
		DodFile:         filepath.Join(tmp, "o"),
		LogFile:         filepath.Join(tmp, "l"),
		Mode:            perModeFull,
		RetryBudget:     1,
		MaxExecAttempts: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	p, e, r := plans, executes, reviews
	mu.Unlock()
	if p != 3 {
		t.Fatalf("plans=%d want 3 (re-Plan after each exhausted cycle)", p)
	}
	if e != 5 || r != 5 {
		t.Fatalf("executes=%d reviews=%d want 5 each", e, r)
	}
	if !strings.Contains(summary, "re-planning") && !strings.Contains(summary, "outer cycle") {
		t.Fatalf("expected multi-cycle markers in summary: %s", summary)
	}
	perLog, _ := os.ReadFile(filepath.Join(tmp, perLogFileName))
	if !strings.Contains(string(perLog), "re-Plan scheduled") {
		t.Fatalf("per_log missing re-Plan: %s", perLog)
	}
}

func TestRunIterativePER_MultiCycleDoneOnLaterCycle(t *testing.T) {
	orig := delegateTaskFunc
	var executes int
	delegateTaskFunc = func(goal string, _ agent.DelegateConfig) agent.SubAgentResult {
		g := strings.ToUpper(goal)
		switch {
		case strings.Contains(g, "PER_PHASE: PLAN"):
			return agent.SubAgentResult{Result: "plan"}
		case strings.Contains(g, "PER_PHASE: EXECUTE"):
			executes++
			return agent.SubAgentResult{Result: "exec"}
		case strings.Contains(g, "PER_PHASE: REVIEW"):
			if executes >= 3 {
				return agent.SubAgentResult{Result: "ok\nPER_STATUS: DONE"}
			}
			return agent.SubAgentResult{Result: "no\nPER_STATUS: RETRY"}
		default:
			return agent.SubAgentResult{Result: "x"}
		}
	}
	defer func() { delegateTaskFunc = orig }()

	tmp := t.TempDir()
	summary, _, err := runIterativePER(context.Background(), perRunOpts{
		AppCfg:          config.Config{},
		ProjectName:     "p",
		TargetDir:       tmp,
		Goal:            "g",
		DesignFile:      filepath.Join(tmp, "d"),
		DodFile:         filepath.Join(tmp, "o"),
		LogFile:         filepath.Join(tmp, "l"),
		Mode:            perModeFull,
		RetryBudget:     1,
		MaxExecAttempts: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if executes != 3 {
		t.Fatalf("executes=%d want 3", executes)
	}
	if !strings.Contains(summary, "satisfied") {
		t.Fatalf("summary=%s", summary)
	}
	sig, _ := os.ReadFile(filepath.Join(tmp, perDoneSignal))
	if !strings.Contains(string(sig), "done") {
		t.Fatalf("signal=%s", sig)
	}
}

func TestRunIterativePER_ExecuteReview_UsesMaxExecNoRePlan(t *testing.T) {
	orig := delegateTaskFunc
	var plans, executes int
	delegateTaskFunc = func(goal string, _ agent.DelegateConfig) agent.SubAgentResult {
		g := strings.ToUpper(goal)
		switch {
		case strings.Contains(g, "PER_PHASE: PLAN"):
			plans++
			return agent.SubAgentResult{Result: "p"}
		case strings.Contains(g, "PER_PHASE: EXECUTE"):
			executes++
			return agent.SubAgentResult{Result: "e"}
		case strings.Contains(g, "PER_PHASE: REVIEW"):
			return agent.SubAgentResult{Result: "retry\nPER_STATUS: RETRY"}
		default:
			return agent.SubAgentResult{Result: "x"}
		}
	}
	defer func() { delegateTaskFunc = orig }()

	tmp := t.TempDir()
	planPath := filepath.Join(tmp, perPlanFileName)
	_ = os.WriteFile(planPath, []byte("prior plan"), 0o644)

	_, _, err := runIterativePER(context.Background(), perRunOpts{
		AppCfg:          config.Config{},
		ProjectName:     "p",
		TargetDir:       tmp,
		Goal:            "g",
		DesignFile:      filepath.Join(tmp, "d"),
		DodFile:         filepath.Join(tmp, "o"),
		LogFile:         filepath.Join(tmp, "l"),
		PlanFile:        planPath,
		Mode:            perModeExecuteReview,
		RetryBudget:     1, // ignored as outer cap; MaxExecAttempts wins
		MaxExecAttempts: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	if plans != 0 {
		t.Fatalf("plans=%d want 0 (execute_review reads plan file)", plans)
	}
	if executes != 4 {
		t.Fatalf("executes=%d want 4", executes)
	}
}

func TestRunIterativePER_CancelMidPhase(t *testing.T) {
	orig := delegateTaskFunc
	started := make(chan struct{})
	delegateTaskFunc = func(_ string, cfg agent.DelegateConfig) agent.SubAgentResult {
		close(started)
		<-cfg.Ctx.Done()
		return agent.SubAgentResult{Error: cfg.Ctx.Err()}
	}
	defer func() { delegateTaskFunc = orig }()

	tmp := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	var summary string
	var runErr error
	go func() {
		summary, _, runErr = runIterativePER(ctx, perRunOpts{
			AppCfg:          config.Config{},
			ProjectName:     "p",
			TargetDir:       tmp,
			Goal:            "g",
			DesignFile:      "d",
			DodFile:         "o",
			LogFile:         filepath.Join(tmp, "l.log"),
			Mode:            perModePlanOnly,
			MaxExecAttempts: 3,
		})
		close(done)
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("phase never started")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("did not return after cancel")
	}
	if runErr == nil {
		t.Fatal("expected error on cancel")
	}
	if !strings.Contains(summary, "interrupt") && !strings.Contains(strings.ToLower(summary), "cancel") {
		t.Fatalf("summary=%q", summary)
	}
}

func TestReviewStatusHelpers(t *testing.T) {
	if !reviewIndicatesDone("x\nPER_STATUS: DONE\n") {
		t.Fatal("done")
	}
	if !reviewIndicatesRetry("PER_STATUS: RETRY") {
		t.Fatal("retry")
	}
	if !reviewIndicatesFailed("PER_STATUS: FAILED") {
		t.Fatal("failed")
	}
}

func TestHandleIterativeResult_PlanKind_StaleGen(t *testing.T) {
	m := Model{thinking: true, thinkingWindow: 0}
	m.windows[0].iterState = IterExecuting
	m.windows[0].iterGen = 3
	m.windows[0].iterCtx.Supervised = true
	if m.handleIterativeResult(iterativeResultMsg{win: 0, gen: 2, result: "plan", kind: perResultKindPlan}) {
		t.Fatal("stale should not repaint")
	}
	if m.windows[0].iterState != IterExecuting {
		t.Fatalf("state=%s", m.windows[0].iterState)
	}
}

func TestHandleIterativeResult_PlanKind_ToAwaitingExecutor(t *testing.T) {
	tmp := t.TempDir()
	m := Model{viewport: viewport.New(40, 10), ready: true, thinking: true, thinkingWindow: 0}
	m.windows[0].iterState = IterExecuting
	m.windows[0].iterGen = 1
	m.windows[0].iterCtx = IterativeContext{
		ProjectName: "p",
		TargetDir:   tmp,
		Goal:        "g",
		Supervised:  true,
		DesignFile:  filepath.Join(tmp, "d.md"),
		DodFile:     filepath.Join(tmp, "o.md"),
		LogFile:     filepath.Join(tmp, "l.log"),
		PlanFile:    filepath.Join(tmp, perPlanFileName),
	}
	m.handleIterativeResult(iterativeResultMsg{
		win: 0, gen: 1, kind: perResultKindPlan,
		result: "=== Plan phase ===\nconcrete plan steps",
	})
	if m.windows[0].iterState != IterAwaitingExecutor {
		t.Fatalf("state=%s want awaiting_executor", m.windows[0].iterState)
	}
	if !strings.Contains(m.windows[0].iterCtx.PlanText, "concrete plan steps") {
		t.Fatalf("PlanText=%q", m.windows[0].iterCtx.PlanText)
	}
	last := m.windows[0].Messages[len(m.windows[0].Messages)-1].Content
	if !strings.Contains(last, "SHIM_BRIEF") {
		t.Fatalf("expected brief: %s", last)
	}
}

func TestNormalizePEROpts_Defaults(t *testing.T) {
	opts := normalizePEROpts(perRunOpts{
		AppCfg: config.Config{},
		Mode:   perModeFull,
	})
	if opts.RetryBudget != perDefaultRetryBudget {
		t.Fatalf("RetryBudget=%d", opts.RetryBudget)
	}
	if opts.MaxExecAttempts != config.DefaultIterativeMaxIterations {
		t.Fatalf("MaxExecAttempts=%d want %d", opts.MaxExecAttempts, config.DefaultIterativeMaxIterations)
	}
	cfg := config.Config{Iterative: config.IterativeSettings{MaxIterations: 7}}
	opts2 := normalizePEROpts(perRunOpts{AppCfg: cfg, Mode: perModeExecuteReview})
	if opts2.MaxExecAttempts != 7 {
		t.Fatalf("MaxExecAttempts=%d", opts2.MaxExecAttempts)
	}
}
