package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/nanami/antisthenes/config"
	openai "github.com/sashabaranov/go-openai"
)

func TestNewTextInput_EnterDoesNotInsertNewline(t *testing.T) {
	ta := newTextInput()
	if key.Matches(tea.KeyMsg{Type: tea.KeyEnter}, ta.KeyMap.InsertNewline) {
		t.Fatal("plain Enter must not map to InsertNewline")
	}
	if !key.Matches(tea.KeyMsg{Type: tea.KeyEnter, Alt: true}, ta.KeyMap.InsertNewline) {
		t.Fatal("Alt+Enter should map to InsertNewline")
	}
}

func TestHandleSubmitKey_ClearsAfterSendAndResponse(t *testing.T) {
	ti := textarea.New()
	ti.SetValue("hello there")
	m := Model{
		textInput: ti,
		ready:     true,
		width:     80,
		height:    24,
		viewport:  viewport.New(80, 8),
		cfg:       config.Config{AgentName: "Test"},
	}
	mp := &m
	mp.handleWindowSize(tea.WindowSizeMsg{Width: 80, Height: 24})

	updated, cmd := modelFromUpdate(mp, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected agent cmd")
	}
	if updated.textInput.Value() != "" {
		t.Fatalf("input not cleared after send: %q", updated.textInput.Value())
	}
	if updated.textInput.LineCount() != 1 {
		t.Fatalf("expected single empty line, got %d lines", updated.textInput.LineCount())
	}

	resp := responseMsg{
		windowIndex: 0,
		messages: []openai.ChatCompletionMessage{
			{Role: "user", Content: "hello there"},
			{Role: "assistant", Content: "hi"},
		},
	}
	final, _ := modelFromUpdate(updated, resp)
	if final.textInput.Value() != "" {
		t.Fatalf("input not cleared after response: %q", final.textInput.Value())
	}
	out := final.View()
	if strings.Count(out, "hello there") > 1 {
		t.Fatalf("submitted text should not remain in input view:\n%s", out)
	}
}

func TestRenderInputBox_BlankWhileThinking(t *testing.T) {
	m := Model{
		textInput:      newTextInput(),
		thinking:       true,
		thinkingWindow: 0,
		activeWindow:   0,
		width:          80,
		cfg:            config.Config{EditHeight: 3},
	}
	m.textInput.SetWidth(inputBoxWidth(80))
	m.textInput.SetHeight(3)
	m.textInput.SetValue("first message must not show")

	box := m.renderInputBox(m.palette())
	if strings.Contains(box, "first message") {
		t.Fatalf("thinking input box must not render submitted text:\n%s", box)
	}
}

func TestRenderInputBox_ConsistentHeightAfterThinking(t *testing.T) {
	m := Model{
		textInput:      newTextInput(),
		thinking:       true,
		thinkingWindow: 0,
		activeWindow:   0,
		width:          80,
		cfg:            config.Config{EditHeight: 3},
	}
	m.textInput.SetWidth(inputBoxWidth(80))
	m.textInput.SetHeight(3)
	m.textInput.SetValue("sent text")

	thinkingBox := m.renderInputBox(m.palette())
	m.thinking = false
	m.clearTextInput()
	idleBox := m.renderInputBox(m.palette())

	thinkingLines := viewDisplayLines(thinkingBox)
	idleLines := viewDisplayLines(idleBox)
	if thinkingLines != idleLines {
		t.Fatalf("input box height drift thinking=%d idle=%d\nthinking:\n%s\nidle:\n%s",
			thinkingLines, idleLines, thinkingBox, idleBox)
	}
	if strings.Count(idleBox, "╭") != 1 {
		t.Fatalf("expected one input top border, got %d:\n%s", strings.Count(idleBox, "╭"), idleBox)
	}
}

func TestView_InputBoxNoBorderStackingAfterCycles(t *testing.T) {
	m := Model{
		textInput: newTextInput(),
		cfg:       config.Config{AgentName: "Test", MaxTokens: 160000, EditHeight: 3},
	}
	mp := &m
	mp.handleWindowSize(tea.WindowSizeMsg{Width: 80, Height: 24})

	for i := 0; i < 3; i++ {
		mp.textInput.SetValue("cycle message")
		mp.submitUserMessage("cycle message")
		resp := responseMsg{
			windowIndex: 0,
			messages: []openai.ChatCompletionMessage{
				{Role: "user", Content: "cycle message"},
				{Role: "assistant", Content: "ok"},
			},
		}
		mp.handleResponseMsg(resp)
	}

	out := mp.View()
	if strings.Count(out, "╭") != 1 {
		t.Fatalf("stacked input borders after cycles: %d top borders in view\n%s", strings.Count(out, "╭"), out)
	}
}

func TestView_SubmittedTextOnlyInChatWhileThinking(t *testing.T) {
	m := Model{
		textInput: newTextInput(),
		cfg:       config.Config{AgentName: "Test", MaxTokens: 160000, EditHeight: 3},
	}
	mp := &m
	mp.handleWindowSize(tea.WindowSizeMsg{Width: 80, Height: 24})
	mp.textInput.SetValue("unique_send_marker")
	mp.submitUserMessage("unique_send_marker")

	out := mp.View()
	if strings.Count(out, "unique_send_marker") != 1 {
		t.Fatalf("submitted text should appear once in chat only, got %d:\n%s", strings.Count(out, "unique_send_marker"), out)
	}
}

func TestRenderInputBox_EmptyVsTypedSameSize(t *testing.T) {
	m := Model{
		textInput: newTextInput(),
		width:     80,
		height:    24,
		cfg:       config.Config{EditHeight: 3},
	}
	mp := &m
	mp.handleWindowSize(tea.WindowSizeMsg{Width: 80, Height: 24})

	emptyBox := mp.renderInputBox(mp.palette())
	mp.textInput, _ = mp.textInput.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	typedBox := mp.renderInputBox(mp.palette())

	if viewDisplayLines(emptyBox) != viewDisplayLines(typedBox) {
		t.Fatalf("input box height empty=%d typed=%d\nempty:\n%s\ntyped:\n%s",
			viewDisplayLines(emptyBox), viewDisplayLines(typedBox), emptyBox, typedBox)
	}
	if lipgloss.Width(emptyBox) != lipgloss.Width(typedBox) {
		t.Fatalf("input box width empty=%d typed=%d", lipgloss.Width(emptyBox), lipgloss.Width(typedBox))
	}
}

func TestView_InputBoxStableOnFirstKeystroke(t *testing.T) {
	m := Model{
		textInput: newTextInput(),
		cfg:       config.Config{AgentName: "Test", MaxTokens: 160000, EditHeight: 3},
	}
	mp := &m
	mp.handleWindowSize(tea.WindowSizeMsg{Width: 80, Height: 24})

	before := viewDisplayLines(mp.View())
	updated, _ := modelFromUpdate(mp, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	after := viewDisplayLines(updated.View())
	if before != after {
		t.Fatalf("view height changed on first keystroke: %d -> %d", before, after)
	}
}

func TestRenderInputBox_NoTextareaPromptBar(t *testing.T) {
	m := Model{
		textInput: newTextInput(),
		width:     80,
		cfg:       config.Config{EditHeight: 3},
	}
	m.textInput.SetWidth(inputBoxWidth(80))
	m.textInput.SetHeight(3)
	m.textInput.SetValue("hello")

	box := m.renderInputBox(m.palette())
	if strings.Contains(box, "┃") {
		t.Fatalf("input box must not render textarea prompt bar:\n%s", box)
	}
	inner := stripANSI(strings.Split(box, "\n")[1])
	if strings.HasPrefix(strings.TrimSpace(inner), "1 ") {
		t.Fatalf("input box must not render textarea line numbers:\n%s", box)
	}
}

func TestEnterFallthrough_DoesNotSplitLine(t *testing.T) {
	ti := newTextInput()
	ti.SetValue("keep")
	updated, _ := ti.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if updated.LineCount() != 1 {
		t.Fatalf("Enter fallthrough created extra lines: %d", updated.LineCount())
	}
	if updated.Value() != "keep" {
		t.Fatalf("unexpected value change: %q", updated.Value())
	}
}
