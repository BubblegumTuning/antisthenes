package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/nanami/antisthenes/config"
	openai "github.com/sashabaranov/go-openai"
)

func TestUpdate_KeyCtrlC(t *testing.T) {
	m := Model{textInput: textarea.New(), ready: true, width: 80}
	updated, cmd := modelFromUpdate(&m, tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("expected Quit cmd")
	}
	_ = updated
}

func TestUpdate_KeyEnter_Help(t *testing.T) {
	ti := textarea.New()
	ti.SetValue("/help")
	m := Model{
		textInput: ti,
		ready:     true,
		width:     80,
		viewport:  viewport.New(80, 20),
		cfg:       config.Config{AgentName: "Test"},
	}
	updated, cmd := modelFromUpdate(&m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("help should not return cmd")
	}
	if len(updated.windows[0].Messages) == 0 || !strings.Contains(updated.windows[0].Messages[len(updated.windows[0].Messages)-1].Content, "/clear") {
		t.Error("help message not appended")
	}
}

func TestUpdate_KeyEnter_Compress(t *testing.T) {
	ti := textarea.New()
	ti.SetValue("/compress")
	m := Model{
		textInput: ti,
		ready:     true,
		width:     80,
		viewport:  viewport.New(80, 20),
		cfg:       config.Config{AgentName: "Test"},
	}
	m.windows[0].Messages = []openai.ChatCompletionMessage{
		{Role: "user", Content: "first"},
		{Role: "tool", Content: "toolres"},
		{Role: "assistant", Content: "a1"},
		{Role: "user", Content: "u2"},
	}
	updated, _ := modelFromUpdate(&m, tea.KeyMsg{Type: tea.KeyEnter})
	if len(updated.windows[0].Messages) < 2 {
		t.Error("compress did not keep messages")
	}
	foundStub := false
	for _, msg := range updated.windows[0].Messages {
		if msg.Role == "tool" && strings.Contains(msg.Content, "stubbed") {
			foundStub = true
		}
	}
	if !foundStub {
		t.Error("tool messages not stubbed")
	}
}

func TestUpdate_KeyEnter_ClearWithoutConfirm(t *testing.T) {
	ti := textarea.New()
	ti.SetValue("/clear")
	m := Model{
		textInput: ti,
		cfg:       config.Config{ClearWithoutConfirm: true},
		ready:     true,
		width:     80,
		viewport:  viewport.New(80, 20),
	}
	updated, _ := modelFromUpdate(&m, tea.KeyMsg{Type: tea.KeyEnter})
	if len(updated.windows[0].Messages) != 0 {
		t.Error("clear should skip modal when clear_without_confirm is true")
	}
}

func TestUpdate_KeyEnter_NewAliasClearsContext(t *testing.T) {
	ti := textarea.New()
	ti.SetValue("/new")
	m := Model{
		textInput: ti,
		cfg:       config.Config{ClearWithoutConfirm: true},
		ready:     true,
		width:     80,
		viewport:  viewport.New(80, 20),
	}
	m.windows[0].Messages = []openai.ChatCompletionMessage{{Role: "user", Content: "hello"}}
	updated, _ := modelFromUpdate(&m, tea.KeyMsg{Type: tea.KeyEnter})
	if len(updated.windows[0].Messages) != 0 {
		t.Error("/new should clear messages like /clear")
	}
}

func TestUpdate_WindowSize(t *testing.T) {
	m := Model{
		textInput: textarea.New(),
		ready:     false,
		width:     0,
		height:    0,
	}
	updated, _ := modelFromUpdate(&m, tea.WindowSizeMsg{Width: 100, Height: 50})
	if updated.width != 100 || updated.height != 50 || !updated.ready {
		t.Errorf("window not applied: w=%d h=%d ready=%v", updated.width, updated.height, updated.ready)
	}
}

func TestUpdate_SpinnerTickNotThinking(t *testing.T) {
	m := Model{thinking: false, spinnerFrame: 0}
	updated, cmd := modelFromUpdate(&m, spinnerTickMsg{})
	if updated.spinnerFrame != 0 {
		t.Error("should not advance spinner")
	}
	if cmd != nil {
		t.Error("no cmd when not thinking")
	}
}

func TestUpdate_ResponseMsg(t *testing.T) {
	m := Model{
		thinking:  true,
		ready:     true,
		width:     80,
		viewport:  viewport.New(80, 20),
		textInput: textarea.New(),
		cfg:       config.Config{AgentName: "T"},
	}
	msg := responseMsg{windowIndex: 0, messages: []openai.ChatCompletionMessage{{Role: "assistant", Content: "reply"}}, err: nil}
	updated, _ := modelFromUpdate(&m, msg)
	if updated.thinking {
		t.Error("should stop thinking")
	}
	if len(updated.windows[0].Messages) == 0 || updated.windows[0].Messages[0].Content != "reply" {
		t.Error("messages not updated from response")
	}
}

func TestUpdate_ResponseMsg_ClearsTextInput(t *testing.T) {
	ti := textarea.New()
	ti.SetValue("stale draft\nline two\nline three")
	m := Model{
		thinking:  true,
		ready:     true,
		width:     80,
		viewport:  viewport.New(80, 20),
		textInput: ti,
		cfg:       config.Config{AgentName: "Test"},
	}
	msg := responseMsg{
		windowIndex: 0,
		messages:    []openai.ChatCompletionMessage{{Role: "assistant", Content: "reply"}},
	}
	updated, _ := modelFromUpdate(&m, msg)
	if updated.textInput.Value() != "" {
		t.Fatalf("textInput should clear after response, got %q", updated.textInput.Value())
	}
}

// Regression: Enter used to schedule callAgent on the pre-submit model copy, so the
// user message was shown in the TUI but omitted from the LLM request. With trailing
// assistant-only history (e.g. repeated /theme), the API returns 400:
// "Cannot have 2 or more assistant messages at the end of the list".
func TestSubmitUserMessage_CallAgentMustUseUpdatedModel(t *testing.T) {
	m := Model{
		textInput: textarea.New(),
		viewport:  viewport.New(80, 20),
		ready:     true,
		width:     80,
		cfg:       config.Config{AgentName: "T"},
		windows: [maxChatWindows]ChatWindow{{
			Messages: []openai.ChatCompletionMessage{
				{Role: "assistant", Content: `Theme "amber" applied`},
				{Role: "assistant", Content: `Theme "green" applied`},
			},
		}},
	}
	input := "create a new file in /tmp"
	submitted := m
	(&submitted).submitUserMessage(input)

	if len(m.windows[0].Messages) != 2 {
		t.Fatalf("original model copy should not gain user message, got %d", len(m.windows[0].Messages))
	}
	if len(submitted.windows[0].Messages) != 3 {
		t.Fatalf("updated model should have user message, got %d", len(submitted.windows[0].Messages))
	}
	last := submitted.windows[0].Messages[len(submitted.windows[0].Messages)-1]
	if last.Role != "user" || last.Content != input {
		t.Fatalf("last message = %+v, want user %q", last, input)
	}
	// callAgentForWindow reads from the model snapshot; it must end with user, not assistant.
	if submitted.windows[0].Messages[len(submitted.windows[0].Messages)-1].Role == "assistant" {
		t.Fatal("agent snapshot would hit trailing-assistant API validation")
	}
}

func TestSubmitUserMessage_ClearsTextInput(t *testing.T) {
	ti := textarea.New()
	ti.SetValue("hello world")
	m := Model{
		textInput: ti,
		viewport:  viewport.New(80, 10),
		ready:     true,
		width:     80,
		cfg:       config.Config{AgentName: "Test"},
	}
	(&m).submitUserMessage("hello world")
	if m.textInput.Value() != "" {
		t.Fatalf("textInput should be empty after send, got %q", m.textInput.Value())
	}
}

func TestUpdate_KeyEnter_ClearsTextInput(t *testing.T) {
	ti := textarea.New()
	ti.SetValue("ping")
	m := Model{
		textInput: ti,
		ready:     true,
		width:     80,
		viewport:  viewport.New(80, 10),
		cfg:       config.Config{AgentName: "Test"},
	}
	updated, cmd := modelFromUpdate(&m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected agent cmd on enter")
	}
	if updated.textInput.Value() != "" {
		t.Fatalf("textInput not cleared after enter: %q", updated.textInput.Value())
	}
}
