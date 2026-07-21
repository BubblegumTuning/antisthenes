package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	openai "github.com/sashabaranov/go-openai"

	"github.com/nanami/antisthenes/config"
	"github.com/nanami/antisthenes/internal/agent"
)

func basePhase8Model() Model {
	return Model{
		ready:     true,
		width:     80,
		height:    24,
		viewport:  viewport.New(76, 10),
		textInput: textarea.New(),
		cfg:       config.Config{AgentName: "Test", MaxTokens: 160000, EditHeight: 3},
		loop:      agent.NewLoop("", "test-model", ""),
	}
}

func TestToolResultStatus(t *testing.T) {
	tests := []struct {
		content string
		want    string
	}{
		{"all good", "OK"},
		{"error: something broke", "ERROR"},
		{"bash: command denied by user", "ERROR"},
		{"list_skills: failed to load index", "ERROR"},
		{"patch read error: permission denied", "ERROR"},
	}
	for _, tt := range tests {
		if got := toolResultStatus(tt.content); got != tt.want {
			t.Errorf("toolResultStatus(%q) = %q, want %q", tt.content, got, tt.want)
		}
	}
}

func TestRenderChat_ToolErrorStatus(t *testing.T) {
	m := basePhase8Model()
	m.windows[0].Messages = []openai.ChatCompletionMessage{
		{Role: "tool", ToolCallID: "c1", Content: "error: file not found"},
		{Role: "tool", ToolCallID: "c2", Content: "ok output"},
	}
	out := m.renderChat()
	if !strings.Contains(out, "[ERROR]") {
		t.Error("expected ERROR status in render")
	}
	if !strings.Contains(out, "[OK]") {
		t.Error("expected OK status in render")
	}
}

func TestUpdate_ToolsSlash(t *testing.T) {
	ti := textarea.New()
	ti.SetValue("/tools")
	m := basePhase8Model()
	m.textInput = ti

	updated, cmd := modelFromUpdate(&m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("/tools should not spawn cmd")
	}
	last := updated.windows[0].Messages[len(updated.windows[0].Messages)-1]
	if !strings.Contains(last.Content, "Registered tools") {
		t.Errorf("unexpected tools listing: %q", last.Content)
	}
	if !strings.Contains(last.Content, "bash") {
		t.Error("expected bash in default registry listing")
	}
}

func TestPhase8_LongSessionRender(t *testing.T) {
	m := basePhase8Model()
	msgs := make([]openai.ChatCompletionMessage, 0, 120)
	for i := 0; i < 60; i++ {
		msgs = append(msgs,
			openai.ChatCompletionMessage{Role: "user", Content: strings.Repeat("u", 40)},
			openai.ChatCompletionMessage{Role: "assistant", Content: strings.Repeat("a", 40)},
		)
	}
	m.windows[0].Messages = msgs
	out := m.renderChat()
	if len(out) < 1000 {
		t.Errorf("long session render too short: %d bytes", len(out))
	}
	if !strings.Contains(out, "You:") || !strings.Contains(out, "Test:") {
		t.Error("long session missing expected prefixes")
	}
}

func TestPhase8_ResizeSequenceStable(t *testing.T) {
	m := basePhase8Model()
	m.ready = false
	sizes := []tea.WindowSizeMsg{
		{Width: 120, Height: 40},
		{Width: 80, Height: 24},
		{Width: 40, Height: 16},
		{Width: 120, Height: 40},
	}
	mp := &m
	for _, sz := range sizes {
		mp.handleWindowSize(sz)
		if mp.width != sz.Width || mp.height != sz.Height {
			t.Fatalf("size not applied: %+v got %dx%d", sz, mp.width, mp.height)
		}
		if mp.viewport.Height < 1 {
			t.Fatalf("viewport invalid after %dx%d: %d", sz.Width, sz.Height, mp.viewport.Height)
		}
		if mp.textInput.Height() < 1 {
			t.Fatal("textarea height invalid after resize")
		}
		out := mp.View()
		if out == "" || strings.Contains(out, "Initializing") {
			t.Fatal("view should be ready after resize")
		}
		if got := viewDisplayLines(out); got != mp.height {
			t.Fatalf("view display lines=%d want %d after %dx%d", got, mp.height, sz.Width, sz.Height)
		}
	}
}

func TestPhase8_ThinkingTogglePreservesViewportHeight(t *testing.T) {
	m := basePhase8Model()
	mp := &m
	mp.handleWindowSize(tea.WindowSizeMsg{Width: 80, Height: 24})
	vh := mp.viewport.Height
	mp.thinking = true
	mp.spinnerFrame = 0
	if mp.viewport.Height != vh {
		t.Errorf("thinking toggle changed viewport height: %d -> %d", vh, mp.viewport.Height)
	}
	_ = mp.View() // render must not panic with thinking active
}

func TestPhase8_CronNotificationInView(t *testing.T) {
	m := basePhase8Model()
	m.windows[0].LastNotification = "Cron: summary written to dump-test.md"
	out := m.View()
	if !strings.Contains(out, "Cron:") {
		t.Error("cron notification missing from status bar")
	}
}

func TestPhase8_IterativeStatusInView(t *testing.T) {
	m := basePhase8Model()
	m.windows[0].iterState = IterPlanning
	out := m.View()
	if !strings.Contains(out, "iterative") && !strings.Contains(out, "planning") {
		t.Error("iterative status not shown in view")
	}
}

func TestPhase8_DumpSummaryFlagPreserved(t *testing.T) {
	m := basePhase8Model()
	m.pendingDumpSummary = true
	m.pendingDumpWindow = 0
	// Flag should survive unrelated key handling until response completes.
	_, _ = (&m).handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if !m.pendingDumpSummary {
		t.Error("pendingDumpSummary cleared unexpectedly")
	}
}
