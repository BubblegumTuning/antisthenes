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
	tea "github.com/charmbracelet/bubbletea"

	"github.com/nanami/antisthenes/config"
	"github.com/nanami/antisthenes/internal/agent"
	openai "github.com/sashabaranov/go-openai"
)

func TestIterativeState_String(t *testing.T) {
	if IterIdle.String() != "idle" || IterAwaitingConfirmation.String() != "awaiting_confirmation" {
		t.Error("unexpected state strings")
	}
	if IterAwaitingSupervised.String() != "awaiting_supervised" || IterAwaitingExecutor.String() != "awaiting_executor" {
		t.Error("unexpected supervised state strings")
	}
}

func TestIsConfirmationAndCancellation(t *testing.T) {
	if !isConfirmation("confirmed") || !isConfirmation("approved") {
		t.Error("expected confirmation")
	}
	if isConfirmation("not ready yet") {
		t.Error("should not confirm")
	}
	if !isCancellation("cancel") || !isCancellation("abort") {
		t.Error("expected cancellation")
	}
}

func TestParseSupervisedAndExecutor(t *testing.T) {
	if s, ok := parseSupervisedChoice(""); !ok || s {
		t.Fatalf("empty → unsupervised, got %v %v", s, ok)
	}
	if s, ok := parseSupervisedChoice("y"); !ok || !s {
		t.Fatalf("y → supervised")
	}
	if s, ok := parseSupervisedChoice("maybe"); ok {
		t.Fatalf("maybe should fail, got %v", s)
	}
	if e, ok := parseExecutorChoice("coder"); !ok || e != "coder" {
		t.Fatalf("coder: %v %v", e, ok)
	}
	if e, ok := parseExecutorChoice("executor: deep-thinker"); !ok || e != "deep-thinker" {
		t.Fatalf("deep-thinker: %v %v", e, ok)
	}
	if _, ok := parseExecutorChoice("gpt"); ok {
		t.Fatal("unknown executor")
	}
}

func TestIsPlanReady(t *testing.T) {
	if !isPlanReady("Please confirm when ready to proceed.") {
		t.Error("expected plan ready")
	}
	if isPlanReady("What is the project scope?") {
		t.Error("question should not be plan ready")
	}
}

func TestHandleIterativeSlash_ReentryGuard(t *testing.T) {
	m := Model{}
	m.windows[0].iterState = IterPlanning
	m.handleIterativeSlash()
	if len(m.windows[0].Messages) != 1 || !strings.Contains(m.windows[0].Messages[0].Content, "already active") {
		t.Fatalf("re-entry guard failed: %+v", m.windows[0].Messages)
	}
}

func TestHandleIterativeSlash_StartsPlanning(t *testing.T) {
	m := Model{}
	m.handleIterativeSlash()
	if m.windows[0].iterState != IterPlanning {
		t.Errorf("state = %s, want planning", m.windows[0].iterState)
	}
	if len(m.windows[0].Messages) != 1 || !strings.Contains(m.windows[0].Messages[0].Content, "name this project") {
		t.Error("project name prompt missing")
	}
}

func TestHandleIterativeInput_ProjectName_AsksSupervised(t *testing.T) {
	m := Model{viewport: viewport.New(40, 10), ready: true}
	m.windows[0].iterState = IterPlanning
	cmd, handled := m.handleIterativeInput("my-project")
	if !handled || cmd != nil {
		t.Fatalf("expected handled with no agent cmd yet: handled=%v cmd=%v", handled, cmd)
	}
	if m.windows[0].iterCtx.ProjectName != "my-project" || m.windows[0].iterCtx.TargetDir != "my-project" {
		t.Errorf("ctx not set: %+v", m.windows[0].iterCtx)
	}
	if m.windows[0].iterState != IterAwaitingSupervised {
		t.Errorf("state=%s want awaiting_supervised", m.windows[0].iterState)
	}
	last := m.windows[0].Messages[len(m.windows[0].Messages)-1].Content
	if !strings.Contains(last, "supervised mode") {
		t.Fatalf("expected supervised prompt, got %q", last)
	}
}

func TestHandleIterativeInput_SupervisedNo_StartsPlanningAgent(t *testing.T) {
	m := Model{
		viewport: viewport.New(40, 10),
		ready:    true,
		cfg:      config.Config{},
	}
	m.windows[0].iterState = IterAwaitingSupervised
	m.windows[0].iterCtx = IterativeContext{ProjectName: "p", TargetDir: "p"}
	cmd, handled := m.handleIterativeInput("n")
	if !handled || cmd == nil {
		t.Fatalf("expected agent batch cmd: handled=%v cmd=%v", handled, cmd)
	}
	if m.windows[0].iterCtx.Supervised {
		t.Error("expected unsupervised")
	}
	if m.windows[0].iterState != IterPlanning {
		t.Errorf("state=%s want planning", m.windows[0].iterState)
	}
}

func TestHandleIterativeInput_SupervisedYes(t *testing.T) {
	m := Model{
		viewport: viewport.New(40, 10),
		ready:    true,
		cfg:      config.Config{},
	}
	m.windows[0].iterState = IterAwaitingSupervised
	m.windows[0].iterCtx = IterativeContext{ProjectName: "p", TargetDir: "p"}
	_, handled := m.handleIterativeInput("y")
	if !handled || !m.windows[0].iterCtx.Supervised {
		t.Fatalf("supervised not set: handled=%v sup=%v", handled, m.windows[0].iterCtx.Supervised)
	}
}

func TestHandleIterativeInput_Cancel(t *testing.T) {
	m := Model{}
	m.windows[0].iterState = IterPlanning
	m.windows[0].iterCtx = IterativeContext{ProjectName: "p"}
	cmd, handled := m.handleIterativeInput("cancel")
	if !handled || cmd != nil {
		t.Fatalf("cancel: handled=%v cmd=%v", handled, cmd)
	}
	if m.windows[0].iterState != IterIdle {
		t.Errorf("state = %s, want idle after cancel", m.windows[0].iterState)
	}
}

func TestOnIterativeAgentResponse_AwaitingConfirmation(t *testing.T) {
	m := Model{}
	m.windows[0].iterState = IterPlanning
	m.windows[0].iterCtx = IterativeContext{ProjectName: "demo"}
	m.windows[0].Messages = []openai.ChatCompletionMessage{
		{Role: "assistant", Content: "Here is the plan. Please confirm when ready to proceed."},
	}
	m.onIterativeAgentResponse(0)
	if m.windows[0].iterState != IterAwaitingConfirmation {
		t.Errorf("state = %s, want awaiting_confirmation", m.windows[0].iterState)
	}
}

func TestCompleteIterativeExecution_AsyncNonBlocking(t *testing.T) {
	orig := delegateTaskFunc
	var mu sync.Mutex
	var calls []string
	var sawCtx context.Context
	delegateTaskFunc = func(goal string, cfg agent.DelegateConfig) agent.SubAgentResult {
		mu.Lock()
		calls = append(calls, goal)
		if sawCtx == nil {
			sawCtx = cfg.Ctx
		}
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		if cfg.Ctx != nil && cfg.Ctx.Err() != nil {
			return agent.SubAgentResult{Error: cfg.Ctx.Err()}
		}
		g := strings.ToUpper(goal)
		switch {
		case strings.Contains(g, "PER_PHASE: PLAN"):
			return agent.SubAgentResult{Result: "plan: step 1 do the thing"}
		case strings.Contains(g, "PER_PHASE: EXECUTE"):
			return agent.SubAgentResult{Result: "executed changes"}
		case strings.Contains(g, "PER_PHASE: REVIEW"):
			return agent.SubAgentResult{Result: "criteria met\nPER_STATUS: DONE"}
		default:
			return agent.SubAgentResult{Result: "mocked worker done"}
		}
	}
	defer func() { delegateTaskFunc = orig }()

	tmp := t.TempDir()
	m := Model{
		viewport: viewport.New(80, 20),
		cfg:      config.Config{AutoScroll: true},
		ready:    true,
	}
	m.windows[0].iterState = IterAwaitingConfirmation
	m.windows[0].iterCtx = IterativeContext{
		ProjectName: "demo",
		TargetDir:   tmp,
		Goal:        "build a thing",
		Supervised:  false,
	}
	m.windows[0].Messages = []openai.ChatCompletionMessage{
		{Role: "user", Content: "build a thing"},
		{Role: "assistant", Content: "Plan ready. Please confirm."},
	}

	start := time.Now()
	cmd := m.completeIterativeExecution(0)
	if elapsed := time.Since(start); elapsed > 40*time.Millisecond {
		t.Fatalf("completeIterativeExecution blocked for %v (must return immediately)", elapsed)
	}
	if cmd == nil {
		t.Fatal("expected tea.Cmd for async worker")
	}
	if m.windows[0].iterState != IterExecuting {
		t.Fatalf("state = %s, want executing", m.windows[0].iterState)
	}
	if !m.thinking {
		t.Error("expected thinking spinner during async worker")
	}
	last := m.windows[0].Messages[len(m.windows[0].Messages)-1].Content
	if !strings.Contains(last, "async") || !strings.Contains(last, "PER") {
		t.Fatalf("expected async PER start message, got %q", last)
	}

	resMsg := collectIterativeResult(t, cmd, 3*time.Second)
	if resMsg.win != 0 {
		t.Fatalf("win=%d want 0", resMsg.win)
	}
	if resMsg.gen != m.windows[0].iterGen {
		t.Fatalf("gen mismatch: msg=%d model=%d", resMsg.gen, m.windows[0].iterGen)
	}
	if resMsg.kind != perResultKindFull {
		t.Fatalf("kind=%q want full", resMsg.kind)
	}
	if sawCtx == nil {
		t.Error("expected DelegateConfig.Ctx to be set")
	}
	mu.Lock()
	nCalls := len(calls)
	mu.Unlock()
	if nCalls < 3 {
		t.Fatalf("expected Plan+Execute+Review (>=3) delegate calls, got %d", nCalls)
	}
	if !strings.Contains(resMsg.result, "Plan phase") && !strings.Contains(resMsg.result, "DONE") {
		t.Fatalf("unexpected result: %q", resMsg.result)
	}

	m.handleIterativeResult(resMsg)
	if m.windows[0].iterState != IterIdle {
		t.Errorf("state = %s, want idle after result", m.windows[0].iterState)
	}
	if m.thinking {
		t.Error("thinking should clear after result")
	}

	// per_log + per_done + plan artifact
	if _, err := os.Stat(filepath.Join(tmp, perLogFileName)); err != nil {
		t.Fatalf("per_log missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, perDoneSignal)); err != nil {
		t.Fatalf("per_done.signal missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, perPlanFileName)); err != nil {
		t.Fatalf("per_plan.md missing: %v", err)
	}

	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) < 3 {
		t.Fatalf("expected design/dod/log under %s, got %v", tmp, entries)
	}
	var designPath string
	for _, e := range entries {
		if strings.Contains(e.Name(), "design") {
			designPath = filepath.Join(tmp, e.Name())
			break
		}
	}
	if designPath == "" {
		t.Fatal("design file missing")
	}
	body, err := os.ReadFile(designPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "build a thing") {
		t.Errorf("design missing goal/transcript: %s", body)
	}
}

func TestCompleteIterative_Supervised_PlanThenShim(t *testing.T) {
	orig := delegateTaskFunc
	delegateTaskFunc = func(goal string, cfg agent.DelegateConfig) agent.SubAgentResult {
		if cfg.Ctx != nil && cfg.Ctx.Err() != nil {
			return agent.SubAgentResult{Error: cfg.Ctx.Err()}
		}
		if strings.Contains(strings.ToUpper(goal), "PER_PHASE: PLAN") {
			return agent.SubAgentResult{Result: "supervised plan body: implement X"}
		}
		return agent.SubAgentResult{Result: "unexpected phase"}
	}
	defer func() { delegateTaskFunc = orig }()

	tmp := t.TempDir()
	m := Model{
		viewport: viewport.New(80, 20),
		cfg:      config.Config{AutoScroll: true},
		ready:    true,
	}
	m.windows[0].iterState = IterAwaitingConfirmation
	m.windows[0].iterCtx = IterativeContext{
		ProjectName: "sup",
		TargetDir:   tmp,
		Goal:        "supervised goal",
		Supervised:  true,
	}
	m.windows[0].Messages = []openai.ChatCompletionMessage{
		{Role: "user", Content: "supervised goal"},
		{Role: "assistant", Content: "Plan. Please confirm."},
	}

	cmd := m.completeIterativeExecution(0)
	if cmd == nil {
		t.Fatal("supervised confirm should start Plan phase async")
	}
	if m.windows[0].iterState != IterExecuting {
		t.Fatalf("state=%s want executing (plan)", m.windows[0].iterState)
	}
	res := collectIterativeResult(t, cmd, 2*time.Second)
	if res.kind != perResultKindPlan {
		t.Fatalf("kind=%q want plan", res.kind)
	}
	m.handleIterativeResult(res)
	if m.windows[0].iterState != IterAwaitingExecutor {
		t.Fatalf("state=%s want awaiting_executor", m.windows[0].iterState)
	}
	last := m.windows[0].Messages[len(m.windows[0].Messages)-1].Content
	if !strings.Contains(last, "<!--SHIM_BRIEF_START-->") || !strings.Contains(last, "<!--SHIM_BRIEF_END-->") {
		t.Fatalf("missing SHIM tags: %s", last)
	}
	if !strings.Contains(last, "supervised plan body") {
		t.Fatalf("brief missing plan output: %s", last)
	}
	if !strings.Contains(last, "Reply with executor:") {
		t.Fatalf("missing executor prompt: %s", last)
	}
	if m.windows[0].iterCtx.DesignFile == "" {
		t.Error("scaffold paths not stored")
	}
	if m.windows[0].iterCtx.PlanText == "" {
		t.Error("PlanText not stored")
	}
}

func TestSupervised_ExecutorStartsExecuteReview(t *testing.T) {
	orig := delegateTaskFunc
	var mu sync.Mutex
	var execNames []string
	var phases []string
	delegateTaskFunc = func(goal string, cfg agent.DelegateConfig) agent.SubAgentResult {
		mu.Lock()
		execNames = append(execNames, cfg.ExecutorName)
		g := strings.ToUpper(goal)
		switch {
		case strings.Contains(g, "PER_PHASE: EXECUTE"):
			phases = append(phases, "execute")
			mu.Unlock()
			return agent.SubAgentResult{Result: "done via coder"}
		case strings.Contains(g, "PER_PHASE: REVIEW"):
			phases = append(phases, "review")
			mu.Unlock()
			return agent.SubAgentResult{Result: "ok\nPER_STATUS: DONE"}
		default:
			phases = append(phases, "other")
			mu.Unlock()
			return agent.SubAgentResult{Result: "other"}
		}
	}
	defer func() { delegateTaskFunc = orig }()

	tmp := t.TempDir()
	design := filepath.Join(tmp, "d.md")
	dod := filepath.Join(tmp, "dod.md")
	logf := filepath.Join(tmp, "l.log")
	plan := filepath.Join(tmp, perPlanFileName)
	_ = os.WriteFile(design, []byte("d"), 0o644)
	_ = os.WriteFile(dod, []byte("o"), 0o644)
	_ = os.WriteFile(logf, []byte("l"), 0o644)
	_ = os.WriteFile(plan, []byte("prior plan"), 0o644)

	m := Model{
		viewport: viewport.New(40, 10),
		cfg:      config.Config{},
		ready:    true,
	}
	m.windows[0].iterState = IterAwaitingExecutor
	m.windows[0].iterCtx = IterativeContext{
		ProjectName: "sup",
		TargetDir:   tmp,
		Goal:        "g",
		Supervised:  true,
		DesignFile:  design,
		DodFile:     dod,
		LogFile:     logf,
		PlanFile:    plan,
		PlanText:    "prior plan",
	}
	cmd, handled := m.handleIterativeInput("coder")
	if !handled || cmd == nil {
		t.Fatalf("expected worker cmd: handled=%v", handled)
	}
	if m.windows[0].iterState != IterExecuting {
		t.Fatalf("state=%s", m.windows[0].iterState)
	}
	res := collectIterativeResult(t, cmd, 2*time.Second)
	if res.kind != perResultKindFull {
		t.Fatalf("kind=%q want full", res.kind)
	}
	mu.Lock()
	defer mu.Unlock()
	foundCoder := false
	for i, p := range phases {
		if p == "execute" && i < len(execNames) && execNames[i] == "coder" {
			foundCoder = true
		}
	}
	if !foundCoder {
		t.Errorf("Execute should use ExecutorName=coder; phases=%v execs=%v", phases, execNames)
	}
	if !strings.Contains(res.result, "done via coder") && !strings.Contains(res.result, "DONE") {
		t.Fatalf("result=%q", res.result)
	}
	m.handleIterativeResult(res)
	if m.windows[0].iterState != IterIdle {
		t.Fatalf("want idle after execute+review, got %s", m.windows[0].iterState)
	}
}

func TestHandleIterativeResult_StaleGenDiscarded(t *testing.T) {
	m := Model{
		thinking: true,
	}
	m.windows[0].iterState = IterExecuting
	m.windows[0].iterGen = 2
	if m.handleIterativeResult(iterativeResultMsg{win: 0, gen: 1, result: "old"}) {
		t.Error("stale result should not repaint")
	}
	if m.windows[0].iterState != IterExecuting {
		t.Error("stale result must not change state")
	}
	if len(m.windows[0].Messages) != 0 {
		t.Error("stale result must not append messages")
	}
}

func TestCancelDuringExecuting_InvalidatesWorker(t *testing.T) {
	orig := delegateTaskFunc
	entered := make(chan struct{})
	delegateTaskFunc = func(_ string, cfg agent.DelegateConfig) agent.SubAgentResult {
		close(entered)
		<-cfg.Ctx.Done()
		return agent.SubAgentResult{Error: cfg.Ctx.Err()}
	}
	defer func() { delegateTaskFunc = orig }()

	tmp := t.TempDir()
	m := Model{
		viewport: viewport.New(40, 10),
		cfg:      config.Config{},
		ready:    true,
	}
	m.windows[0].iterState = IterAwaitingConfirmation
	m.windows[0].iterCtx = IterativeContext{
		ProjectName: "x",
		TargetDir:   tmp,
		Goal:        "g",
	}
	cmd := m.completeIterativeExecution(0)
	genBeforeCancel := m.windows[0].iterGen
	if cmd == nil {
		t.Fatal("expected cmd")
	}

	done := make(chan iterativeResultMsg, 1)
	go func() {
		msg := collectIterativeResult(t, cmd, 3*time.Second)
		done <- msg
	}()

	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("worker never started")
	}

	m.cancelIterative()
	if m.windows[0].iterState != IterIdle {
		t.Fatalf("want idle after cancel, got %s", m.windows[0].iterState)
	}
	if m.windows[0].iterGen <= genBeforeCancel {
		t.Error("cancel should bump iterGen")
	}

	var late iterativeResultMsg
	select {
	case late = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not finish after cancel")
	}
	if late.gen != genBeforeCancel {
		t.Fatalf("late msg gen=%d want %d", late.gen, genBeforeCancel)
	}
	_ = m.handleIterativeResult(late)
	for _, msg := range m.windows[0].Messages {
		if strings.Contains(msg.Content, "Sub-agent completed") || strings.Contains(msg.Content, "should not") {
			t.Fatalf("late result leaked: %+v", m.windows[0].Messages)
		}
	}
}

func TestCtrlCDuringExecuting_DoesNotQuit(t *testing.T) {
	m := Model{ready: true, viewport: viewport.New(40, 10)}
	m.windows[0].iterState = IterExecuting
	_, cancel := context.WithCancel(context.Background())
	m.windows[0].iterCancel = cancel
	m.windows[0].iterGen = 1

	cmd, handled := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !handled {
		t.Fatal("expected handled")
	}
	if cmd != nil {
		t.Fatalf("expected nil cmd (not Quit), got %T", cmd)
	}
	if m.windows[0].iterState != IterIdle {
		t.Errorf("state=%s want idle", m.windows[0].iterState)
	}
}

func TestPlanningNotesFromMessages(t *testing.T) {
	notes := planningNotesFromMessages([]openai.ChatCompletionMessage{
		{Role: "system", Content: "ignore"},
		{Role: "user", Content: "I want a CLI"},
		{Role: "assistant", Content: "What language?"},
		{Role: "user", Content: "confirmed"},
	})
	if !strings.Contains(notes, "I want a CLI") || !strings.Contains(notes, "What language?") {
		t.Fatalf("notes incomplete: %s", notes)
	}
	if strings.Contains(notes, "confirmed") {
		t.Error("confirmation should be filtered")
	}
}

func TestPrepareIterativeScaffold_StructuredContent(t *testing.T) {
	dir := t.TempDir()
	messages := []openai.ChatCompletionMessage{
		{Role: "user", Content: "Build a todo CLI in Go with add and list"},
		{Role: "assistant", Content: "What storage?"},
		{Role: "user", Content: "JSON file, stdlib only"},
		{Role: "assistant", Content: `Agreed plan for the todo CLI.

## Requirements
- Add todos to a local JSON file
- List all todos
- Use Go standard library only

## Approach
1. Create main.go with subcommands via flag.Args
2. Persist todos in ./todos.json

## Risks
- Concurrent writes are out of scope for v1

## Definition of Done
- [ ] go build ./... succeeds
- [ ] add command appends a todo
- [ ] list command prints todos

Please confirm when ready to proceed.`},
	}

	designPath, dodPath, logPath, err := prepareIterativeScaffold("todo CLI in Go", dir, messages)
	if err != nil {
		t.Fatal(err)
	}
	if designPath == "" || dodPath == "" || logPath == "" {
		t.Fatalf("empty paths: %q %q %q", designPath, dodPath, logPath)
	}

	design, err := os.ReadFile(designPath)
	if err != nil {
		t.Fatal(err)
	}
	ds := string(design)
	for _, want := range []string{
		"## Goal",
		"todo CLI in Go",
		"## Requirements",
		"Add todos to a local JSON file",
		"List all todos",
		"Use Go standard library only",
		"## Approach",
		"Create main.go",
		"## Risks",
		"Concurrent writes",
		"## Planning Transcript",
		"JSON file, stdlib only",
	} {
		if !strings.Contains(ds, want) {
			t.Errorf("design missing %q\n---\n%s", want, ds)
		}
	}
	if strings.Contains(ds, "(See planning transcript above; refine during implementation as needed.)") {
		t.Error("design still uses empty approach stub")
	}
	if strings.Contains(ds, "## Clarified Requirements") && !strings.Contains(ds, "## Requirements") {
		t.Error("expected structured Requirements section")
	}

	dod, err := os.ReadFile(dodPath)
	if err != nil {
		t.Fatal(err)
	}
	dods := string(dod)
	for _, want := range []string{
		"## Success Criteria",
		"- [ ] go build ./... succeeds",
		"- [ ] add command appends a todo",
		"- [ ] list command prints todos",
		"todo CLI in Go",
	} {
		if !strings.Contains(dods, want) {
			t.Errorf("DoD missing %q\n---\n%s", want, dods)
		}
	}

	logBody, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logBody), "todo CLI in Go") {
		t.Errorf("log missing goal: %s", logBody)
	}
}

func TestLastAssistantPlanMessage_PrefersPlanReady(t *testing.T) {
	msgs := []openai.ChatCompletionMessage{
		{Role: "assistant", Content: "Please name this project."},
		{Role: "assistant", Content: "Early thoughts without confirm cue."},
		{Role: "assistant", Content: "Final plan with steps.\nPlease confirm."},
		{Role: "assistant", Content: "<!--SHIM_BRIEF_START-->x<!--SHIM_BRIEF_END-->"},
	}
	got := lastAssistantPlanMessage(msgs)
	if !strings.Contains(got, "Final plan with steps") {
		t.Fatalf("want plan-ready message, got %q", got)
	}
}

func TestExtractRequirementAndDoD_FromFixture(t *testing.T) {
	plan := `## Requirements
- Must support dry-run
## Definition of Done
- [ ] dry-run prints plan only
Please confirm.`
	goal := "deploy helper"
	msgs := []openai.ChatCompletionMessage{
		{Role: "user", Content: "Need a deploy helper"},
		{Role: "assistant", Content: plan},
	}
	reqs := extractRequirementBullets(goal, plan, msgs)
	if !containsString(reqs, "Must support dry-run") {
		t.Fatalf("reqs=%v", reqs)
	}
	items := extractDoDItems(goal, plan, reqs)
	if !containsString(items, "dry-run prints plan only") {
		t.Fatalf("dod=%v", items)
	}
}

func containsString(items []string, want string) bool {
	for _, s := range items {
		if s == want || strings.Contains(s, want) {
			return true
		}
	}
	return false
}

func TestBuildShimBrief(t *testing.T) {
	s := buildShimBrief(IterativeContext{
		ProjectName: "p",
		TargetDir:   "p",
		Goal:        "g",
		DesignFile:  "d.md",
		DodFile:     "o.md",
		LogFile:     "l.log",
		PlanFile:    "per_plan.md",
	}, "notes here")
	if !strings.Contains(s, "<!--SHIM_BRIEF_START-->") || !strings.Contains(s, "notes here") {
		t.Fatal(s)
	}
	if !strings.Contains(s, "Plan phase output") || !strings.Contains(s, "per_plan.md") {
		t.Fatalf("expected plan fields: %s", s)
	}
}

func TestBuildIterativeWorkerGoal_UsesConfigThresholds(t *testing.T) {
	cfg := config.Config{Iterative: config.IterativeSettings{
		ContextRemindPercent:  41,
		ContextSummaryPercent: 72,
		MaxIterations:         9,
	}}
	got := buildIterativeWorkerGoal(cfg, "/tmp/proj", "d.md", "o.md", "l.log")
	for _, want := range []string{
		"~41%",
		"~72%",
		"9 iterations",
		"d.md",
		"o.md",
		"l.log",
		"/tmp/proj",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("worker goal missing %q\n%s", want, got)
		}
	}
	// Defaults when unset
	def := buildIterativeWorkerGoal(config.Config{}, ".", "a", "b", "c")
	if !strings.Contains(def, "~55%") || !strings.Contains(def, "~60%") || !strings.Contains(def, "40 iterations") {
		t.Fatalf("defaults missing in seed:\n%s", def)
	}
}

func TestConcurrentIterative_TwoWindowsIndependent(t *testing.T) {
	orig := delegateTaskFunc
	var mu sync.Mutex
	starts := 0
	delegateTaskFunc = func(_ string, cfg agent.DelegateConfig) agent.SubAgentResult {
		mu.Lock()
		starts++
		mu.Unlock()
		// stay until cancelled or short sleep
		select {
		case <-cfg.Ctx.Done():
			return agent.SubAgentResult{Error: cfg.Ctx.Err()}
		case <-time.After(50 * time.Millisecond):
			return agent.SubAgentResult{Result: "worker-ok"}
		}
	}
	defer func() { delegateTaskFunc = orig }()

	tmpA := t.TempDir()
	tmpB := t.TempDir()
	m := Model{
		viewport:     viewport.New(40, 10),
		cfg:          config.Config{},
		ready:        true,
		activeWindow: 0,
	}
	m.windows[0].SessionID = "a"
	m.windows[1].SessionID = "b"

	// Start job on window 0
	m.windows[0].iterState = IterAwaitingConfirmation
	m.windows[0].iterCtx = IterativeContext{ProjectName: "a", TargetDir: tmpA, Goal: "ga"}
	cmdA := m.completeIterativeExecution(0)
	if cmdA == nil || m.windows[0].iterState != IterExecuting {
		t.Fatalf("A not executing: state=%s cmd=%v", m.windows[0].iterState, cmdA != nil)
	}
	genA := m.windows[0].iterGen

	// Switch active and start job on window 1 without cancelling A
	m.activeWindow = 1
	m.windows[1].iterState = IterAwaitingConfirmation
	m.windows[1].iterCtx = IterativeContext{ProjectName: "b", TargetDir: tmpB, Goal: "gb"}
	cmdB := m.completeIterativeExecution(1)
	if cmdB == nil || m.windows[1].iterState != IterExecuting {
		t.Fatalf("B not executing: state=%s", m.windows[1].iterState)
	}
	if m.windows[0].iterState != IterExecuting {
		t.Fatalf("A should still execute, got %s", m.windows[0].iterState)
	}
	genB := m.windows[1].iterGen

	// Progress for A must not touch B's messages/snippet
	beforeB := len(m.windows[1].Messages)
	m.handleIterativeLogProgress(iterativeLogProgressMsg{
		win: 0, gen: genA, chunk: "log-a\n", newOffset: 6,
	})
	if len(m.windows[1].Messages) != beforeB {
		t.Fatalf("B polluted by A progress: before=%d after=%d msgs=%+v", beforeB, len(m.windows[1].Messages), m.windows[1].Messages)
	}
	for _, msg := range m.windows[1].Messages {
		if strings.Contains(msg.Content, "log-a") || strings.Contains(msg.Content, "[iterative log]") {
			t.Fatalf("B got A log progress: %+v", msg)
		}
	}
	if m.windows[0].iterProgressSnippet != "log-a" {
		t.Fatalf("A snippet=%q", m.windows[0].iterProgressSnippet)
	}
	if m.windows[1].iterProgressSnippet != "" {
		t.Fatalf("B snippet should be empty, got %q", m.windows[1].iterProgressSnippet)
	}

	// Result for A while active is B: A completes, B still running
	m.handleIterativeResult(iterativeResultMsg{win: 0, gen: genA, result: "done-a"})
	if m.windows[0].iterState != IterIdle {
		t.Fatalf("A want idle, got %s", m.windows[0].iterState)
	}
	if m.windows[1].iterState != IterExecuting {
		t.Fatalf("B should still execute, got %s", m.windows[1].iterState)
	}
	// B was thinking owner after start — thinking may still be true for win 1
	foundA := false
	for _, msg := range m.windows[0].Messages {
		if strings.Contains(msg.Content, "done-a") {
			foundA = true
		}
	}
	if !foundA {
		t.Fatalf("A missing completion: %+v", m.windows[0].Messages)
	}
	for _, msg := range m.windows[1].Messages {
		if strings.Contains(msg.Content, "done-a") {
			t.Fatalf("B got A's result: %+v", m.windows[1].Messages)
		}
	}

	// Cancel B (active) leaves A idle (already) and B idle
	m.cancelIterative()
	if m.windows[1].iterState != IterIdle {
		t.Fatalf("B cancel want idle, got %s", m.windows[1].iterState)
	}
	_ = genB
}

func TestConcurrentIterative_ReentryPerWindow(t *testing.T) {
	m := Model{activeWindow: 0}
	m.windows[0].iterState = IterPlanning
	m.windows[1].SessionID = "b"
	m.handleIterativeSlash()
	if len(m.windows[0].Messages) == 0 || !strings.Contains(m.windows[0].Messages[0].Content, "already active") {
		t.Fatalf("win0 re-entry: %+v", m.windows[0].Messages)
	}
	// other idle window can start
	m.activeWindow = 1
	m.handleIterativeSlash()
	if m.windows[1].iterState != IterPlanning {
		t.Fatalf("win1 state=%s want planning", m.windows[1].iterState)
	}
	if m.windows[0].iterState != IterPlanning {
		t.Fatalf("win0 should remain planning, got %s", m.windows[0].iterState)
	}
}

func TestConcurrentIterative_CancelActiveLeavesOther(t *testing.T) {
	m := Model{ready: true, viewport: viewport.New(40, 10), activeWindow: 0}
	_, cancel0 := context.WithCancel(context.Background())
	_, cancel1 := context.WithCancel(context.Background())
	m.windows[0].iterState = IterExecuting
	m.windows[0].iterGen = 1
	m.windows[0].iterCancel = cancel0
	m.windows[1].SessionID = "b"
	m.windows[1].iterState = IterExecuting
	m.windows[1].iterGen = 2
	m.windows[1].iterCancel = cancel1

	m.cancelIterative() // cancels active (0)
	if m.windows[0].iterState != IterIdle {
		t.Fatalf("win0=%s", m.windows[0].iterState)
	}
	if m.windows[1].iterState != IterExecuting {
		t.Fatalf("win1 should still execute, got %s", m.windows[1].iterState)
	}
	if m.windows[1].iterGen != 2 {
		t.Fatalf("win1 gen bumped unexpectedly: %d", m.windows[1].iterGen)
	}
}

// collectIterativeResult runs a tea.Cmd tree (including BatchMsg) until iterativeResultMsg.
func collectIterativeResult(t *testing.T, cmd tea.Cmd, timeout time.Duration) iterativeResultMsg {
	t.Helper()
	if cmd == nil {
		t.Fatal("nil cmd")
	}
	deadline := time.Now().Add(timeout)
	results := make(chan tea.Msg, 16)

	worker := func(c tea.Cmd) {
		if c == nil {
			return
		}
		results <- c()
	}

	go worker(cmd)

	for {
		if time.Now().After(deadline) {
			t.Fatal("timeout waiting for iterativeResultMsg")
		}
		select {
		case msg := <-results:
			if msg == nil {
				continue
			}
			if batch, ok := msg.(tea.BatchMsg); ok {
				for _, bc := range batch {
					if bc == nil {
						continue
					}
					go worker(bc)
				}
				continue
			}
			if res, ok := msg.(iterativeResultMsg); ok {
				return res
			}
		case <-time.After(20 * time.Millisecond):
		}
	}
}
