package tui

import (
	"strings"
	"testing"

	"github.com/nanami/antisthenes/internal/agent"
	openai "github.com/sashabaranov/go-openai"
)

func TestIterativeState_String(t *testing.T) {
	if IterIdle.String() != "idle" || IterAwaitingConfirmation.String() != "awaiting_confirmation" {
		t.Error("unexpected state strings")
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

func TestIsPlanReady(t *testing.T) {
	if !isPlanReady("Please confirm when ready to proceed.") {
		t.Error("expected plan ready")
	}
	if isPlanReady("What is the project scope?") {
		t.Error("question should not be plan ready")
	}
}

func TestHandleIterativeSlash_ReentryGuard(t *testing.T) {
	m := Model{iterState: IterPlanning}
	m.handleIterativeSlash()
	if len(m.windows[0].Messages) != 1 || !strings.Contains(m.windows[0].Messages[0].Content, "already active") {
		t.Fatalf("re-entry guard failed: %+v", m.windows[0].Messages)
	}
}

func TestHandleIterativeSlash_StartsPlanning(t *testing.T) {
	m := Model{}
	m.handleIterativeSlash()
	if m.iterState != IterPlanning {
		t.Errorf("state = %s, want planning", m.iterState)
	}
	if len(m.windows[0].Messages) != 1 || !strings.Contains(m.windows[0].Messages[0].Content, "name this project") {
		t.Error("project name prompt missing")
	}
}

func TestHandleIterativeInput_ProjectName(t *testing.T) {
	m := Model{iterState: IterPlanning}
	cmd, handled := m.handleIterativeInput("my-project")
	if !handled || cmd == nil {
		t.Fatalf("expected handled with agent cmd: handled=%v cmd=%v", handled, cmd)
	}
	if m.iterCtx.ProjectName != "my-project" || m.iterCtx.TargetDir != "my-project" {
		t.Errorf("ctx not set: %+v", m.iterCtx)
	}
}

func TestHandleIterativeInput_Cancel(t *testing.T) {
	m := Model{iterState: IterPlanning, iterCtx: IterativeContext{ProjectName: "p"}}
	cmd, handled := m.handleIterativeInput("cancel")
	if !handled || cmd != nil {
		t.Fatalf("cancel: handled=%v cmd=%v", handled, cmd)
	}
	if m.iterState != IterIdle {
		t.Errorf("state = %s, want idle after cancel", m.iterState)
	}
}

func TestOnIterativeAgentResponse_AwaitingConfirmation(t *testing.T) {
	m := Model{
		iterState: IterPlanning,
		iterCtx:   IterativeContext{ProjectName: "demo"},
	}
	m.windows[0].Messages = []openai.ChatCompletionMessage{
		{Role: "assistant", Content: "Here is the plan. Please confirm when ready to proceed."},
	}
	m.onIterativeAgentResponse()
	if m.iterState != IterAwaitingConfirmation {
		t.Errorf("state = %s, want awaiting_confirmation", m.iterState)
	}
}

func TestCompleteIterativeExecution(t *testing.T) {
	orig := delegateTaskFunc
	delegateTaskFunc = func(_ string, _ agent.DelegateConfig) agent.SubAgentResult {
		return agent.SubAgentResult{Result: "mocked worker done"}
	}
	defer func() { delegateTaskFunc = orig }()

	m := Model{
		iterState: IterAwaitingConfirmation,
		iterCtx: IterativeContext{
			ProjectName: "demo",
			TargetDir:   "demo",
			Goal:        "build a thing",
		},
	}
	m.completeIterativeExecution()
	if m.iterState != IterIdle {
		t.Errorf("state = %s, want idle after completion", m.iterState)
	}
	if len(m.windows[0].Messages) != 1 || !strings.Contains(m.windows[0].Messages[0].Content, "mocked worker done") {
		t.Errorf("completion message wrong: %+v", m.windows[0].Messages)
	}
}
