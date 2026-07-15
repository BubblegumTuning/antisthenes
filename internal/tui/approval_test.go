package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/nanami/antisthenes/config"
	"github.com/nanami/antisthenes/internal/agent"
	openai "github.com/sashabaranov/go-openai"
)

func TestApprovalUI_RequestRespond(t *testing.T) {
	a := newApprovalUI()
	done := make(chan approvalDecision, 1)
	ready := make(chan struct{})
	go func() {
		close(ready)
		done <- a.begin(agent.ApprovalRequest{Tool: "bash", Command: "rm -rf /tmp/foo"})
	}()
	<-ready
	if !a.isActive() || a.currentTool() != "bash" || a.currentCommand() != "rm -rf /tmp/foo" {
		t.Fatal("approval should be active")
	}
	if !a.respond(approvalDecision{approved: true, level: agent.ApprovalOnce}) {
		t.Fatal("respond failed")
	}
	d := <-done
	if !d.approved {
		t.Error("expected approved")
	}
	if a.isActive() {
		t.Error("should be inactive after respond")
	}
}

func TestRenderModalOverlay_Approval(t *testing.T) {
	m := Model{
		ready:  true,
		width:  80,
		height: 24,
		approval: func() *approvalUI {
			a := newApprovalUI()
			a.mu.Lock()
			a.active = true
			a.tool = "bash"
			a.command = "rm -rf /tmp"
			a.mu.Unlock()
			return a
		}(),
	}
	out := m.renderModalOverlay("background")
	if !strings.Contains(out, "Approval Required") || !strings.Contains(out, "bash: rm -rf /tmp") {
		t.Errorf("modal missing content: %s", out)
	}
}

func TestRenderModalOverlay_Centered(t *testing.T) {
	m := Model{
		ready:          true,
		width:          80,
		height:         24,
		confirmCommand: "/clear",
	}
	out := m.renderModalOverlay("background layout")
	lines := strings.Split(out, "\n")
	if len(lines) != 24 {
		t.Fatalf("centered overlay height = %d, want 24", len(lines))
	}
	idx := -1
	for i, line := range lines {
		if strings.Contains(line, "Confirm Action") {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatal("modal title not found")
	}
	if idx == 0 {
		t.Error("modal should not be on first line when centered")
	}
	if idx >= 20 {
		t.Errorf("modal too low (line %d); expected vertical centering", idx)
	}
}

func TestHandleKeyMsg_BlocksWhenModalActive(t *testing.T) {
	m := Model{
		ready:          true,
		width:          80,
		height:         24,
		confirmCommand: "/clear",
		textInput:      textarea.New(),
	}
	m.textInput.SetValue("not a command")
	_, handled := (&m).handleKeyMsg(tea.KeyMsg{Type: tea.KeyEnter})
	if !handled {
		t.Fatal("expected modal to consume Enter")
	}
	if m.confirmCommand != "/clear" {
		t.Error("confirm should remain until y/n")
	}
	if m.thinking {
		t.Error("modal should block agent submit")
	}
	_, handled = (&m).handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if !handled {
		t.Fatal("expected modal to block unrelated keys")
	}
}

func TestHandleModalKey_DenyApproval(t *testing.T) {
	a := newApprovalUI()
	done := make(chan approvalDecision, 1)
	ready := make(chan struct{})
	go func() {
		close(ready)
		done <- a.begin(agent.ApprovalRequest{Tool: "kill_process", Command: "shutdown now"})
	}()
	<-ready

	m := Model{
		approval:  a,
		width:     80,
		height:    24,
		ready:     true,
		textInput: textarea.New(),
	}
	_, handled := (&m).handleModalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if !handled {
		t.Fatal("expected handled")
	}
	d := <-done
	if d.approved {
		t.Error("expected deny")
	}
	if a.isActive() {
		t.Error("should clear after respond")
	}
}

func TestHandleModalKey_ApprovalAlwaysPersistsConfig(t *testing.T) {
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWD) }()

	if err := os.WriteFile("config.json", []byte(`{"agent_name":"T"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	a := newApprovalUI()
	done := make(chan approvalDecision, 1)
	ready := make(chan struct{})
	go func() {
		close(ready)
		done <- a.begin(agent.ApprovalRequest{Tool: "bash", Command: "rm -rf /tmp/x"})
	}()
	<-ready

	m := Model{
		approval:  a,
		cfg:       config.Load(),
		ready:     true,
		width:     80,
		height:    24,
		textInput: textarea.New(),
	}
	_, handled := (&m).handleModalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if !handled {
		t.Fatal("expected handled")
	}
	d := <-done
	if !d.approved {
		t.Fatal("always should approve")
	}
	if !m.cfg.ApprovalsWithoutConfirm["bash"] {
		t.Fatal("always should set approvals_without_confirm.bash on model")
	}
	reloaded := config.Load()
	if !reloaded.ApprovalsWithoutConfirm["bash"] {
		t.Fatal("always should persist approvals_without_confirm.bash to config.json")
	}
}

func TestHandleModalKey_ClearAlwaysPersistsConfig(t *testing.T) {
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWD) }()

	if err := os.WriteFile("config.json", []byte(`{"agent_name":"T"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	m := Model{
		confirmCommand: "/clear",
		ready:          true,
		width:          80,
		height:         24,
		viewport:       viewport.New(80, 18),
		textInput:      textarea.New(),
		cfg:            config.Load(),
	}
	m.windows[0].Messages = []openai.ChatCompletionMessage{{Role: "user", Content: "hello"}}

	_, handled := (&m).handleModalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if !handled {
		t.Fatal("expected handled")
	}
	if len(m.windows[0].Messages) != 0 {
		t.Error("always should clear messages")
	}
	if !m.cfg.ClearWithoutConfirm {
		t.Error("always should set clear_without_confirm on model")
	}
	reloaded := config.Load()
	if !reloaded.ClearWithoutConfirm {
		t.Error("always should persist clear_without_confirm to config.json")
	}
}

func TestView_ClearConfirmModal(t *testing.T) {
	m := Model{
		ready:          true,
		width:          80,
		height:         24,
		viewport:       viewport.New(80, 18),
		textInput:      textarea.New(),
		cfg:            config.Config{EditHeight: 3, MaxTokens: 160000},
		confirmCommand: "/clear",
		approval:       newApprovalUI(),
	}
	out := m.View()
	if !strings.Contains(out, "Confirm Action") || !strings.Contains(out, "/clear") {
		t.Errorf("clear modal not rendered: %s", out)
	}
}
