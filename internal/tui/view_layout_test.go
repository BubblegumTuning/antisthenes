package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/nanami/antisthenes/config"
	openai "github.com/sashabaranov/go-openai"
)

func locateTokenBarLines(out string) []int {
	var idx []int
	for i, line := range strings.Split(out, "\n") {
		if strings.Contains(stripANSI(line), "tokens |") {
			idx = append(idx, i)
		}
	}
	return idx
}

func TestView_ExactTerminalHeight_AllStates(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*Model)
	}{
		{"baseline", func(m *Model) {}},
		{"slash", func(m *Model) { m.textInput.SetValue("/") }},
		{"thinking", func(m *Model) { m.thinking = true }},
		{"error", func(m *Model) { m.lastError = "boom" }},
		{"iterative", func(m *Model) { m.iterState = IterPlanning }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := Model{
				textInput: textarea.New(),
				cfg:       config.Config{AgentName: "Test", MaxTokens: 160000, EditHeight: 3},
			}
			mp := &m
			mp.handleWindowSize(tea.WindowSizeMsg{Width: 80, Height: 24})
			mp.windows[0].Messages = []openai.ChatCompletionMessage{
				{Role: "user", Content: "hello"},
			}
			mp.syncViewport()
			tc.setup(mp)

			out := mp.View()
			if got := viewDisplayLines(out); got != m.height {
				t.Fatalf("display lines=%d want %d", got, m.height)
			}
			if bars := locateTokenBarLines(out); len(bars) != 1 {
				t.Fatalf("token bar count=%d want 1 at %v", len(bars), bars)
			}
		})
	}
}

func TestRenderStatusRowSlot_FixedThreeLines(t *testing.T) {
	m := Model{width: 80, cfg: config.Config{AgentName: "Test"}}
	p := m.palette()

	for name, setup := range map[string]func(){
		"empty":    func() {},
		"thinking": func() { m.thinking = true },
		"error":    func() { m.lastError = "boom" },
	} {
		t.Run(name, func(t *testing.T) {
			setup()
			slot := (&m).renderStatusRowSlot(p)
			if got := viewDisplayLines(slot); got != statusRowLines {
				t.Fatalf("status row lines=%d want %d", got, statusRowLines)
			}
		})
	}
}

func TestRenderStatusRowSlot_SlashHintFixedHeight(t *testing.T) {
	m := Model{width: 80, cfg: config.Config{AgentName: "Test"}}
	m.textInput = textarea.New()
	m.textInput.SetValue("/")
	p := m.palette()
	slot := (&m).renderStatusRowSlot(p)
	if got := viewDisplayLines(slot); got != statusRowLines {
		t.Fatalf("slash hint status row lines=%d want %d", got, statusRowLines)
	}
	if !strings.Contains(slot, "/") && !strings.Contains(slot, "Slash:") {
		t.Fatalf("slash hint missing from status row: %q", slot)
	}
}

func TestView_TinyTerminalExactHeight(t *testing.T) {
	m := Model{textInput: textarea.New(), cfg: config.Config{EditHeight: 5}}
	mp := &m
	mp.handleWindowSize(tea.WindowSizeMsg{Width: 30, Height: 14})
	static := mp.measureStaticChromeLines()
	out := mp.View()
	got := viewDisplayLines(out)
	if got != mp.height {
		t.Fatalf("static=%d vph=%d view lines=%d want %d", static, mp.viewport.Height, got, mp.height)
	}
	if strings.Count(mp.View(), "tokens |") != 1 {
		t.Fatal("expected single status bar on tiny terminal")
	}
}
