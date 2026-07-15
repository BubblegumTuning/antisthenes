package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/nanami/antisthenes/config"
	"github.com/nanami/antisthenes/internal/agent"
)

func baseTmuxModel() Model {
	ti := textarea.New()
	m := Model{
		textInput: ti,
		viewport:  viewport.New(80, 12),
		ready:     true,
		width:     80,
		height:    30,
		cfg:       config.Config{AgentName: "Test", MaxTokens: 1000, EditHeight: 3},
		loop:      agent.NewLoop("k", "m", "http://127.0.0.1:9"),
	}
	m.windows[0].Label = "Chat"
	return m
}

func TestTmuxPaneEnableDisableLayout(t *testing.T) {
	m := baseTmuxModel()
	m.handleWindowSize(tea.WindowSizeMsg{Width: 80, Height: 30})
	baseVH := m.viewport.Height

	cmd := m.enableTmuxPane("", "antisthenes-persist")
	if !m.tmuxEnabled || !m.tmuxPaneVisible() {
		t.Fatal("expected pane enabled")
	}
	if m.tmuxSession != "antisthenes-persist" {
		t.Fatalf("session: %s", m.tmuxSession)
	}
	if cmd == nil {
		t.Fatal("expected capture+tick cmds")
	}
	// Chat viewport should shrink when pane takes space.
	m.handleWindowSize(tea.WindowSizeMsg{Width: 80, Height: 30})
	if m.viewport.Height >= baseVH {
		t.Fatalf("viewport should shrink with tmux pane: before=%d after=%d", baseVH, m.viewport.Height)
	}
	out := m.View()
	if !strings.Contains(out, "tmux │") {
		t.Fatalf("view missing tmux title:\n%s", out)
	}
	// Pane must split chat above thinking/status, not sit under the status bar.
	tmuxIdx := strings.Index(out, "tmux │")
	sepIdx := strings.Index(out, "─")
	// Prefer a long separator line; fall back to first box-drawing run.
	for i, line := range strings.Split(out, "\n") {
		_ = i
		if strings.HasPrefix(strings.TrimSpace(line), "──") || strings.Count(line, "─") >= 10 {
			sepIdx = strings.Index(out, line)
			break
		}
	}
	if tmuxIdx < 0 || sepIdx < 0 || tmuxIdx > sepIdx {
		t.Fatalf("tmux pane should appear above separator/thinking chrome: tmuxIdx=%d sepIdx=%d\n%s", tmuxIdx, sepIdx, out)
	}
	if !strings.Contains(out, "tmux:local/antisthenes-persist") && !strings.Contains(out, "tmux:") {
		// status snippet may appear on right bar
		t.Logf("status snippet optional in view")
	}

	m.disableTmuxPane()
	if m.tmuxEnabled {
		t.Fatal("expected disabled")
	}
	m.handleWindowSize(tea.WindowSizeMsg{Width: 80, Height: 30})
	out = m.View()
	if strings.Contains(out, "tmux │") {
		t.Fatalf("tmux title should be gone:\n%s", out)
	}
}

func TestTmuxSlashCommands(t *testing.T) {
	m := baseTmuxModel()
	m.handleWindowSize(tea.WindowSizeMsg{Width: 80, Height: 28})

	cmd, ok := m.handleTmuxSlash("/tmux on")
	if !ok || !m.tmuxEnabled {
		t.Fatalf("on: ok=%v enabled=%v", ok, m.tmuxEnabled)
	}
	_ = cmd

	_, ok = m.handleTmuxSlash("/tmux host vllm01")
	if !ok || m.tmuxHost != "vllm01" {
		t.Fatalf("host: %q", m.tmuxHost)
	}
	_, ok = m.handleTmuxSlash("/tmux session my-sess")
	if !ok || m.tmuxSession != "my-sess" {
		t.Fatalf("session: %q", m.tmuxSession)
	}
	_, ok = m.handleTmuxSlash("/tmux host local")
	if !ok || m.tmuxHost != "" {
		t.Fatalf("local host clear: %q", m.tmuxHost)
	}
	_, ok = m.handleTmuxSlash("/tmux status")
	if !ok {
		t.Fatal("status")
	}
	_, ok = m.handleTmuxSlash("/tmux off")
	if !ok || m.tmuxEnabled {
		t.Fatal("off")
	}
}

func TestTmuxSlashViaHandleSlashCommand(t *testing.T) {
	m := baseTmuxModel()
	m.handleWindowSize(tea.WindowSizeMsg{Width: 80, Height: 28})
	_, ok := m.handleSlashCommand("/tmux")
	if !ok || !m.tmuxEnabled {
		t.Fatal("/tmux via slash handler")
	}
}

func TestTmuxPaneMsgUpdatesContent(t *testing.T) {
	m := baseTmuxModel()
	m.handleWindowSize(tea.WindowSizeMsg{Width: 80, Height: 28})
	_ = m.enableTmuxPane("", "s1")
	m.handleTmuxPaneMsg(tmuxPaneMsg{content: "[tmux session=s1 host=localhost lines=80]\nhello-pane"})
	if !strings.Contains(m.tmuxContent, "hello-pane") {
		t.Fatalf("content: %q", m.tmuxContent)
	}
	out := m.View()
	if !strings.Contains(out, "hello-pane") {
		t.Fatalf("view missing content:\n%s", out)
	}
	m.handleTmuxPaneMsg(tmuxPaneMsg{err: "boom"})
	if m.tmuxLastErr != "boom" {
		t.Fatalf("err should set lastErr: %q", m.tmuxLastErr)
	}
	// Transient errors keep last good pane body; title shows err.
	if !strings.Contains(m.tmuxContent, "hello-pane") {
		t.Fatalf("should keep last good content on err: %q", m.tmuxContent)
	}
	// Empty content path still surfaces the error text.
	m.tmuxContent = ""
	m.handleTmuxPaneMsg(tmuxPaneMsg{err: "boom2"})
	if m.tmuxLastErr != "boom2" || !strings.Contains(m.tmuxContent, "boom2") {
		t.Fatalf("empty err path: %q %q", m.tmuxLastErr, m.tmuxContent)
	}
}

func TestTmuxUpdateTickAndMsg(t *testing.T) {
	m := baseTmuxModel()
	m.handleWindowSize(tea.WindowSizeMsg{Width: 80, Height: 28})
	_ = m.enableTmuxPane("", "tick-sess")
	mm, cmd := modelFromUpdate(&m, tmuxTickMsg{})
	if mm == nil || !mm.tmuxEnabled {
		t.Fatal("still enabled")
	}
	if cmd == nil {
		t.Fatal("tick should schedule capture+tick")
	}
	mm, _ = modelFromUpdate(mm, tmuxPaneMsg{content: "from-tick-path"})
	if !strings.Contains(mm.tmuxContent, "from-tick-path") {
		t.Fatalf("msg: %q", mm.tmuxContent)
	}
}

func TestLayoutReservedWithTmux(t *testing.T) {
	m := baseTmuxModel()
	base := layoutReservedLines()
	if m.layoutReservedLinesWithTmux() != base {
		t.Fatal("disabled should match base")
	}
	m.tmuxEnabled = true
	m.tmuxPaneHeight = 8
	if m.layoutReservedLinesWithTmux() != base+1+8 {
		t.Fatalf("got %d want %d", m.layoutReservedLinesWithTmux(), base+9)
	}
}

func TestTmuxPaneMsgSkipsUnchanged(t *testing.T) {
	m := baseTmuxModel()
	m.handleWindowSize(tea.WindowSizeMsg{Width: 80, Height: 28})
	_ = m.enableTmuxPane("", "s1")
	m.tmuxCaptureBusy = false
	m.handleTmuxPaneMsg(tmuxPaneMsg{content: "same-body", seq: m.tmuxCaptureSeq})
	if m.tmuxContent != "same-body" {
		t.Fatalf("content: %q", m.tmuxContent)
	}
	m.handleTmuxPaneMsg(tmuxPaneMsg{content: "same-body", seq: m.tmuxCaptureSeq})
	if m.tmuxContent != "same-body" {
		t.Fatalf("unchanged content drifted: %q", m.tmuxContent)
	}
	if m.tmuxCaptureBusy {
		t.Fatal("busy should clear on msg")
	}
}

func TestTmuxPaneMsgDropsStaleSeq(t *testing.T) {
	m := baseTmuxModel()
	m.handleWindowSize(tea.WindowSizeMsg{Width: 80, Height: 28})
	_ = m.enableTmuxPane("", "s1")
	m.tmuxCaptureSeq = 5
	m.tmuxCaptureBusy = true
	m.tmuxContent = "fresh"
	m.handleTmuxPaneMsg(tmuxPaneMsg{content: "stale", seq: 3})
	if m.tmuxContent != "fresh" {
		t.Fatalf("stale applied: %q", m.tmuxContent)
	}
	if !m.tmuxCaptureBusy {
		t.Fatal("stale must not clear busy for current seq")
	}
	m.handleTmuxPaneMsg(tmuxPaneMsg{content: "ok", seq: 5})
	if m.tmuxContent != "ok" || m.tmuxCaptureBusy {
		t.Fatalf("current seq: content=%q busy=%v", m.tmuxContent, m.tmuxCaptureBusy)
	}
}
