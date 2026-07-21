package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/nanami/antisthenes/config"
	openai "github.com/sashabaranov/go-openai"
)

func TestOrderSel(t *testing.T) {
	a, b := orderSel(selPos{2, 5}, selPos{1, 9})
	if a.line != 1 || b.line != 2 {
		t.Fatalf("order failed: %+v %+v", a, b)
	}
	a, b = orderSel(selPos{3, 8}, selPos{3, 2})
	if a.col != 2 || b.col != 8 {
		t.Fatalf("same-line order failed: %+v %+v", a, b)
	}
}

func TestExtractSelectionText_MultiLine(t *testing.T) {
	content := "hello world\nsecond line\nthird"
	// Select "lo wor" from first line through "sec" of second → cell-based
	got := extractSelectionText(content, selPos{0, 3}, selPos{1, 3})
	want := "lo world\nsec"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestExtractSelectionText_Empty(t *testing.T) {
	if extractSelectionText("abc", selPos{0, 1}, selPos{0, 1}) != "" {
		t.Fatal("zero-width should be empty")
	}
}

func TestApplySelectionHighlight(t *testing.T) {
	content := "abcdef"
	out := applySelectionHighlight(content, selPos{0, 1}, selPos{0, 4})
	plain := ansi.Strip(out)
	if plain != "abcdef" {
		t.Fatalf("highlight should preserve plain text, got %q", plain)
	}
	// Reverse styling may be a no-op under ascii color profile in tests;
	// ensure the helper at least returns stably for the selected range.
	if out == "" {
		t.Fatal("unexpected empty highlight output")
	}
}

func TestMouseToChatPos(t *testing.T) {
	m := Model{
		width:        80,
		height:       40,
		mouseEnabled: true,
		viewport:     viewport.New(80, 10),
		cfg:          config.Config{AgentName: "Bot"},
	}
	m.windows[0].Messages = []openai.ChatCompletionMessage{
		{Role: "user", Content: "line0"},
		{Role: "assistant", Content: "line1"},
	}
	m.viewport.SetContent(m.renderChat())
	m.viewport.YOffset = 0

	// Terminal row 2 is first viewport row (title+window bar).
	pos, ok := m.mouseToChatPos(tea.MouseMsg{X: 0, Y: 2, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if !ok || pos.line != 0 {
		t.Fatalf("expected line 0 at Y=2, got ok=%v pos=%+v", ok, pos)
	}
	_, ok = m.mouseToChatPos(tea.MouseMsg{X: 0, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if ok {
		t.Fatal("title row should be outside chat")
	}
}

func TestHandleMouseSlash(t *testing.T) {
	m := Model{textInput: textarea.New(), mouseEnabled: true}
	cmd, handled := (&m).handleMouseSlash("/mouse off")
	if !handled || m.mouseEnabled {
		t.Fatalf("off failed handled=%v enabled=%v", handled, m.mouseEnabled)
	}
	if cmd == nil {
		t.Fatal("expected DisableMouse cmd")
	}
	cmd, handled = (&m).handleMouseSlash("/mouse on")
	if !handled || !m.mouseEnabled {
		t.Fatalf("on failed handled=%v enabled=%v", handled, m.mouseEnabled)
	}
	if cmd == nil {
		t.Fatal("expected EnableMouseCellMotion cmd")
	}
	_, handled = (&m).handleMouseSlash("/mouse")
	if !handled || !strings.Contains(m.windows[0].LastNotification, "Mouse mode:") {
		t.Fatalf("status: %q", m.windows[0].LastNotification)
	}
}

func TestHandleMouseMsg_DragCopy(t *testing.T) {
	m := Model{
		width:        80,
		height:       40,
		mouseEnabled: true,
		viewport:     viewport.New(80, 12),
		textInput:    textarea.New(),
		cfg:          config.Config{AgentName: "Bot"},
	}
	m.windows[0].Messages = []openai.ChatCompletionMessage{
		{Role: "user", Content: "ABCDEFGH"},
	}
	m.refreshViewportContent()

	// Drag across first content line in viewport (Y=2).
	_, handled := (&m).handleMouseMsg(tea.MouseMsg{
		X: 0, Y: 2, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	})
	if !handled || !m.selSelecting {
		t.Fatalf("press: handled=%v selecting=%v", handled, m.selSelecting)
	}
	_, handled = (&m).handleMouseMsg(tea.MouseMsg{
		X: 4, Y: 2, Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft,
	})
	if !handled {
		t.Fatal("motion not handled")
	}
	_, handled = (&m).handleMouseMsg(tea.MouseMsg{
		X: 4, Y: 2, Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft,
	})
	if !handled {
		t.Fatal("release not handled")
	}
	if m.selSelecting || m.selHasRange {
		t.Fatal("selection should clear after copy")
	}
	note := m.windows[0].LastNotification
	if !strings.Contains(note, "Copied selection") && !strings.Contains(note, "Saved selection") {
		t.Fatalf("unexpected notification: %q", note)
	}
}

func TestHandleMouseMsg_Disabled(t *testing.T) {
	m := Model{mouseEnabled: false, viewport: viewport.New(80, 10)}
	_, handled := (&m).handleMouseMsg(tea.MouseMsg{
		X: 0, Y: 2, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	})
	if handled {
		t.Fatal("disabled mouse should not handle events")
	}
}
